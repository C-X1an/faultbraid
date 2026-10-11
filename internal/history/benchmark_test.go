package history

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkCheck12(b *testing.B) {
	h := History{Version: 1, Operations: []Operation{}}
	for i := 0; i < 6; i++ {
		call := int64(i * 4)
		value := int64(i + 1)
		h.Operations = append(h.Operations, Operation{ID: string(rune('a' + i)), Kind: "write", Value: value, Call: call, Return: end(call + 1)}, Operation{ID: string(rune('A' + i)), Kind: "read", Value: value, Call: call + 2, Return: end(call + 3)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	var states uint64
	for i := 0; i < b.N; i++ {
		r, e := Check(context.Background(), h, Options{100000})
		if e != nil || r.Status != Valid {
			b.Fatal("benchmark invariant", r, e)
		}
		states += uint64(r.States)
	}
	b.ReportMetric(float64(states)/float64(b.N), "states/op")
}

// A completed overlapping read of 999 cannot be explained by writes whose
// values are all below 24. Larger finite searches may exhaust the declared
// budget instead of proving that fact; those outcomes remain INCONCLUSIVE.
func BenchmarkSearchBudgetCurve(b *testing.B) {
	for _, operations := range []int{4, 8, 12, 16, 24} {
		b.Run(fmt.Sprintf("operations-%d", operations), func(b *testing.B) {
			h := History{Version: 1, Operations: []Operation{}}
			for i := 0; i < operations-1; i++ {
				h.Operations = append(h.Operations, Operation{ID: fmt.Sprintf("write-%d", i), Kind: "write", Value: int64(i + 1), Call: 0, Return: end(100)})
			}
			h.Operations = append(h.Operations, Operation{ID: "impossible-read", Kind: "read", Value: 999, Call: 0, Return: end(100)})
			b.ReportAllocs()
			b.ResetTimer()
			var states, invalid, unknown uint64
			for i := 0; i < b.N; i++ {
				result, err := Check(context.Background(), h, Options{MaxStates: 4096})
				if err != nil || result.States > 4096 {
					b.Fatal("bounded search benchmark failed", result, err)
				}
				switch result.Status {
				case Invalid:
					invalid++
				case Inconclusive:
					unknown++
				default:
					b.Fatal("impossible completed read became valid", result)
				}
				states += uint64(result.States)
			}
			b.ReportMetric(float64(states)/float64(b.N), "states/op")
			b.ReportMetric(float64(invalid)/float64(b.N), "invalid/op")
			b.ReportMetric(float64(unknown)/float64(b.N), "inconclusive/op")
		})
	}
}
