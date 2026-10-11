package labreal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

// This is a collector unit control. The separate system test supplies real
// child processes, HTTP responses, journals, and native kill/restart evidence.
func TestCollectorPreservesEarlierUncertainAdmissionAfterRejectedRetry(t *testing.T) {
	id := "same-logical-write"
	action := scenario.Action{ID: "first", Kind: "invoke_write", Node: scenario.Node(1), RequestID: &id, Value: scenario.Value(7)}
	l := &lab{requests: map[string]*logicalRequest{id: {operation: history.Operation{ID: id, Kind: "write", Value: 7, Call: 1}}}, requestOrder: []string{id}, ordinal: 1}
	if err := l.clientResult(clientOutcome{task: clientTask{action: action}, response: response{ID: id, Status: "pending", Admission: "unknown"}}); err != nil {
		t.Fatal(err)
	}
	action.ID = "retry"
	if err := l.clientResult(clientOutcome{task: clientTask{action: action}, response: response{ID: id, Status: "rejected", Admission: "rejected-before-admission"}}); err != nil {
		t.Fatal(err)
	}
	l.collectHistory()
	if len(l.result.History.Operations) != 1 || l.result.History.Operations[0].Return != nil || l.result.History.Operations[0].Value != 7 {
		t.Fatal("rejected retry erased an earlier possible effect")
	}
	l.requests[id].possible = false
	l.collectHistory()
	if len(l.result.History.Operations) != 0 {
		t.Fatal("affirmative rejected-only intent became an admitted semantic operation")
	}
}

func TestChildOutputEnforcesLineAndTotalBounds(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	w := &childOutput{ctx: context.Background(), file: file, budget: &outputBudget{}, protocol: true}
	if _, err := w.Write(make([]byte, maxWireLine+1)); !errors.Is(err, ErrLimit) {
		t.Fatal("unbounded child protocol line accepted")
	}
	w = &childOutput{ctx: context.Background(), file: file, budget: &outputBudget{bytes: maxAllOutput}, protocol: false}
	if _, err := w.Write([]byte("x")); !errors.Is(err, ErrLimit) {
		t.Fatal("aggregate child output bound ignored")
	}
}

func TestCancelledChildOutputKeepsDrainingBoundedProtocol(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	line, err := json.Marshal(wire{Type: "stop", Node: 1, PID: 42})
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	w := &childOutput{ctx: ctx, file: file, budget: &outputBudget{}, protocol: true, events: make(chan observation)}
	for _, part := range [][]byte{line[:5], line[5:], line} {
		if n, err := w.Write(part); err != nil || n != len(part) {
			t.Fatalf("cancelled observation closed the child's output pipe: n=%d err=%v", n, err)
		}
	}
	actual, err := os.ReadFile(file.Name())
	if err != nil || !bytes.Equal(actual, bytes.Repeat(line, 2)) || len(w.partial) != 0 {
		t.Fatal("shutdown lost retained protocol bytes")
	}
	w.budget.bytes = maxAllOutput
	if _, err := w.Write(line); !errors.Is(err, ErrLimit) {
		t.Fatal("shutdown disabled the aggregate output limit")
	}
}

func TestOwnedDirectoryExclusiveCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owned")
	if err := createOwnedDirectory(path); err != nil {
		t.Fatal(err)
	}
	if err := createOwnedDirectory(path); err == nil {
		t.Fatal("existing output directory overwritten")
	}
}
