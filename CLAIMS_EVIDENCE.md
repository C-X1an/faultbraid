# Claims and evidence

Public statements must remain within the scope of these actual results. Executed commit `007ef4ee5d47dddc8878d1c09607979b2b69c7fb`, complete source SHA256 `abad9b97745475faf3d7d26d48f9a77ee4c6b90edf190acb1de274a4fca0db8b` (276 files). Later prose/evidence curation does not rewrite that identity; execution inputs are compared by the release gate.

| Claim | Evidence | Scope |
|---|---|---|
| Bounded integer-register checking, pending write effects and honest uncertainty | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) | Constructive/exhaustive verdicts only within declared budgets. |
| Explicit election entropy and conservative Ready/Store durability around real Raft | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json), [patch lock](third_party/raft-patch/lock.json) | Instrumented etcd/raft v3.7.0, three fixed voters. |
| Virtual faults, source-bound target replay and executable scenario reduction | [20261011T170547-4066dd239e](verification/curated/20261011T170547-4066dd239e/run.json), [reduction](verification/release/demo/reduced/reduction.json) | Finite scenarios; bounded1-minimal, no global-minimum claim. |
| Agreement with an independent permutation oracle and real Porcupine | [differential observations](verification/release/test-summary.json) |47,989 enumerated/1,024 generated/34 actual-target histories; explicit UNKNOWN limits. |
| Measured1,000 seed outcomes and10 matching selected unsafe replays | [campaign](verification/release/campaign-summary.json), [all histories](verification/release/campaign-histories.jsonl) | Finite schedule/workload coverage; unsafe mode is deliberate diagnostic behavior. |
| Same integration in actual owned loopback processes and checksummed journals | [native artifacts](verification/release/native-summary.json) | Observed concurrency1/4/16; no physical crash guarantee/native replay. |
| Supported Windows toolchain and clean committed reproduction | [20261011T165127-3ea1dec7ce](verification/curated/20261011T165127-3ea1dec7ce/run.json), [20261011T170345-f9cecc1fdf](verification/curated/20261011T170345-f9cecc1fdf/run.json) | Fresh build cache, same host/shared verified module cache. |
| Measured checker/model/target/reduction costs | [BENCHMARKS](BENCHMARKS.md) | All7 samples per13 groups; INCONCLUSIVE cases retained. |
| Separate full-project final review / hosted CI / publication | Pending / NOT RUN / NOT RUN | Do not infer these from local tests. |

No production users, production scale, cloud deployment, universal correctness, physical power-loss certification or security certification is claimed. Hashes are integrity references, not trusted external attestations. Original legacy kernel records are historical evidence; current workbench claims use the recorded A3 commands above.
