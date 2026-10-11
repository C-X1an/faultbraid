package labreal

// Independent reviewer lifecycle and handler controls. The failing helper is
// a synthetic child used only to exercise owned process cleanup; it is not a
// Raft conformance result or a production target replacement.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/target"
)

func TestMain(m *testing.M) {
	root, err := filepath.Abs("../..")
	if err != nil {
		fmt.Fprintln(os.Stderr, "independent source unavailable")
		os.Exit(2)
	}
	identity, err := buildinfo.FromFiles(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "independent identity unavailable")
		os.Exit(2)
	}
	buildinfo.SourceTreeSHA, buildinfo.ModuleLockSHA, buildinfo.PatchSHA = identity.SourceTreeSHA, identity.ModuleLockSHA, identity.PatchSHA
	if len(os.Args) > 1 && os.Args[1] == "s42-intentional-exit" {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			os.Exit(3)
		}
		executable, err := os.Executable()
		if err != nil {
			os.Exit(3)
		}
		sha, _, err := fileHash(executable)
		if err != nil {
			os.Exit(3)
		}
		record := wire{Type: "ready", Node: 1, Incarnation: 1, PID: os.Getpid(), Address: listener.Addr().String(), Identity: identity, BinarySHA: sha, Snapshot: Snapshot{Status: target.Status{ID: 1, Started: true}}}
		if json.NewEncoder(os.Stdout).Encode(record) != nil {
			os.Exit(3)
		}
		if listener.Close() != nil {
			os.Exit(3)
		}
		os.Exit(2) // Intentional unexpected child failure, before the parent's kill.
	}
	if len(os.Args) > 1 && os.Args[1] == "lab-node" {
		os.Exit(NodeMain(os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func TestIndependentUnexpectedExitedChildMustNotBecomeSuccessfulOwnedStop(t *testing.T) {
	directory := t.TempDir()
	stdout, err := os.OpenFile(filepath.Join(directory, "child.stdout.ndjson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.OpenFile(filepath.Join(directory, "child.stderr.ndjson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binarySHA, _, err := fileHash(executable)
	if err != nil {
		t.Fatal(err)
	}
	c := &child{index: 0, node: 1, incarnation: 1, stdout: stdout, stderr: stderr, waited: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := &lab{ctx: ctx, cancel: cancel, directory: directory, options: Options{Identity: buildinfo.Current()}, events: make(chan observation, 8)}
	l.result.BinarySHA = binarySHA
	l.result.Processes = []Process{{Node: 1, Incarnation: 1, Identity: buildinfo.Current(), BinarySHA: binarySHA, EntropySeed: 0, Stdout: "child.stdout.ndjson"}}
	c.command = exec.Command(executable, "s42-intentional-exit")
	c.command.Stdout = &childOutput{ctx: ctx, file: stdout, child: c, events: l.events, budget: &l.budget, protocol: true}
	c.command.Stderr = &childOutput{ctx: ctx, file: stderr, child: c, events: l.events, budget: &l.budget}
	c.command.WaitDelay = time.Second
	c.owner, err = startOwned(c.command)
	if err != nil {
		t.Fatal(err)
	}
	defer c.owner.Close()
	l.result.Processes[0].PID = c.command.Process.Pid
	go func() { c.waitErr = c.command.Wait(); close(c.waited) }()
	select {
	case <-c.waited:
	case <-time.After(5 * time.Second):
		_ = c.owner.Kill()
		t.Fatal("intentional helper did not finish")
	}
	if c.waitErr == nil || c.command.ProcessState.ExitCode() != 2 {
		t.Fatalf("helper did not actually fail: %v", c.waitErr)
	}
	var ready observation
	select {
	case ready = <-l.events:
	case <-time.After(time.Second):
		t.Fatal("actual readiness record not retained")
	}
	if ready.wire == nil || ready.wire.Type != "ready" || ready.wire.PID != c.command.Process.Pid {
		t.Fatal("invalid actual helper identity")
	}
	c.address = ready.wire.Address
	l.result.Processes[0].Address = c.address
	stopErr := l.stop(c, "scenario-kill")
	if !l.result.Processes[0].Waited || !l.result.Processes[0].SocketClosed {
		t.Fatal("owned helper cleanup did not finish")
	}
	if stopErr == nil {
		t.Fatalf("unexpected real child exit 2 was silently converted to a successful owned stop; waitErr=%v", c.waitErr)
	}
}

func TestIndependentHTTPInvalidProposalNeverEnqueues(t *testing.T) {
	n := &nodeServer{ctx: context.Background(), requests: make(chan nodeRequest, 1)}
	for _, item := range []struct{ method, contentType, body string }{
		{http.MethodGet, "application/json", `{"request_id":"r","kind":"read","value":0}`},
		{http.MethodPost, "text/plain", `{"request_id":"r","kind":"read","value":0}`},
		{http.MethodPost, "application/json", `{"request_id":"r","kind":"read","value":0,"kind":"write"}`},
		{http.MethodPost, "application/json", `{"request_id":"r","kind":"read","value":null}`},
		{http.MethodPost, "application/json", `{"request_id":"r","kind":"read","value":0,"path":"x"}`},
		{http.MethodPost, "application/json", strings.Repeat(" ", maxProposeBytes+1)},
	} {
		r := httptest.NewRequest(item.method, "http://127.0.0.1/v1/propose", strings.NewReader(item.body))
		r.Header.Set("Content-Type", item.contentType)
		w := httptest.NewRecorder()
		n.propose(w, r)
		if w.Code != http.StatusBadRequest || len(n.requests) != 0 {
			t.Fatal("malformed proposal was admitted")
		}
		var result response
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Admission != "rejected-before-admission" {
			t.Fatal("pre-admission error was misclassified")
		}
	}
}

func TestIndependentHTTPInvalidFrameNeverEnqueues(t *testing.T) {
	n := &nodeServer{ctx: context.Background(), requests: make(chan nodeRequest, 1)}
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/v1/message", strings.NewReader(strings.Repeat("x", target.MaxFrameBytes+1)))
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	n.message(w, r)
	if w.Code != http.StatusBadRequest || len(n.requests) != 0 {
		t.Fatal("oversize frame was admitted")
	}
}

func TestIndependentListenerCloseReleasesWaitingAccept(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l := &boundedListener{Listener: listener, slots: make(chan struct{}, 1), done: make(chan struct{})}
	l.slots <- struct{}{}
	done := make(chan error, 1)
	go func() { _, err := l.Accept(); done <- err }()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed listener accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("bounded listener blocked after close")
	}
}
