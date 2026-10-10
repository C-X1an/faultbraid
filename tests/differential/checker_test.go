// Package differential checks the public register contract with two references
// that do not call the optimized checker's search internals. The finite census
// is explicitly bounded; it is not a proof for arbitrary 24-operation inputs.
package differential

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/anishathalye/porcupine"
)

type answerCounts struct {
	Valid   uint64 `json:"valid"`
	Invalid uint64 `json:"invalid"`
	Unknown uint64 `json:"unknown"`
}

func (counts *answerCounts) add(t *testing.T, status history.Verdict) {
	t.Helper()
	switch status {
	case history.Valid:
		counts.Valid++
	case history.Invalid:
		counts.Invalid++
	case history.Inconclusive:
		counts.Unknown++
	default:
		t.Fatalf("unrecognized differential answer %q", status)
	}
}

type census struct {
	Cases                       uint64       `json:"cases"`
	Core                        answerCounts `json:"core"`
	Tiny                        answerCounts `json:"tiny"`
	Porcupine                   answerCounts `json:"porcupine"`
	DefinitiveThreeWayCases     uint64       `json:"definitive_three_way_cases"`
	WitnessesChecked            uint64       `json:"witnesses_checked"`
	CoreStates                  uint64       `json:"core_states"`
	TinyCompletePermutations    uint64       `json:"tiny_complete_permutations"`
	TinyProjections             uint64       `json:"tiny_projections"`
	PorcupineCheckedProjections uint64       `json:"porcupine_checked_projections"`
	PorcupineUnknownProjections uint64       `json:"porcupine_unknown_projections"`
}

func returnAt(value int64) *int64 { return &value }

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// Preserve the first discrepancy as synthetic JSON in an ignored, newly owned
// directory. Unlike t.TempDir this survives a failed test for diagnosis. No
// successful corpus is invented and no pre-existing file is overwritten.
func discrepancy(t *testing.T, label, reason string, h history.History, core history.Result, tiny, third referenceResult, trace *scenario.Trace) {
	t.Helper()
	data, err := json.MarshalIndent(struct {
		Case      string          `json:"case"`
		Reason    string          `json:"reason"`
		History   history.History `json:"history"`
		Core      history.Result  `json:"core"`
		Tiny      referenceResult `json:"tiny"`
		Porcupine referenceResult `json:"porcupine"`
		Trace     *scenario.Trace `json:"target_trace,omitempty"`
	}{label, reason, h, core, tiny, third, trace}, "", "  ")
	if err != nil {
		t.Fatalf("%s: %s; could not encode discrepancy: %v", label, reason, err)
	}
	base := filepath.Join(moduleRoot(t), ".local-runs", "differential-discrepancies")
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatalf("%s: %s; could not retain discrepancy: %v", label, reason, err)
	}
	directory, err := os.MkdirTemp(base, "mismatch-")
	if err != nil {
		t.Fatalf("%s: %s; could not retain discrepancy: %v", label, reason, err)
	}
	path := filepath.Join(directory, "discrepancy.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		_, err = file.Write(append(data, '\n'))
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		t.Fatalf("%s: %s; discrepancy write failed: %v", label, reason, err)
	}
	t.Fatalf("%s: %s; retained discrepancy %s", label, reason, path)
}

func (counts *census) compare(t *testing.T, label string, h history.History, trace *scenario.Trace) {
	t.Helper()
	if err := h.Validate(); err != nil {
		t.Fatalf("generator emitted malformed history %s: %v", label, err)
	}
	core, err := history.Check(context.Background(), h, history.Options{MaxStates: 100000})
	if err != nil {
		t.Fatalf("checker rejected a validated history %s: %v", label, err)
	}
	tiny := tinyOracle(context.Background(), h, standardTinyLimits())
	third := porcupineOracle(context.Background(), h, standardPorcupineLimits())
	counts.Cases++
	counts.Core.add(t, core.Status)
	counts.Tiny.add(t, tiny.Status)
	counts.Porcupine.add(t, third.Status)
	counts.CoreStates += uint64(core.States)
	counts.TinyCompletePermutations += tiny.Work
	counts.TinyProjections += tiny.Projections
	counts.PorcupineCheckedProjections += third.Work
	counts.PorcupineUnknownProjections += third.UnknownProjections
	if core.Status == history.Valid {
		if !legalWitness(h, core.Witness) {
			discrepancy(t, label, "production witness is not a legal covering order", h, core, tiny, third, trace)
		}
		counts.WitnessesChecked++
	}
	answers := []history.Verdict{core.Status, tiny.Status, third.Status}
	for left := 0; left < len(answers); left++ {
		for right := left + 1; right < len(answers); right++ {
			if answers[left] != history.Inconclusive && answers[right] != history.Inconclusive && answers[left] != answers[right] {
				discrepancy(t, label, "definitive disagreement", h, core, tiny, third, trace)
			}
		}
	}
	if core.Status != history.Inconclusive && tiny.Status != history.Inconclusive && third.Status != history.Inconclusive {
		counts.DefinitiveThreeWayCases++
	}
	if trace != nil && trace.Result.Status != core.Status {
		discrepancy(t, label, "target's recorded history verdict disagrees with rechecking", h, core, tiny, third, trace)
	}
}

func (counts census) log(t *testing.T, domain string) {
	t.Helper()
	data, err := json.Marshal(counts)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("AC-016 %s: %s", domain, data)
}

func (counts census) requireComplete(t *testing.T, expected uint64) {
	t.Helper()
	if counts.Cases != expected || counts.DefinitiveThreeWayCases != expected || counts.Core.Unknown != 0 || counts.Tiny.Unknown != 0 || counts.Porcupine.Unknown != 0 {
		t.Fatalf("census did not finish every bounded case definitively: %+v expected=%d", counts, expected)
	}
	if counts.WitnessesChecked != counts.Core.Valid {
		t.Fatal("not every VALID production witness was independently verified")
	}
}

func TestDifferentialFiniteCensus(t *testing.T) {
	// Exact domain: initial=0; labelled slots op0..op(n-1), n=0..3;
	// kind in {read,write}; value in {0,7}; call in {0,1,2}; return
	// either pending or any member of {1,2,3} strictly greater than call.
	// There are 9 intervals, 36 choices per slot and
	// 1 + 36 + 36^2 + 36^3 = 47989 histories. Pending-read values are
	// deliberately enumerated even though their output is unobserved.
	choices := []history.Operation{}
	for _, kind := range []string{"read", "write"} {
		for _, value := range []int64{0, 7} {
			for call := int64(0); call <= 2; call++ {
				choices = append(choices, history.Operation{Kind: kind, Value: value, Call: call})
				for finish := int64(1); finish <= 3; finish++ {
					if finish > call {
						choices = append(choices, history.Operation{Kind: kind, Value: value, Call: call, Return: returnAt(finish)})
					}
				}
			}
		}
	}
	if len(choices) != 36 {
		t.Fatalf("finite domain drifted: %d slot choices", len(choices))
	}
	counts := census{}
	for length := 0; length <= 3; length++ {
		h := history.History{Version: 1, Initial: 0, Operations: make([]history.Operation, length)}
		var enumerate func(int)
		enumerate = func(slot int) {
			if slot == length {
				counts.compare(t, fmt.Sprintf("finite-%d", counts.Cases), h, nil)
				return
			}
			for _, choice := range choices {
				choice.ID = fmt.Sprintf("op%d", slot)
				h.Operations[slot] = choice
				enumerate(slot + 1)
			}
		}
		enumerate(0)
	}
	counts.log(t, "finite initial0 n0..3 values[0,7] calls[0,1,2] returns[pending,1,2,3] with return>call")
	counts.requireComplete(t, 47989)
	if counts.Core.Valid == 0 || counts.Core.Invalid == 0 {
		t.Fatal("census failed to exercise both semantic verdicts")
	}
}

func TestDifferentialGeneratedHistories(t *testing.T) {
	// Separate, deterministic PCG histories cover n=0..7, negative values,
	// int64 boundaries, ties, arbitrary overlaps, and pending operations.
	// For n<=7 even the loose bound 2^7*7! is below the tiny budget.
	values := []int64{math.MinInt64, -7, 0, 7, math.MaxInt64}
	counts := census{}
	for seed := uint64(0); seed < 1024; seed++ {
		random := rand.New(rand.NewPCG(seed, 0x4641554c54425241))
		h := history.History{Version: 1, Initial: 0, Operations: []history.Operation{}}
		length := random.IntN(8)
		for index := 0; index < length; index++ {
			call := int64(random.IntN(12))
			op := history.Operation{ID: fmt.Sprintf("op%d", index), Kind: "read", Value: values[random.IntN(len(values))], Call: call}
			if random.IntN(2) == 0 {
				op.Kind = "write"
			}
			if random.IntN(4) != 0 {
				op.Return = returnAt(call + 1 + int64(random.IntN(5)))
			}
			h.Operations = append(h.Operations, op)
		}
		counts.compare(t, fmt.Sprintf("pcg-seed-%d", seed), h, nil)
	}
	counts.log(t, "PCG seeds0..1023 n0..7 initial0 values[minInt64,-7,0,7,maxInt64] calls0..11 returns[pending,call+1..5]")
	counts.requireComplete(t, 1024)
}

func TestDifferentialSemanticControls(t *testing.T) {
	cases := []struct {
		name string
		ops  []history.Operation
		want history.Verdict
	}{
		{"strict-after-write", []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1, Return: returnAt(2)}, {ID: "r", Kind: "read", Value: 0, Call: 3, Return: returnAt(4)}}, history.Invalid},
		{"overlap", []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1, Return: returnAt(4)}, {ID: "r", Kind: "read", Value: 0, Call: 2, Return: returnAt(3)}}, history.Valid},
		{"return-call-tie", []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 0, Return: returnAt(2)}, {ID: "r", Kind: "read", Value: 0, Call: 2, Return: returnAt(3)}}, history.Valid},
		{"pending-write-observed", []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1}, {ID: "r", Kind: "read", Value: 7, Call: 3, Return: returnAt(4)}}, history.Valid},
		{"pending-write-omitted", []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1}, {ID: "r", Kind: "read", Value: 0, Call: 3, Return: returnAt(4)}}, history.Valid},
		{"pending-read-unobserved", []history.Operation{{ID: "pending", Kind: "read", Value: math.MaxInt64, Call: 100}, {ID: "r", Kind: "read", Value: 0, Call: 1, Return: returnAt(2)}}, history.Valid},
		{"pending-write-too-late", []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 5}, {ID: "r", Kind: "read", Value: 7, Call: 1, Return: returnAt(2)}}, history.Invalid},
		{"two-pending-effects", []history.Operation{{ID: "w7", Kind: "write", Value: 7, Call: 0}, {ID: "w9", Kind: "write", Value: 9, Call: 0}, {ID: "r7", Kind: "read", Value: 7, Call: 2, Return: returnAt(3)}, {ID: "r9", Kind: "read", Value: 9, Call: 4, Return: returnAt(5)}}, history.Valid},
		{"negative-boundary", []history.Operation{{ID: "w", Kind: "write", Value: math.MinInt64, Call: 0, Return: returnAt(1)}, {ID: "r", Kind: "read", Value: math.MinInt64, Call: 2, Return: returnAt(3)}}, history.Valid},
	}
	counts := census{}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			h := history.History{Version: 1, Initial: 0, Operations: test.ops}
			counts.compare(t, test.name, h, nil)
			core, err := history.Check(context.Background(), h, history.Options{MaxStates: 100000})
			if err != nil || core.Status != test.want {
				t.Fatalf("semantic control: got %+v error=%v want=%s", core, err, test.want)
			}
		})
	}
	counts.log(t, "named semantic positive and known-bad controls")
	counts.requireComplete(t, uint64(len(cases)))
}

func TestDifferentialUnknownControls(t *testing.T) {
	h := history.History{Version: 1, Initial: 0, Operations: []history.Operation{
		{ID: "w", Kind: "write", Value: 7, Call: 1},
		{ID: "r", Kind: "read", Value: 7, Call: 3, Return: returnAt(4)},
	}}
	core, err := history.Check(context.Background(), h, history.Options{MaxStates: 1})
	if err != nil || core.Status != history.Inconclusive {
		t.Fatalf("core budget changed UNKNOWN: %+v %v", core, err)
	}
	tiny := standardTinyLimits()
	tiny.MaxPermutations = 0
	if result := tinyOracle(context.Background(), h, tiny); result.Status != history.Inconclusive {
		t.Fatalf("tiny permutation budget changed UNKNOWN: %+v", result)
	}
	tiny = standardTinyLimits()
	tiny.MaxProjections = 1
	if result := tinyOracle(context.Background(), h, tiny); result.Status != history.Inconclusive || result.Projections != 1 {
		t.Fatalf("tiny subset budget changed UNKNOWN: %+v", result)
	}
	third := standardPorcupineLimits()
	third.MaxProjections = 1
	if result := porcupineOracle(context.Background(), h, third); result.Status != history.Inconclusive || result.Work != 1 {
		t.Fatalf("Porcupine subset budget converted rejected omission to INVALID: %+v", result)
	}
	for _, which := range []string{"operations", "pending", "time"} {
		t.Run("capacity-"+which, func(t *testing.T) {
			tiny, third := standardTinyLimits(), standardPorcupineLimits()
			switch which {
			case "operations":
				tiny.MaxOperations, third.MaxOperations = 0, 0
			case "pending":
				tiny.MaxPending, third.MaxPending = 0, 0
			case "time":
				tiny.MaxProjections = 0
				third.TotalTimeout = 0
			}
			if tinyOracle(context.Background(), h, tiny).Status != history.Inconclusive || porcupineOracle(context.Background(), h, third).Status != history.Inconclusive {
				t.Fatal("reference capacity converted incomplete checking to a verdict")
			}
		})
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	core, err = history.Check(cancelled, h, history.Options{MaxStates: 100000})
	if err != nil || core.Status != history.Inconclusive || tinyOracle(cancelled, h, standardTinyLimits()).Status != history.Inconclusive || porcupineOracle(cancelled, h, standardPorcupineLimits()).Status != history.Inconclusive {
		t.Fatalf("cancellation lost UNKNOWN: core=%+v error=%v", core, err)
	}
	if fromPorcupine(porcupine.Unknown) != history.Inconclusive || fromPorcupine(porcupine.CheckResult("unsupported-result")) != history.Inconclusive {
		t.Fatal("Unknown or unrecognized Porcupine result became a semantic verdict")
	}

	// This is an explicit synthetic timeout control for the real Porcupine
	// checker. The callback waits for its cancellation; it is not target data.
	model := registerModel(0)
	model.StepContext = func(ctx context.Context, state, input, output any) (bool, any) {
		<-ctx.Done()
		return false, state
	}
	start := time.Now()
	answer, _ := porcupine.CheckOperationsVerbose(model, []porcupine.Operation{{ClientId: 0, Input: registerInput{}, Output: int64(0), Call: 0, Return: 1}}, time.Millisecond)
	if answer != porcupine.Unknown || fromPorcupine(answer) != history.Inconclusive {
		t.Fatalf("actual Porcupine timeout mapped to %q", answer)
	}
	t.Logf("AC-016 unknown controls: core-state-budget=UNKNOWN references-capacity=UNKNOWN cancelled-all=UNKNOWN actual-Porcupine-timeout=%s elapsed=%s", answer, time.Since(start))
}

func TestDifferentialProjectionTimestampCapacity(t *testing.T) {
	h := history.History{Version: 1, Initial: 0, Operations: []history.Operation{
		{ID: "w", Kind: "write", Value: 7, Call: 0},
		{ID: "r", Kind: "read", Value: 7, Call: math.MaxInt64 - 1, Return: returnAt(math.MaxInt64)},
	}}
	counts := census{}
	counts.compare(t, "timestamp-capacity-inclusion", h, nil)
	core, err := history.Check(context.Background(), h, history.Options{MaxStates: 100000})
	tiny := tinyOracle(context.Background(), h, standardTinyLimits())
	third := porcupineOracle(context.Background(), h, standardPorcupineLimits())
	if err != nil || core.Status != history.Valid || tiny.Status != history.Valid || third.Status != history.Inconclusive || third.UnknownProjections != 1 || third.Work != 1 {
		t.Fatalf("int64 projection bound misclassified: core=%+v err=%v tiny=%+v Porcupine=%+v", core, err, tiny, third)
	}
	if !legalWitness(h, core.Witness) {
		t.Fatal("int64 core witness invalid")
	}
	// A definitely valid omitted subset remains sufficient even if including
	// a pending write would require an unrepresentable artificial timestamp.
	h.Operations[1].Value = 0
	counts.compare(t, "timestamp-capacity-omission", h, nil)
	third = porcupineOracle(context.Background(), h, standardPorcupineLimits())
	if third.Status != history.Valid || third.Work != 1 {
		t.Fatalf("known legal omitted projection was lost: %+v", third)
	}
	counts.log(t, "timestamp-capacity controls with one explicit Porcupine UNKNOWN")
	if counts.Cases != 2 || counts.DefinitiveThreeWayCases != 1 || counts.Porcupine.Unknown != 1 || counts.PorcupineUnknownProjections != 1 || counts.Core.Unknown != 0 || counts.Tiny.Unknown != 0 {
		t.Fatalf("timestamp-capacity UNKNOWN was not counted separately: %+v", counts)
	}
}

func TestDifferentialLockedDependencyIdentity(t *testing.T) {
	const module = "github.com/anishathalye/porcupine"
	const version = "v1.3.1"
	const sum = "h1:fBZ4/NGNPnIDdd6xNtrNk9/GiEQ0L4FO5+scINN+t0E="
	root := moduleRoot(t)
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.Contains(string(mod), module+" "+version) {
		t.Fatalf("Porcupine module version not locked: %v", err)
	}
	sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil || !strings.Contains(string(sums), module+" "+version+" "+sum+"\n") {
		t.Fatalf("Porcupine module sum not locked: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "dependency.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Modules []struct {
			Module  string `json:"module"`
			Version string `json:"version"`
			Sum     string `json:"sum"`
			ZipSHA  string `json:"upstream_zip_sha256"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, dependency := range lock.Modules {
		if dependency.Module == module {
			found = dependency.Version == version && dependency.Sum == sum && dependency.ZipSHA == "ed7d64fdef1df58ed8a3c6aa31e185c9f5649bf6607bb6708144c91253073ecc"
		}
	}
	if !found {
		t.Fatal("Porcupine dependency-lock identity drifted")
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dependency := range info.Deps {
			if dependency.Path == module {
				if dependency.Replace != nil || dependency.Version != version || dependency.Sum != sum {
					t.Fatalf("linked Porcupine differs from recorded dependency: %+v", dependency)
				}
				t.Logf("linked Porcupine: %s %s %s", dependency.Path, dependency.Version, dependency.Sum)
			}
		}
	}
	t.Logf("AC-016 dependency fixture: %s %s %s; Go=%s; runtime build metadata may omit test-binary dependencies", module, version, sum, runtime.Version())
}
