package system

import (
	"context"
	"testing"

	"github.com/C-X1an/faultbraid/internal/buildinfo"
	"github.com/C-X1an/faultbraid/internal/campaign"
	"github.com/C-X1an/faultbraid/internal/history"
	"github.com/C-X1an/faultbraid/internal/scenario"
)

// This measures the instrumented target, virtual scheduler, trace capture and
// history check together. It is not database throughput or native IO latency.
func BenchmarkTargetHealthy(b *testing.B) {
	s := scenario.Default(0)
	s.EndTime = scenario.Node(60)
	invokeAll(&s, 30, "invoke_write", "write7", 7)
	invokeAll(&s, 45, "invoke_read", "read7", 0)
	b.ReportAllocs()
	b.ResetTimer()
	var events, states uint64
	for i := 0; i < b.N; i++ {
		trace, err := scenario.Run(context.Background(), s, scenario.Options{Identity: buildinfo.Current()})
		if err != nil || trace.Result.Status != history.Valid || len(trace.History.Operations) != 2 {
			b.Fatal("actual healthy target benchmark invariant", err, trace.Result)
		}
		for _, operation := range trace.History.Operations {
			if operation.Return == nil || operation.Value != 7 {
				b.Fatal("actual target benchmark write/read did not complete")
			}
		}
		events += uint64(len(trace.Events))
		states += uint64(trace.Result.States)
	}
	b.ReportMetric(float64(events)/float64(b.N), "events/op")
	b.ReportMetric(float64(states)/float64(b.N), "states/op")
}

func BenchmarkScenarioReduction(b *testing.B) {
	s := campaign.KnownBadScenario()
	s.Actions = append([]scenario.Action{{ID: "irrelevant-delay", At: 20, Kind: "delay_link", From: scenario.Node(1), To: scenario.Node(3), Ticks: scenario.Node(1)}}, s.Actions...)
	baseline, err := scenario.Run(context.Background(), s, scenario.Options{Identity: buildinfo.Current()})
	if err != nil || baseline.Result.Status != history.Invalid || baseline.Failure == nil {
		b.Fatal("actual unsafe benchmark baseline did not fail its invariant", err)
	}
	b.Run("baseline-run", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trace, err := scenario.Run(context.Background(), s, scenario.Options{Identity: buildinfo.Current()})
			if err != nil || trace.Result.Status != history.Invalid || trace.Failure == nil || *trace.Failure != *baseline.Failure {
				b.Fatal("actual unsafe target baseline changed", err)
			}
		}
		b.ReportMetric(float64(len(s.Actions)), "actions/result")
	})
	for _, strategy := range []string{campaign.Greedy, campaign.DDMin} {
		b.Run(strategy, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			var checks, runs, replays, actions uint64
			for i := 0; i < b.N; i++ {
				reduced, err := campaign.Reduce(context.Background(), baseline, campaign.Options{Identity: buildinfo.Current(), MaxChecks: 1000, Strategy: strategy})
				if err != nil || reduced.Status != history.Invalid || !reduced.OneMinimal || !reduced.BaselineReplayed || reduced.Trace.Failure == nil || *reduced.Trace.Failure != *baseline.Failure || len(reduced.Trace.Scenario.Actions) >= len(s.Actions) {
					b.Fatal("actual same-fingerprint executable reduction invariant", err)
				}
				checks += uint64(reduced.Checks)
				runs += uint64(reduced.ExecutedChecks)
				replays += uint64(reduced.ReplayChecks)
				actions += uint64(len(reduced.Trace.Scenario.Actions))
			}
			b.ReportMetric(float64(checks)/float64(b.N), "candidate-checks/op")
			b.ReportMetric(float64(runs)/float64(b.N), "target-executions/op")
			b.ReportMetric(float64(replays)/float64(b.N), "replays/op")
			b.ReportMetric(float64(actions)/float64(b.N), "actions/result")
		})
	}
}
