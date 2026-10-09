package security

import (
	"context"
	"fmt"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/sim"
	"strings"
	"testing"
)

func TestResourceLimits(t *testing.T) {
	if _, e := history.Decode(strings.NewReader(strings.Repeat(" ", (1<<20)+1))); e == nil {
		t.Fatal("byte bound")
	}
	h := history.History{Version: 1}
	for i := 0; i < 25; i++ {
		h.Operations = append(h.Operations, history.Operation{ID: fmt.Sprint(i), Kind: "write"})
	}
	if e := h.Validate(); e == nil {
		t.Fatal("operation bound")
	}
	if _, e := sim.Run(make([]sim.Event, 100001)); e == nil {
		t.Fatal("event bound")
	}
	h.Operations = nil
	done := int64(100)
	for i := 0; i < 16; i++ {
		h.Operations = append(h.Operations, history.Operation{ID: fmt.Sprint(i), Kind: "write", Value: int64(i), Call: 0, Return: &done})
	}
	h.Operations = append(h.Operations, history.Operation{ID: "impossible", Kind: "read", Value: 999, Call: 0, Return: &done})
	r, e := history.Check(context.Background(), h, history.Options{MaxStates: 64})
	if e != nil || r.Status != history.Inconclusive || r.States > 64 {
		t.Fatalf("work bound: %+v %v", r, e)
	}
}
