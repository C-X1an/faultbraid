# Benchmark methodology and measured results

A separate warmup followed by seven measured repetitions per group produced91 samples across13 groups;100ms requested intervals and CPU4. [20261011T195258-68f9d1928b](verification/curated/20261011T195258-68f9d1928b/run.json) (20.282s); warmup [20261011T195223-c535472744](verification/curated/20261011T195223-c535472744/run.json) (30.631s). Executed commit `9bdbf63ae0236b7cc8649f2867293d1e2109efc6`, complete source SHA256 `8dbf60c32285bcadea6429cc5b5a06f03fbab8a78ab82e24011574bf730a7155` (963 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

Intel Core i9-13900H,20 logical CPUs,34,048,978,944 host bytes; Windows11 amd64 and Go1.27.2. Host totals are not allocation limits. No other FaultBraid heavy work was scheduled during the measured run; the shared desktop was not an exclusive benchmark machine. Calibration, setup and GC may affect samples. No fastest-run selection or cross-host speedup claim.

| Workload | Median ns/op | Min–max | Q1–Q3 | Median B/op | Median allocs/op | States / result |
|---|---:|---:|---:|---:|---:|---|
| `BenchmarkCheck12-4` | 841.4 | 771–1055 | 812.35–861.2 | 936 | 6 | 13.0 |
| `BenchmarkCheckOverlap12-4` | 136612 | 125854–194102 | 130798.5–154070.5 | 109440 | 26 | 1911.0 |
| `BenchmarkOracleSequential12-4` | 1351 | 1330–1923 | 1337.5–1481.5 | 616 | 6 | — |
| `BenchmarkSearchBudgetCurve/operations-12-4` | 182341 | 175208–187137 | 180456.5–183411.5 | 109248 | 25 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-16-4` | 176435 | 173819–188069 | 175599–177180 | 110272 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-24-4` | 207750 | 182471–292093 | 185636.5–230269.5 | 110464 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-4-4` | 735.7 | 701.6–757 | 715.75–745.15 | 520 | 4 | 16.0; INVALID |
| `BenchmarkSearchBudgetCurve/operations-8-4` | 63452 | 62082–70237 | 62721.5–65138.5 | 54088 | 17 | 1352.0; INVALID |
| `BenchmarkModel10000-4` | 7405450 | 7166453–8095136 | 7295951–7455760 | 13579253 | 22586 | — |
| `BenchmarkScenarioReduction/baseline-run-4` | 2960821 | 2852167–3629671 | 2875172–3148179 | 2805868 | 33586 | 12.0 actions |
| `BenchmarkScenarioReduction/ddmin-4` | 1.426638e+08 | 1.227557e+08–2.119395e+08 | 1.363174e+08–1.598394e+08 | 106511256 | 1237507 | 5.0 actions |
| `BenchmarkScenarioReduction/greedy-4` | 1.064212e+08 | 1.030305e+08–1.11104e+08 | 1.047265e+08–1.078536e+08 | 99365008 | 1161973 | 5.0 actions |
| `BenchmarkTargetHealthy-4` | 2920577 | 2732266–3136410 | 2785165–2959411 | 2812327 | 33693 | 3.0 |

All raw calibrated samples and inclusive quartiles are in [benchmark summary](verification/release-a5/benchmark-summary.json). `Model10000` measures one10,000-event write/sync/crash/read execution (2,500 groups), not one event or transaction. Healthy target includes trace/history work, not request throughput. Histories hitting4,096 states remain INCONCLUSIVE. Greedy/ddmin compare one fixed12-action unsafe fixture and return5 actions; counts and bytes are in the samples. The separate CLI fixture starts with11 actions. No global minimum is claimed.

The complete1,000-seed command took11.501s; outer campaign 55.673s including build and ten selected unsafe replays. Native1/4/16 conformance took6059/6059/6157ms; this is one conformance run/profile, not seven-repeat performance evidence. Native CPU/RSS/request p50/p95/p99 are NOT RUN; aggregate timings cannot supply those tails. No production-scale extrapolation.

Targets: common12-operation checking<1s;10,000 model events<2s; ten matching replay digests and1,000 outcomes. These are finite named targets, not statistical correctness confidence.

Historical measurements below retain their original toolchain and workload; legacy Model10000 uses sequential writes and is not comparable to the current crash/durability workload.

## Observed local-kernel results - 2026-10-10 Singapore

[20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json). Executed source commit `1ff0c7e093fee8f552c889cb8cf9c9ac59f4efcf`; Go1.23.2 linux/amd64, AMD EPYC 9V74 host, shared container. Seven repetitions, Go benchmark calibration, 100 ms requested benchmark interval per repetition. The `-5` suffix records the runtime processor setting; host model name is not an exclusive 80-core allocation.

| Synthetic workload | Median per operation | Min-max across repetitions | Median B/op | Median allocations/op |
|---|---:|---:|---:|---:|
| Check12 | 1.310 us | 1.286-1.362 us | 823 | 4 |
| OracleSequential12 | 2.782 us | 2.723-2.996 us | 661 | 7 |
| CheckOverlap12 | 221.976 us | 215.419-230.028 us | 98,464 | 71 |
| Model10000 | 12059.463 us | 10089.965-14460.769 us | 9,828,701 | 20062 |

`Model10000` means one 10,000-event local-model run, not one event or distributed transaction. The 12-operation examples are deliberately small and cannot characterize worst-case exponential search. The independent oracle is a correctness reference, not an optimized competing product. These are medians of benchmark summaries, not latency p50/p95/p99. No request-throughput, real-user scale or distributed-target claim follows. That historical run used Go1.23.2; current supported-toolchain measurements are above. Full samples and environment are in [benchmark-summary.json](verification/curated/benchmark-summary.json).
