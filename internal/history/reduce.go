package history

import (
	"context"
	"errors"
)

type Reduction struct {
	History                History `json:"history"`
	Checks                 int     `json:"checks"`
	OneMinimal             bool    `json:"one_minimal"`
	InconclusiveCandidates int     `json:"inconclusive_candidates"`
	Classification         string  `json:"classification"`
	Status                 Verdict `json:"status"`
	Reason                 string  `json:"reason"`
}

// Minimize deletes operations only when rechecking gives a definitive INVALID.
// This is diagnostic history reduction, NOT an executable scenario reproducer.
func Minimize(ctx context.Context, h History, opts Options, maxChecks int) (Reduction, error) {
	out := Reduction{History: h, Classification: "diagnostic_only", Status: Inconclusive}
	if maxChecks < 1 || maxChecks > 100000 {
		return out, errors.New("invalid reduction budget")
	}
	baseline, err := Check(ctx, h, opts)
	if err != nil {
		return out, err
	}
	out.Checks++
	if baseline.Status == Inconclusive {
		out.Reason = "baseline " + baseline.Reason
		return out, nil
	}
	if baseline.Status != Invalid {
		return out, errors.New("reduction requires a definitive INVALID history")
	}
	out.Status = Invalid
	out.Reason = "retained history is definitively INVALID"
	out.History.Operations = append([]Operation{}, h.Operations...)
	for {
		changed := false
		unknown := false
		for i := 0; i < len(out.History.Operations); i++ {
			if out.Checks >= maxChecks {
				out.Status = Inconclusive
				out.Reason = "reduction check budget exhausted; retained history is definitively INVALID"
				return out, nil
			}
			if ctx.Err() != nil {
				out.Status = Inconclusive
				out.Reason = "reduction cancelled; retained history is definitively INVALID"
				return out, nil
			}
			candidate := out.History
			candidate.Operations = make([]Operation, 0, len(out.History.Operations)-1)
			candidate.Operations = append(candidate.Operations, out.History.Operations[:i]...)
			candidate.Operations = append(candidate.Operations, out.History.Operations[i+1:]...)
			r, err := Check(ctx, candidate, opts)
			if err != nil {
				return out, err
			}
			out.Checks++
			if r.Status == Inconclusive {
				unknown = true
				out.InconclusiveCandidates++
			}
			if r.Status == Invalid {
				out.History = candidate
				changed = true
				break
			}
		}
		if !changed {
			if unknown {
				out.Status = Inconclusive
				out.Reason = "candidate checks were inconclusive; retained history is definitively INVALID"
				return out, nil
			}
			out.OneMinimal = true
			out.Status = Invalid
			out.Reason = "delete-one minimal INVALID history"
			return out, nil
		}
	}
}
