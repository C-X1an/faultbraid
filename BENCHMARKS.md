# Benchmark methodology
No performance values in this document are assumed. Observed seed measurements, when present, are in verification/SEED_RESULTS.md with raw synthetic logs.

| Experiment | Claim / workload | Instrument / repetitions | Interpretation |
|---|---|---|---|
| Register checker | Common sequential/overlapping 12-operation histories | Go benchmark, allocation reporting, 1 warmup and 7 measured repeats, fixed fixture | ns/op, allocations and explored states; not target throughput |
| Difficult search | Ambiguous histories approaching state budget | 7 repeats per operation count 4,8,12,16,24; count INCONCLUSIVE | Budget/time curve, not universal solver speed |
| Queue/disk model | 10000 events with deterministic equal-time ties and crashes | Go benchmark, 7 repeats, memory reporting | Scheduler/model overhead only; no disk IO claim |
| Target campaign | 1000 seeds across fixed three-voter workloads/faults | Parent monotonic clock, logical steps, source/seed identifiers, all results saved | Discovery counts and replay fidelity, not production availability |
| Scenario reduction | Deliberately broken integration variants | Compare no reduction, greedy deletion and ddmin on same failures; 7 repetitions | original/reduced actions, replay count, elapsed time, same-invariant retention |
| Real-process lab | Three local processes, fixed register load at concurrency1,4,16 | Actual sockets/fsync, 1 warmup+7 runs, CPU/RSS and p50/p95/p99 | Same-host synthetic conformance; never extrapolate to cloud production |

Every run records CPU model/logical cores, OS/kernel, memory limit, Go/dependency versions, source/dirty tree, seed, warmup, count and command. Noise controls: no competing benchmarks, fixed GOMAXPROCS, avoid measuring setup inside timed loop, record CPU throttling/shared host caveat. Report median and range/IQR over repeats; raw per-run values remain available. Do not select the fastest run. Tail latencies require enough individual samples; do not calculate p99 from seven aggregate means.

Raw outputs: `.local-runs/benchmarks/<run-id>/`; reviewed synthetic copies may be exported under `verification/benchmarks/`. A standard-library summarizer creates numeric tables; plots are optional and must retain units/error representation and provenance. Measure real hardware only after documenting it; unknown hardware is UNKNOWN, not guessed.

TARGETS: common12-operation check<1s;10000 model events<2s;10/10 identical target replay digests;1000-seed campaign. A missed target remains visible. Seeds are tests, not statistical confidence of correctness or proof of absence of bugs. DESIGN CAPACITY, TESTED CAPACITY, OBSERVED CAPACITY and real production scale stay separate. The initial seed has no production users or completed Raft performance evidence.

## Observed local-kernel results - 2026-10-10 Singapore

[20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json). Executed source commit `1ff0c7e093fee8f552c889cb8cf9c9ac59f4efcf`; Go1.23.2 linux/amd64, AMD EPYC 9V74 host, shared container. Seven repetitions, Go benchmark calibration, 100ms requested benchmark interval per repetition. The `-5` suffix records the runtime processor setting; host model name is not an exclusive 80-core allocation.

| Synthetic workload | Median per operation | Min-max across repetitions | Median B/op | Median allocations/op |
|---|---:|---:|---:|---:|
| Check12 | 1.310 us | 1.286-1.362 us | 823 | 4 |
| OracleSequential12 | 2.782 us | 2.723-2.996 us | 661 | 7 |
| CheckOverlap12 | 221.976 us | 215.419-230.028 us | 98,464 | 71 |
| Model10000 | 12059.463 us | 10089.965-14460.769 us | 9,828,701 | 20062 |

`Model10000` means one 10,000-event local-model run, not one event or distributed transaction. The 12-operation examples are deliberately small and cannot characterize worst-case exponential search. The independent oracle is a correctness reference, not an optimized competing product. These are medians of benchmark summaries, not latency p50/p95/p99. No request-throughput, real-user scale or distributed-target claim follows. Supported-toolchain results remain NOT RUN. Full samples and environment are in [benchmark-summary.json](verification/curated/benchmark-summary.json).
