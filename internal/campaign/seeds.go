package campaign

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

const MaxSeeds = 1000

type BoundaryCount struct {
	Boundary string `json:"boundary"`
	Count    int    `json:"count"`
}

// Coverage counts observed effects, not merely configured faults. Delayed
// deliveries are message transitions later than one tick after their cause.
// This is finite model coverage; it is not exhaustive correctness evidence.
type Coverage struct {
	Events            int             `json:"events"`
	DelayedDeliveries int             `json:"delayed_deliveries"`
	DropFaultDrops    int             `json:"drop_fault_drops"`
	PartitionDrops    int             `json:"partition_drops"`
	IncarnationDrops  int             `json:"incarnation_or_down_drops"`
	DuplicateEffects  int             `json:"duplicate_effects"`
	ActionCrashes     int             `json:"action_crashes"`
	BarrierCrashes    int             `json:"barrier_crashes"`
	Restarts          int             `json:"restarts"`
	TriggeredBarriers []BoundaryCount `json:"triggered_barriers"`
}

type Operations struct {
	Total           int `json:"total"`
	CompletedWrites int `json:"completed_writes"`
	CompletedReads  int `json:"completed_reads"`
	PendingWrites   int `json:"pending_writes"`
	PendingReads    int `json:"pending_reads"`
}

type RunEntry struct {
	Seed            uint64            `json:"seed"`
	Status          string            `json:"status"`
	Reason          string            `json:"reason"`
	Error           string            `json:"error"`
	Digest          string            `json:"digest"`
	Failure         *scenario.Failure `json:"failure"`
	Variant         string            `json:"target_variant"`
	Horizon         uint64            `json:"declared_horizon"`
	ObservedUntil   uint64            `json:"observed_until"`
	Actions         int               `json:"actions"`
	Operations      Operations        `json:"operations"`
	Coverage        Coverage          `json:"coverage"`
	WorkloadChoices []WorkloadChoice  `json:"workload_choices"`
}

type Index struct {
	Version         int                `json:"version"`
	Model           string             `json:"model"`
	Identity        buildinfo.Identity `json:"identity"`
	WorkloadVersion string             `json:"workload_version"`
	SeedStart       uint64             `json:"seed_start"`
	Requested       int                `json:"requested"`
	Attempted       int                `json:"attempted"`
	Unrun           int                `json:"unrun"`
	Valid           int                `json:"valid"`
	Invalid         int                `json:"invalid"`
	Inconclusive    int                `json:"inconclusive"`
	Errors          int                `json:"errors"`
	CallbackErrors  int                `json:"callback_errors"`
	Status          string             `json:"status"`
	Reason          string             `json:"reason"`
	Coverage        Coverage           `json:"coverage"`
	Operations      Operations         `json:"operations"`
	Entries         []RunEntry         `json:"entries"`
}

// RunSeeds executes each seed once, keeping only bounded summaries. The
// callback can stream the full trace into exclusive output files. Each attempt
// is indexed even when execution or that callback fails. A callback error stops
// immediately and is returned with its cause intact; remaining seeds are Unrun.
// Execution errors are indexed and independent seeds continue, then a non-nil
// aggregate error prevents callers treating an incomplete experiment as success.
func RunSeeds(ctx context.Context, start uint64, count int, identity buildinfo.Identity, onRun func(RunEntry, scenario.Trace) error) (Index, error) {
	if ctx == nil || count < 1 || count > MaxSeeds || uint64(count-1) > math.MaxUint64-start {
		return Index{}, errors.New("invalid campaign context or seed range")
	}
	if identity == (buildinfo.Identity{}) {
		identity = buildinfo.Current()
	}
	if !identity.Identified() {
		return Index{}, errors.New("identified build required for seed campaign")
	}
	index := Index{Version: 1, Model: "raft-campaign-v1", Identity: identity, WorkloadVersion: WorkloadVersion,
		SeedStart: start, Requested: count, Unrun: count, Status: "INCONCLUSIVE", Entries: []RunEntry{}, Coverage: emptyCoverage()}
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			index.Reason = "cancelled campaign; remaining seeds not run"
			return index, nil
		}
		seed := start + uint64(i)
		s, choices := generatedScenario(seed)
		trace, err := scenario.Run(ctx, s, scenario.Options{Identity: identity})
		entry := summarize(seed, s, choices, trace, err)
		index.Attempted++
		index.Unrun--
		switch entry.Status {
		case string(history.Valid):
			index.Valid++
		case string(history.Invalid):
			index.Invalid++
		case string(history.Inconclusive):
			index.Inconclusive++
		default:
			index.Errors++
		}
		addCoverage(&index.Coverage, entry.Coverage)
		addOperations(&index.Operations, entry.Operations)
		index.Entries = append(index.Entries, entry)
		if onRun != nil {
			// Slice fields in the callback's summary cannot mutate the index.
			callbackEntry := entry
			callbackEntry.WorkloadChoices = append([]WorkloadChoice{}, entry.WorkloadChoices...)
			callbackEntry.Coverage.TriggeredBarriers = append([]BoundaryCount{}, entry.Coverage.TriggeredBarriers...)
			if entry.Failure != nil {
				failure := *entry.Failure
				callbackEntry.Failure = &failure
			}
			if err := onRun(callbackEntry, trace); err != nil {
				index.CallbackErrors++
				index.Status, index.Reason = "FAILED", "trace callback failed; remaining seeds not run"
				return index, fmt.Errorf("seed %d callback: %w", seed, err)
			}
		}
	}
	if index.Errors != 0 {
		index.Status, index.Reason = "FAILED", "one or more target executions failed"
		return index, fmt.Errorf("%d target execution error(s)", index.Errors)
	}
	index.Status, index.Reason = "COMPLETE", "all requested seeds counted; coverage is finite and schedule-specific"
	return index, nil
}

func emptyCoverage() Coverage {
	c := Coverage{TriggeredBarriers: make([]BoundaryCount, len(boundaries))}
	for i, boundary := range boundaries {
		c.TriggeredBarriers[i].Boundary = string(boundary)
	}
	return c
}

func summarize(seed uint64, s scenario.Scenario, choices []WorkloadChoice, trace scenario.Trace, err error) RunEntry {
	entry := RunEntry{Seed: seed, Status: string(trace.Result.Status), Reason: trace.Result.Reason,
		Digest: trace.Digest, Variant: string(s.TargetVariant()), Horizon: s.Until(), Actions: len(s.Actions),
		Coverage: emptyCoverage(), WorkloadChoices: choices}
	if err != nil {
		entry.Status = "ERROR"
		entry.Error = err.Error()
		if len(entry.Error) > 512 {
			entry.Error = entry.Error[:512]
		}
	}
	if trace.Failure != nil {
		failure := *trace.Failure
		entry.Failure = &failure
	}
	for _, op := range trace.History.Operations {
		entry.Operations.Total++
		if op.Kind == "write" {
			if op.Return == nil {
				entry.Operations.PendingWrites++
			} else {
				entry.Operations.CompletedWrites++
			}
		} else if op.Return == nil {
			entry.Operations.PendingReads++
		} else {
			entry.Operations.CompletedReads++
		}
	}
	entry.Coverage.Events = len(trace.Events)
	for _, event := range trace.Events {
		entry.ObservedUntil = max(entry.ObservedUntil, event.At)
		switch event.Kind {
		case "drop":
			switch event.Detail {
			case "drop-next":
				entry.Coverage.DropFaultDrops++
			case "partition":
				entry.Coverage.PartitionDrops++
			case "incarnation-or-down":
				entry.Coverage.IncarnationDrops++
			}
		case "duplicate":
			entry.Coverage.DuplicateEffects++
		case "message":
			if parent, ok := parentRecord(trace, event.Parent); ok && event.At > parent.At && event.At-parent.At > 1 {
				entry.Coverage.DelayedDeliveries++
			}
		case "start":
			if event.Incarnation > 1 {
				entry.Coverage.Restarts++
			}
		case "crash":
			if event.Detail == "action" {
				entry.Coverage.ActionCrashes++
			} else if event.Detail == "barrier" {
				entry.Coverage.BarrierCrashes++
				if parent, ok := parentRecord(trace, event.Parent); ok {
					for i, boundary := range boundaries {
						if strings.HasSuffix(parent.Detail, string(boundary)) {
							entry.Coverage.TriggeredBarriers[i].Count++
							break
						}
					}
				}
			}
		}
	}
	return entry
}

func parentRecord(trace scenario.Trace, id uint64) (scenario.Record, bool) {
	if id == 0 || id > uint64(len(trace.Events)) || trace.Events[id-1].ID != id {
		return scenario.Record{}, false
	}
	return trace.Events[id-1], true
}

func addCoverage(dst *Coverage, src Coverage) {
	dst.Events += src.Events
	dst.DelayedDeliveries += src.DelayedDeliveries
	dst.DropFaultDrops += src.DropFaultDrops
	dst.PartitionDrops += src.PartitionDrops
	dst.IncarnationDrops += src.IncarnationDrops
	dst.DuplicateEffects += src.DuplicateEffects
	dst.ActionCrashes += src.ActionCrashes
	dst.BarrierCrashes += src.BarrierCrashes
	dst.Restarts += src.Restarts
	for i := range dst.TriggeredBarriers {
		dst.TriggeredBarriers[i].Count += src.TriggeredBarriers[i].Count
	}
}

func addOperations(dst *Operations, src Operations) {
	dst.Total += src.Total
	dst.CompletedWrites += src.CompletedWrites
	dst.CompletedReads += src.CompletedReads
	dst.PendingWrites += src.PendingWrites
	dst.PendingReads += src.PendingReads
}
