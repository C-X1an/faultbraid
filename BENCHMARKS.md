# Benchmark methodology and measured results

A separate warmup followed by seven measured repetitions per group produced 91 samples across 13 groups;100 ms requested intervals and CPU4. [20261011T175804-10e36b68c3](verification/curated/20261011T175804-10e36b68c3/run.json) (20.796 s); warmup [20261011T175752-97e0f8660d](verification/curated/20261011T175752-97e0f8660d/run.json) (9.515 s). Executed commit `2e8379a7ee18c388d31b6b963c67ed04faa5a41e`, complete source SHA256 `53eb9fa9ddeca5e426a7b2eab34976b5ddd72e555eddaecb11125101ad4db86a` (622 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

Intel Core i9-13900H,20 logical CPUs,34,048,978,944 host bytes; Windows11 amd64 and Go1.27.2. Host totals are not allocation limits. No other FaultBraid heavy work was scheduled during the measured run; the shared desktop was not an exclusive benchmark machine. Calibration, setup and GC may affect samples. No fastest-run selection or cross-host speedup claim.

| Workload | Median ns/op | Min–max | Q1–Q3 | Median B/op | Median allocs/op | States / result |
|---|---:|---:|---:|---:|---:|---|
| `BenchmarkCheck12-4` | 842.5 | 748.7–1044 | 787.6–871.8 | 936 | 6 | 13.0 |
| `BenchmarkCheckOverlap12-4` | 124685 | 120762–128943 | 122385–127027.5 | 109440 | 26 | 1911.0 |
| `BenchmarkOracleSequential12-4` | 1294 | 1278–1329 | 1288.5–1308 | 616 | 6 | — |
| `BenchmarkSearchBudgetCurve/operations-12-4` | 170437 | 163590–183652 | 166268–175088.5 | 109248 | 25 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-16-4` | 169811 | 166828–173027 | 167841.5–172482.5 | 110272 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-24-4` | 184599 | 171318–189143 | 179345.5–185239.5 | 110464 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-4-4` | 758.6 | 739.2–845.8 | 747.75–791.3 | 520 | 4 | 16.0; INVALID |
| `BenchmarkSearchBudgetCurve/operations-8-4` | 62275 | 59688–70311 | 60162.5–64811.5 | 54088 | 17 | 1352.0; INVALID |
| `BenchmarkModel10000-4` | 6915607 | 6745688–7060887 | 6786710–6924241 | 13579247 | 22586 | — |
| `BenchmarkScenarioReduction/baseline-run-4` | 2740442 | 2676438–3119882 | 2712612–2859882 | 2793782 | 33584 | 12.0 actions |
| `BenchmarkScenarioReduction/ddmin-4` | 1.265664e+08 | 1.14233e+08–1.347039e+08 | 1.174805e+08–1.311446e+08 | 106134880 | 1237436 | 5.0 actions |
| `BenchmarkScenarioReduction/greedy-4` | 1.033919e+08 | 9.817445e+07–1.483382e+08 | 1.011768e+08–1.190752e+08 | 99583868 | 1161876 | 5.0 actions |
| `BenchmarkTargetHealthy-4` | 2772184 | 2592996–2903707 | 2701794–2854350 | 2808946 | 33692 | 3.0 |

All raw calibrated samples and inclusive quartiles are in [benchmark summary](verification/release-a4/benchmark-summary.json). `Model10000` measures one 10,000-event write/sync/crash/read execution (2,500 groups), not one event or transaction. Healthy target includes trace/history work, not request throughput. Histories hitting4,096 states remain INCONCLUSIVE. Greedy/ddmin compare one fixed12-action unsafe fixture and return5 actions; counts and bytes are in the samples. The separate CLI fixture starts with 11 actions. No global minimum is claimed.

The complete1,000-seed command took 10.633 s; outer campaign 38.818 s including build and ten selected unsafe replays. Native1/4/16 conformance took 5991/5992/6035 ms; this is one conformance run/profile, not seven-repeat performance evidence. Native CPU/RSS/request p50/p95/p99 are NOT RUN; aggregate timings cannot supply those tails. No production-scale extrapolation.

Targets: common12-operation checking<1 s;10,000 model events<2 s; ten matching replay digests and1,000 outcomes. These are finite named targets, not statistical correctness confidence.

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
