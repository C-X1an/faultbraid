package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/C-X1an/faultbraid/internal/history"
	"strings"
	"testing"
)

type cancelAfterChecks struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *cancelAfterChecks) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestStdinAndErrorEnvelope(t *testing.T) {
	var out, err bytes.Buffer
	code := run([]string{"check"}, strings.NewReader(`{"version":1,"initial":0,"operations":[]}`), &out, &err)
	if code != 0 || !strings.Contains(out.String(), "VALID") {
		t.Fatalf("stdin %d %s %s", code, &out, &err)
	}
	out.Reset()
	err.Reset()
	code = run([]string{"check"}, strings.NewReader(`{"secret":"do-not-echo"}`), &out, &err)
	if code != 2 || strings.Contains(err.String(), "do-not-echo") {
		t.Fatal("error leaks payload")
	}
}

func TestUnsupportedSchemaClassification(t *testing.T) {
	for _, tt := range []struct {
		args  []string
		input string
	}{
		{[]string{"check"}, `{"version":2,"initial":0,"operations":[]}`},
		{[]string{"replay"}, `{"version":2,"model":"disk-register-v1","events":[],"steps":[],"digest":""}`},
		{[]string{"replay"}, `{"version":1,"model":"other-model","events":[],"steps":[],"digest":""}`},
	} {
		var out, stderr bytes.Buffer
		if code := run(tt.args, strings.NewReader(tt.input), &out, &stderr); code != 2 {
			t.Fatalf("%v exit=%d stderr=%s", tt.args, code, stderr.String())
		}
		var envelope struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil || envelope.Error.Code != "UNSUPPORTED_VERSION" {
			t.Fatalf("%v stderr=%s err=%v", tt.args, stderr.String(), err)
		}
	}
}

func TestMinimizeUncertaintyIsExitThree(t *testing.T) {
	input := `{"version":1,"initial":0,"operations":[{"id":"w","kind":"write","value":7,"call":1,"return":2},{"id":"r","kind":"read","value":0,"call":3,"return":4}]}`
	for _, args := range [][]string{
		{"minimize", "-max-states", "1"},
		{"minimize", "-max-checks", "1"},
	} {
		var out, stderr bytes.Buffer
		if code := run(args, strings.NewReader(input), &out, &stderr); code != 3 {
			t.Fatalf("%v exit=%d stdout=%s stderr=%s", args, code, out.String(), stderr.String())
		}
		var result history.Reduction
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("%v output=%s err=%v", args, out.String(), err)
		}
		if result.Status != history.Inconclusive || result.OneMinimal || result.Classification != "diagnostic_only" {
			t.Fatalf("%v result=%+v", args, result)
		}
		if len(result.History.Operations) != 2 {
			t.Fatalf("%v lost retained projection: %+v", args, result)
		}
	}
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &cancelAfterChecks{Context: base, cancel: cancel, remaining: 4}
	h := history.History{Version: 1, Operations: []history.Operation{
		{ID: "w", Kind: "write", Value: 7, Call: 1, Return: int64Pointer(2)},
		{ID: "r", Kind: "read", Value: 0, Call: 3, Return: int64Pointer(4)},
	}}
	result, err := history.Minimize(ctx, h, history.Options{MaxStates: 100000}, 100)
	if err != nil || result.Status != history.Inconclusive || result.OneMinimal || result.InconclusiveCandidates != 1 || !strings.Contains(result.Reason, "cancelled") {
		t.Fatalf("candidate cancellation was not preserved: %+v err=%v", result, err)
	}
}

func int64Pointer(value int64) *int64 { return &value }

type brokenWriter struct{}

func (brokenWriter) Write(b []byte) (int, error) { return 0, errors.New("broken output") }
func TestOutputFailureNotSuccess(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"check"}, strings.NewReader(`{"version":1,"initial":0,"operations":[]}`), brokenWriter{}, &stderr); code != 2 {
		t.Fatal("output failure reported success")
	}
}
