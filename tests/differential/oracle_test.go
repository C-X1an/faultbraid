package differential

import (
	"context"

	"github.com/C-X1an/faultbraid/internal/history"
)

// referenceResult uses UNKNOWN as an explicit answer, never as an alias for
// VALID. Work counts full permutations for the tiny oracle and checked optional
// write projections for Porcupine. These are different units.
type referenceResult struct {
	Status             history.Verdict `json:"status"`
	Reason             string          `json:"reason"`
	Work               uint64          `json:"work"`
	Projections        uint64          `json:"projections"`
	UnknownProjections uint64          `json:"unknown_projections"`
}

type tinyLimits struct {
	MaxOperations   int
	MaxPending      int
	MaxProjections  uint64
	MaxPermutations uint64
}

func standardTinyLimits() tinyLimits {
	return tinyLimits{MaxOperations: 8, MaxPending: 8, MaxProjections: 256, MaxPermutations: 1000000}
}

// tinyOracle is deliberately unoptimized: select a subset of pending writes,
// enumerate complete permutations by swapping a slice, and only then check
// all interval pairs and execute the sequential register specification. It
// shares no predecessor masks, search state, memoization, or transition helper
// with the production checker or the Porcupine model.
//
// Inputs to the differential tests have already passed history.Validate.
func tinyOracle(ctx context.Context, h history.History, limits tinyLimits) referenceResult {
	r := referenceResult{Status: history.Inconclusive}
	unknown := func(reason string) referenceResult { r.Reason = reason; return r }
	if ctx == nil || ctx.Err() != nil {
		return unknown("cancelled")
	}
	if limits.MaxOperations < 0 || len(h.Operations) > limits.MaxOperations {
		return unknown("tiny operation capacity")
	}
	mandatory, optional := []int{}, []int{}
	for index, op := range h.Operations {
		if op.Return != nil {
			mandatory = append(mandatory, index)
		} else if op.Kind == "write" {
			optional = append(optional, index)
		}
	}
	if limits.MaxPending < 0 || len(optional) > limits.MaxPending || len(optional) > 8 {
		return unknown("tiny pending-write capacity")
	}
	stopped := ""
	for subset := uint64(0); subset < uint64(1)<<len(optional); subset++ {
		if ctx.Err() != nil {
			return unknown("cancelled")
		}
		if r.Projections >= limits.MaxProjections {
			return unknown("tiny projection budget")
		}
		r.Projections++
		order := append([]int{}, mandatory...)
		for bit, index := range optional {
			if subset&(uint64(1)<<bit) != 0 {
				order = append(order, index)
			}
		}
		var permute func(int) bool
		permute = func(at int) bool {
			if ctx.Err() != nil {
				stopped = "cancelled"
				return false
			}
			if at == len(order) {
				if r.Work >= limits.MaxPermutations {
					stopped = "tiny permutation budget"
					return false
				}
				r.Work++
				// Check every pair after constructing the entire permutation.
				for earlier := 0; earlier < len(order); earlier++ {
					for later := earlier + 1; later < len(order); later++ {
						lateOp := h.Operations[order[later]]
						earlyOp := h.Operations[order[earlier]]
						if lateOp.Return != nil && *lateOp.Return < earlyOp.Call {
							return false
						}
					}
				}
				register := h.Initial
				for _, index := range order {
					op := h.Operations[index]
					if op.Kind == "write" {
						register = op.Value
					} else if op.Value != register {
						return false
					}
				}
				return true
			}
			for next := at; next < len(order); next++ {
				order[at], order[next] = order[next], order[at]
				valid := permute(at + 1)
				order[at], order[next] = order[next], order[at]
				if valid {
					return true
				}
				if stopped != "" {
					return false
				}
			}
			return false
		}
		if permute(0) {
			r.Status, r.Reason = history.Valid, "legal complete permutation"
			return r
		}
		if stopped != "" {
			return unknown(stopped)
		}
	}
	r.Status, r.Reason = history.Invalid, "all subsets and full permutations exhausted"
	return r
}

// legalWitness separately verifies the production checker's constructive
// witness, including coverage, uniqueness, pending-read omission and strict
// real-time precedence. It does not call either reference search algorithm.
func legalWitness(h history.History, witness []string) bool {
	position := make(map[string]int, len(witness))
	ops := make(map[string]history.Operation, len(h.Operations))
	for _, op := range h.Operations {
		ops[op.ID] = op
	}
	register := h.Initial
	for index, id := range witness {
		op, exists := ops[id]
		if !exists {
			return false
		}
		if _, duplicate := position[id]; duplicate || op.Kind == "read" && op.Return == nil {
			return false
		}
		position[id] = index
		if op.Kind == "write" {
			register = op.Value
		} else if op.Value != register {
			return false
		}
	}
	for _, before := range h.Operations {
		beforeAt, included := position[before.ID]
		if before.Return != nil && !included {
			return false
		}
		if !included || before.Return == nil {
			continue
		}
		for _, after := range h.Operations {
			afterAt, selected := position[after.ID]
			if selected && *before.Return < after.Call && beforeAt >= afterAt {
				return false
			}
		}
	}
	return true
}
