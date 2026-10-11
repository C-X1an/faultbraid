package labreal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

// This is an actual HTTP transport/collector unit fixture, not a Raft target
// or evidence of three-process conformance. The peer accepts a proposal, sends
// complete response headers but an interrupted body, then closes the socket.
// There is no affirmative before-admission rejection in those observed bytes.
func TestIndependentInterruptedProposalBodyRetainsPendingWrite(t *testing.T) {
	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/propose" || r.Method != http.MethodPost {
			t.Errorf("actual proposal endpoint was not called: %s %s", r.Method, r.URL.Path)
		}
		received <- struct{}{}
		connection, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		fmt.Fprint(buffer, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 200\r\nConnection: close\r\n\r\n{\"request_id\":\"uncertain\",")
		if err := buffer.Flush(); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	id := "uncertain"
	action := scenario.Action{ID: "invoked", Kind: "invoke_write", Node: scenario.Node(1), RequestID: &id, Value: scenario.Value(7)}
	c := &child{address: strings.TrimPrefix(server.URL, "http://"), command: &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}}}
	l := &lab{ctx: context.Background(), http: server.Client(), requests: map[string]*logicalRequest{id: {operation: history.Operation{ID: id, Kind: "write", Value: 7, Call: 1}}}, requestOrder: []string{id}, ordinal: 1}
	outcome := l.propose(clientTask{action: action, child: c})
	select {
	case <-received:
	default:
		t.Fatal("HTTP peer never received the actual proposal")
	}
	if outcome.err == nil {
		t.Fatal("truncated proposal response was accepted as definitive")
	}
	if err := l.clientResult(outcome); err == nil {
		t.Fatal("interrupted response protocol error was concealed")
	}
	l.collectHistory()
	if len(l.result.History.Operations) != 1 || l.result.History.Operations[0].ID != id || l.result.History.Operations[0].Return != nil || l.result.History.Operations[0].Value != 7 {
		t.Fatalf("accepted proposal with incomplete response disappeared from pending history: %+v", l.result.History)
	}
}
