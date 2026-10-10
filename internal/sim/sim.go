// Package sim contains deterministic model primitives, not a physical disk.
package sim

import (
	"container/heap"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
)

const MaxEvents = 100000

var ErrUnsupportedVersion = errors.New("unsupported trace version or model")

type Event struct {
	At    uint64 `json:"at"`
	Kind  string `json:"kind"`
	Value int64  `json:"value"`
}
type Scheduled struct {
	Event
	Sequence uint64 `json:"sequence"`
}
type eventHeap []Scheduled

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	if h[i].At == h[j].At {
		return h[i].Sequence < h[j].Sequence
	}
	return h[i].At < h[j].At
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *eventHeap) Push(x any)   { *h = append(*h, x.(Scheduled)) }
func (h *eventHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = Scheduled{}
	*h = old[:n-1]
	return x
}

type Queue struct {
	events    eventHeap
	now, next uint64
	limit     int
}

func NewQueue(limit int) (*Queue, error) {
	if limit < 1 || limit > MaxEvents {
		return nil, errors.New("invalid queue limit")
	}
	return &Queue{limit: limit}, nil
}
func (q *Queue) Len() int { return len(q.events) }
func (q *Queue) Push(e Event) error {
	if q.limit < 1 || len(q.events) >= q.limit {
		return errors.New("queue limit exceeded")
	}
	if e.At < q.now {
		return errors.New("virtual time reversal")
	}
	if q.next == math.MaxUint64 {
		return errors.New("sequence overflow")
	}
	heap.Push(&q.events, Scheduled{e, q.next})
	q.next++
	return nil
}
func (q *Queue) After(delay uint64, kind string, value int64) error {
	if delay > math.MaxUint64-q.now {
		return errors.New("virtual time overflow")
	}
	return q.Push(Event{q.now + delay, kind, value})
}
func (q *Queue) Pop() (Scheduled, bool) {
	if len(q.events) == 0 {
		return Scheduled{}, false
	}
	e := heap.Pop(&q.events).(Scheduled)
	q.now = e.At
	return e, true
}

type Disk struct {
	Volatile int64
	Durable  int64
}

func (d *Disk) Write(v int64) { d.Volatile = v }
func (d *Disk) Sync()         { d.Durable = d.Volatile }
func (d *Disk) Crash()        { d.Volatile = d.Durable }
func (d *Disk) Read() int64   { return d.Volatile }

type Step struct {
	At       uint64 `json:"at"`
	Sequence uint64 `json:"sequence"`
	Kind     string `json:"kind"`
	Volatile int64  `json:"volatile"`
	Durable  int64  `json:"durable"`
	Observed *int64 `json:"observed,omitempty"`
}
type Trace struct {
	Version int     `json:"version"`
	Model   string  `json:"model"`
	Events  []Event `json:"events"`
	Steps   []Step  `json:"steps"`
	Digest  string  `json:"digest"`
}

func Run(events []Event) (Trace, error) {
	t := Trace{Version: 1, Model: "disk-register-v1", Events: append([]Event{}, events...), Steps: []Step{}}
	if len(events) > MaxEvents {
		return t, errors.New("event limit exceeded")
	}
	q, _ := NewQueue(MaxEvents)
	for _, e := range events {
		if e.Kind != "write" && e.Kind != "read" && e.Kind != "sync" && e.Kind != "crash" {
			return t, errors.New("unknown model event")
		}
		if e.Kind != "write" && e.Value != 0 {
			return t, errors.New("non-write event has value")
		}
		if err := q.Push(e); err != nil {
			return t, err
		}
	}
	var d Disk
	for q.Len() > 0 {
		e, _ := q.Pop()
		s := Step{At: e.At, Sequence: e.Sequence, Kind: e.Kind}
		switch e.Kind {
		case "write":
			d.Write(e.Value)
		case "sync":
			d.Sync()
		case "crash":
			d.Crash()
		case "read":
			v := d.Read()
			s.Observed = &v
		}
		s.Volatile = d.Volatile
		s.Durable = d.Durable
		t.Steps = append(t.Steps, s)
	}
	b, err := json.Marshal(t)
	if err != nil {
		return t, err
	}
	sum := sha256.Sum256(b)
	t.Digest = hex.EncodeToString(sum[:])
	return t, nil
}

// Replay executes the model again and compares the entire canonical envelope.
func Replay(t Trace) error {
	if t.Version != 1 || t.Model != "disk-register-v1" {
		return ErrUnsupportedVersion
	}
	if len(t.Steps) > MaxEvents {
		return errors.New("step limit exceeded")
	}
	want, err := Run(t.Events)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(want, t) {
		return errors.New("replay mismatch")
	}
	return nil
}

// RNG is the specified SplitMix64-v1 stream; not cryptographic randomness.
type RNG struct{ State uint64 }

func (r *RNG) Next() uint64 {
	r.State += 0x9e3779b97f4a7c15
	z := r.State
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Bounded uses rejection to avoid modulo bias. Zero is invalid.
func (r *RNG) Bounded(n uint64) (uint64, error) {
	if n == 0 {
		return 0, errors.New("zero entropy bound")
	}
	threshold := -n % n
	for {
		v := r.Next()
		if v >= threshold {
			return v % n, nil
		}
	}
}
