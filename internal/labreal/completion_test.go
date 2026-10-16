package labreal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func completionScenario() scenario.Scenario {
	s := scenario.Default(0)
	s.Actions = []scenario.Action{
		{ID: "first", At: 1, Kind: "invoke_write", Node: scenario.Node(1), RequestID: scenario.Text("write"), Value: scenario.Value(7)},
		{ID: "retry", At: 2, Kind: "invoke_write", Node: scenario.Node(2), RequestID: scenario.Text("write"), Value: scenario.Value(7)},
		{ID: "read", At: 3, Kind: "invoke_read", Node: scenario.Node(1), RequestID: scenario.Text("read")},
		{ID: "read-later", At: 4, Kind: "invoke_read", Node: scenario.Node(1), RequestID: scenario.Text("later")},
	}
	return s
}

func TestNativeCompletionValidationRejectsBeforeOutputOrChildren(t *testing.T) {
	s := completionScenario()
	cases := []struct {
		name         string
		dependencies []CompletionDependency
		terminal     []string
	}{
		{"unknown-action", []CompletionDependency{{ActionID: "missing", RequestID: "write"}}, nil},
		{"unknown-request", []CompletionDependency{{ActionID: "read", RequestID: "missing"}}, nil},
		{"future-or-cycle", []CompletionDependency{{ActionID: "first", RequestID: "read"}}, nil},
		{"unknown-filter", []CompletionDependency{{ActionID: "read", RequestID: "write", AttemptActions: []string{"missing"}}}, nil},
		{"future-filter", []CompletionDependency{{ActionID: "retry", RequestID: "write", AttemptActions: []string{"read"}}}, nil},
		{"same-action-filter", []CompletionDependency{{ActionID: "retry", RequestID: "write", AttemptActions: []string{"retry"}}}, nil},
		{"wrong-request-filter", []CompletionDependency{{ActionID: "read-later", RequestID: "write", AttemptActions: []string{"read"}}}, nil},
		{"duplicate-filter", []CompletionDependency{{ActionID: "read", RequestID: "write", AttemptActions: []string{"first", "first"}}}, nil},
		{"duplicate-dependency", []CompletionDependency{{ActionID: "read", RequestID: "write"}, {ActionID: "read", RequestID: "write"}}, nil},
		{"unknown-terminal", nil, []string{"missing"}},
		{"duplicate-terminal", nil, []string{"read", "read"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "uncreated")
			result, err := Run(context.Background(), s, Options{Executable: os.Args[0], Directory: output, Identity: buildinfo.Current(), Concurrency: 1, CompletionDependencies: test.dependencies, RequiredCompletions: test.terminal})
			if !errors.Is(err, ErrCompletionDependency) || len(result.Processes) != 0 {
				t.Fatalf("invalid dependency accepted: err=%v processes=%d", err, len(result.Processes))
			}
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid dependency created output: %v", err)
			}
		})
	}
}

func TestNativeCompletionOptionsCopyNestedFilters(t *testing.T) {
	input := Options{CompletionDependencies: []CompletionDependency{{ActionID: "read", RequestID: "write", AttemptActions: []string{"retry"}}}, RequiredCompletions: []string{"read"}}
	copy, err := checkedCompletionOptions(completionScenario(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.CompletionDependencies[0].AttemptActions[0] = "changed"
	input.RequiredCompletions[0] = "changed"
	if copy.CompletionDependencies[0].AttemptActions[0] != "retry" || copy.RequiredCompletions[0] != "read" {
		t.Fatal("caller mutation changed owned completion plan")
	}
}

// The following synthetic collector/event controls exercise only causality
// filtering and cancellation; they provide no Raft/process/durability credit.
func completionFixture(t *testing.T) *lab {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return &lab{ctx: ctx, cancel: cancel, events: make(chan observation, 8), requests: map[string]*logicalRequest{"write": {operation: history.Operation{ID: "write", Kind: "write", Value: 7, Call: 1}}}, result: Result{Scenario: scenario.Default(0)}}
}
func completionOutcome(action string, ordinal int64, status, admission string) clientOutcome {
	return clientOutcome{ordinal: ordinal, task: clientTask{action: scenario.Action{ID: action, Kind: "invoke_write", Node: scenario.Node(1), RequestID: scenario.Text("write"), Value: scenario.Value(7)}}, response: response{ID: "write", Status: status, Admission: admission, Value: 7}}
}
func TestNativeCompletionFilteredWaitNeedsActualAdmittedRetry(t *testing.T) {
	l := completionFixture(t)
	if err := l.clientResult(completionOutcome("first", 2, "complete", "admitted")); err != nil {
		t.Fatal(err)
	}
	dependency := CompletionDependency{ActionID: "read", RequestID: "write", AttemptActions: []string{"retry"}}
	for _, classification := range [][2]string{{"pending", "unknown"}, {"rejected", "rejected-before-admission"}, {"complete", "unknown"}} {
		if err := l.clientResult(completionOutcome("retry", 4, classification[0], classification[1])); err != nil {
			t.Fatal(err)
		}
		if _, ordinal := l.completionAcknowledgment(dependency); ordinal != 0 {
			t.Fatalf("non-acknowledgment released retry dependency: %v", classification)
		}
	}
	done := make(chan error, 1)
	go func() { done <- l.awaitCompletion(dependency) }()
	select {
	case err := <-done:
		t.Fatalf("initial logical completion released filtered retry: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	complete := completionOutcome("retry", 8, "complete", "admitted")
	l.events <- observation{client: &complete}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(l.result.CompletionBarriers) != 1 || l.result.CompletionBarriers[0].AcknowledgmentOrdinal != 8 || l.result.CompletionBarriers[0].AcknowledgmentActionID != "retry" {
		t.Fatalf("actual retry acknowledgment evidence missing: %+v", l.result.CompletionBarriers)
	}
	if *l.requests["write"].operation.Return != 2 || l.requests["write"].operation.Value != 7 {
		t.Fatal("dependency tracking changed earliest logical history/value")
	}
}
func TestNativeCompletionUnfilteredWaitRejectsPendingAndRejection(t *testing.T) {
	for _, classification := range [][2]string{{"pending", "unknown"}, {"rejected", "rejected-before-admission"}, {"complete", "unknown"}} {
		l := completionFixture(t)
		if err := l.clientResult(completionOutcome("first", 2, classification[0], classification[1])); err != nil {
			t.Fatal(err)
		}
		if _, ordinal := l.completionAcknowledgment(CompletionDependency{RequestID: "write"}); ordinal != 0 {
			t.Fatalf("non-admitted response established completion: %v", classification)
		}
	}
}
func TestNativeCompletionWaitCancellationAndObservationError(t *testing.T) {
	l := completionFixture(t)
	l.cancel()
	if err := l.awaitCompletion(CompletionDependency{RequestID: "write"}); !errors.Is(err, context.Canceled) || len(l.result.CompletionBarriers) != 0 {
		t.Fatalf("canceled wait released or lost cancellation: %v", err)
	}
	l = completionFixture(t)
	bad := errors.New("controlled observation error")
	l.events <- observation{err: bad}
	if err := l.awaitCompletion(CompletionDependency{RequestID: "write"}); !errors.Is(err, bad) {
		t.Fatalf("observation failure lost: %v", err)
	}
}

func TestNativeCompletionActionGateActuallyWaits(t *testing.T) {
	l := completionFixture(t)
	l.options.CompletionDependencies = []CompletionDependency{{ActionID: "dependent", RequestID: "write"}}
	done := make(chan error, 1)
	go func() { done <- l.awaitActionDependencies("dependent") }()
	select {
	case err := <-done:
		t.Fatalf("action gate ignored missing completion: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	complete := completionOutcome("first", 2, "complete", "admitted")
	l.events <- observation{client: &complete}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(l.result.CompletionBarriers) != 1 || l.result.CompletionBarriers[0].ActionID != "dependent" {
		t.Fatal("satisfied action gate was not recorded")
	}
}

func TestNativeCompletionPlanLimitsHaveInvalidPlanAndResourceClassification(t *testing.T) {
	for _, options := range []Options{{CompletionDependencies: make([]CompletionDependency, scenario.MaxActions+1)}, {RequiredCompletions: make([]string, scenario.MaxActions+1)}} {
		if _, err := checkedCompletionOptions(completionScenario(), options); !errors.Is(err, ErrCompletionDependency) || !errors.Is(err, ErrLimit) {
			t.Fatalf("plan resource failure lost precise classification: %v", err)
		}
	}
}

// Synthetic dispatch controls establish only observation semantics, not target credit.
func TestNativeNonRequestDispatchRecordsCommonCounterFloorOnce(t *testing.T) {
	l := completionFixture(t)
	l.requests["write"].ackOrdinal, l.requests["write"].ackAction = 5, "first"
	l.requests["read"] = &logicalRequest{ackOrdinal: 7, ackAction: "first-read"}
	l.options.CompletionDependencies = []CompletionDependency{{ActionID: "kill", RequestID: "write"}, {ActionID: "kill", RequestID: "read"}, {ActionID: "request-gate", RequestID: "write"}}
	l.ordinal = 11 // Includes invocation and response stamps; a floor, not an event time.
	if err := l.awaitActionDependencies("kill"); err != nil {
		t.Fatal(err)
	}
	if err := l.action(scenario.Action{ID: "kill", Kind: "crash", Node: scenario.Node(2)}); !errors.Is(err, scenario.ErrCausality) {
		t.Fatalf("expected no-child action-entry control, got %v", err)
	}
	if len(l.result.ActionDispatches) != 1 || l.result.ActionDispatches[0] != (ActionDispatch{ActionID: "kill", Kind: "crash", ObservedOrdinal: 11}) || l.ordinal != 11 || len(l.result.CompletionBarriers) != 2 {
		t.Fatalf("action entry did not retain one exact common-counter floor: %+v ordinal=%d", l.result.ActionDispatches, l.ordinal)
	}
	if err := l.action(scenario.Action{ID: "unconfigured", Kind: "crash", Node: scenario.Node(2)}); !errors.Is(err, scenario.ErrCausality) {
		t.Fatal(err)
	}
	l.clients = make(chan clientTask, 1)
	if err := l.action(scenario.Action{ID: "request-gate", Kind: "invoke_write", Node: scenario.Node(1), RequestID: scenario.Text("write"), Value: scenario.Value(7)}); err != nil {
		t.Fatal(err)
	}
	if len(l.result.ActionDispatches) != 1 || len(l.result.Attempts) != 1 || l.result.Attempts[0].Ordinal != 12 || l.ordinal != 12 {
		t.Fatal("unconfigured/non-request filtering changed request stamps")
	}
}
