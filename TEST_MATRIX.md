# Requirement and test matrix

All24 applicable local rows retain actual corrected-source A6 commands. Independent all24 assessment passed on C5; recorded pre-cleanup C7 live full Windows verification, actual Linux kernel/full/CodeQL workflows, exact publication and anonymous contents passed. See [last verified pre-cleanup jobs](verification/publication/last-verified-ci.json). Later metadata changes no execution inputs; source-bound rows below are not retagged.

Executed commit `65d10e1c83950f97d0857b3145ba9f91fe848e2f`, complete source SHA256 `89a9af213b2c131854762a0e312108fbd90bba07bee9754a3a611327f4a60f39` (1512 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

| Acceptance | Requirement | Locked contract | Local result | Evidence |
|---|---|---|---|---|
| AC-001 | FR-001 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-002 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-003 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-004 | FR-003 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-005 | FR-004 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-006 | FR-005 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-007 | FR-006 | `internal/sim/sim_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-008 | FR-007 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-009 | FR-008 | `tests/integration/cli_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-010 | FR-012 | `internal/report/report_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-011 | FR-009 | `tests/system/raft_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-012 | FR-010 | `tests/system/raft_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-013 | REL-002 | `tests/system/persistence_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-014 | REL-003 | `tests/system/replay_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json), [20261012T175252-97054b8391](verification/curated/20261012T175252-97054b8391/run.json) |
| AC-015 | FR-011 | `tests/system/reducer_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json), [20261012T175252-97054b8391](verification/curated/20261012T175252-97054b8391/run.json) |
| AC-016 | FR-002 | `tests/differential/checker_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-017 | SEC-002 | `tests/security/limits_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json) |
| AC-018 | NFR-003 | `scripts/test_evidence.py` | PASS | [20261012T173653-5e5eaec209](verification/curated/20261012T173653-5e5eaec209/run.json) |
| AC-019 | FR-013 | `tests/system/real_process_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json), [20261012T175046-f884daa970](verification/curated/20261012T175046-f884daa970/run.json) |
| AC-020 | NFR-004 | `scripts/clean_verify.py` | PASS | [20261012T170911-acbca9c7a0](verification/curated/20261012T170911-acbca9c7a0/run.json) |
| AC-021 | SEC-003 | `scripts/public_audit.py` | PASS | [20261012T174000-2fc097e0ec](verification/curated/20261012T174000-2fc097e0ec/run.json) |
| AC-022 | PERF-001 | `internal/history/checker_test.go` | PASS | [20261012T174444-800c31eb52](verification/curated/20261012T174444-800c31eb52/run.json) |
| AC-023 | PERF-002 | `internal/sim/sim_test.go` | PASS | [20261012T174444-800c31eb52](verification/curated/20261012T174444-800c31eb52/run.json) |
| AC-024 | PERF-003 | `tests/system/campaign_test.go` | PASS | [20261012T173406-6e96970fb7](verification/curated/20261012T173406-6e96970fb7/run.json), [20261012T174559-dbbc24c104](verification/curated/20261012T174559-dbbc24c104/run.json) |

AC-024 includes the actual1,000-seed campaign and32-seed package smoke test. AC-016 retains genuine UNKNOWN controls. AC-019 runs production native processes and File.Sync. AC-020 is a fresh committed clone/cache on the same host. AC-021 requires the final public export audit separately. Five corrected byte-exact kernel controls and the Store.Flush control are linked in [VERIFICATION_REPORT](VERIFICATION_REPORT.md). Original A3 kernel byte-hash/restoration defects remain labelled historical. No locked assertion is weakened or skipped.

Original source-bound results remain historical after authorized metadata/history cleanup. Executable inputs and locked assertions are unchanged. Generate fresh same-source traces with `make demo`; old recorded commit hashes are not checkout instructions. See [evidence contents](verification/README.md).
