# Locked acceptance specification

`acceptance_spec_locked: true`

Canonical behavioral data: `tests/acceptance_locked/spec.json`. Its SHA-256 is stored in `tests/acceptance_locked/spec.sha256`. Locked before implementation. Add stricter tests; do not weaken/delete criteria or rewrite the lock to make checks green. Genuine errors require a preserved TEST_SPEC_EXCEPTION_<ID>.md, evidence, an independent assessment, and explicit revision history. Hashes detect accidental changes; an actor able to rewrite both files can defeat that guard. CI review and retained package provenance remain necessary.

Product verdicts VALID/INVALID/INCONCLUSIVE are distinct from verification statuses PASS/FAIL/BLOCKED/NOT RUN/NOT YET MEASURED. Missing test files below are future required work, not skipped successes.

## AC-001 - FR-001
Precondition: history-v1 decoder.

Input: valid JSON plus unknown fields, duplicate keys, duplicate IDs, negative/overlapping own intervals. Behavior: reject malformed/ambiguous input before checking.

Expected: valid accepted; malformed returns input error, not INVALID. Tolerance: zero misclassifications.

Level: contract. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: parser fixtures + raw test JSON.

## AC-002 - FR-002
Precondition: initial register=0.

Input: write(7) completes before read returns 0. Behavior: exhaust all admissible orders.

Expected: INVALID. Tolerance: exact.

Level: acceptance. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: known-bad fixture.

## AC-003 - FR-002
Precondition: initial register=0.

Input: overlapping write(7) and read(0). Behavior: respect interval rather than serialization by completion time.

Expected: VALID with independently checked witness. Tolerance: exact.

Level: acceptance. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: witness test.

## AC-004 - FR-003
Precondition: history includes pending write(7).

Input: later completed read(7), and separately read(0). Behavior: consider inclusion or omission of pending write.

Expected: both histories VALID where real-time constraints permit. Tolerance: exact.

Level: acceptance. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: pending-operation fixtures.

## AC-005 - FR-004
Precondition: nonempty valid history.

Input: state budget=1 or cancelled context. Behavior: stop search without a semantic verdict.

Expected: INCONCLUSIVE. Tolerance: never VALID/INVALID due to budget.

Level: acceptance. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: budget and cancellation tests.

## AC-006 - FR-005
Precondition: INVALID history.

Input: bounded delete-one reductions. Behavior: retain only INVALID candidates and expose unknown checks.

Expected: output INVALID; minimal flag only after full definitive final deletion pass. Tolerance: exact.

Level: acceptance. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: rechecked reduced history.

## AC-007 - FR-006
Precondition: events with equal and unequal timestamps.

Input: enqueue/pop, overflow, late insertion. Behavior: stable order and bounded admission.

Expected: time then sequence order; errors for invalid admission. Tolerance: exact.

Level: unit. Test: `internal/sim/sim_test.go`. Evidence: queue fixtures.

## AC-008 - FR-007
Precondition: disk initially0.

Input: write7/crash/read then write9/sync/crash/read. Behavior: discard only volatile state on crash.

Expected: reads0 then9. Tolerance: exact.

Level: acceptance. Test: `tests/acceptance_locked/acceptance_test.go`. Evidence: modeled durability fixture.

## AC-009 - FR-008
Precondition: recorded disk-model trace.

Input: replay unchanged; alter output/digest/event. Behavior: execute and compare canonical trace.

Expected: unchanged matches; tampering rejected. Tolerance: exact.

Level: integration. Test: `tests/integration/cli_test.go`. Evidence: real CLI process and trace.

## AC-010 - FR-012
Precondition: valid history and generated summary.

Input: render report, attempt unsafe text/path injection. Behavior: escape text; no executable content or automatic external resource loads.

Expected: semantic HTML, no unescaped attacker script; exclusive output files. Tolerance: exact.

Level: security. Test: `internal/report/report_test.go`. Evidence: HTML and output-path tests.

## AC-011 - FR-009
Precondition: three-voter instrumented Raft target.

Input: write7, majority acknowledgement, later quorum-safe read. Behavior: use real pinned library and explicit entropy/persistence boundaries.

Expected: read7; recorded target/dependency identity. Tolerance: exact.

Level: system. Test: `tests/system/raft_test.go`. Evidence: same-target source/trace and run manifest.

## AC-012 - FR-010
Precondition: partition leader from majority.

Input: attempt writes/reads through isolated leader then heal. Behavior: do not report uncommitted writes or unsafe local reads successful.

Expected: safety holds; timeouts remain pending; eventual progress only after healing within declared schedule. Tolerance: safety exact; liveness bounded schedule-specific.

Level: fault injection. Test: `tests/system/raft_test.go`. Evidence: fault trace with majority/minority visibility.

## AC-013 - REL-002
Precondition: durable adapter with named barriers.

Input: crash at every append/flush/apply/response boundary. Behavior: recover correct committed prefix and preserve uncertainty.

Expected: no completed acknowledged write lost in declared model. Tolerance: exact.

Level: fault injection. Test: `tests/system/persistence_test.go`. Evidence: crash matrix + wrapper mutation negative control.

## AC-014 - REL-003
Precondition: fixed source/lock/seed/scenario.

Input: repeat target run10 times and replay saved trace. Behavior: control all target entropy and logical event decisions.

Expected: identical canonical logical trace digests. Tolerance: 10/10 required; no OS determinism claim.

Level: system. Test: `tests/system/replay_test.go`. Evidence: digest manifest with source/entropy identities.

## AC-015 - FR-011
Precondition: known bad target variant and failure trace.

Input: reduce scenario under budget. Behavior: rerun each candidate; preserve invariant fingerprint and original source identity.

Expected: smaller executable reproducer still fails same invariant; no global-minimal claim. Tolerance: exact fingerprint; reduction size observed.

Level: system. Test: `tests/system/reducer_test.go`. Evidence: before/after traces and replay outputs.

## AC-016 - FR-002
Precondition: independent Porcupine model and tiny exhaustive oracle.

Input: enumerate finite short histories incl pending writes, plus seeded generated histories. Behavior: compare definitive answers; preserve UNKNOWN.

Expected: no definitive disagreements; unknown counted separately. Tolerance: all enumerated cases, domain explicitly finite.

Level: property/differential. Test: `tests/differential/checker_test.go`. Evidence: enumeration counts and discrepancy corpus.

## AC-017 - SEC-002
Precondition: bounded parser/checker/scheduler.

Input: large inputs, high nesting, maximum state budget, cancellation. Behavior: reject or terminate within configured resource limits.

Expected: no panic, uncontrolled allocation or false success. Tolerance: configured bounds, measured RSS supplemental.

Level: fuzz/security. Test: `tests/security/limits_test.go`. Evidence: fuzz logs and limits table.

## AC-018 - NFR-003
Precondition: clean or accurately recorded dirty source.

Input: run success and intentional failure commands. Behavior: capture real exit, stdout/stderr hashes and runtime fingerprint.

Expected: manifest matches files and actual exit status. Tolerance: exact.

Level: integration. Test: `scripts/test_evidence.py`. Evidence: machine-generated run manifest.

## AC-019 - FR-013
Precondition: three real loopback child processes.

Input: write/read/kill/restart selected child. Behavior: same integration state machine, real sockets and journal fsync.

Expected: conformance invariants hold; OS scheduling limitation visible. Tolerance: no exact real-process timing guarantee.

Level: system. Test: `tests/system/real_process_test.go`. Evidence: real PID/socket/disk fixture and cleanup evidence.

## AC-020 - NFR-004
Precondition: fresh committed checkout.

Input: install/build/lint/tests/demo/teardown. Behavior: execute from clean checkout without hidden inputs.

Expected: all implemented checks pass; full release separately requires all criteria. Tolerance: exact.

Level: reproducibility. Test: `scripts/clean_verify.py`. Evidence: fresh path and commit/source manifest.

## AC-021 - SEC-003
Precondition: explicit public export allowlist.

Input: inspect tracked files, history, archives and evidence. Behavior: reject private paths and detected sensitive content.

Expected: only allowlisted source/tests/docs/synthetic artifacts exported. Tolerance: zero known leaks; scanner limitations stated.

Level: release. Test: `scripts/public_audit.py`. Evidence: release-readiness inventory.

## AC-022 - PERF-001
Precondition: documented benchmark host.

Input: common 12-operation histories, repeated samples. Behavior: measure state count, duration, allocation and variability.

Expected: report observed values separately from 1-second TARGET. Tolerance: target miss is reported, not hidden.

Level: performance. Test: `internal/history/checker_test.go`. Evidence: raw benchmark files.

## AC-023 - PERF-002
Precondition: documented benchmark host.

Input: 10000 model events across repetitions. Behavior: measure scheduler/disk workload not network DB throughput.

Expected: report observed values separately from2-second TARGET. Tolerance: target miss recorded.

Level: performance. Test: `internal/sim/sim_test.go`. Evidence: raw benchmark files.

## AC-024 - PERF-003
Precondition: working Raft target.

Input: 1000 deterministic seeds,10 replays on selected failures. Behavior: record coverage, failures and inconclusive checks.

Expected: all runs counted; no claim of exhaustive correctness. Tolerance: zero omitted failures.

Level: campaign. Test: `tests/system/campaign_test.go`. Evidence: campaign index and raw runs.
