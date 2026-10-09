package history

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func end(n int64) *int64 { return &n }
func check(t *testing.T, h History) Result {
	t.Helper()
	r, e := Check(context.Background(), h, Options{100000})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestSequentialViolation(t *testing.T) {
	h := History{1, 0, []Operation{{"w", "write", 7, 1, end(2)}, {"r", "read", 0, 3, end(4)}}}
	if r := check(t, h); r.Status != Invalid {
		t.Fatalf("stale read accepted: %+v", r)
	}
}
func TestOverlap(t *testing.T) {
	h := History{1, 0, []Operation{{"w", "write", 7, 1, end(4)}, {"r", "read", 0, 2, end(3)}}}
	r := check(t, h)
	if r.Status != Valid || len(r.Witness) != 2 || r.Witness[0] != "r" {
		t.Fatalf("wrong witness: %+v", r)
	}
}
func TestPendingWrite(t *testing.T) {
	for _, v := range []int64{0, 7} {
		h := History{1, 0, []Operation{{"w", "write", 7, 1, nil}, {"r", "read", v, 2, end(3)}}}
		if r := check(t, h); r.Status != Valid {
			t.Fatalf("pending write inclusion/omission v=%d: %+v", v, r)
		}
	}
	// A write invoked after the read returned cannot explain that read.
	h := History{1, 0, []Operation{{"r", "read", 7, 1, end(2)}, {"w", "write", 7, 3, nil}}}
	if r := check(t, h); r.Status != Invalid {
		t.Fatalf("future write explains old read: %+v", r)
	}
}
func TestBudget(t *testing.T) {
	h := History{1, 0, []Operation{{"r", "read", 0, 1, end(2)}}}
	r, e := Check(context.Background(), h, Options{1})
	if e != nil || r.Status != Inconclusive || r.States != 1 {
		t.Fatalf("budget is not unknown: %+v %v", r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = Check(ctx, h, Options{100})
	if e != nil || r.Status != Inconclusive || r.States != 0 {
		t.Fatalf("cancel is not unknown: %+v %v", r, e)
	}
	for _, n := range []int{0, -1, MaxStatesLimit + 1} {
		if _, e := Check(context.Background(), h, Options{n}); e == nil {
			t.Fatalf("accepted budget %d", n)
		}
	}
}
func TestReadTimeTiesAndLimits(t *testing.T) {
	h := History{1, 0, []Operation{{"w", "write", 7, 1, end(2)}, {"r", "read", 0, 2, end(3)}}}
	if check(t, h).Status != Valid {
		t.Fatal("tie treated as strict precedence")
	}
	h = History{1, -9223372036854775808, []Operation{{"r", "read", -9223372036854775808, 0, end(9223372036854775807)}}}
	if check(t, h).Status != Valid {
		t.Fatal("int64 boundary mishandled")
	}
	h.Operations[0].Return = nil
	if r := check(t, h); r.Status != Valid || len(r.Witness) != 0 {
		t.Fatalf("pending read not omitted: %+v", r)
	}
}
func TestMinimize(t *testing.T) {
	h := History{1, 0, []Operation{{"noise", "read", 0, 0, end(1)}, {"w", "write", 7, 2, end(3)}, {"r", "read", 0, 4, end(5)}}}
	out, e := Minimize(context.Background(), h, Options{100000}, 1000)
	if e != nil || !out.OneMinimal || out.Classification != "diagnostic_only" || len(out.History.Operations) != 2 {
		t.Fatalf("reduction %+v %v", out, e)
	}
	if check(t, out.History).Status != Invalid {
		t.Fatal("reduction lost failure")
	}
	for i := range out.History.Operations {
		c := out.History
		c.Operations = append(append([]Operation{}, c.Operations[:i]...), c.Operations[i+1:]...)
		if check(t, c).Status != Valid {
			t.Fatal("not delete-one minimal")
		}
	}
	small, e := Minimize(context.Background(), h, Options{100000}, 1)
	if e != nil || small.OneMinimal || small.Checks != 1 {
		t.Fatalf("budget falsely minimal %+v %v", small, e)
	}
	if _, e := Minimize(context.Background(), History{1, 0, []Operation{}}, Options{100}, 10); e == nil {
		t.Fatal("reduced valid history")
	}
}

// independentOracle enumerates optional subsets and complete permutations,
// then verifies the whole order directly. It does not share the checker's
// predecessor masks, DFS state transitions or failed-state memoization.
func independentOracle(h History) bool {
	required := []int{}
	optional := []int{}
	for i, o := range h.Operations {
		if o.Return != nil {
			required = append(required, i)
		} else if o.Kind == "write" {
			optional = append(optional, i)
		}
	}
	for subset := 0; subset < (1 << len(optional)); subset++ {
		order := append([]int{}, required...)
		for j, i := range optional {
			if subset&(1<<j) != 0 {
				order = append(order, i)
			}
		}
		found := false
		var permute func(int)
		permute = func(pos int) {
			if found {
				return
			}
			if pos == len(order) {
				positions := map[int]int{}
				for rank, i := range order {
					positions[i] = rank
				}
				for _, i := range order {
					for _, j := range order {
						a, b := h.Operations[i], h.Operations[j]
						if a.Return != nil && *a.Return < b.Call && positions[i] > positions[j] {
							return
						}
					}
				}
				value := h.Initial
				for _, i := range order {
					o := h.Operations[i]
					switch o.Kind {
					case "write":
						value = o.Value
					case "read":
						if o.Value != value {
							return
						}
					}
				}
				found = true
				return
			}
			for j := pos; j < len(order); j++ {
				order[pos], order[j] = order[j], order[pos]
				permute(pos + 1)
				order[pos], order[j] = order[j], order[pos]
			}
		}
		permute(0)
		if found {
			return true
		}
	}
	return false
}
func verifyWitness(t *testing.T, h History, r Result) {
	t.Helper()
	seen := map[string]int{}
	byID := map[string]Operation{}
	for _, o := range h.Operations {
		byID[o.ID] = o
	}
	value := h.Initial
	for i, id := range r.Witness {
		o, ok := byID[id]
		if !ok {
			t.Fatal("unknown witness operation")
		}
		if _, ok := seen[id]; ok {
			t.Fatal("duplicate witness")
		}
		seen[id] = i
		if o.Kind == "write" {
			value = o.Value
		} else {
			if o.Return == nil || o.Value != value {
				t.Fatal("illegal witness read")
			}
		}
	}
	for _, a := range h.Operations {
		if a.Return == nil {
			continue
		}
		pa, ok := seen[a.ID]
		if !ok {
			t.Fatal("missing completed op")
		}
		for _, b := range h.Operations {
			if pb, in := seen[b.ID]; in && *a.Return < b.Call && pa > pb {
				t.Fatal("witness violates real-time order")
			}
		}
	}
}
func TestFiniteIndependentOracle(t *testing.T) {
	variants := []Operation{}
	for _, interval := range [][2]int64{{0, 2}, {1, 3}, {3, 4}} {
		for _, kind := range []string{"read", "write"} {
			for _, value := range []int64{0, 1} {
				variants = append(variants, Operation{Kind: kind, Value: value, Call: interval[0], Return: end(interval[1])})
			}
		}
		for _, value := range []int64{0, 1} {
			variants = append(variants, Operation{Kind: "write", Value: value, Call: interval[0]})
		}
		variants = append(variants, Operation{Kind: "read", Value: 1, Call: interval[0]})
	}
	count := 0
	for n := 0; n <= 3; n++ {
		ops := make([]Operation, n)
		var enumerate func(int)
		enumerate = func(i int) {
			if i < n {
				for _, o := range variants {
					o.ID = fmt.Sprintf("op%d", i)
					ops[i] = o
					enumerate(i + 1)
				}
				return
			}
			h := History{1, 0, append([]Operation{}, ops...)}
			r := check(t, h)
			want := independentOracle(h)
			count++
			if r.Status == Inconclusive || (r.Status == Valid) != want {
				t.Fatalf("differential mismatch %+v got %+v want %t", h, r, want)
			}
			if r.Status == Valid {
				verifyWitness(t, h, r)
			}
		}
		enumerate(0)
	}
	if count != 9724 {
		t.Fatalf("enumeration count %d", count)
	}
	t.Logf("Enumerated %d histories, <=3 operations, 2 values, 3 interval templates, including pending reads/writes. Not an unbounded proof or Porcupine integration.", count)
}
func FuzzHistory(f *testing.F) {
	f.Add([]byte(`{"version":1,"initial":0,"operations":[]}`))
	f.Add([]byte(`{"version":1,"initial":0,"operations":[{"id":"r","kind":"read","value":0,"call":1,"return":2}]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		h, e := Decode(strings.NewReader(string(b)))
		if e != nil {
			return
		}
		r, e := Check(context.Background(), h, Options{1000})
		if e != nil {
			t.Fatal(e)
		}
		if r.Status == Valid {
			verifyWitness(t, h, r)
		}
		if r.States > 1000 {
			t.Fatal("state bound violated")
		}
	})
}

func TestDecodeAdversarial(t *testing.T) {
	bad := []string{
		`{"version":1,"initial":0,"operations":[{"id":"x","kind":"read","value":0,"call":0,"return":1},{"id":"x","kind":"read","value":0,"call":2,"return":3}]}`,
		`{"version":1,"initial":0,"operations":[{"id":"x","kind":"read","value":0,"call":-1,"return":1}]}`,
		`{"version":1,"initial":0,"operations":[{"id":"x","kind":"read","value":0,"call":1,"return":1}]}`,
		`{"version":1,"initial":0,"operations":[{"id":"x","kind":"read","value":9223372036854775808,"call":0,"return":1}]}`,
		`{"version":1,"initial":0,"operations":[{"id":"x","kind":"read","value":0.0,"call":0,"return":1}]}`,
		`{"version":1,"initial":null,"operations":[]}`,
		`{"version":1,"operations":[]}`,
		`{"version":1,"initial":0,"operations":[{"id":"bad space","kind":"read","value":0,"call":0,"return":1}]}`,
	}
	for _, b := range bad {
		if _, e := Decode(strings.NewReader(b)); e == nil {
			t.Errorf("accepted malformed %.120s", b)
		}
	}
}
func TestReductionPreservesUnknown(t *testing.T) {
	h := History{1, 0, []Operation{{"bad", "read", 1, 0, end(1)}, {"w", "write", 7, 2, end(3)}}}
	out, e := Minimize(context.Background(), h, Options{1}, 2)
	if e != nil || out.OneMinimal || out.InconclusiveCandidates != 1 || len(out.History.Operations) != 2 {
		t.Fatalf("unknown candidate removed or minimality overstated: %+v %v", out, e)
	}
}
