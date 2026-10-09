# Verification report

**Kernel: PASS on the recorded legacy toolchain. Full project: incomplete. Supported-release toolchain and independent review: NOT RUN.**

Research date: 2026-10-10, Asia/Singapore. Raw timestamps use UTC on October9, which is October10 in Singapore. Original machine records and stdout/stderr bytes are preserved in [curated/index.json](verification/curated/index.json); paths in original records identify the execution location, while the relocated streams sit beside each record. SHA256 verifies stream integrity, not a trusted attestation against an author who can rewrite the whole bundle.

| Executed check | Result | Evidence |
|---|---|---|
| Build, formatting, vet, compile, all Go/Python tests, race detector, inventory | PASS | [20261009T183437-2fd67af1c6](verification/curated/20261009T183437-2fd67af1c6/run.json) |
| Go JSON tests: 25 top-level test/fuzz-target records, 37 including subtests/corpus,8packages,0skips | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| Finite independent oracle: 9,724 histories, <=3operations,2values,3interval templates | PASS in that finite domain | same structured-tests stream |
| History fuzz:22,619 executions; replay fuzz:59,774 executions,3s requested each | PASS for these bounded campaigns | [20261009T183457-ceed810e39](verification/curated/20261009T183457-ceed810e39/run.json) |
| Five selected semantic mutations: baseline PASS / mutant assertion FAIL / restored PASS | PASS; not overall mutation coverage | [20261009T183511-bc9553683f](verification/curated/20261009T183511-bc9553683f/run.json); [detail](verification/curated/mutation-summary.json) |
| Seven-repetition synthetic microbenchmarks | MEASURED | [20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json); [results](BENCHMARKS.md) |
| Real CLI local demo generates histories, traces and HTML | PASS | [20261009T183521-10c48d8638](verification/curated/20261009T183521-10c48d8638/run.json) |
| Fresh local committed checkout, empty build cache, build/tests/race/demo/teardown | PASS; same host/toolchain, not a fresh VM | [20261009T183735-e527918963](verification/curated/20261009T183735-e527918963/run.json) |
| Supported-toolchain online vulnerability scan prerequisite | FAIL / environment BLOCKED | [20261009T183606-e06a94f610](verification/curated/20261009T183606-e06a94f610/run.json) |
| Full-release prerequisite gate | FAIL as required for an unfinished product | [20261009T183643-97be6f78b7](verification/curated/20261009T183643-97be6f78b7/run.json) |

## Limits on the result
No instrumented Raft target, Porcupine comparison, causal failure reducer, real three-process conformance,1000-seed distributed campaign, supported Go1.27.2 run, GitHub CI run, graphical-browser QA, online dependency scan or separate-agent review was completed. No cloud deployment or GitHub publication occurred. The static report can be opened locally at [invalid-history.html](docs/demo/invalid-history.html); HTML escaping/output guards were tested, but this is not a browser accessibility audit.

The finite oracle uses optional-subset and full-permutation enumeration rather than copying the production DFS. This provides an independent algorithm, not an independent organizational reviewer. Mutation targets cover real-time order, pending-write effect, budget uncertainty, crash durability and replay re-execution.

## Source provenance
Most kernel runs used `1ff0c7e093fee8f552c889cb8cf9c9ac59f4efcf`. The fresh-checkout run after adding demo/teardown used `1394f5579d6bbc4f1cf77296608545326a6a999b`. Subsequent curation and documentation do not imply those earlier commits contained the evidence reports themselves. Source-file digests in each record support exact comparison. All generated fixtures are synthetic.
