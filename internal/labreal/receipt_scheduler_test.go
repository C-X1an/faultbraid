package labreal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/strictjson"
)

// These are actual loopback HTTP/collector scheduling fixtures. The synthetic
// endpoint supplies responses only to control parent receipt versus collection;
// it is not a Raft target or three-process conformance evidence. The unchanged
// system test separately exercises real targets, File.Sync, kill and restart.
func schedulingFixture(t *testing.T, respond func(proposal) response) *lab {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input proposal
		if r.Method != http.MethodPost || r.URL.Path != "/v1/propose" || strictjson.Decode(io.LimitReader(r.Body, maxProposeBytes+1), &input) != nil {
			t.Error("collector fixture did not receive a real proposal")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(respond(input)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	l := &lab{ctx: context.Background(), http: server.Client(), clients: make(chan clientTask, 8), requests: map[string]*logicalRequest{}}
	l.active[1] = &child{address: strings.TrimPrefix(server.URL, "http://"), command: &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}}}
	return l
}

func schedulingInvoke(t *testing.T, l *lab, id, kind string, value int64) clientTask {
	t.Helper()
	action := scenario.Action{ID: id + "-attempt", Kind: kind, Node: scenario.Node(1), RequestID: scenario.Text(id)}
	if kind == "invoke_write" {
		action.Value = scenario.Value(value)
	}
	if err := l.action(action); err != nil {
		t.Fatal(err)
	}
	return <-l.clients
}

func TestNativeCollectorReceiptPrecedesLaterInvocationDespiteDelayedCollection(t *testing.T) {
	l := schedulingFixture(t, func(input proposal) response {
		return response{ID: input.ID, Status: "complete", Admission: "admitted", Value: input.Value}
	})
	write := schedulingInvoke(t, l, "receipt-write", "invoke_write", 47)
	// The HTTP response is completely received. Deliberately advance the action
	// scheduler before collecting it, as a ready timer may do with queued events.
	outcome := l.propose(write)
	if outcome.err != nil || outcome.response.Status != "complete" {
		t.Fatalf("actual fixture HTTP completion missing: %+v", outcome)
	}
	schedulingInvoke(t, l, "later-read", "invoke_read", 0)
	if err := l.clientResult(outcome); err != nil {
		t.Fatal(err)
	}
	operation := l.requests["receipt-write"].operation
	later := l.requests["later-read"].operation
	if operation.Return == nil || *operation.Return >= later.Call {
		t.Fatalf("received HTTP completion was moved after later invocation: write=%+v read=%+v", operation, later)
	}
	l.collectHistory()
	for i := 1; i < len(l.result.Attempts); i++ {
		if l.result.Attempts[i-1].Ordinal >= l.result.Attempts[i].Ordinal {
			t.Fatal("retained attempts do not preserve receipt ordinals")
		}
	}
}

func TestNativeCollectorPreservesActualHTTPOverlap(t *testing.T) {
	received, release := make(chan struct{}), make(chan struct{})
	released := false
	l := schedulingFixture(t, func(input proposal) response {
		close(received)
		<-release
		return response{ID: input.ID, Status: "complete", Admission: "admitted", Value: input.Value}
	})
	t.Cleanup(func() {
		if !released {
			close(release)
		}
	})
	write := schedulingInvoke(t, l, "in-flight-write", "invoke_write", 47)
	done := make(chan clientOutcome, 1)
	go func() { done <- l.propose(write) }()
	select {
	case <-received:
	case outcome := <-done:
		t.Fatalf("proposal ended before the fixture received it: %+v", outcome)
	}
	schedulingInvoke(t, l, "overlapping-read", "invoke_read", 0)
	close(release)
	released = true
	outcome := <-done
	if outcome.err != nil || outcome.response.Status != "complete" {
		t.Fatalf("actual fixture HTTP completion missing: %+v", outcome)
	}
	if err := l.clientResult(outcome); err != nil {
		t.Fatal(err)
	}
	operation := l.requests["in-flight-write"].operation
	later := l.requests["overlapping-read"].operation
	if operation.Return == nil || *operation.Return <= later.Call {
		t.Fatalf("unfinished HTTP call acquired false precedence: write=%+v read=%+v", operation, later)
	}
}

func TestNativeCollectorKeepsFirstReceiptWhenRetryCollectionIsReordered(t *testing.T) {
	value := int64(17)
	l := schedulingFixture(t, func(input proposal) response {
		observed := value
		value = 29
		return response{ID: input.ID, Status: "complete", Admission: "admitted", Value: observed}
	})
	first := l.propose(schedulingInvoke(t, l, "retried-read", "invoke_read", 0))
	schedulingInvoke(t, l, "between-replies", "invoke_read", 0)
	second := l.propose(schedulingInvoke(t, l, "retried-read", "invoke_read", 0))
	if first.err != nil || second.err != nil || first.response.Status != "complete" || second.response.Status != "complete" {
		t.Fatal("actual fixture HTTP retry completions missing")
	}
	for _, outcome := range []clientOutcome{second, first} {
		if err := l.clientResult(outcome); err != nil {
			t.Fatal(err)
		}
	}
	operation := l.requests["retried-read"].operation
	between := l.requests["between-replies"].operation
	if operation.Return == nil || *operation.Return >= between.Call || operation.Value != first.response.Value {
		t.Fatalf("collection reordered the first logical completion: read=%+v between=%+v", operation, between)
	}
}
