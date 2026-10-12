# Requirement and test matrix

All24 applicable local rows use actual corrected-source A5 commands. Independent current all24 assessment passed on C3; repaired hosted CI remains a separate pending gate. Executed commit `9bdbf63ae0236b7cc8649f2867293d1e2109efc6`, complete source SHA256 `8dbf60c32285bcadea6429cc5b5a06f03fbab8a78ab82e24011574bf730a7155` (963 files). Later prose/evidence curation preserves original identities; the gate compares every execution input.

| Acceptance | Requirement | Locked contract | Local result | Evidence |
|---|---|---|---|---|
| AC-001 | FR-001 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-002 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-003 | FR-002 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-004 | FR-003 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-005 | FR-004 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-006 | FR-005 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-007 | FR-006 | `internal/sim/sim_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-008 | FR-007 | `tests/acceptance_locked/acceptance_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-009 | FR-008 | `tests/integration/cli_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-010 | FR-012 | `internal/report/report_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-011 | FR-009 | `tests/system/raft_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-012 | FR-010 | `tests/system/raft_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-013 | REL-002 | `tests/system/persistence_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-014 | REL-003 | `tests/system/replay_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json), [20261011T195531-5258857361](verification/curated/20261011T195531-5258857361/run.json) |
| AC-015 | FR-011 | `tests/system/reducer_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json), [20261011T195531-5258857361](verification/curated/20261011T195531-5258857361/run.json) |
| AC-016 | FR-002 | `tests/differential/checker_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-017 | SEC-002 | `tests/security/limits_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json) |
| AC-018 | NFR-003 | `scripts/test_evidence.py` | PASS | [20261011T194939-a7a9d9997e](verification/curated/20261011T194939-a7a9d9997e/run.json) |
| AC-019 | FR-013 | `tests/system/real_process_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json), [20261011T195425-81550f3722](verification/curated/20261011T195425-81550f3722/run.json) |
| AC-020 | NFR-004 | `scripts/clean_verify.py` | PASS | [20261011T193327-e0f119c876](verification/curated/20261011T193327-e0f119c876/run.json) |
| AC-021 | SEC-003 | `scripts/public_audit.py` | PASS | [20261011T195101-ab1310ba29](verification/curated/20261011T195101-ab1310ba29/run.json) |
| AC-022 | PERF-001 | `internal/history/checker_test.go` | PASS | [20261011T195258-68f9d1928b](verification/curated/20261011T195258-68f9d1928b/run.json) |
| AC-023 | PERF-002 | `internal/sim/sim_test.go` | PASS | [20261011T195258-68f9d1928b](verification/curated/20261011T195258-68f9d1928b/run.json) |
| AC-024 | PERF-003 | `tests/system/campaign_test.go` | PASS | [20261011T194819-2cfbc732da](verification/curated/20261011T194819-2cfbc732da/run.json), [20261011T195324-b9af34a98f](verification/curated/20261011T195324-b9af34a98f/run.json) |

AC-024 includes the actual1,000-seed campaign and32-seed package smoke test. AC-016 retains genuine UNKNOWN controls. AC-019 runs production native processes and File.Sync. AC-020 is a fresh committed clone/cache on the same host. AC-021 requires the final public export audit separately. Five corrected byte-exact kernel controls and the Store.Flush control are linked in [VERIFICATION_REPORT](VERIFICATION_REPORT.md). Original A3 kernel byte-hash/restoration defects remain labelled historical. No locked assertion is weakened or skipped.


The [independent current all24 assessment](verification/reviews/final-c3-assessment.json) is a separate read-only review of all24 locked criteria, complete current packet and exact C3 export. Original C1/C2 and rejected B1 results remain historical with unchanged source identities. Actual live full verification, final integrated export and repaired hosted execution are separate pending gates.


Current candidate corrects a supplemental output-kind predicate under the [independently reviewed exception](TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND.md). Production and locked acceptance bytes are unchanged. Fresh corrected-source acceptance, all24 review and hosted runtime are pending; historical full approvals cover previous test inputs. Actual C4 Windows verification, publication/anonymous contents and CodeQL passed, while Linux kernel/full failed at the original predicate. [Original C4 observations](verification/publication/c4/index.json) retain those outcomes.
