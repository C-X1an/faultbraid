package main

import (
	"bytes"
	"errors"
	"io"

	"github.com/C-X1an/faultbraid/internal/labreal"
	"github.com/C-X1an/faultbraid/internal/strictjson"
)

const maxNativeCompletionPlanBytes = 64 << 10

// Native completion plans are separate from replayable scenario input. The
// native adapter validates every predecessor before creating output or children.
type nativeCompletionPlan struct {
	Version                int                            `json:"version"`
	CompletionDependencies []labreal.CompletionDependency `json:"completion_dependencies"`
	RequiredCompletions    []string                       `json:"required_completions"`
}

func decodeNativeCompletionPlan(reader io.Reader) (nativeCompletionPlan, error) {
	var plan nativeCompletionPlan
	data, err := io.ReadAll(io.LimitReader(reader, maxNativeCompletionPlanBytes+1))
	if err != nil {
		return plan, errors.New("cannot read native completion plan")
	}
	if len(data) > maxNativeCompletionPlanBytes {
		return plan, errors.New("native completion plan exceeds 64 KiB")
	}
	if err := strictjson.Decode(bytes.NewReader(data), &plan); err != nil {
		return plan, err
	}
	if plan.Version != 1 {
		return plan, errors.New("unsupported native completion plan version")
	}
	return plan, nil
}
