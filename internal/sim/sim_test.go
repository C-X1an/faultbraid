package sim

import (
	"encoding/json"
	"github.com/C-X1an/faultbraid/internal/strictjson"
	"math"
	"strings"
	"testing"
	"time"
)

func TestQueueOrderingAndBounds(t *testing.T) {
	q, e := NewQueue(3)
	if e != nil {
		t.Fatal(e)
	}
	for _, event := range []Event{{At: 5, Kind: "read"}, {At: 2, Kind: "read"}, {At: 2, Kind: "sync"}} {
		if e := q.Push(event); e != nil {
			t.Fatal(e)
		}
	}
	if e := q.Push(Event{At: 6}); e == nil {
		t.Fatal("queue overflow")
	}
	for i, want := range []uint64{1, 2, 0} {
		s, ok := q.Pop()
		if !ok || s.Sequence != want {
			t.Fatalf("position%d got %+v", i, s)
		}
	}
	if e := q.Push(Event{At: 4}); e == nil {
		t.Fatal("time reversal")
	}
	if e := q.After(math.MaxUint64, "read", 0); e == nil {
		t.Fatal("time overflow")
	}
	q.next = math.MaxUint64
	if e := q.Push(Event{At: 5}); e == nil {
		t.Fatal("sequence overflow")
	}
	if _, e := NewQueue(0); e == nil {
		t.Fatal("invalid capacity")
	}
}
func TestCrashDurability(t *testing.T) {
	var d Disk
	d.Write(7)
	d.Crash()
	if d.Read() != 0 {
		t.Fatal("unflushed write survived")
	}
	d.Write(9)
	d.Sync()
	d.Crash()
	if d.Read() != 9 {
		t.Fatal("flushed write lost")
	}
}
func TestReplayReexecutes(t *testing.T) {
	events := []Event{{1, "write", 7}, {2, "crash", 0}, {3, "read", 0}, {4, "write", 9}, {5, "sync", 0}, {6, "crash", 0}, {7, "read", 0}}
	trace, e := Run(events)
	if e != nil {
		t.Fatal(e)
	}
	if e := Replay(trace); e != nil {
		t.Fatal(e)
	}
	if *trace.Steps[2].Observed != 0 || *trace.Steps[6].Observed != 9 {
		t.Fatal("wrong disk observations")
	}
	for i := 0; i < 10; i++ {
		r, e := Run(events)
		if e != nil || r.Digest != trace.Digest {
			t.Fatal("nondeterministic model")
		}
	}
	original := trace.Steps[0].Volatile
	trace.Steps[0].Volatile = 999
	if e := Replay(trace); e == nil {
		t.Fatal("tampered output accepted")
	}
	trace.Steps[0].Volatile = original
	trace.Events[0].Value = 8
	if e := Replay(trace); e == nil {
		t.Fatal("tampered event accepted")
	}
	trace.Events[0].Value = 7
	trace.Digest = strings.Repeat("0", 64)
	if e := Replay(trace); e == nil {
		t.Fatal("tampered digest accepted")
	}
}
func TestRNGGolden(t *testing.T) {
	r := RNG{}
	for _, want := range []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f} {
		if got := r.Next(); got != want {
			t.Fatalf("SplitMix64-v1: got%x want%x", got, want)
		}
	}
	if _, e := r.Bounded(0); e == nil {
		t.Fatal("zero bound")
	}
	for i := 0; i < 1000; i++ {
		n, e := r.Bounded(7)
		if e != nil || n >= 7 {
			t.Fatal("out of bound entropy")
		}
	}
}
func FuzzReplay(f *testing.F) {
	tr, _ := Run([]Event{{At: 1, Kind: "read"}})
	b, _ := json.Marshal(tr)
	f.Add(b)
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		var tr Trace
		if e := strictjson.Decode(strings.NewReader(string(b)), &tr); e != nil {
			return
		}
		_ = Replay(tr)
	})
}

func TestModel10000Target(t *testing.T) {
	events := make([]Event, 10000)
	for i := range events {
		events[i] = Event{At: uint64(i), Kind: "write", Value: int64(i)}
	}
	start := time.Now()
	tr, e := Run(events)
	elapsed := time.Since(start)
	if e != nil || len(tr.Steps) != 10000 || tr.Steps[9999].Volatile != 9999 || elapsed > 2*time.Second {
		t.Fatalf("AC-023 model target elapsed=%s error=%v", elapsed, e)
	}
	t.Logf("AC-023 model10000: duration=%s; no Raft/process throughput claim", elapsed)
}
