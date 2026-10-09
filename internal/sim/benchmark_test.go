package sim

import "testing"

func BenchmarkModel10000(b *testing.B) {
	events := make([]Event, 10000)
	for i := range events {
		events[i] = Event{At: uint64(i), Kind: "write", Value: int64(i)}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr, e := Run(events)
		if e != nil || len(tr.Steps) != 10000 {
			b.Fatal("benchmark invariant", e)
		}
	}
}
