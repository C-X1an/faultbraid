package system

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/labreal"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func nativeConformanceDependencies() []labreal.CompletionDependency {
	retries := []string{"write7:1-retry", "write7:2-retry", "write7:3-retry"}
	dependencies := []labreal.CompletionDependency{{ActionID: "kill-node2", RequestID: "read7"}}
	for _, id := range []string{"read7:1", "read7:2", "read7:3"} {
		dependencies = append(dependencies, labreal.CompletionDependency{ActionID: id, RequestID: "write7"})
	}
	for _, id := range retries {
		dependencies = append(dependencies, labreal.CompletionDependency{ActionID: id, RequestID: "write9"})
	}
	for _, id := range []string{"read9:1", "read9:2", "read9:3"} {
		dependencies = append(dependencies, labreal.CompletionDependency{ActionID: id, RequestID: "write9"}, labreal.CompletionDependency{ActionID: id, RequestID: "write7", AttemptActions: append([]string(nil), retries...)})
	}
	return dependencies
}

func assertActualRetryAfterLaterAck(t *testing.T, result labreal.Result, write9Ack int64) {
	t.Helper()
	assertActualNonRequestDispatch(t, result)
	read9 := nativeOperation(t, result, "read9")
	invocations := map[string]int64{}
	for _, attempt := range result.Attempts {
		if strings.HasSuffix(attempt.Action, "-retry") && attempt.Phase == "invoke" {
			invocations[attempt.Action] = attempt.Ordinal
		}
	}
	for _, attempt := range result.Attempts {
		if strings.HasSuffix(attempt.Action, "-retry") && attempt.Phase == "complete" && attempt.Admission == "admitted" && invocations[attempt.Action] > write9Ack && attempt.Ordinal < read9.Call {
			return
		}
	}
	t.Fatal("no genuine old retry was invoked after write9 acknowledgment and completed before read9 invocation")
}

func TestNativeCompletionBarrierCancellationRetainsOwnedCleanup(t *testing.T) {
	s := scenario.Default(0)
	s.Actions = []scenario.Action{
		{ID: "kill-two", At: 1, Kind: "crash", Node: scenario.Node(2)},
		{ID: "never-ack", At: 2, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("never")},
		{ID: "blocked", At: 3, Kind: "invoke_read", Node: scenario.Node(1), RequestID: scenario.Text("blocked")},
	}
	s.EndTime = scenario.Node(10)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	directory := realArtifactDirectory(t)
	result, err := labreal.Run(ctx, s, labreal.Options{Executable: executable, Directory: directory, Identity: buildinfo.Current(), Concurrency: 1, CompletionDependencies: []labreal.CompletionDependency{{ActionID: "blocked", RequestID: "never"}}})
	retainNativeResult(t, directory, result)
	if !errors.Is(err, context.DeadlineExceeded) || result.Check.Status != history.Inconclusive || !result.CleanupComplete {
		t.Fatalf("blocked completion cancellation/cleanup failed: err=%v status=%s cleanup=%v", err, result.Check.Status, result.CleanupComplete)
	}
	observedPending := false
	for _, attempt := range result.Attempts {
		if attempt.Action == "blocked" {
			t.Fatal("pending predecessor released dependent action")
		}
		if attempt.Action == "never-ack" && attempt.Phase == "pending" && attempt.Admission == "unknown" {
			observedPending = true
		}
	}
	if !observedPending || len(result.Processes) != 3 {
		t.Fatal("actual pending dead-node response and three owned processes were not observed")
	}
	for _, process := range result.Processes {
		if !process.Waited || !process.SocketClosed {
			t.Fatal("completion cancellation leaked an owned child or socket")
		}
	}
}

func TestNativeCompletionTerminalCancellationRetainsOwnedCleanup(t *testing.T) {
	s := scenario.Default(0)
	s.Actions = []scenario.Action{
		{ID: "kill-two", At: 1, Kind: "crash", Node: scenario.Node(2)},
		{ID: "never-ack", At: 2, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("never")},
	}
	s.EndTime = scenario.Node(10)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	directory := realArtifactDirectory(t)
	result, err := labreal.Run(ctx, s, labreal.Options{Executable: executable, Directory: directory, Identity: buildinfo.Current(), Concurrency: 1, RequiredCompletions: []string{"never"}})
	retainNativeResult(t, directory, result)
	if !errors.Is(err, context.DeadlineExceeded) || result.Check.Status != history.Inconclusive || !result.CleanupComplete {
		t.Fatalf("terminal completion wait ignored pending/cancellation/cleanup: err=%v status=%s cleanup=%v", err, result.Check.Status, result.CleanupComplete)
	}
	observedPending := false
	for _, attempt := range result.Attempts {
		if attempt.Action == "never-ack" && attempt.Phase == "pending" && attempt.Admission == "unknown" {
			observedPending = true
		}
	}
	if !observedPending || len(result.Processes) != 3 || len(result.CompletionBarriers) != 0 {
		t.Fatal("terminal control did not retain actual pending request and owned process observations")
	}
	for _, process := range result.Processes {
		if !process.Waited || !process.SocketClosed {
			t.Fatal("terminal cancellation leaked an owned child or socket")
		}
	}
}

func assertActualNonRequestDispatch(t *testing.T, result labreal.Result) {
	t.Helper()
	read7 := nativeOperation(t, result, "read7")
	if read7.Return == nil {
		t.Fatal("read7 acknowledgment missing before crash")
	}
	matched := 0
	for _, dispatch := range result.ActionDispatches {
		if dispatch.ActionID == "kill-node2" {
			matched++
			if dispatch.Kind != "crash" || dispatch.ObservedOrdinal < *read7.Return {
				t.Fatal("actual crash dispatch preceded its admitted read7 acknowledgment")
			}
		}
	}
	if matched != 1 {
		t.Fatalf("expected one actual configured crash dispatch observation, got %d", matched)
	}
}
