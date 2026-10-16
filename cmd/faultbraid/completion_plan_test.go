package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/scenario"
)

func TestNativeCompletionPlanStrictBoundaries(t *testing.T) {
	valid := `{"version":1,"completion_dependencies":[{"action_id":"read","request_id":"write","attempt_actions":["retry"]}],"required_completions":["read-id"]}`
	plan, err := decodeNativeCompletionPlan(strings.NewReader(valid))
	if err != nil || plan.Version != 1 || len(plan.CompletionDependencies) != 1 || plan.CompletionDependencies[0].AttemptActions[0] != "retry" || plan.RequiredCompletions[0] != "read-id" {
		t.Fatalf("valid plan decoding failed: %v", err)
	}
	for name, data := range map[string]string{
		"unknown":         strings.Replace(valid, `"version":1`, `"version":1,"unknown":0`, 1),
		"duplicate":       strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1),
		"case-alias":      strings.Replace(valid, `"version":1`, `"Version":1`, 1),
		"wrong-version":   strings.Replace(valid, `"version":1`, `"version":2`, 1),
		"missing-version": `{"completion_dependencies":[],"required_completions":[]}`,
		"null":            strings.Replace(valid, `"required_completions":["read-id"]`, `"required_completions":null`, 1),
		"nested-unknown":  strings.Replace(valid, `"action_id":"read"`, `"action_id":"read","unknown":true`, 1),
		"trailing":        valid + `{}`,
		"oversize":        valid + strings.Repeat(" ", maxNativeCompletionPlanBytes),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeNativeCompletionPlan(strings.NewReader(data)); err == nil {
				t.Fatal("ambiguous or out-of-contract completion plan accepted")
			}
		})
	}
	if _, err := decodeNativeCompletionPlan(io.MultiReader(strings.NewReader(valid), failedPlanReader{})); err == nil {
		t.Fatal("input read failure accepted")
	}
}

type failedPlanReader struct{}

func (failedPlanReader) Read([]byte) (int, error) { return 0, errors.New("intentional read failure") }

func TestNativeCLIRejectsMalformedCompletionPlanBeforeOutput(t *testing.T) {
	for name, data := range map[string]string{"version": `{"version":2}`, "unknown": `{"version":1,"extra":true}`, "duplicate": `{"version":1,"version":1}`, "oversize": strings.Repeat(" ", maxNativeCompletionPlanBytes+1)} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			path := filepath.Join(base, "plan.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(base, "lab")
			s, err := json.Marshal(scenario.Default(0))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := scenario.Decode(bytes.NewReader(s)); err != nil {
				t.Fatalf("invalid scenario fixture: %v", err)
			}
			var stdout, stderr bytes.Buffer
			code := runReal([]string{"-scenario", "-", "-outdir", directory, "-completion-plan", path}, bytes.NewReader(s), &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "INVALID_INPUT") || !strings.Contains(stderr.String(), "invalid native completion plan") {
				t.Fatalf("malformed plan did not reach its prelaunch guard: code=%d stderr=%s", code, stderr.String())
			}
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatalf("invalid plan created output: %v", err)
			}
		})
	}
}
