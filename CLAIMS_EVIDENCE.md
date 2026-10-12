# Claims and evidence

Executed commit `9bdbf63ae0236b7cc8649f2867293d1e2109efc6`, complete source SHA256 `8dbf60c32285bcadea6429cc5b5a06f03fbab8a78ab82e24011574bf730a7155` (963 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

| Claim | Actual evidence | Scope |
|---|---|---|
| Pending-write register checking and explicit uncertainty | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) | Bounded constructive/exhaustive search only |
| Explicit entropy and conservative Ready/Store durability in real Raft | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json); [patch lock](third_party/raft-patch/lock.json) | Instrumented etcd/raft v3.7.0, three fixed voters |
| Virtual faults, target replay and causal reduction | [20261011T195531-5258857361](verification/curated/20261011T195531-5258857361/run.json) | Finite input-bound execution; bounded1-minimal, no global minimum |
| Independent oracle / linked Porcupine comparison | [observations](verification/release-a5/test-summary.json) | Finite comparisons with UNKNOWN retained |
| 1,000 outcomes / ten selected unsafe matching replays | [20261011T195324-b9af34a98f](verification/curated/20261011T195324-b9af34a98f/run.json) | Complete finite campaign accounting; deliberately unsafe variant distinct |
| Same target in real owned loopback processes and journals | [20261011T195425-81550f3722](verification/curated/20261011T195425-81550f3722/run.json) | Observed concurrency1/4/16; no physical crash or native replay proof |
| Supported toolchain and clean reproduction | [20261011T193327-e0f119c876](verification/curated/20261011T193327-e0f119c876/run.json); [20261011T195056-652385607d](verification/curated/20261011T195056-652385607d/run.json) | Exact committed clone and fresh cache; same host/module cache |
| Actual byte-preserving semantic mutation controls | [controls](verification/release-a5/kernel-mutations.json) | Five selected controls, not an overall mutation score |
| Measured checker/model/target/reduction costs | [BENCHMARKS](BENCHMARKS.md) | All91 calibrated samples; uncertainty and spread retained |
| Separate current final review / hosted CI / publication | PASS on C3 / initial C2 FAIL before tests; repair rerun pending / PASS on C2 | Local commands do not imply these gates; anonymous initial exact remote verification passed |

No production users, scale, cloud deployment, universal correctness or security/physical crash certification is claimed. Original A3/B1 receipts and rejected byte evidence remain historical.


The [independent current all24 assessment](verification/reviews/final-c3-assessment.json) is a separate read-only review of all24 locked criteria, complete current packet and exact C3 export. Original C1/C2 and rejected B1 results remain historical with unchanged source identities. Actual live full verification, final integrated export and repaired hosted execution are separate pending gates.
