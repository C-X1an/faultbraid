package history

import (
	"context"
	"errors"
)

type searchKey struct {
	Mask  uint64
	Value int64
}

// Check constructs a legal sequential witness or exhausts the bounded search.
// Pending writes can be included or omitted. Pending reads can be omitted.
// Caller must not concurrently mutate the history while Check is running.
func Check(ctx context.Context, h History, opts Options) (Result, error) {
	r := Result{Witness: []string{}}
	if ctx == nil {
		return r, errors.New("nil context")
	}
	if err := h.Validate(); err != nil {
		return r, err
	}
	if opts.MaxStates < 1 || opts.MaxStates > MaxStatesLimit {
		return r, errors.New("invalid state budget")
	}
	ops := h.Operations
	n := len(ops)
	pred := make([]uint64, n)
	var completed uint64
	for i, op := range ops {
		if op.Return != nil {
			completed |= uint64(1) << i
		}
		for j, prev := range ops {
			if prev.Return != nil && *prev.Return < op.Call {
				pred[i] |= uint64(1) << j
			}
		}
	}
	failed := make(map[searchKey]bool)
	path := make([]string, 0, n)
	stopReason := ""
	var visit func(uint64, int64) bool
	visit = func(mask uint64, value int64) bool {
		if ctx.Err() != nil {
			stopReason = "cancelled"
			return false
		}
		if r.States >= opts.MaxStates {
			stopReason = "state budget exhausted"
			return false
		}
		r.States++
		if mask&completed == completed {
			r.Witness = append(r.Witness, path...)
			return true
		}
		key := searchKey{mask, value}
		if failed[key] {
			return false
		}
		for i, op := range ops {
			bit := uint64(1) << i
			if mask&bit != 0 || pred[i]&mask != pred[i] {
				continue
			}
			if op.Kind == "read" && op.Return == nil {
				continue
			}
			next := value
			if op.Kind == "read" {
				if op.Value != value {
					continue
				}
			} else {
				next = op.Value
			}
			path = append(path, op.ID)
			if visit(mask|bit, next) {
				return true
			}
			path = path[:len(path)-1]
			if stopReason != "" {
				return false
			}
		}
		// Only a fully explored failed subtree is safe to memoize.
		failed[key] = true
		return false
	}
	if visit(0, h.Initial) {
		r.Status = Valid
		r.Reason = "legal sequential witness"
	} else if stopReason != "" {
		r.Status = Inconclusive
		r.Reason = stopReason
	} else {
		r.Status = Invalid
		r.Reason = "all admissible orders exhausted"
	}
	return r, nil
}
