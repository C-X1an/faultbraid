package raftadapter

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitMix64Golden(t *testing.T) {
	want := []uint64{
		0xe220a8397b1dcdaf,
		0x6e789e6aa1b965f4,
		0x06c45d188009454f,
		0xf88bb8a8724c81ec,
	}
	r := NewSplitMix64(0)
	for i, w := range want {
		if got := r.Uint64(); got != w {
			t.Fatalf("vector %d: got %016x want %016x", i, got, w)
		}
	}
}

func TestRecorderReplayAndRange(t *testing.T) {
	recorder, err := NewRecorder(2, 99)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	for _, bound := range []int{1, 2, 10, 97} {
		v, err := recorder.Draw("election", bound)
		if err != nil {
			t.Fatal(err)
		}
		if v < 0 || v >= bound {
			t.Fatalf("value %d outside bound %d", v, bound)
		}
		values = append(values, v)
	}
	replay, err := NewReplayer(2, recorder.Choices())
	if err != nil {
		t.Fatal(err)
	}
	var replayed []int
	for _, bound := range []int{1, 2, 10, 97} {
		v, err := replay.Draw("election", bound)
		if err != nil {
			t.Fatal(err)
		}
		replayed = append(replayed, v)
	}
	if !reflect.DeepEqual(values, replayed) {
		t.Fatalf("replayed %v want %v", replayed, values)
	}
	if err := replay.Finish(); err != nil {
		t.Fatal(err)
	}
}

func TestReplayerRejectsChoiceDrift(t *testing.T) {
	base := []Choice{{Node: 1, Stream: "election", Sequence: 0, Bound: 10, Value: 4}}
	tests := []struct {
		name    string
		choices []Choice
		node    uint64
		stream  string
		bound   int
		match   string
	}{
		{name: "missing", node: 1, stream: "election", bound: 10, match: "missing"},
		{name: "node", choices: base, node: 2, stream: "election", bound: 10, match: "mismatch"},
		{name: "stream", choices: base, node: 1, stream: "other", bound: 10, match: "mismatch"},
		{name: "bound", choices: base, node: 1, stream: "election", bound: 9, match: "mismatch"},
		{name: "sequence", choices: []Choice{{Node: 1, Stream: "election", Sequence: 1, Bound: 10, Value: 4}}, node: 1, stream: "election", bound: 10, match: "mismatch"},
		{name: "negative", choices: []Choice{{Node: 1, Stream: "election", Sequence: 0, Bound: 10, Value: -1}}, node: 1, stream: "election", bound: 10, match: "outside"},
		{name: "too-large", choices: []Choice{{Node: 1, Stream: "election", Sequence: 0, Bound: 10, Value: 10}}, node: 1, stream: "election", bound: 10, match: "outside"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewReplayer(tt.node, tt.choices)
			if err != nil {
				t.Fatal(err)
			}
			_, err = r.Draw(tt.stream, tt.bound)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("got %v, want error containing %q", err, tt.match)
			}
		})
	}
}

func TestReplayerRejectsExtraAndReorderedChoices(t *testing.T) {
	choices := []Choice{
		{Node: 1, Stream: "election", Sequence: 1, Bound: 10, Value: 2},
		{Node: 1, Stream: "election", Sequence: 0, Bound: 10, Value: 3},
	}
	r, _ := NewReplayer(1, choices)
	if _, err := r.Draw("election", 10); err == nil {
		t.Fatal("reordered choices accepted")
	}
	r, _ = NewReplayer(1, []Choice{{Node: 1, Stream: "election", Sequence: 0, Bound: 10, Value: 2}})
	if err := r.Finish(); err == nil {
		t.Fatal("extra choice accepted")
	}
}

func TestElectionRandomPreservesReplayError(t *testing.T) {
	r, _ := NewReplayer(1, nil)
	e, err := NewElectionRandom(r, "election")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Func()(10); got != 0 {
		t.Fatalf("fallback got %d want 0", got)
	}
	if e.Err() == nil {
		t.Fatal("missing replay error was not preserved")
	}
	if got := e.Func()(10); got != 0 {
		t.Fatalf("sticky fallback got %d want 0", got)
	}
}
