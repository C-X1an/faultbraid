# Benchmark methodology and measured results

A separate warmup followed by seven measured repetitions per group produced91 samples across13 groups;100ms requested intervals and CPU4. [20261012T174444-800c31eb52](verification/curated/20261012T174444-800c31eb52/run.json) (26.806s); warmup [20261012T174402-281571dc69](verification/curated/20261012T174402-281571dc69/run.json) (8.338s). Executed commit `65d10e1c83950f97d0857b3145ba9f91fe848e2f`, complete source SHA256 `89a9af213b2c131854762a0e312108fbd90bba07bee9754a3a611327f4a60f39` (1512 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

Intel Core i9-13900H,20 logical CPUs,34,048,978,944 host bytes; Windows11 amd64 and Go1.27.2. Host totals are not allocation limits. No other FaultBraid heavy work was scheduled during the measured run; the shared desktop was not an exclusive benchmark machine. Calibration, setup and GC may affect samples. No fastest-run selection or cross-host speedup claim.

| Workload | Median ns/op | Min–max | Q1–Q3 | Median B/op | Median allocs/op | States / result |
|---|---:|---:|---:|---:|---:|---|
| `BenchmarkCheck12-4` | 769.1 | 713.2–833.5 | 747–801.05 | 936 | 6 | 13.0 |
| `BenchmarkCheckOverlap12-4` | 621980 | 483582–1072601 | 577656.5–840187 | 109441 | 26 | 1911.0 |
| `BenchmarkOracleSequential12-4` | 12116 | 5516–24434 | 6756–13724.5 | 616 | 6 | — |
| `BenchmarkSearchBudgetCurve/operations-12-4` | 901436 | 162941–2320889 | 536199–1599060 | 109248 | 25 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-16-4` | 1883570 | 732965–2380987 | 807475–2104808 | 110272 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-24-4` | 1170282 | 647832–2243851 | 1099680–1419133 | 110464 | 27 | 4096.0; INCONCLUSIVE |
| `BenchmarkSearchBudgetCurve/operations-4-4` | 685 | 669.9–698.6 | 683.35–690.8 | 520 | 4 | 16.0; INVALID |
| `BenchmarkSearchBudgetCurve/operations-8-4` | 58579 | 56016–61672 | 57076.5–60088 | 54088 | 17 | 1352.0; INVALID |
| `BenchmarkModel10000-4` | 6700988 | 6264788–7230667 | 6523581–6741133 | 13164087 | 22583 | — |
| `BenchmarkScenarioReduction/baseline-run-4` | 2631625 | 2573146–3125162 | 2603507–2680389 | 2793141 | 33583 | 12.0 actions |
| `BenchmarkScenarioReduction/ddmin-4` | 4.030334e+08 | 3.708788e+08–4.419329e+08 | 3.920024e+08–4.181005e+08 | 105763824 | 1237395 | 5.0 actions |
| `BenchmarkScenarioReduction/greedy-4` | 3.692716e+08 | 9.714805e+07–3.821955e+08 | 1.462174e+08–3.790102e+08 | 99588160 | 1161911 | 5.0 actions |
| `BenchmarkTargetHealthy-4` | 2696134 | 2540738–3209993 | 2623779–2834636 | 2810051 | 33693 | 3.0 |

All raw calibrated samples and inclusive quartiles are in [benchmark summary](verification/release-a6/benchmark-summary.json). `Model10000` measures one10,000-event write/sync/crash/read execution (2,500 groups), not one event or transaction. Healthy target includes trace/history work, not request throughput. Histories hitting4,096 states remain INCONCLUSIVE. Greedy/ddmin compare one fixed12-action unsafe fixture and return5 actions; counts and bytes are in the samples. The separate CLI fixture starts with11 actions. No global minimum is claimed.

The complete1,000-seed command took22.253s; outer campaign 250.493s including build and ten selected unsafe replays. Native1/4/16 conformance took6065/6267/6096ms; this is one conformance run/profile, not seven-repeat performance evidence. Native CPU/RSS/request p50/p95/p99 are NOT RUN; aggregate timings cannot supply those tails. No production-scale extrapolation.

Targets: common12-operation checking<1s;10,000 model events<2s; ten matching replay digests and1,000 outcomes. These are finite named targets, not statistical correctness confidence.

These are original A6 measurements; cleanup does not retag them as executions on a later commit. Repeat `make benchmark` to measure the current complete checkout. Removed legacy kernel-only packets used a different workload and are not comparison data for this distributed target.
