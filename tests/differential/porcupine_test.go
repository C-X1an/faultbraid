package differential

import (
	"context"
	"math"
	"time"

	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/anishathalye/porcupine"
)

type registerInput struct {
	Write bool
	Value int64
}

// This model is independently expressed for Porcupine. Completed writes have
// no meaningful output. Reads return an int64 and leave the register unchanged.
func registerModel(initial int64) porcupine.Model {
	return porcupine.Model{
		Init: func() any { return initial },
		Step: func(state, input, output any) (bool, any) {
			command := input.(registerInput)
			if command.Write {
				return true, command.Value
			}
			return output.(int64) == state.(int64), state
		},
		Equal: func(left, right any) bool { return left.(int64) == right.(int64) },
	}
}

type porcupineLimits struct {
	MaxOperations     int
	MaxPending        int
	MaxProjections    uint64
	ProjectionTimeout time.Duration
	TotalTimeout      time.Duration
}

func standardPorcupineLimits() porcupineLimits {
	return porcupineLimits{MaxOperations: 8, MaxPending: 8, MaxProjections: 256, ProjectionTimeout: 2 * time.Second, TotalTimeout: 5 * time.Second}
}

func fromPorcupine(result porcupine.CheckResult) history.Verdict {
	switch result {
	case porcupine.Ok:
		return history.Valid
	case porcupine.Illegal:
		return history.Invalid
	default:
		return history.Inconclusive
	}
}

// porcupineOracle enumerates optional pending-write subsets because Porcupine's
// Operation API expects a response for every included operation. A selected
// pending write keeps its original call and receives an artificial return
// strictly after every recorded call or response. This adds no precedence
// edges from that pending write. Omitted pending reads have no observed output.
// Each operation gets its own ClientId, so this translation adds no per-client
// sequencing assumptions. Timestamp ties remain overlapping closed intervals.
//
// Existential semantics: one Ok projection proves VALID; only exhausting every
// projection as Illegal proves INVALID. A timed-out/unrepresentable projection
// or a projection budget produces UNKNOWN unless another projection is Ok.
// The total deadline also bounds the whole projection enumeration.
func porcupineOracle(ctx context.Context, h history.History, limits porcupineLimits) referenceResult {
	r := referenceResult{Status: history.Inconclusive}
	unknown := func(reason string) referenceResult { r.Reason = reason; return r }
	if ctx == nil || ctx.Err() != nil {
		return unknown("cancelled")
	}
	if limits.MaxOperations < 0 || len(h.Operations) > limits.MaxOperations {
		return unknown("Porcupine operation capacity")
	}
	if limits.ProjectionTimeout <= 0 || limits.TotalTimeout <= 0 {
		return unknown("Porcupine time budget")
	}
	deadline, cancel := context.WithTimeout(ctx, limits.TotalTimeout)
	defer cancel()
	mandatory, optional := []int{}, []int{}
	var last int64
	for index, op := range h.Operations {
		if op.Call > last {
			last = op.Call
		}
		if op.Return != nil {
			mandatory = append(mandatory, index)
			if *op.Return > last {
				last = *op.Return
			}
		} else if op.Kind == "write" {
			optional = append(optional, index)
		}
	}
	if limits.MaxPending < 0 || len(optional) > limits.MaxPending || len(optional) > 8 {
		return unknown("Porcupine pending-write capacity")
	}
	model := registerModel(h.Initial)
	step := model.Step
	model.StepContext = func(inner context.Context, state, input, output any) (bool, any) {
		if deadline.Err() != nil || inner.Err() != nil {
			return false, state
		}
		return step(state, input, output)
	}
	for subset := uint64(0); subset < uint64(1)<<len(optional); subset++ {
		if deadline.Err() != nil {
			return unknown("Porcupine total timeout or cancellation")
		}
		if r.Projections >= limits.MaxProjections {
			return unknown("Porcupine projection budget")
		}
		r.Projections++
		if subset != 0 && last == math.MaxInt64 {
			// int64 cannot encode a return later than all observed events.
			// The omitted projection is still checked; inclusion stays UNKNOWN.
			r.UnknownProjections++
			continue
		}
		selected := append([]int{}, mandatory...)
		for bit, index := range optional {
			if subset&(uint64(1)<<bit) != 0 {
				selected = append(selected, index)
			}
		}
		projection := make([]porcupine.Operation, 0, len(selected))
		for _, index := range selected {
			op := h.Operations[index]
			var finish int64
			if op.Return != nil {
				finish = *op.Return
			} else {
				finish = last + 1 // The representability guard above applies.
			}
			projection = append(projection, porcupine.Operation{
				ClientId: index,
				Input:    registerInput{Write: op.Kind == "write", Value: op.Value},
				Output:   op.Value,
				Call:     op.Call,
				Return:   finish,
			})
		}
		end, _ := deadline.Deadline()
		timeout := time.Until(end)
		if timeout > limits.ProjectionTimeout {
			timeout = limits.ProjectionTimeout
		}
		if timeout <= 0 {
			return unknown("Porcupine total timeout or cancellation")
		}
		// Verbose waits for all checker workers on timeout, so no abandoned
		// model callbacks are left behind when a bounded projection returns.
		answer, _ := porcupine.CheckOperationsVerbose(model, projection, timeout)
		r.Work++
		if deadline.Err() != nil {
			return unknown("Porcupine total timeout or cancellation")
		}
		switch fromPorcupine(answer) {
		case history.Valid:
			r.Status, r.Reason = history.Valid, "Porcupine accepted an optional-write projection"
			return r
		case history.Inconclusive:
			r.UnknownProjections++
		}
	}
	if r.UnknownProjections != 0 {
		return unknown("Porcupine unknown optional-write projections")
	}
	r.Status, r.Reason = history.Invalid, "Porcupine rejected every optional-write projection"
	return r
}
