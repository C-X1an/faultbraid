package system

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func independentTraceClone(t *testing.T, input scenario.Trace) scenario.Trace {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output scenario.Trace
	if err := json.Unmarshal(b, &output); err != nil {
		t.Fatal(err)
	}
	return output
}

func TestIndependentReducerFixedImplicitHorizon(t *testing.T) {
	s := campaign.KnownBadScenario()
	s.EndTime = nil
	s.Actions = append(s.Actions, scenario.Action{ID: "final-domain-anchor", At: 58, Kind: "delay_link", From: scenario.Node(1), To: scenario.Node(3), Ticks: scenario.Node(1)})
	baseline := run(t, s)
	if baseline.Result.Status != history.Invalid {
		t.Fatal("real unsafe fixture did not fail")
	}
	r, err := campaign.Reduce(context.Background(), baseline, campaign.Options{Identity: identity(t), MaxChecks: 1000})
	if err != nil || !r.OneMinimal || r.Status != history.Invalid || r.CausalRejections == 0 || r.Trace.Scenario.EndTime != nil || r.Trace.Scenario.Until() != s.Until() {
		t.Fatalf("implicit fixed domain changed: %+v err=%v", r, err)
	}
	last := r.Trace.Scenario.Actions[len(r.Trace.Scenario.Actions)-1]
	if last.ID != "final-domain-anchor" {
		t.Fatal("deleting the anchor silently changed the horizon")
	}
	for i := range r.Trace.Scenario.Actions {
		candidate := r.Trace.Scenario
		candidate.Actions = append(append([]scenario.Action{}, candidate.Actions[:i]...), candidate.Actions[i+1:]...)
		if candidate.Until() != s.Until() {
			continue
		}
		actual, err := scenario.Run(context.Background(), candidate, scenario.Options{Identity: identity(t)})
		if err != nil {
			t.Fatal(err)
		}
		if actual.Result.Status == history.Inconclusive || actual.Result.Status == history.Invalid && actual.Failure != nil && *actual.Failure == *baseline.Failure {
			t.Fatalf("1-minimal fixed domain claim fails at action%d: %+v", i, actual.Result)
		}
	}
	t.Logf("independent implicit horizon=%d actions=%d->%d causal/domain rejections=%d", s.Until(), len(s.Actions), len(r.Trace.Scenario.Actions), r.CausalRejections)
}

func TestIndependentReducerRejectsRehashedIdentityHeaderFingerprint(t *testing.T) {
	baseline := run(t, campaign.KnownBadScenario())
	mutations := []struct {
		name   string
		mutate func(*scenario.Trace)
	}{
		{"source", func(v *scenario.Trace) { v.Header.SourceSHA = strings.Repeat("a", 64) }},
		{"seed", func(v *scenario.Trace) { v.Scenario.Seed++; v.Header.Seed++ }},
		{"scheduler", func(v *scenario.Trace) { v.Header.Scheduler += "-other" }},
		{"fingerprint", func(v *scenario.Trace) { v.Failure.Fingerprint = strings.Repeat("b", 64) }},
		{"unknown-with-failure", func(v *scenario.Trace) { v.Result.Status = history.Inconclusive }},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			v := independentTraceClone(t, baseline)
			m.mutate(&v)
			rehashTrace(t, &v)
			if _, err := campaign.Reduce(context.Background(), v, campaign.Options{Identity: identity(t)}); err == nil {
				t.Fatal("rehashed inconsistent baseline admitted")
			}
		})
	}
}

func TestIndependentCampaignSummaryOwnershipAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var expectedChoice uint64
	var expectedBarrier int
	index, err := campaign.RunSeeds(ctx, 17, 4, identity(t), func(entry campaign.RunEntry, trace scenario.Trace) error {
		expectedChoice = entry.WorkloadChoices[0].Value
		expectedBarrier = entry.Coverage.TriggeredBarriers[0].Count
		entry.WorkloadChoices[0].Value++
		entry.Coverage.TriggeredBarriers[0].Count += 1000
		cancel()
		return nil
	})
	if err != nil || index.Status != "INCONCLUSIVE" || index.Attempted != 1 || index.Unrun != 3 || len(index.Entries) != 1 || index.Valid+index.Invalid+index.Inconclusive+index.Errors != 1 {
		t.Fatalf("mid-campaign cancellation miscounted: %+v err=%v", index, err)
	}
	if index.Entries[0].WorkloadChoices[0].Value != expectedChoice || index.Entries[0].Coverage.TriggeredBarriers[0].Count != expectedBarrier {
		t.Fatal("callback changed retained evidence")
	}
}

func TestIndependentReducerActualUnknownCandidate(t *testing.T) {
	for budget := 1; budget <= 5; budget++ {
		s := campaign.KnownBadScenario()
		// Removing a critical partition can reject the first read before
		// admission. Retrying it on the majority then retains a completed legal
		// read, whose witness can exceed the failing baseline's state budget.
		s.Actions = append(s.Actions,
			scenario.Action{ID: "retry-stale-majority-1", At: 58, Kind: "invoke_read", Node: scenario.Node(1), RequestID: scenario.Text("stale")},
			scenario.Action{ID: "retry-stale-majority-3", At: 59, Kind: "invoke_read", Node: scenario.Node(3), RequestID: scenario.Text("stale")},
		)
		s.CheckerStateLimit = budget
		baseline := run(t, s)
		if baseline.Result.Status != history.Invalid {
			continue
		}
		r, err := campaign.Reduce(context.Background(), baseline, campaign.Options{Identity: identity(t), MaxChecks: 1000})
		if err != nil {
			t.Fatal(err)
		}
		if r.InconclusiveCandidates == 0 {
			continue
		}
		if r.Status != history.Inconclusive || r.OneMinimal || !r.BaselineReplayed || r.Trace.Result.Status != history.Invalid {
			t.Fatalf("unknown candidate became definitive: %+v", r)
		}
		t.Logf("actual target unknown-candidate fixture checker-budget=%d baseline-states=%d unknown-candidates=%d retained-actions=%d minimal=%v", budget, baseline.Result.States, r.InconclusiveCandidates, len(r.Trace.Scenario.Actions), r.OneMinimal)
		return
	}
	t.Fatal("finite review search found no actual UNKNOWN candidate; this path is not established by this probe")
}
