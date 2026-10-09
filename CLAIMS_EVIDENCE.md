# Claims and evidence

No README or portfolio statement may exceed this scope.

| ID | Claim | Requirement | Evidence | Status |
|---|---|---|---|---|
| C001 | Checks bounded integer-register histories with pending writes and explicit uncertainty | FR-002/003/004 | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) | PASS |
| C002 | Re-executes the implemented disk model and detects trace changes | FR-008 | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) | PASS |
| C003 | Five selected tests detect their deliberately introduced semantic defects | NFR-003 | [20261009T183511-bc9553683f](verification/curated/20261009T183511-bc9553683f/run.json) | PASS |
| C004 | Kernel reproduces from a fresh local checkout and empty build cache | NFR-004 | [20261009T183735-e527918963](verification/curated/20261009T183735-e527918963/run.json) | PASS |
| C005 | Synthetic kernel benchmark observations in BENCHMARKS.md | PERF-001/002 | [20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json) | MEASURED |
| C006 | Three-voter Raft fault/replay/campaign behavior | FR-009/010/011/013 | Absent; do not claim | NOT RUN |
| C007 | Supported-toolchain and independent full-project release verification | SEC-003 | Absent; do not claim | NOT RUN |

No production users, production scale, deployment success, universal correctness, physical crash certification or security certification is claimed. Hashes are integrity references, not a trusted external attestation.
