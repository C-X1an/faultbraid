# Evaluation and evidence
A passing seed suite is not full-project acceptance. Canonical criteria live in ACCEPTANCE_SPEC.md. Product verdicts and test statuses are different: a correct test may PASS because the checker returned INVALID for a known-bad history.

## Layers
Unit tests cover register transitions, real-time predecessor masks, queue ordering, disk-stage/flush/crash semantics and codecs. Contract tests enforce versioned JSON and error behavior. Integration/E2E tests launch the actual CLI, create exclusive outputs, replay traces and inspect HTML. Locked acceptance uses independently specified known-good/bad histories, pending writes and exhausted budgets. Property tests compare finite enumerations to a separately written permutation oracle. Full implementation additionally compares to Porcupine, exercises the instrumented Raft target, real processes, and campaign/fault matrices. Fuzz parser/history/trace entrypoints with size limits. Race detection is useful for host tooling but does not substitute for distributed consistency tests.

## Test-the-tests protocol
Run correct code/test in an isolated copy; record PASS. Apply one targeted defect (ignore real-time order, drop a pending write, report budget exhaustion as valid, preserve volatile state across crash, or trust tampered trace). Run the SAME behavioral test and require a semantic assertion failure, not compilation failure. Preserve stdout/stderr/exit/hash. Remove the disposable copy, verify source hashes unchanged and rerun baseline. A surviving mutant is a finding; do not delete it to inflate a score. Seed mutation counts describe only the explicit mutants actually run, not a general mutation-coverage percentage.

## Independent verification
A different agent/session begins read-only, receives specs/source/commands but not implementation rationalizations, and writes findings before fixes. Ask what can be broken while all tests stay green; whether pending operations are treated honestly; whether there are mocks masquerading as integration; whether the same target code runs in both IO modes; and whether shrink/replay prove the advertised property. The final integrator fixes validated findings and reruns affected gates. A self-review in this package is not labeled an independent review.

## Anti-fake rules
Never weaken or delete failing locked criteria, change a test to match broken code, silently skip/xfail/quarantine, swallow errors, handwrite terminal output, call a stub live, cherry-pick benchmarks or claim missing checks passed. Test source must not duplicate the implementation as the sole oracle. Genuine criterion errors get a preserved exception record and independent assessment. Every high-criticality requirement maps to tests and negative controls in TEST_MATRIX.md.

## Machine-captured runs
`scripts/run_evidence.py --category <category> --requirements <ID> -- <command ...>` executes without a shell, captures actual stdout/stderr, exit code and duration, start/end UTC, source SHA/dirty state/tree hash, runtime/platform, command and working directory. Raw files and their SHA256 hashes are stored under ignored `.local-runs/evidence/`; full execution manifests remain private unless reviewed as synthetic public evidence. A zero exit code records command PASS, not unearned acceptance of all requirements. Evidence is reproducible provenance, not cryptographic proof of an honest runner.

## Clean environment
`scripts/clean_verify.py` requires reviewed clean committed source, makes a real local clone with complete history and no hardlinks, compares every source-file byte, and uses a fresh build cache. It runs verify-seed,fuzz,mutation,benchmark,online security,campaign,native lab,demo and teardown, retaining the checkout and artifacts. This detects hidden workspace/build-cache dependencies but shares the host OS/toolchain and verified module cache. Hosted Linux runtime/CodeQL or a disposable VM is a separate check; a second directory is not a clean VM/container.

## Saved source-bound traces

The single retained `verification/release-a6/` packet records the reviewed output-kind correction and its original measurements. Its source JSON, command records, saved traces and hashes are historical evidence and remain unchanged. Public history cleanup removes obsolete packets and operational files; the original complete source trees and old commit identifiers cannot be reconstructed from the cleaned repository.

To execute a reproducer, run `make demo` or `simulate` with an example scenario on the current checkout, then replay or reduce the newly generated trace with the same identified binary. Full source identity includes ordinary documentation and evidence files: a metadata cleanup changes that identity even when executable inputs are identical. The binary must reject a historical A6 trace with a different identity. Do not edit hashes, weaken replay checks or describe the saved historical traces as current-build replays. Native mode has no replay.
