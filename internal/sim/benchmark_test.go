package sim

import "testing"

func model10000Workload() []Event {
	events := make([]Event, 10000)
	for i := 0; i < 2500; i++ {
		// Equal-time ties exercise insertion order and crash recovery.
		at := uint64(i)
		events[i*4] = Event{At: at, Kind: "write", Value: int64(i + 1)}
		events[i*4+1] = Event{At: at, Kind: "sync"}
		events[i*4+2] = Event{At: at, Kind: "crash"}
		events[i*4+3] = Event{At: at, Kind: "read"}
	}
	return events
}

func BenchmarkModel10000(b *testing.B) {
	events := model10000Workload()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr, e := Run(events)
		if e != nil || len(tr.Steps) != 10000 {
			b.Fatal("benchmark invariant", e)
		}
	}
}
