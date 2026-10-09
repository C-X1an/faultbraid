package history

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func common12() History {
	h := History{Version: 1, Operations: []Operation{}}
	for i := 0; i < 6; i++ {
		c := int64(i * 4)
		v := int64(i + 1)
		h.Operations = append(h.Operations, Operation{ID: fmt.Sprintf("w%d", i), Kind: "write", Value: v, Call: c, Return: end(c + 1)}, Operation{ID: fmt.Sprintf("r%d", i), Kind: "read", Value: v, Call: c + 2, Return: end(c + 3)})
	}
	return h
}
func TestCommonHistoryTarget(t *testing.T) {
	start := time.Now()
	r, e := Check(context.Background(), common12(), Options{100000})
	elapsed := time.Since(start)
	if e != nil || r.Status != Valid || elapsed > time.Second {
		t.Fatalf("common12 target failed: elapsed=%s result=%+v error=%v", elapsed, r, e)
	}
	t.Logf("AC-022 common sequential12: duration=%s states=%d; host-specific target, not universal bound", elapsed, r.States)
}
func BenchmarkOracleSequential12(b *testing.B) {
	h := common12()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if !independentOracle(h) {
			b.Fatal("reference oracle")
		}
	}
}
func BenchmarkCheckOverlap12(b *testing.B) {
	h := History{Version: 1}
	for i := 0; i < 6; i++ {
		h.Operations = append(h.Operations, Operation{ID: fmt.Sprintf("w%d", i), Kind: "write", Value: int64(i + 1), Return: end(100)})
	}
	for i := 0; i < 6; i++ {
		h.Operations = append(h.Operations, Operation{ID: fmt.Sprintf("r%d", i), Kind: "read", Value: int64(i + 1), Return: end(100)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, e := Check(context.Background(), h, Options{100000})
		if e != nil || r.Status != Valid {
			b.Fatalf("overlap %+v %v", r, e)
		}
	}
}
