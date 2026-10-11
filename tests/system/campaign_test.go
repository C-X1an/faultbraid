package system

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

func TestAC024CampaignCountCoverageAndSeedReplay(t *testing.T) {
	seen := 0
	index, err := campaign.RunSeeds(context.Background(), 0, 32, identity(t), func(entry campaign.RunEntry, trace scenario.Trace) error {
		if entry.Seed != uint64(seen) || entry.Digest != trace.Digest || trace.Identity() != identity(t) {
			t.Fatal("streamed seed identity differs")
		}
		seen++
		if trace.Result.Status == history.Invalid {
			t.Fatalf("safe campaign found a real invariant failure at seed%d: %+v", entry.Seed, trace.Failure)
		}
		if entry.Seed < 2 {
			if err := scenario.Replay(context.Background(), trace, identity(t)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || index.Status != "COMPLETE" || seen != 32 || index.Attempted != 32 || index.Unrun != 0 || index.Valid+index.Invalid+index.Inconclusive+index.Errors != 32 || index.Errors != 0 || index.CallbackErrors != 0 {
		t.Fatalf("campaign omitted outcomes: %+v %v", index, err)
	}
	if index.Coverage.PartitionDrops == 0 || index.Coverage.DropFaultDrops == 0 || index.Coverage.DuplicateEffects == 0 || index.Coverage.DelayedDeliveries == 0 || index.Coverage.Restarts == 0 || index.Operations.CompletedWrites == 0 || index.Operations.CompletedReads == 0 {
		t.Fatalf("actual workload effects missing: %+v", index)
	}
	for _, entry := range index.Entries {
		if entry.Digest == "" || entry.WorkloadChoices == nil || entry.Coverage.Events == 0 || entry.Horizon != 94 {
			t.Fatal("campaign evidence incomplete")
		}
	}
	t.Logf("AC-024 bounded author smoke campaign requested=%d attempted=%d valid=%d invalid=%d inconclusive=%d errors=%d observed-events=%d completed-writes=%d completed-reads=%d barrier-crashes=%d; full1000 campaign is a separate measured run", index.Requested, index.Attempted, index.Valid, index.Invalid, index.Inconclusive, index.Errors, index.Coverage.Events, index.Operations.CompletedWrites, index.Operations.CompletedReads, index.Coverage.BarrierCrashes)
	baseline := campaign.KnownBadScenario()
	for repeat := 0; repeat < 10; repeat++ {
		trace := run(t, baseline)
		if trace.Result.Status != history.Invalid {
			t.Fatal("selected actual failure vanished")
		}
		if err := scenario.Replay(context.Background(), trace, identity(t)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCampaignCancellationCallbackErrorAndBounds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	index, err := campaign.RunSeeds(ctx, 0, 4, identity(t), nil)
	if err != nil || index.Status != "INCONCLUSIVE" || index.Attempted != 0 || index.Unrun != 4 || len(index.Entries) != 0 {
		t.Fatalf("cancelled seeds fabricated: %+v %v", index, err)
	}
	sentinel := errors.New("owned synthetic callback failure")
	index, err = campaign.RunSeeds(context.Background(), 0, 4, identity(t), func(entry campaign.RunEntry, trace scenario.Trace) error { return sentinel })
	if !errors.Is(err, sentinel) || index.Status != "FAILED" || index.Attempted != 1 || index.Unrun != 3 || index.CallbackErrors != 1 || len(index.Entries) != 1 {
		t.Fatalf("callback failure omitted: %+v %v", index, err)
	}
	for _, input := range []struct {
		start uint64
		count int
	}{{0, 0}, {0, 1001}, {math.MaxUint64, 2}} {
		if _, err := campaign.RunSeeds(context.Background(), input.start, input.count, identity(t), nil); err == nil {
			t.Fatal("unbounded/overflow seed range accepted")
		}
	}
	if !reflect.DeepEqual(campaign.BaselineScenario(17), campaign.BaselineScenario(17)) {
		t.Fatal("workload entropy not deterministic")
	}
}
