# Requirement and test matrix

Statuses are generated from the inspected execution records; missing full-project criteria are not passed by kernel fixtures.

| Acceptance | Requirement | Test contract | Status | Evidence / limitation |
|---|---|---|---|---|
| AC-001 | FR-001 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-002 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-003 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-004 | FR-003 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-005 | FR-004 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-006 | FR-005 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-007 | FR-006 | `internal/sim/sim_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-008 | FR-007 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-009 | FR-008 | `tests/integration/cli_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-010 | FR-012 | `internal/report/report_test.go` | PASS | [20261009T183455-ec6edb2cb7](verification/curated/20261009T183455-ec6edb2cb7/run.json) |
| AC-011 | FR-009 | `tests/system/raft_test.go` | NOT RUN | Required full-target behavior has not been executed. |
| AC-012 | FR-010 | `tests/system/raft_test.go` | NOT RUN | Required full-target behavior has not been executed. |
| AC-013 | REL-002 | `tests/system/persistence_test.go` | NOT RUN | Required full-target behavior has not been executed. |
| AC-014 | REL-003 | `tests/system/replay_test.go` | NOT RUN | Required full-target behavior has not been executed. |
| AC-015 | FR-011 | `tests/system/reducer_test.go` | NOT RUN | Required full-target behavior has not been executed. |
| AC-016 | FR-002 | `tests/differential/checker_test.go` | NOT RUN | 9,724-case local oracle passed, but required Porcupine integration remains NOT RUN; criterion is not passed. |
| AC-017 | SEC-002 | `tests/security/limits_test.go` | PASS | [20261009T183457-ceed810e39](verification/curated/20261009T183457-ceed810e39/run.json) |
| AC-018 | NFR-003 | `scripts/test_evidence.py` | PASS | [20261009T183437-2fd67af1c6](verification/curated/20261009T183437-2fd67af1c6/run.json) |
| AC-019 | FR-013 | `tests/system/real_process_test.go` | NOT RUN | Required full-target behavior has not been executed. |
| AC-020 | NFR-004 | `scripts/clean_verify.py` | PASS | [20261009T183735-e527918963](verification/curated/20261009T183735-e527918963/run.json) |
| AC-021 | SEC-003 | `scripts/public_audit.py` | PASS | [Inventory/history/negative-control audit](verification/public-inventory-audit.json); final ZIP checked against exact inventory; no online scanner claim |
| AC-022 | PERF-001 | `internal/history/checker_test.go` | PASS | [20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json) |
| AC-023 | PERF-002 | `internal/sim/sim_test.go` | PASS | [20261009T183516-11cad33b9f](verification/curated/20261009T183516-11cad33b9f/run.json) |
| AC-024 | PERF-003 | `tests/system/campaign_test.go` | NOT RUN | Required full-target behavior has not been executed. |

## Negative controls

| Invariant | Deliberate defect | Expected failure |
|---|---|---|
| Real-time order | Ignore predecessor ordering | TestSequentialViolation |
| Pending writes | Drop write effect | TestPendingWrite |
| Search budget | Convert unknown to VALID | TestBudget |
| Modeled durability | Preserve volatile value on crash | TestCrashDurability |
| Replay | Trust stored output | TestReplayReexecutes |

See [mutation-summary.json](verification/curated/mutation-summary.json) for actual baseline, mutant and restored run IDs. No mutation was left in the public source.
