# Requirement and test matrix

All24 local result rows below are backed by actual commands on A3. Separate final review and hosted CI retain their own statuses. Executed commit `007ef4ee5d47dddc8878d1c09607979b2b69c7fb`, complete source SHA256 `abad9b97745475faf3d7d26d48f9a77ee4c6b90edf190acb1de274a4fca0db8b` (276 files). Later prose/evidence curation does not rewrite that identity; execution inputs are compared by the release gate.

| Acceptance | Requirement | Locked test contract | Local result | Actual evidence |
|---|---|---|---|---|
| AC-001 | FR-001 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-002 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-003 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-004 | FR-003 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-005 | FR-004 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-006 | FR-005 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-007 | FR-006 | `internal/sim/sim_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-008 | FR-007 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-009 | FR-008 | `tests/integration/cli_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-010 | FR-012 | `internal/report/report_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-011 | FR-009 | `tests/system/raft_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-012 | FR-010 | `tests/system/raft_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-013 | REL-002 | `tests/system/persistence_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-014 | REL-003 | `tests/system/replay_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json), [20261011T170547-4066dd239e](verification/curated/20261011T170547-4066dd239e/run.json) |
| AC-015 | FR-011 | `tests/system/reducer_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json), [20261011T170547-4066dd239e](verification/curated/20261011T170547-4066dd239e/run.json) |
| AC-016 | FR-002 | `tests/differential/checker_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-017 | SEC-002 | `tests/security/limits_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| AC-018 | NFR-003 | `scripts/test_evidence.py` | PASS | [20261011T170233-b5771def95](verification/curated/20261011T170233-b5771def95/run.json) |
| AC-019 | FR-013 | `tests/system/real_process_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json), [20261011T170516-2d33cec802](verification/curated/20261011T170516-2d33cec802/run.json) |
| AC-020 | NFR-004 | `scripts/clean_verify.py` | PASS | [20261011T165127-3ea1dec7ce](verification/curated/20261011T165127-3ea1dec7ce/run.json) |
| AC-021 | SEC-003 | `scripts/public_audit.py` | PASS | [20261011T170348-a68d9c0feb](verification/curated/20261011T170348-a68d9c0feb/run.json) |
| AC-022 | PERF-001 | `internal/history/checker_test.go` | PASS | [20261011T170421-048ed5cb2f](verification/curated/20261011T170421-048ed5cb2f/run.json) |
| AC-023 | PERF-002 | `internal/sim/sim_test.go` | PASS | [20261011T170421-048ed5cb2f](verification/curated/20261011T170421-048ed5cb2f/run.json) |
| AC-024 | PERF-003 | `tests/system/campaign_test.go` | PASS | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json), [20261011T170441-46975730ac](verification/curated/20261011T170441-46975730ac/run.json) |

AC-024 uses the actual measured1,000-seed campaign as well as the32-seed package smoke test. AC-016 includes real Porcupine UNKNOWN controls. AC-019 uses native production PID/HTTP/File.Sync evidence. AC-020 is an exact committed clone/fresh cache on the same host, not a fresh VM. AC-021 final curated ZIP/history audit remains separate until executed. No behavioral test is skipped.

Five kernel mutants, skipped Store.Flush and independently authored native uncertainty/lifecycle regression controls require baseline PASS / semantic assertion FAIL / restored PASS. See [kernel](verification/release/kernel-mutations.json), [target](verification/release/target-mutation.json), [uncertainty](verification/release/process-persistence-independent-uncertainty-control.json) and [lifecycle](verification/release/native-exit-semantic-control.json). Counts describe selected controls, not an overall mutation score.
