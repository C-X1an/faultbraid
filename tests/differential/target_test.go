package differential

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func invokeEveryVoter(s *scenario.Scenario, at uint64, kind, id string, value int64) {
	for node := uint64(1); node <= 3; node++ {
		action := scenario.Action{ID: fmt.Sprintf("%s-at-%d", id, node), At: at + node - 1, Kind: kind, Node: scenario.Node(node), RequestID: scenario.Text(id)}
		if kind == "invoke_write" {
			action.Value = scenario.Value(value)
		}
		s.Actions = append(s.Actions, action)
	}
}

// Each seed executes the instrumented Raft Target through scenario.Run. The
// scheduling is finite and explicit, with six logical requests and one of four
// fault patterns. Retried requests keep their ID and payload. This is target
// integration data, not generated register answers or a mocked transport run.
func generatedTargetScenario(seed uint64) scenario.Scenario {
	s := scenario.Default(seed)
	for round := uint64(0); round < 3; round++ {
		value := int64(round+1) * 7
		invokeEveryVoter(&s, 30+round*15, "invoke_write", fmt.Sprintf("w%d", round), value)
		invokeEveryVoter(&s, 38+round*15, "invoke_read", fmt.Sprintf("r%d", round), 0)
	}
	switch seed % 4 {
	case 1:
		s.Actions = append(s.Actions,
			scenario.Action{ID: "delay", At: 24, Kind: "delay_link", From: scenario.Node(1), To: scenario.Node(2), Ticks: scenario.Node(2)},
			scenario.Action{ID: "duplicate", At: 25, Kind: "duplicate_next", From: scenario.Node(1), To: scenario.Node(3), Count: scenario.Count(2)},
			scenario.Action{ID: "drop", At: 26, Kind: "drop_next", From: scenario.Node(3), To: scenario.Node(2), Count: scenario.Count(1)},
		)
	case 2:
		for _, other := range []uint64{2, 3} {
			s.Actions = append(s.Actions,
				scenario.Action{ID: fmt.Sprintf("isolate-out-%d", other), At: 40, Kind: "partition", From: scenario.Node(1), To: scenario.Node(other)},
				scenario.Action{ID: fmt.Sprintf("isolate-in-%d", other), At: 40, Kind: "partition", From: scenario.Node(other), To: scenario.Node(1)},
				scenario.Action{ID: fmt.Sprintf("heal-out-%d", other), At: 58, Kind: "heal", From: scenario.Node(1), To: scenario.Node(other)},
				scenario.Action{ID: fmt.Sprintf("heal-in-%d", other), At: 58, Kind: "heal", From: scenario.Node(other), To: scenario.Node(1)},
			)
		}
	case 3:
		s.Actions = append(s.Actions,
			scenario.Action{ID: "crash", At: 40, Kind: "crash", Node: scenario.Node(3)},
			scenario.Action{ID: "restart", At: 43, Kind: "restart", Node: scenario.Node(3)},
		)
	}
	sort.SliceStable(s.Actions, func(left, right int) bool { return s.Actions[left].At < s.Actions[right].At })
	s.EndTime = scenario.Node(85)
	return s
}

// Seed0's observed voter2 election plus its finite CheckQuorum window yields a
// stale local read after the majority writes9. Safe ReadIndex leaves that read
// pending; the explicit unsafe-local-read variant completes it with7. Both
// executions use the same actual consensus integration, not handcrafted output.
func staleTargetScenario(unsafe bool) scenario.Scenario {
	s := scenario.Default(0)
	invokeEveryVoter(&s, 30, "invoke_write", "write7", 7)
	for _, other := range []uint64{1, 3} {
		s.Actions = append(s.Actions,
			scenario.Action{ID: fmt.Sprintf("isolate-out-%d", other), At: 40, Kind: "partition", From: scenario.Node(2), To: scenario.Node(other)},
			scenario.Action{ID: fmt.Sprintf("isolate-in-%d", other), At: 40, Kind: "partition", From: scenario.Node(other), To: scenario.Node(2)},
		)
	}
	invokeEveryVoter(&s, 52, "invoke_write", "write9", 9)
	s.Actions = append(s.Actions, scenario.Action{ID: "stale-read", At: 57, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("stale")})
	s.EndTime = scenario.Node(64)
	if unsafe {
		s.Variant = scenario.Text("unsafe-local-read")
	}
	return s
}

func TestDifferentialActualTargetHistories(t *testing.T) {
	identity, err := buildinfo.FromFiles(moduleRoot(t))
	if err != nil || !identity.Identified() {
		t.Fatalf("actual target source identity unavailable: %+v %v", identity, err)
	}
	counts := census{}
	var targetEvents, targetChoices uint64
	check := func(label string, s scenario.Scenario, want history.Verdict) scenario.Trace {
		t.Helper()
		trace, err := scenario.Run(context.Background(), s, scenario.Options{Identity: identity})
		if err != nil {
			t.Fatalf("actual target %s did not execute: %v", label, err)
		}
		if trace.Identity() != identity || len(trace.Choices) == 0 || len(trace.Events) == 0 {
			t.Fatalf("actual target %s lacks dependency/entropy/execution identity", label)
		}
		targetEvents += uint64(len(trace.Events))
		targetChoices += uint64(len(trace.Choices))
		counts.compare(t, label, trace.History, &trace)
		if trace.Result.Status != want {
			tiny := tinyOracle(context.Background(), trace.History, standardTinyLimits())
			third := porcupineOracle(context.Background(), trace.History, standardPorcupineLimits())
			discrepancy(t, label, "actual target control did not produce the expected verdict", trace.History, trace.Result, tiny, third, &trace)
		}
		return trace
	}
	for seed := uint64(0); seed < 32; seed++ {
		trace := check(fmt.Sprintf("target-safe-seed-%d", seed), generatedTargetScenario(seed), history.Valid)
		if len(trace.History.Operations) != 6 {
			t.Fatalf("actual target seed%d lost a logical request: %d operations", seed, len(trace.History.Operations))
		}
		completed := 0
		for _, op := range trace.History.Operations {
			if op.Return != nil {
				completed++
			}
		}
		if completed == 0 {
			t.Fatalf("actual target seed%d produced only unobserved operations", seed)
		}
	}
	safe := check("target-safe-minority-read", staleTargetScenario(false), history.Valid)
	unsafe := check("target-unsafe-minority-read", staleTargetScenario(true), history.Invalid)
	find := func(trace scenario.Trace, id string) history.Operation {
		for _, op := range trace.History.Operations {
			if op.ID == id {
				return op
			}
		}
		t.Fatalf("actual target control omitted %s", id)
		return history.Operation{}
	}
	if read := find(safe, "stale"); read.Return != nil {
		t.Fatalf("safe minority read completed: %+v", read)
	}
	write, read := find(unsafe, "write9"), find(unsafe, "stale")
	if write.Return == nil || read.Return == nil || *write.Return >= read.Call || read.Value != 7 || unsafe.Failure == nil || unsafe.Failure.Requirement != "FR-002" {
		t.Fatalf("unsafe variant did not retain a real strict stale-read counterexample: write=%+v read=%+v failure=%+v", write, read, unsafe.Failure)
	}
	counts.log(t, "actual Raft histories: safe seeds0..31 four fault patterns plus safe/unsafe minority-read controls")
	counts.requireComplete(t, 34)
	t.Logf("AC-016 actual-target events=%d entropy-choices=%d source=%s module=%s patch=%s", targetEvents, targetChoices, identity.SourceTreeSHA, identity.ModuleLockSHA, identity.PatchSHA)
}
