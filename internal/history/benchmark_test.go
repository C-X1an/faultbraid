package history

import (
	"context"
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
