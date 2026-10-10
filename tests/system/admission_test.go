package system

import (
	"context"
	"testing"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func majorityFollower(t *testing.T, trace scenario.Trace) uint64 {
	t.Helper()
	for _, event := range trace.Events {
		if event.Kind == "reply" && event.Detail == "write9" && (event.Node == 1 || event.Node == 3) {
			return 4 - event.Node
		}
	}
	t.Fatal("actual majority write9 responder missing")
	return 0
}

func rejectedAttempt(t *testing.T, trace scenario.Trace, action string, node uint64) {
	t.Helper()
	var actionID, stepID uint64
	for _, event := range trace.Events {
		if event.Kind == "action" && event.Detail == action+":invoke_write" {
			actionID = event.ID
		}
		if event.Kind == "invoke-write" && event.Parent == actionID && event.Node == node {
			stepID = event.ID
		}
		if stepID != 0 && event.Kind == "rejected" && event.Parent == stepID && event.Detail == "not-leader-before-admission" {
			return
		}
	}
	t.Fatal("fixture attempt was not explicitly rejected before admission")
}

func TestAC012DefinitelyRejectedWriteCannotMaskActualStaleRead(t *testing.T) {
	s := staleReadScenario(true)
	baseline := run(t, s)
	if baseline.Result.Status != history.Invalid {
		t.Fatal("actual unsafe baseline did not fail")
	}
	follower := majorityFollower(t, baseline)
	last := s.Actions[len(s.Actions)-1]
	s.Actions = append(s.Actions[:len(s.Actions)-1], scenario.Action{ID: "mask", At: last.At, Kind: "invoke_write", Node: scenario.Node(follower), RequestID: scenario.Text("definitely-rejected"), Value: scenario.Value(7)}, last)
	trace := run(t, s)
	rejectedAttempt(t, trace, "mask", follower)
	for _, op := range trace.History.Operations {
		if op.ID == "definitely-rejected" {
			t.Fatal("rejected-only write became an optional history effect")
		}
	}
	if trace.Result.Status != history.Invalid || trace.Failure == nil || *trace.Failure != *baseline.Failure {
		t.Fatalf("definitely rejected write masked stale read: %+v", trace.Result)
	}
	if err := scenario.Replay(context.Background(), trace, identity(t)); err != nil {
		t.Fatal(err)
	}
}

func TestAC012AdmittedPendingWriteSurvivesRejectedRetry(t *testing.T) {
	s := staleReadScenario(true)
	follower := majorityFollower(t, run(t, s))
	last := s.Actions[len(s.Actions)-1]
	actions := append([]scenario.Action(nil), s.Actions[:len(s.Actions)-4]...)
	actions = append(actions, scenario.Action{ID: "uncertain", At: 49, Kind: "invoke_write", Node: scenario.Node(2), RequestID: scenario.Text("uncertain"), Value: scenario.Value(11)})
	actions = append(actions, s.Actions[len(s.Actions)-4:len(s.Actions)-1]...)
	s.Actions = append(actions, scenario.Action{ID: "retry-rejected", At: last.At, Kind: "invoke_write", Node: scenario.Node(follower), RequestID: scenario.Text("uncertain"), Value: scenario.Value(11)}, last)
	trace := run(t, s)
	rejectedAttempt(t, trace, "retry-rejected", follower)
	op := operation(t, trace, "uncertain")
	if op.Return != nil || op.Value != 11 {
		t.Fatalf("prior admitted uncertain write was lost or fabricated as complete: %+v", op)
	}
	if err := scenario.Replay(context.Background(), trace, identity(t)); err != nil {
		t.Fatal(err)
	}
}
