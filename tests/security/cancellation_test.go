package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

// Controlled cancellation fixture; it runs the actual target and cancels a real
// context. Snapshotting Err before cancellation models cancellation immediately
// after a legitimate nil observation. No target/history/entropy result is mocked.
type cancelAfterPoll struct {
	context.Context
	calls  atomic.Int64
	at     int64
	cancel context.CancelFunc
}

func (c *cancelAfterPoll) Err() error {
	before := c.Context.Err()
	if c.calls.Add(1) == c.at {
		c.cancel()
	}
	return before
}

func executedCancellationTrace(t *testing.T) (scenario.Trace, buildinfo.Identity) {
	t.Helper()
	identity, err := buildinfo.FromFiles("../..")
	if err != nil {
		t.Fatal(err)
	}
	trace, err := scenario.Run(context.Background(), campaign.KnownBadScenario(), scenario.Options{Identity: identity})
	if err != nil || trace.Result.Status != history.Invalid || trace.Failure == nil {
		t.Fatalf("actual diagnostic defect not established: verdict=%s err=%v", trace.Result.Status, err)
	}
	return trace, identity
}

func TestReplayRejectsCancellationBeforeAndDuringValidation(t *testing.T) {
	saved, identity := executedCancellationTrace(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := scenario.Replay(ctx, saved, identity); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled replay must retain cancellation classification: %v", err)
	}
	// 570 is the measured legacy finalization checkpoint for this fixed real
	// scenario. Freeze that schedule: recalibrating it to a repaired terminal
	// guard would instead test cancellation after its legitimate observation.
	// 576 additionally pins the repaired after-fresh-run observation: cancel
	// there before the final comparison, and the sole terminal guard must reject.
	for _, checkpoint := range []int64{200, 570, 576} {
		t.Run(strconv.FormatInt(checkpoint, 10), func(t *testing.T) {
			base, stop := context.WithCancel(context.Background())
			defer stop()
			controlled := &cancelAfterPoll{Context: base, at: checkpoint, cancel: stop}
			err := scenario.Replay(controlled, saved, identity)
			if controlled.calls.Load() < checkpoint || base.Err() != context.Canceled {
				t.Fatal("controlled cancellation schedule was not reached")
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled replay returned a semantic answer: %v", err)
			}
		})
	}
}

func TestRunTerminalCancellationPreservesUnknownTraceAndDigest(t *testing.T) {
	baseline, identity := executedCancellationTrace(t)
	base, stop := context.WithCancel(context.Background())
	defer stop()
	// The original finalization window was reached after 285 context polls.
	controlled := &cancelAfterPoll{Context: base, at: 285, cancel: stop}
	trace, err := scenario.Run(controlled, baseline.Scenario, scenario.Options{Identity: identity})
	if base.Err() != context.Canceled || controlled.calls.Load() < 285 {
		t.Fatal("terminal cancellation schedule was not reached")
	}
	if err != nil || trace.Result.Status != history.Inconclusive || trace.Failure != nil || len(trace.Result.Witness) != 0 {
		t.Fatalf("cancelled simulation changed unknown into a semantic/input result: status=%s failure=%v err=%v", trace.Result.Status, trace.Failure, err)
	}
	if trace.Result.States != baseline.Result.States {
		t.Fatal("terminal cancellation erased the actually observed checker-state count")
	}
	if trace.Identity() != identity || !reflect.DeepEqual(trace.History, baseline.History) || !reflect.DeepEqual(trace.Events, baseline.Events) || !reflect.DeepEqual(trace.Choices, baseline.Choices) {
		t.Fatal("terminal cancellation discarded actual target observations")
	}
	claimed := trace.Digest
	trace.Digest = ""
	data, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if claimed != hex.EncodeToString(hash[:]) {
		t.Fatal("unknown verdict was not resealed into the canonical trace digest")
	}
}

func TestCancellationAfterReplayReturnDoesNotInvalidateCompletedWork(t *testing.T) {
	saved, identity := executedCancellationTrace(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scenario.Replay(ctx, saved, identity); err != nil || ctx.Err() != nil {
		t.Fatalf("actual replay did not complete before cancellation: %v", err)
	}
	cancel() // Ordinary context; strictly after the successful return.
	if ctx.Err() != context.Canceled || saved.Identity() != identity || saved.Result.Status != history.Invalid {
		t.Fatal("later cancellation altered the already completed replay or source identity")
	}
}
