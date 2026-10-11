package campaign

import (
	"fmt"
	"sort"

	"github.com/C-X1an/faultbraid/internal/raftadapter"
	"github.com/C-X1an/faultbraid/internal/scenario"
	"github.com/C-X1an/faultbraid/internal/target"
)

const WorkloadVersion = "splitmix64-v1/fault-workload-v2"

type WorkloadChoice struct {
	Stream   string `json:"stream"`
	Sequence uint64 `json:"sequence"`
	Bound    uint64 `json:"bound"`
	Value    uint64 `json:"value"`
}

type workloadEntropy struct {
	prng    *raftadapter.SplitMix64
	choices []WorkloadChoice
}

func (w *workloadEntropy) draw(stream string, bound uint64) uint64 {
	// Rejection matches the pinned SplitMix64 bounded-choice convention. This
	// generator is independent of all node election streams.
	threshold := -bound % bound
	for {
		value := w.prng.Uint64()
		if value < threshold {
			continue
		}
		value %= bound
		w.choices = append(w.choices, WorkloadChoice{stream, uint64(len(w.choices)), bound, value})
		return value
	}
}

func invokeAll(s *scenario.Scenario, at uint64, kind, id string, value int64) {
	for node := uint64(1); node <= 3; node++ {
		a := scenario.Action{ID: fmt.Sprintf("%s-%d-%d", id, at, node), At: at + node - 1, Kind: kind, Node: scenario.Node(node), RequestID: scenario.Text(id)}
		if kind == "invoke_write" {
			a.Value = scenario.Value(value)
		}
		s.Actions = append(s.Actions, a)
	}
}

// KnownBadScenario is a genuine unsafe-local-read integration defect. Seed 0
// elects voter 2; its brief CheckQuorum window overlaps a majority election.
// The minority returns 7 after a majority has acknowledged write(9).
func KnownBadScenario() scenario.Scenario {
	s := scenario.Default(0)
	s.Variant = scenario.Text(string(target.UnsafeLocalRead))
	invokeAll(&s, 30, "invoke_write", "write7", 7)
	for _, node := range []uint64{1, 3} {
		s.Actions = append(s.Actions,
			scenario.Action{ID: fmt.Sprintf("partition-%d-out", node), At: 40, Kind: "partition", From: scenario.Node(2), To: scenario.Node(node)},
			scenario.Action{ID: fmt.Sprintf("partition-%d-in", node), At: 40, Kind: "partition", From: scenario.Node(node), To: scenario.Node(2)},
		)
	}
	invokeAll(&s, 52, "invoke_write", "write9", 9)
	s.Actions = append(s.Actions, scenario.Action{ID: "stale", At: 57, Kind: "invoke_read", Node: scenario.Node(2), RequestID: scenario.Text("stale")})
	s.EndTime = scenario.Node(64)
	return s
}

// BaselineScenario supplies a bounded safe-target workload, not a proof of
// arbitrary faults. Recorded choices vary nodes, link policies and crash types;
// each scenario includes retries and a majority/minority interval. Explicit
// action-crash profiles recover. Barrier profiles do not presume that an armed
// contingent fault has fired; actual trigger coverage is recorded separately.
func BaselineScenario(seed uint64) scenario.Scenario {
	s, _ := generatedScenario(seed)
	return s
}

var boundaries = []target.Barrier{
	target.BeforeAppend, target.AfterStageBeforeFlush, target.AfterFlushBeforeSend,
	target.AfterSendBeforeApply, target.AfterApplyBeforeResponse, target.AfterResponse,
}

func generatedScenario(seed uint64) (scenario.Scenario, []WorkloadChoice) {
	w := workloadEntropy{prng: raftadapter.NewSplitMix64(seed ^ 0xd1b54a32d192ed03), choices: []WorkloadChoice{}}
	node := 1 + w.draw("workload.crash-node", 3)
	from := 1 + w.draw("workload.link-from", 3)
	to := 1 + (from-1+1+w.draw("workload.link-offset", 2))%3
	delay := 1 + w.draw("workload.delay", 4)
	drops := 1 + w.draw("workload.drop-count", 3)
	duplicates := 1 + w.draw("workload.duplicate-count", 2)
	barrier := boundaries[w.draw("workload.crash-boundary", uint64(len(boundaries)))]
	barrierMode := w.draw("workload.crash-mode", 2) == 1
	s := scenario.Default(seed)
	s.EndTime = scenario.Node(94)
	invokeAll(&s, 30, "invoke_write", "first-write", 7)
	invokeAll(&s, 37, "invoke_read", "first-read", 0)
	s.Actions = append(s.Actions,
		scenario.Action{ID: "delay", At: 40, Kind: "delay_link", From: scenario.Node(from), To: scenario.Node(to), Ticks: scenario.Node(delay)},
		scenario.Action{ID: "drop", At: 40, Kind: "drop_next", From: scenario.Node(from), To: scenario.Node(to), Count: scenario.Count(int(drops))},
		scenario.Action{ID: "duplicate", At: 40, Kind: "duplicate_next", From: scenario.Node(from), To: scenario.Node(to), Count: scenario.Count(int(duplicates))},
	)
	crash := scenario.Action{ID: "crash", At: 42, Kind: "crash", Node: scenario.Node(node)}
	if barrierMode {
		crash.Kind, crash.Boundary, crash.Occurrence = "crash_at", scenario.Text(string(barrier)), scenario.Count(1)
	}
	s.Actions = append(s.Actions, crash)
	if barrierMode {
		// Retrying a durably replicated identity exercises response barriers on
		// followers as well as leaders. Other boundaries fire on normal Ready.
		s.Actions = append(s.Actions, scenario.Action{ID: "barrier-retry", At: 43, Kind: "invoke_write", Node: scenario.Node(node), RequestID: scenario.Text("first-write"), Value: scenario.Value(7)})
	}
	if !barrierMode {
		s.Actions = append(s.Actions, scenario.Action{ID: "restart", At: 54, Kind: "restart", Node: scenario.Node(node)})
	}
	for peer := uint64(1); peer <= 3; peer++ {
		if peer == node {
			continue
		}
		s.Actions = append(s.Actions,
			scenario.Action{ID: fmt.Sprintf("partition-%d-out", peer), At: 60, Kind: "partition", From: scenario.Node(node), To: scenario.Node(peer)},
			scenario.Action{ID: fmt.Sprintf("partition-%d-in", peer), At: 60, Kind: "partition", From: scenario.Node(peer), To: scenario.Node(node)},
			scenario.Action{ID: fmt.Sprintf("heal-%d-out", peer), At: 80, Kind: "heal", From: scenario.Node(node), To: scenario.Node(peer)},
			scenario.Action{ID: fmt.Sprintf("heal-%d-in", peer), At: 80, Kind: "heal", From: scenario.Node(peer), To: scenario.Node(node)},
		)
	}
	invokeAll(&s, 65, "invoke_write", "second-write", 9)
	invokeAll(&s, 73, "invoke_read", "partition-read", 0)
	s.Actions = append(s.Actions, scenario.Action{ID: "restore-delay", At: 80, Kind: "delay_link", From: scenario.Node(from), To: scenario.Node(to), Ticks: scenario.Node(1)})
	invokeAll(&s, 81, "invoke_write", "second-write", 9)
	invokeAll(&s, 85, "invoke_read", "final-read", 0)
	// Stable sorting preserves the chosen insertion order for tied actions.
	sort.SliceStable(s.Actions, func(i, j int) bool { return s.Actions[i].At < s.Actions[j].At })
	return s, w.choices
}
