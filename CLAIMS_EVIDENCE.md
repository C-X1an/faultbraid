# Claims and evidence

Executed commit `65d10e1c83950f97d0857b3145ba9f91fe848e2f`, complete source SHA256 `89a9af213b2c131854762a0e312108fbd90bba07bee9754a3a611327f4a60f39` (1512 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

| Claim | Actual evidence | Scope |
|---|---|---|
| Pending-write register checking and explicit uncertainty | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) | Bounded constructive/exhaustive search only |
| Explicit entropy and conservative Ready/Store durability in real Raft | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json); [patch lock](third_party/raft-patch/lock.json) | Instrumented etcd/raft v3.7.0, three fixed voters |
| Virtual faults, target replay and causal reduction | [20261012T175252-97054b8391](verification/curated/20261012T175252-97054b8391/run.json) | Finite input-bound execution; bounded1-minimal, no global minimum |
| Independent oracle / linked Porcupine comparison | [observations](verification/release-a6/test-summary.json) | Finite comparisons with UNKNOWN retained |
| 1,000 outcomes / ten selected unsafe matching replays | [20261012T174559-dbbc24c104](verification/curated/20261012T174559-dbbc24c104/run.json) | Complete finite campaign accounting; deliberately unsafe variant distinct |
| Same target in real owned loopback processes and journals | [20261012T175046-f884daa970](verification/curated/20261012T175046-f884daa970/run.json) | Observed concurrency1/4/16; no physical crash or native replay proof |
| Supported toolchain and clean reproduction | [20261012T170911-acbca9c7a0](verification/curated/20261012T170911-acbca9c7a0/run.json); [20261012T173919-435d881ee0](verification/curated/20261012T173919-435d881ee0/run.json) | Exact committed clone and fresh cache; same host/module cache |
| Actual byte-preserving semantic mutation controls | [controls](verification/release-a6/kernel-mutations.json) | Five selected controls, not an overall mutation score |
| Measured checker/model/target/reduction costs | [BENCHMARKS](BENCHMARKS.md) | All91 calibrated samples; uncertainty and spread retained |
| Separate current final review / hosted CI / publication | A6 approval pending / C4 bootstrap and CodeQL PASS, kernel/full FAIL; A6 retests pending / C4 published PASS | Local commands do not imply these gates; anonymous initial exact remote verification passed |

No production users, scale, cloud deployment, universal correctness or security/physical crash certification is claimed. Original A3/B1 receipts and rejected byte evidence remain historical.
