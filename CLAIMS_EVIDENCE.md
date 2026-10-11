# Claims and evidence

Executed commit `2e8379a7ee18c388d31b6b963c67ed04faa5a41e`, complete source SHA256 `53eb9fa9ddeca5e426a7b2eab34976b5ddd72e555eddaecb11125101ad4db86a` (622 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

| Claim | Actual evidence | Scope |
|---|---|---|
| Pending-write register checking and explicit uncertainty | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) | Bounded constructive/exhaustive search only |
| Explicit entropy and conservative Ready/Store durability in real Raft | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json); [patch lock](third_party/raft-patch/lock.json) | Instrumented etcd/raft v3.7.0, three fixed voters |
| Virtual faults, target replay and causal reduction | [20261011T175945-40cbff969b](verification/curated/20261011T175945-40cbff969b/run.json) | Finite input-bound execution; bounded1-minimal, no global minimum |
| Independent oracle / linked Porcupine comparison | [observations](verification/release-a4/test-summary.json) | Finite comparisons with UNKNOWN retained |
| 1,000 outcomes / ten selected unsafe matching replays | [20261011T175828-1b131e43d7](verification/curated/20261011T175828-1b131e43d7/run.json) | Complete finite campaign accounting; deliberately unsafe variant distinct |
| Same target in real owned loopback processes and journals | [20261011T175911-780718a2e3](verification/curated/20261011T175911-780718a2e3/run.json) | Observed concurrency1/4/16; no physical crash or native replay proof |
| Supported toolchain and clean reproduction | [20261011T174337-073ea8eb96](verification/curated/20261011T174337-073ea8eb96/run.json); [20261011T175708-5af6bac592](verification/curated/20261011T175708-5af6bac592/run.json) | Exact committed clone and fresh cache; same host/module cache |
| Actual byte-preserving semantic mutation controls | [controls](verification/release-a4/kernel-mutations.json) | Five selected controls, not an overall mutation score |
| Measured checker/model/target/reduction costs | [BENCHMARKS](BENCHMARKS.md) | All91 calibrated samples; uncertainty and spread retained |
| Separate final review / hosted CI / publication | PASS / NOT RUN / NOT RUN | Local commands do not imply these gates |

No production users, scale, cloud deployment, universal correctness or security/physical crash certification is claimed. Original A3/B1 receipts and rejected byte evidence remain historical.


The [independent all 24 assessment](verification/reviews/final-c1-assessment.json) is a separate read-only review, scoped to all 24 locked criteria and the exact C1 packet/export. FV-01–04 are resolved; original rejected assessments and failures remain inspectable. Live full verification and hosted/publication results must be recorded separately.
