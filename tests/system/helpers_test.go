package system

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

var sourceOnce sync.Once
var sourceIdentity buildinfo.Identity
var sourceError error

func identity(t *testing.T) buildinfo.Identity {
	t.Helper()
	sourceOnce.Do(func() {
		root, err := filepath.Abs("../..")
		if err != nil {
			sourceError = err
			return
		}
		sourceIdentity, sourceError = buildinfo.FromFiles(root)
	})
	if sourceError != nil {
		t.Fatal(sourceError)
	}
	return sourceIdentity
}

func run(t *testing.T, s scenario.Scenario) scenario.Trace {
	t.Helper()
	trace, err := scenario.Run(context.Background(), s, scenario.Options{Identity: identity(t)})
	if err != nil {
		t.Fatal(err)
	}
	return trace
}

func operation(t *testing.T, trace scenario.Trace, id string) history.Operation {
	t.Helper()
	for _, op := range trace.History.Operations {
		if op.ID == id {
			return op
		}
	}
	t.Fatalf("operation %s absent", id)
	return history.Operation{}
}

func invokeAll(s *scenario.Scenario, at uint64, kind, id string, value int64) {
	for node := uint64(1); node <= 3; node++ {
		a := scenario.Action{ID: id + ":" + string(rune('0'+node)), At: at + node - 1, Kind: kind, Node: scenario.Node(node), RequestID: scenario.Text(id)}
		if kind == "invoke_write" {
			a.Value = scenario.Value(value)
		}
		s.Actions = append(s.Actions, a)
	}
}

func majorityScenario(seed uint64) scenario.Scenario {
	s := scenario.Default(seed)
	invokeAll(&s, 30, "invoke_write", "write7", 7)
	invokeAll(&s, 45, "invoke_read", "read7", 0)
	s.EndTime = scenario.Node(60)
	return s
}

// Discovered by a retained finite real-target experiment: seed0 elected voter2,
// and its CheckQuorum window overlaps the majority's new election briefly.
func staleReadScenario(unsafe bool) scenario.Scenario {
	s := scenario.Default(0)
	invokeAll(&s, 30, "invoke_write", "write7", 7)
	for _, node := range []uint64{1, 3} {
		s.Actions = append(s.Actions, scenario.Action{ID: fmt.Sprintf("partition-%d-out", node), At: 40, Kind: "partition", From: scenario.Node(2), To: scenario.Node(node)}, scenario.Action{ID: fmt.Sprintf("partition-%d-in", node), At: 40, Kind: "partition", From: scenario.Node(node), To: scenario.Node(2)})
	}
	invokeAll(&s, 52, "invoke_write", "write9", 9)
	s.Actions = append(s.Actions, scenario.Action{ID: "stale", At: 57, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("stale")})
	s.EndTime = scenario.Node(64)
	if unsafe {
		s.Variant = scenario.Text("unsafe-local-read")
	}
	return s
}
