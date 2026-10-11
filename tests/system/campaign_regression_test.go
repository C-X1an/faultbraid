package system

import (
	"context"
	"testing"

	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func TestCampaignDoesNotPresumeArmedResponseCrashFired(t *testing.T) {
	// Discovered in the first actual1000-seed run, not specially handled by the
	// generator. In v1 first-write produced no response before restart54, so
	// this attempted to restart a live node. All original errors are retained.
	for _, seed := range []uint64{365, 854} {
		s := campaign.BaselineScenario(seed)
		armed := false
		for _, action := range s.Actions {
			if action.Kind == "crash_at" {
				armed = true
			}
			if action.Kind == "restart" {
				t.Fatal("contingent barrier profile still assumes its node died")
			}
		}
		if !armed {
			t.Fatal("regression removed the named barrier fault")
		}
		trace, err := scenario.Run(context.Background(), s, scenario.Options{Identity: identity(t)})
		if err != nil || trace.Result.Status != history.Valid {
			t.Fatalf("seed%d does not execute safely: %v %+v", seed, err, trace.Result)
		}
		if err := scenario.Replay(context.Background(), trace, identity(t)); err != nil {
			t.Fatal(err)
		}
		t.Logf("regression seed%d actual events%d digest%s", seed, len(trace.Events), trace.Digest)
	}
}
