# Benchmark methodology and measured results

The current run records91 calibrated samples across13 groups: a separate warmup, then seven measured repetitions per group,100ms requested interval, CPU4. [20261011T170421-048ed5cb2f](verification/curated/20261011T170421-048ed5cb2f/run.json) (17.426s); warmup [20261011T170410-6fedc5bb4f](verification/curated/20261011T170410-6fedc5bb4f/run.json) (8.459s). Executed commit `007ef4ee5d47dddc8878d1c09607979b2b69c7fb`, complete source SHA256 `abad9b97745475faf3d7d26d48f9a77ee4c6b90edf190acb1de274a4fca0db8b` (276 files). Later prose/evidence curation does not rewrite that identity; execution inputs are compared by the release gate.

Hardware: Intel Core i9-13900H,20 logical CPUs,34,048,978,944 host bytes; Windows11 amd64,Go1.27.2, actual pinned dependency/patch identities. Host memory is not an inferred allocation limit. GOMAXPROCS is selected by `-cpu=4`. No other FaultBraid heavy command was scheduled during this measured run, but the shared desktop was not an exclusive benchmark machine. Calibration/setup and GC may affect samples; native child startup is measured separately. No fastest-run selection or cross-host speedup claim.

| Workload | Median ns/op | Min–max ns/op | Q1–Q3 ns/op | Median B/op | Median allocs/op | Observed states / result |
|---|---:|---:|---:|---:|---:|---|
| `BenchmarkCheck12-4` | 812.9 | 747.4–941.1 | 777.85–856.65 | 936 | 6 | 13.0 |
| `BenchmarkCheckOverlap12-4` | 133406 | 127248–146084 | 130779–138417.5 | 109440 | 26 | 1911.0 |
| `BenchmarkOracleSequential12-4` | 1416 | 1344–1529 | 1361.5–1453.5 | 616 | 6 | — |
| `BenchmarkSearchBudgetCurve/operations-12-4` | 180651 | 173913–206684 | 176124–200790 | 109248 | 25 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-16-4` | 182837 | 177186–196386 | 179932.5–188654.5 | 110272 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-24-4` | 231441 | 190754–333872 | 209555–254865 | 110464 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-4-4` | 735 | 711.4–787.6 | 715.85–780.4 | 520 | 4 | 16.0; INVALID |
| `BenchmarkSearchBudgetCurve/operations-8-4` | 63590 | 60515–65667 | 62683.5–64966 | 54088 | 17 | 1352.0; INVALID |
| `BenchmarkModel10000-4` | 7295793 | 6953367–8856179 | 7133120–7354746 | 13104785 | 22583 | — |
| `BenchmarkScenarioReduction/baseline-run-4` | 2966259 | 2786092–3218217 | 2918570–3037108 | 2804985 | 33587 | 12.0 actions |
| `BenchmarkScenarioReduction/ddmin-4` | 1.211991e+08 | 1.148882e+08–2.040019e+08 | 1.198908e+08–1.28172e+08 | 105797904 | 1237377 | 5.0 actions |
| `BenchmarkScenarioReduction/greedy-4` | 1.087114e+08 | 1.071354e+08–1.175331e+08 | 1.072352e+08–1.093955e+08 | 99740064 | 1161940 | 5.0 actions |
| `BenchmarkTargetHealthy-4` | 2862748 | 2731616–3023604 | 2803116–2918867 | 2819555 | 33694 | 3.0 |

All individual calibrated samples and inclusive quartiles are in [benchmark-summary.json](verification/release/benchmark-summary.json), with its original input SHA256. `Model10000` is one10,000-event model execution comprising2,500 equal-time write/sync/crash/read groups, not one event or distributed transaction. Healthy-target cost includes403 trace events and history checking; it is not request throughput. Difficult12/16/24 histories reach4,096 explored states and remain INCONCLUSIVE.

Reduction benchmarks compare strategies on one fixed deliberate unsafe fixture:12 original actions,5 final actions; greedy24 candidate checks/8 replays and ddmin32 checks/6 replays. Both retain the same failure invariant and bounded1-minimality, with no global-minimality claim. The separate CLI demo fixture reduces11 actions to5 with24 candidate checks/4 replay checks. The separate1,000-seed campaign command lasted10.397s (outer campaign32.148s including build/unsafe/replays), counted every outcome, and matched10/10 selected unsafe replay digests. These satisfy the named finite targets, not statistical correctness confidence.

The separate native conformance observations at request limits1/4/16 lasted6,092/6,030/6,020ms with observed peaks1/4/16,856/884/888 successful HTTP posts and2,328/3,651/4,092 journal bytes. This is one recorded conformance run per profile, not seven repeated performance samples. Native CPU/RSS and individual-request p50/p95/p99 are **NOT RUN**; aggregate means cannot supply those tails. No cloud/production-scale extrapolation follows.

Target thresholds: common12-operation checking<1s;10,000 model events<2s;10/10 matching target replay digests;1,000 requested seed outcomes. Raw measured variability remains visible. Capacity bounds, tested examples and observed production scale are distinct; there are no production users.

Historical results below retain their original workload/toolchain. Model10000 in the legacy run was10,000 sequential writes, so its values are not a speed comparison with the current durability/crash workload.

## Observed local-kernel results - 2026-10-10 Singapore

[20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json). Executed source commit `1ff0c7e093fee8f552c889cb8cf9c9ac59f4efcf`; Go1.23.2 linux/amd64, AMD EPYC 9V74 host, shared container. Seven repetitions, Go benchmark calibration, 100ms requested benchmark interval per repetition. The `-5` suffix records the runtime processor setting; host model name is not an exclusive 80-core allocation.

| Synthetic workload | Median per operation | Min-max across repetitions | Median B/op | Median allocations/op |
|---|---:|---:|---:|---:|
| Check12 | 1.310 us | 1.286-1.362 us | 823 | 4 |
| OracleSequential12 | 2.782 us | 2.723-2.996 us | 661 | 7 |
| CheckOverlap12 | 221.976 us | 215.419-230.028 us | 98,464 | 71 |
| Model10000 | 12059.463 us | 10089.965-14460.769 us | 9,828,701 | 20062 |

`Model10000` means one 10,000-event local-model run, not one event or distributed transaction. The 12-operation examples are deliberately small and cannot characterize worst-case exponential search. The independent oracle is a correctness reference, not an optimized competing product. These are medians of benchmark summaries, not latency p50/p95/p99. No request-throughput, real-user scale or distributed-target claim follows. That historical run used Go1.23.2; current supported-toolchain measurements are above. Full samples and environment are in [benchmark-summary.json](verification/curated/benchmark-summary.json).
