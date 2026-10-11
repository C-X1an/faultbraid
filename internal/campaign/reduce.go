// Package campaign executes bounded target experiments and executable reduction.
package campaign

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

const (
	DDMin          = "ddmin"
	Greedy         = "greedy"
	ActionDeletion = "fixed-horizon-action-deletion-v1"
	MaxChecks      = 100000
)

type Options struct {
	Identity  buildinfo.Identity
	MaxChecks int
	Strategy  string
}

// AcceptedStep identifies an executed and replayed smaller scenario. It is not
// a projection of the original history: its digest belongs to a new execution.
type AcceptedStep struct {
	Check          int      `json:"check"`
	Actions        int      `json:"actions"`
	Removed        []string `json:"removed_action_ids"`
	Digest         string   `json:"digest"`
	Fingerprint    string   `json:"fingerprint"`
	ReplayVerified bool     `json:"replay_verified"`
}

type Reduction struct {
	Trace                  scenario.Trace  `json:"trace"`
	Status                 history.Verdict `json:"status"`
	Reason                 string          `json:"reason"`
	Strategy               string          `json:"strategy"`
	Transform              string          `json:"transform"`
	OriginalActions        int             `json:"original_actions"`
	Checks                 int             `json:"checks"`
	ExecutedChecks         int             `json:"executed_checks"`
	ReplayChecks           int             `json:"replay_checks"`
	BaselineReplayed       bool            `json:"baseline_replayed"`
	FinalPassChecks        int             `json:"final_pass_checks"`
	OneMinimal             bool            `json:"one_minimal"`
	ValidCandidates        int             `json:"valid_candidates"`
	InvalidCandidates      int             `json:"invalid_candidates"`
	InconclusiveCandidates int             `json:"inconclusive_candidates"`
	CausalRejections       int             `json:"causal_rejections"`
	FingerprintRejections  int             `json:"fingerprint_rejections"`
	Errors                 int             `json:"errors"`
	Accepted               int             `json:"accepted"`
	Steps                  []AcceptedStep  `json:"accepted_steps"`
}

// Reduce preserves source, locks, seed, limits, variant and the original
// resolved horizon. The only transform removes actions. Checks bounds all
// candidate evaluations, including causal rejection before target execution;
// ExecutedChecks counts actual scenario.Run calls. Baseline replay is separate.
// A complete final delete-one pass proves only 1-minimality in this executable
// deletion domain. Any unknown candidate conservatively clears that claim.
func Reduce(ctx context.Context, original scenario.Trace, opts Options) (Reduction, error) {
	if ctx == nil {
		return Reduction{}, errors.New("reducer requires a context")
	}
	if opts.Identity == (buildinfo.Identity{}) {
		opts.Identity = buildinfo.Current()
	}
	if opts.MaxChecks == 0 {
		opts.MaxChecks = 1000
	}
	if opts.Strategy == "" {
		opts.Strategy = DDMin
	}
	if opts.MaxChecks < 1 || opts.MaxChecks > MaxChecks || opts.Strategy != DDMin && opts.Strategy != Greedy {
		return Reduction{}, errors.New("invalid reducer budget or strategy")
	}
	if err := validateBaseline(original, opts.Identity); err != nil {
		return Reduction{}, err
	}
	// Own the input slices and pointers, so reduction cannot mutate its evidence.
	data, err := json.Marshal(original)
	if err != nil {
		return Reduction{}, err
	}
	var owned scenario.Trace
	if err := json.Unmarshal(data, &owned); err != nil {
		return Reduction{}, err
	}
	r := &reducer{ctx: ctx, opts: opts, horizon: original.Scenario.Until(), result: Reduction{
		Trace: owned, Status: history.Inconclusive, Strategy: opts.Strategy,
		Transform: ActionDeletion, OriginalActions: len(original.Scenario.Actions), Steps: []AcceptedStep{},
	}}
	if original.Result.Status == history.Valid {
		return r.result, errors.New("scenario reduction requires an INVALID baseline")
	}
	if original.Result.Status == history.Inconclusive {
		r.result.Reason = "baseline is INCONCLUSIVE; no reduction or baseline replay claimed"
		return r.result, nil
	}
	if ctx.Err() != nil {
		r.result.Reason = "cancelled before baseline replay"
		return r.result, nil
	}
	r.result.ReplayChecks++
	if err := scenario.Replay(ctx, owned, opts.Identity); err != nil {
		if ctx.Err() != nil {
			r.result.Reason = "cancelled during baseline replay"
			return r.result, nil
		}
		return r.result, fmt.Errorf("baseline replay: %w", err)
	}
	r.result.BaselineReplayed = true
	if opts.Strategy == Greedy {
		err = r.greedy()
	} else {
		err = r.ddmin()
	}
	if err != nil {
		return r.result, err
	}
	if !r.stopped {
		err = r.finalPass()
	}
	if err != nil {
		return r.result, err
	}
	if r.stopped {
		return r.result, nil
	}
	if r.result.InconclusiveCandidates != 0 {
		r.result.Reason = "an INCONCLUSIVE candidate prevents a minimality claim"
		return r.result, nil
	}
	r.result.Status = history.Invalid
	r.result.OneMinimal = true
	r.result.Reason = "1-minimal under executable fixed-horizon action deletion; no global-minimum claim"
	return r.result, nil
}

func validateBaseline(t scenario.Trace, id buildinfo.Identity) error {
	if t.Version != 1 || t.Model != "raft-trace-v1" {
		return scenario.ErrUnsupportedVersion
	}
	if err := t.Scenario.Validate(); err != nil {
		return err
	}
	if !id.Identified() || t.Identity() != id {
		return errors.New("reducer source, module or patch identity mismatch")
	}
	if t.Header.Scheduler != buildinfo.SchedulerVersion || t.Header.Entropy != buildinfo.EntropyVersion || t.Header.Seed != t.Scenario.Seed || t.Header.Variant != string(t.Scenario.TargetVariant()) || t.Header.Initial != 0 {
		return errors.New("reducer trace header does not match its scenario")
	}
	want := t.Digest
	t.Digest = ""
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != want {
		return errors.New("reducer trace digest mismatch")
	}
	switch t.Result.Status {
	case history.Valid, history.Inconclusive:
		if t.Failure != nil {
			return errors.New("non-INVALID baseline contains a failure fingerprint")
		}
	case history.Invalid:
		f := t.Failure
		if f == nil || f.Requirement != "FR-002" || f.Variant != t.Header.Variant || f.Kind != "nonlinearizable-register" {
			return errors.New("INVALID baseline lacks the declared invariant fingerprint")
		}
		fingerprint := sha256.Sum256([]byte(f.Requirement + "\x00" + f.Variant + "\x00" + f.Kind))
		if f.Fingerprint != hex.EncodeToString(fingerprint[:]) {
			return errors.New("baseline failure fingerprint mismatch")
		}
	default:
		return errors.New("unsupported baseline verdict")
	}
	return nil
}

type reducer struct {
	ctx     context.Context
	opts    Options
	horizon uint64
	result  Reduction
	stopped bool
}

func (r *reducer) stop(reason string) {
	r.stopped = true
	r.result.Status = history.Inconclusive
	r.result.OneMinimal = false
	r.result.Reason = reason
}

func (r *reducer) check(begin, end int, final bool) (bool, error) {
	if r.ctx.Err() != nil {
		r.stop("cancelled reduction")
		return false, nil
	}
	if r.result.Checks >= r.opts.MaxChecks {
		r.stop("candidate check budget exhausted")
		return false, nil
	}
	r.result.Checks++
	if final {
		r.result.FinalPassChecks++
	}
	current := r.result.Trace.Scenario
	candidate := current
	candidate.Actions = make([]scenario.Action, 0, len(current.Actions)-(end-begin))
	candidate.Actions = append(candidate.Actions, current.Actions[:begin]...)
	candidate.Actions = append(candidate.Actions, current.Actions[end:]...)
	// An implicit horizon depends on the last action. Removing it would change
	// more than the action set, so that transform is outside this fixed domain.
	if candidate.Until() != r.horizon || candidate.Validate() != nil {
		r.result.CausalRejections++
		return false, nil
	}
	r.result.ExecutedChecks++
	trace, err := scenario.Run(r.ctx, candidate, scenario.Options{Identity: r.opts.Identity})
	if errors.Is(err, scenario.ErrCausality) {
		r.result.CausalRejections++
		return false, nil
	}
	if err != nil {
		if r.ctx.Err() != nil {
			r.result.InconclusiveCandidates++
			r.stop("cancelled candidate execution")
			return false, nil
		}
		r.result.Errors++
		r.stop("candidate execution error")
		return false, fmt.Errorf("candidate execution: %w", err)
	}
	switch trace.Result.Status {
	case history.Valid:
		r.result.ValidCandidates++
		return false, nil
	case history.Inconclusive:
		r.result.InconclusiveCandidates++
		if r.ctx.Err() != nil {
			r.stop("cancelled candidate execution")
		}
		return false, nil
	case history.Invalid:
		if trace.Failure == nil || *trace.Failure != *r.result.Trace.Failure {
			r.result.InvalidCandidates++
			r.result.FingerprintRejections++
			return false, nil
		}
	default:
		r.result.Errors++
		r.stop("candidate produced an unsupported verdict")
		return false, errors.New("candidate produced an unsupported verdict")
	}
	r.result.ReplayChecks++
	if err := scenario.Replay(r.ctx, trace, r.opts.Identity); err != nil {
		if r.ctx.Err() != nil {
			r.result.InconclusiveCandidates++
			r.stop("cancelled candidate replay")
			return false, nil
		}
		r.result.Errors++
		r.stop("candidate replay error")
		return false, fmt.Errorf("candidate replay: %w", err)
	}
	removed := make([]string, 0, end-begin)
	for _, action := range current.Actions[begin:end] {
		removed = append(removed, action.ID)
	}
	r.result.InvalidCandidates++
	r.result.Accepted++
	r.result.Steps = append(r.result.Steps, AcceptedStep{
		Check: r.result.Checks, Actions: len(candidate.Actions), Removed: removed,
		Digest: trace.Digest, Fingerprint: trace.Failure.Fingerprint, ReplayVerified: true,
	})
	r.result.Trace = trace
	return true, nil
}

func (r *reducer) greedy() error {
	for i := 0; i < len(r.result.Trace.Scenario.Actions) && !r.stopped; {
		accepted, err := r.check(i, i+1, false)
		if err != nil {
			return err
		}
		if accepted {
			i = 0
		} else {
			i++
		}
	}
	return nil
}

func (r *reducer) ddmin() error {
	parts := 2
	for len(r.result.Trace.Scenario.Actions) >= 2 && !r.stopped {
		length := len(r.result.Trace.Scenario.Actions)
		width := (length + parts - 1) / parts
		accepted := false
		for begin := 0; begin < length && !r.stopped; begin += width {
			end := min(begin+width, length)
			ok, err := r.check(begin, end, false)
			if err != nil {
				return err
			}
			if ok {
				parts = max(2, parts-1)
				accepted = true
				break
			}
		}
		if accepted {
			continue
		}
		if parts >= length {
			break
		}
		parts = min(length, 2*parts)
	}
	return nil
}

func (r *reducer) finalPass() error {
	for i := 0; i < len(r.result.Trace.Scenario.Actions) && !r.stopped; {
		accepted, err := r.check(i, i+1, true)
		if err != nil {
			return err
		}
		if accepted {
			i = 0
		} else {
			i++
		}
	}
	return nil
}
