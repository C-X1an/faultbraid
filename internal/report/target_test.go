package report

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func reportFixture() scenario.Trace {
	// Explicitly synthetic renderer fixture, without executable provenance.
	return scenario.Trace{Version: 1, Model: "raft-trace-v1", Scenario: scenario.Default(0), History: history.History{Version: 1, Operations: []history.Operation{}}, Result: history.Result{Status: history.Inconclusive, Witness: []string{}}, Events: []scenario.Record{}, Digest: strings.Repeat("0", 64)}
}

func TestTargetInspectorEscapesAndHasOfflineSemanticNavigation(t *testing.T) {
	trace := reportFixture()
	payload := `<script src="https://attacker.invalid/x">alert(1)</script>`
	trace.Result.Reason = payload
	trace.Events = append(trace.Events, scenario.Record{Kind: "message", Detail: payload, PayloadSHA: strings.Repeat("0", 64)})
	view, err := RenderTarget(trace)
	if err != nil {
		t.Fatal(err)
	}
	text := string(view)
	if strings.Contains(text, "<script") || !strings.Contains(text, "&lt;script") {
		t.Fatal("target report did not escape hostile text")
	}
	for _, need := range []string{"Content-Security-Policy", "default-src 'none'", "scope=\"col\"", "aria-label=\"Trace sections\"", "href=\"#events\"", "tabindex=\"0\"", "INCONCLUSIVE", "Pending writes"} {
		if !strings.Contains(text, need) {
			t.Fatalf("missing semantic/control text %s", need)
		}
	}
	if strings.Contains(text, "<iframe") || strings.Contains(text, "<img") || strings.Contains(text, "<link ") {
		t.Fatal("report loads an active or external resource")
	}
}

func TestTargetInspectorStopsBeforeUnboundedGrowth(t *testing.T) {
	trace := reportFixture()
	trace.Events = make([]scenario.Record, 100000)
	for i := range trace.Events {
		trace.Events[i] = scenario.Record{ID: uint64(i + 1), Kind: "message", Detail: strings.Repeat("x", 1024), PayloadSHA: strings.Repeat("0", 64)}
	}
	if _, err := RenderTarget(trace); err == nil {
		t.Fatal("oversized report emitted")
	}
}

func TestExclusiveOwnedDirectoryAndAliasParents(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "new-run")
	if err := NewDirectory(dir); err != nil {
		t.Fatal(err)
	}
	if err := NewDirectory(dir); err == nil {
		t.Fatal("reused pre-existing output directory")
	}
	assertProtectedOutput(t, dir)
	alias := filepath.Join(root, "alias")
	makeParentAlias(t, dir, alias)
	if err := NewDirectory(filepath.Join(alias, "outside")); err == nil {
		t.Fatal("followed output directory alias")
	}
}
