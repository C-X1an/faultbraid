package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func rehashTrace(t *testing.T, trace *scenario.Trace) {
	t.Helper()
	trace.Digest = ""
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	trace.Digest = hex.EncodeToString(sum[:])
}

func TestAC015ExecutableCausalReductionAndReplay(t *testing.T) {
	s := campaign.KnownBadScenario()
	s.Actions = append([]scenario.Action{{ID: "irrelevant-delay", At: 20, Kind: "delay_link", From: scenario.Node(1), To: scenario.Node(3), Ticks: scenario.Node(1)}}, s.Actions...)
	baseline := run(t, s)
	before, _ := json.Marshal(baseline)
	if baseline.Result.Status != history.Invalid {
		t.Fatal("same-target unsafe baseline did not fail")
	}
	for _, strategy := range []string{campaign.DDMin, campaign.Greedy} {
		t.Run(strategy, func(t *testing.T) {
			r, err := campaign.Reduce(context.Background(), baseline, campaign.Options{Identity: identity(t), MaxChecks: 1000, Strategy: strategy})
			if err != nil || r.Status != history.Invalid || !r.OneMinimal || !r.BaselineReplayed || r.InconclusiveCandidates != 0 || len(r.Trace.Scenario.Actions) >= len(s.Actions) || r.Accepted == 0 {
				t.Fatalf("reduction failed: %+v error=%v", r, err)
			}
			if r.Trace.Identity() != baseline.Identity() || r.Trace.Scenario.Seed != s.Seed || r.Trace.Scenario.Until() != s.Until() || *r.Trace.Failure != *baseline.Failure || r.ReplayChecks != r.Accepted+1 {
				t.Fatal("reducer changed identity, seed, horizon or fingerprint or fabricated replay accounting")
			}
			for _, step := range r.Steps {
				if !step.ReplayVerified || step.Fingerprint != baseline.Failure.Fingerprint || step.Digest == "" {
					t.Fatal("accepted candidate lacks actual replay evidence")
				}
			}
			if err := scenario.Replay(context.Background(), r.Trace, identity(t)); err != nil {
				t.Fatal(err)
			}
			// Independently execute every allowed delete-one candidate. Only
			// invalid causality is outside this declared executable transform.
			for i := range r.Trace.Scenario.Actions {
				candidate := r.Trace.Scenario
				candidate.Actions = append(append([]scenario.Action{}, candidate.Actions[:i]...), candidate.Actions[i+1:]...)
				if candidate.Until() != s.Until() {
					continue
				}
				actual, err := scenario.Run(context.Background(), candidate, scenario.Options{Identity: identity(t)})
				if errors.Is(err, scenario.ErrCausality) {
					continue
				}
				if err != nil || actual.Result.Status == history.Inconclusive {
					t.Fatalf("final pass cannot claim minimality: %v %+v", err, actual.Result)
				}
				if actual.Result.Status == history.Invalid && actual.Failure != nil && *actual.Failure == *baseline.Failure {
					t.Fatal("output has a smaller executable same-fingerprint deletion")
				}
			}
			t.Logf("AC-015 strategy=%s actions=%d->%d candidate-checks=%d executed=%d replays=%d accepted=%d causal-rejections=%d final-pass=%d digest=%s fingerprint=%s", strategy, len(s.Actions), len(r.Trace.Scenario.Actions), r.Checks, r.ExecutedChecks, r.ReplayChecks, r.Accepted, r.CausalRejections, r.FinalPassChecks, r.Trace.Digest, r.Trace.Failure.Fingerprint)
		})
	}
	after, _ := json.Marshal(baseline)
	if string(after) != string(before) {
		t.Fatal("reducer modified original evidence")
	}
}

func TestScenarioReductionUncertaintyAndBaselineValidation(t *testing.T) {
	baseline := run(t, campaign.KnownBadScenario())
	r, err := campaign.Reduce(context.Background(), baseline, campaign.Options{Identity: identity(t), MaxChecks: 1})
	if err != nil || r.Status != history.Inconclusive || r.OneMinimal || r.Checks != 1 || !strings.Contains(r.Reason, "budget") {
		t.Fatalf("budget became definitive: %+v %v", r, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = campaign.Reduce(cancelled, baseline, campaign.Options{Identity: identity(t)})
	if err != nil || r.Status != history.Inconclusive || r.OneMinimal || r.BaselineReplayed || r.Checks != 0 {
		t.Fatalf("cancelled baseline replay claimed: %+v %v", r, err)
	}
	s := campaign.KnownBadScenario()
	s.EventLimit = 1
	unknown := run(t, s)
	r, err = campaign.Reduce(context.Background(), unknown, campaign.Options{Identity: identity(t)})
	if err != nil || r.Status != history.Inconclusive || r.OneMinimal || r.BaselineReplayed || r.ReplayChecks != 0 {
		t.Fatalf("unknown baseline claimed a failure: %+v %v", r, err)
	}
	if _, err := campaign.Reduce(context.Background(), run(t, majorityScenario(0)), campaign.Options{Identity: identity(t)}); err == nil {
		t.Fatal("VALID baseline accepted")
	}
	tampered := baseline
	tampered.History.Operations = append([]history.Operation{}, baseline.History.Operations...)
	tampered.History.Operations[0].Value++
	rehashTrace(t, &tampered)
	if _, err := campaign.Reduce(context.Background(), tampered, campaign.Options{Identity: identity(t)}); err == nil {
		t.Fatal("rehashed recorded-state tampering skipped baseline target replay")
	}
	if !reflect.DeepEqual(r.Trace, unknown) {
		t.Fatal("unknown reduction changed trace")
	}
}
