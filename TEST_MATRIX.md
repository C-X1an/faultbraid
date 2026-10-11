# Requirement and test matrix

All24 applicable local rows use actual corrected-source A4 commands. Independent final assessment passed on C1; hosted CI remains a separate pending gate. Executed commit `2e8379a7ee18c388d31b6b963c67ed04faa5a41e`, complete source SHA256 `53eb9fa9ddeca5e426a7b2eab34976b5ddd72e555eddaecb11125101ad4db86a` (622 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

| Acceptance | Requirement | Locked contract | Local result | Evidence |
|---|---|---|---|---|
| AC-001 | FR-001 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-002 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-003 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-004 | FR-003 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-005 | FR-004 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-006 | FR-005 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-007 | FR-006 | `internal/sim/sim_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-008 | FR-007 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-009 | FR-008 | `tests/integration/cli_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-010 | FR-012 | `internal/report/report_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-011 | FR-009 | `tests/system/raft_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-012 | FR-010 | `tests/system/raft_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-013 | REL-002 | `tests/system/persistence_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-014 | REL-003 | `tests/system/replay_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json), [20261011T175945-40cbff969b](verification/curated/20261011T175945-40cbff969b/run.json) |
| AC-015 | FR-011 | `tests/system/reducer_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json), [20261011T175945-40cbff969b](verification/curated/20261011T175945-40cbff969b/run.json) |
| AC-016 | FR-002 | `tests/differential/checker_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-017 | SEC-002 | `tests/security/limits_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json) |
| AC-018 | NFR-003 | `scripts/test_evidence.py` | PASS | [20261011T175556-359deaa09c](verification/curated/20261011T175556-359deaa09c/run.json) |
| AC-019 | FR-013 | `tests/system/real_process_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json), [20261011T175911-780718a2e3](verification/curated/20261011T175911-780718a2e3/run.json) |
| AC-020 | NFR-004 | `scripts/clean_verify.py` | PASS | [20261011T174337-073ea8eb96](verification/curated/20261011T174337-073ea8eb96/run.json) |
| AC-021 | SEC-003 | `scripts/public_audit.py` | PASS | [20261011T175712-a5e53254ce](verification/curated/20261011T175712-a5e53254ce/run.json) |
| AC-022 | PERF-001 | `internal/history/checker_test.go` | PASS | [20261011T175804-10e36b68c3](verification/curated/20261011T175804-10e36b68c3/run.json) |
| AC-023 | PERF-002 | `internal/sim/sim_test.go` | PASS | [20261011T175804-10e36b68c3](verification/curated/20261011T175804-10e36b68c3/run.json) |
| AC-024 | PERF-003 | `tests/system/campaign_test.go` | PASS | [20261011T175440-7751cf1b5d](verification/curated/20261011T175440-7751cf1b5d/run.json), [20261011T175828-1b131e43d7](verification/curated/20261011T175828-1b131e43d7/run.json) |

AC-024 includes the actual1,000-seed campaign and32-seed package smoke test. AC-016 retains genuine UNKNOWN controls. AC-019 runs production native processes and File.Sync. AC-020 is a fresh committed clone/cache on the same host. AC-021 requires the final public export audit separately. Five corrected byte-exact kernel controls and the Store.Flush control are linked in [VERIFICATION_REPORT](VERIFICATION_REPORT.md). Original A3 kernel byte-hash/restoration defects remain labelled historical. No locked assertion is weakened or skipped.


The [independent all 24 assessment](verification/reviews/final-c1-assessment.json) is a separate read-only review, scoped to all 24 locked criteria and the exact C1 packet/export. FV-01–04 are resolved; original rejected assessments and failures remain inspectable. Live full verification and hosted/publication results must be recorded separately.
