package system

import (
	"fmt"
	"testing"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func TestAC012MinorityReadPendingThenHealProgress(t *testing.T) {
	s := staleReadScenario(false)
	isolated := run(t, s)
	if operation(t, isolated, "write9").Return == nil || operation(t, isolated, "stale").Return != nil || isolated.Result.Status != history.Valid {
		t.Fatalf("safe isolated read: %+v", isolated.History)
	}
	for _, node := range []uint64{1, 3} {
		s.Actions = append(s.Actions, scenario.Action{ID: fmt.Sprintf("heal-%d-out", node), At: 65, Kind: "heal", From: scenario.Node(2), To: scenario.Node(node)}, scenario.Action{ID: fmt.Sprintf("heal-%d-in", node), At: 65, Kind: "heal", From: scenario.Node(node), To: scenario.Node(2)})
	}
	invokeAll(&s, 70, "invoke_read", "stale", 0)
	s.EndTime = scenario.Node(85)
	healed := run(t, s)
	r := operation(t, healed, "stale")
	if r.Return == nil || r.Value != 9 || healed.Result.Status != history.Valid {
		t.Fatalf("healed schedule: %+v result=%+v", healed.History, healed.Result)
	}
}

func TestSameTargetUnsafeLocalReadIsDetected(t *testing.T) {
	trace := run(t, staleReadScenario(true))
	w, r := operation(t, trace, "write9"), operation(t, trace, "stale")
	if w.Return == nil || r.Return == nil || *w.Return >= r.Call || r.Value != 7 || trace.Result.Status != history.Invalid || trace.Failure == nil {
		t.Fatalf("negative control failed to expose stale read: %+v result=%+v", trace.History, trace.Result)
	}
	t.Logf("actual unsafe control fingerprint=%s digest=%s", trace.Failure.Fingerprint, trace.Digest)
}

func TestAC011RealPinnedRaftMajorityWriteAndRead(t *testing.T) {
	trace := run(t, majorityScenario(0))
	w, r := operation(t, trace, "write7"), operation(t, trace, "read7")
	if w.Return == nil || r.Return == nil || r.Value != 7 || trace.Result.Status != history.Valid {
		t.Fatalf("majority history: %+v result=%+v", trace.History, trace.Result)
	}
	if *w.Return >= r.Call {
		t.Fatal("fixture did not complete write before read invocation")
	}
	if trace.Identity() != identity(t) || len(trace.Choices) < 3 || len(trace.Events) < 10 {
		t.Fatal("actual source/dependency/entropy evidence absent")
	}
	for i, e := range trace.Events {
		if e.ID != uint64(i+1) || e.Parent >= e.ID {
			t.Fatal("invalid causal record order")
		}
	}
	t.Logf("AC-011 actual source=%s patch=%s choices=%d events=%d digest=%s", trace.Header.SourceSHA, trace.Header.PatchSHA, len(trace.Choices), len(trace.Events), trace.Digest)
}

func TestVirtualTargetLimitsStayInconclusive(t *testing.T) {
	for _, kind := range []string{"events", "queue", "checker", "trace"} {
		s := majorityScenario(0)
		switch kind {
		case "events":
			s.EventLimit = 5
		case "queue":
			s.QueueLimit = 1
		case "checker":
			s.CheckerStateLimit = 1
		case "trace":
			s.EndTime = scenario.Node(10000)
		}
		trace := run(t, s)
		if trace.Result.Status != history.Inconclusive || trace.Failure != nil {
			t.Fatalf("%s limit manufactured verdict %+v", kind, trace.Result)
		}
	}
}
