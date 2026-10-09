// Package raftadapter contains the deterministic boundary around the
// instrumented etcd/raft dependency.
package raftadapter

import (
	"errors"
	"fmt"
)

const EntropyVersion = "splitmix64-v1"

// Choice is one bounded, node-scoped entropy decision. Sequence is zero-based.
type Choice struct {
	Node     uint64 `json:"node"`
	Stream   string `json:"stream"`
	Sequence uint64 `json:"sequence"`
	Bound    int    `json:"bound"`
	Value    int    `json:"value"`
}

// Entropy supplies a recorded bounded choice.
type Entropy interface {
	Draw(stream string, bound int) (int, error)
}

// SplitMix64 is a small deterministic generator for simulation decisions. It
// is not suitable for secrets, keys, or tokens.
type SplitMix64 struct{ state uint64 }

func NewSplitMix64(seed uint64) *SplitMix64 { return &SplitMix64{state: seed} }

func (s *SplitMix64) Uint64() uint64 {
	s.state += 0x9e3779b97f4a7c15
	z := s.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (s *SplitMix64) bounded(bound uint64) uint64 {
	// Rejection removes modulo bias and is deterministic for a fixed version.
	threshold := -bound % bound
	for {
		r := s.Uint64()
		if r >= threshold {
			return r % bound
		}
	}
}

// Recorder owns one node's generator and appends every successful draw.
type Recorder struct {
	node    uint64
	prng    *SplitMix64
	next    uint64
	choices []Choice
}

func NewRecorder(node, seed uint64) (*Recorder, error) {
	if node == 0 {
		return nil, errors.New("entropy node ID must be nonzero")
	}
	return &Recorder{node: node, prng: NewSplitMix64(seed)}, nil
}

func (r *Recorder) Draw(stream string, bound int) (int, error) {
	if stream == "" {
		return 0, errors.New("entropy stream must be nonempty")
	}
	if bound <= 0 {
		return 0, fmt.Errorf("entropy bound must be positive: %d", bound)
	}
	value := int(r.prng.bounded(uint64(bound)))
	r.choices = append(r.choices, Choice{
		Node: r.node, Stream: stream, Sequence: r.next, Bound: bound, Value: value,
	})
	r.next++
	return value, nil
}

func (r *Recorder) Choices() []Choice { return append([]Choice(nil), r.choices...) }

// Replayer consumes an exact recorded choice stream. Draw rejects missing,
// reordered, mismatched, or out-of-range choices; Finish rejects extras.
type Replayer struct {
	node    uint64
	choices []Choice
	next    uint64
}

func NewReplayer(node uint64, choices []Choice) (*Replayer, error) {
	if node == 0 {
		return nil, errors.New("entropy node ID must be nonzero")
	}
	return &Replayer{node: node, choices: append([]Choice(nil), choices...)}, nil
}

func (r *Replayer) Draw(stream string, bound int) (int, error) {
	if stream == "" {
		return 0, errors.New("entropy stream must be nonempty")
	}
	if bound <= 0 {
		return 0, fmt.Errorf("entropy bound must be positive: %d", bound)
	}
	if r.next >= uint64(len(r.choices)) {
		return 0, fmt.Errorf("entropy choice %d missing", r.next)
	}
	c := r.choices[r.next]
	if c.Node != r.node || c.Stream != stream || c.Sequence != r.next || c.Bound != bound {
		return 0, fmt.Errorf("entropy choice %d mismatch: got node=%d stream=%q sequence=%d bound=%d, want node=%d stream=%q sequence=%d bound=%d", r.next, c.Node, c.Stream, c.Sequence, c.Bound, r.node, stream, r.next, bound)
	}
	if c.Value < 0 || c.Value >= bound {
		return 0, fmt.Errorf("entropy choice %d value %d outside [0,%d)", r.next, c.Value, bound)
	}
	r.next++
	return c.Value, nil
}

func (r *Replayer) Finish() error {
	if r.next != uint64(len(r.choices)) {
		return fmt.Errorf("entropy has %d unconsumed choice(s)", uint64(len(r.choices))-r.next)
	}
	return nil
}

// ElectionRandom adapts fallible entropy to raft.Config.ElectionRandom. Since
// the upstream callback cannot return an error, it stores the first error for
// the adapter to check immediately after each raft call.
type ElectionRandom struct {
	entropy Entropy
	stream  string
	err     error
}

func NewElectionRandom(entropy Entropy, stream string) (*ElectionRandom, error) {
	if entropy == nil {
		return nil, errors.New("election entropy is required")
	}
	if stream == "" {
		return nil, errors.New("election entropy stream must be nonempty")
	}
	return &ElectionRandom{entropy: entropy, stream: stream}, nil
}

func (e *ElectionRandom) Func() func(int) int {
	return func(bound int) int {
		if e.err != nil {
			return 0
		}
		var value int
		value, e.err = e.entropy.Draw(e.stream, bound)
		return value
	}
}

func (e *ElectionRandom) Err() error { return e.err }
