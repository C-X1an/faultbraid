package acceptance_locked

import (
	"context"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/sim"
	"os"
	"strings"
	"testing"
)

func at(n int64) *int64 { return &n }
func TestLockedKernelAcceptance(t *testing.T) {
	ctx := context.Background()
	opts := history.Options{MaxStates: 100000}
	t.Run("AC001", func(t *testing.T) {
		for _, b := range []string{`{"version":1,"initial":0,"operations":[],"oops":1}`, `{"version":1,"version":1,"initial":0,"operations":[]}`, `{"version":1,"initial":0,"operations":[{"id":"x","kind":"read","value":0,"call":2,"return":1}]}`} {
			if _, e := history.Decode(strings.NewReader(b)); e == nil {
				t.Fatal("invalid contract accepted")
			}
		}
	})
	tests := []struct {
		name   string
		h      history.History
		want   history.Verdict
		budget int
	}{
		{"AC002", history.History{Version: 1, Operations: []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1, Return: at(2)}, {ID: "r", Kind: "read", Value: 0, Call: 3, Return: at(4)}}}, history.Invalid, 100000},
		{"AC003", history.History{Version: 1, Operations: []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1, Return: at(4)}, {ID: "r", Kind: "read", Value: 0, Call: 2, Return: at(3)}}}, history.Valid, 100000},
		{"AC004Include", history.History{Version: 1, Operations: []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1}, {ID: "r", Kind: "read", Value: 7, Call: 2, Return: at(3)}}}, history.Valid, 100000},
		{"AC004Omit", history.History{Version: 1, Operations: []history.Operation{{ID: "w", Kind: "write", Value: 7, Call: 1}, {ID: "r", Kind: "read", Value: 0, Call: 2, Return: at(3)}}}, history.Valid, 100000},
		{"AC005", history.History{Version: 1, Operations: []history.Operation{{ID: "r", Kind: "read", Call: 1, Return: at(2)}}}, history.Inconclusive, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, e := history.Check(ctx, tt.h, history.Options{MaxStates: tt.budget})
			if e != nil || r.Status != tt.want {
				t.Fatalf("got %+v %v want%s", r, e, tt.want)
			}
		})
	}
	t.Run("AC006", func(t *testing.T) {
		b, e := os.Open("../../examples/invalid.json")
		if e != nil {
			t.Fatal(e)
		}
		defer b.Close()
		h, e := history.Decode(b)
		if e != nil {
			t.Fatal(e)
		}
		r, e := history.Minimize(ctx, h, opts, 100)
		if e != nil {
			t.Fatal(e)
		}
		result, e := history.Check(ctx, r.History, opts)
		if e != nil || result.Status != history.Invalid || !r.OneMinimal {
			t.Fatal("reduction contract")
		}
	})
	t.Run("AC008", func(t *testing.T) {
		var d sim.Disk
		d.Write(7)
		d.Crash()
		if d.Read() != 0 {
			t.Fatal("volatile survived")
		}
		d.Write(9)
		d.Sync()
		d.Crash()
		if d.Read() != 9 {
			t.Fatal("durable lost")
		}
	})
}
