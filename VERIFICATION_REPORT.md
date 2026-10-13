# Verification report

All 24 locked criteria have original source-bound acceptance evidence and a genuinely separate full-project assessment. The retained A6 measurements below ran on Go 1.27.2 Windows; later metadata cleanup does not retag those executions. Every executable input, test, dependency pin and release-gate implementation remains unchanged.

The last pre-cleanup published head `aba5038d3068d6070a0b1aba909c199207676835` completed all three Linux workflows successfully. [Actual workflow/job/step observations](verification/publication/last-verified-ci.json) and [CodeQL analysis/alerts](verification/publication/last-verified-codeql.json) retain that head and observation date. Current-head checks are visible in [GitHub Actions](https://github.com/C-X1an/faultbraid/actions); a prior result or dispatch receipt does not claim a later execution passed.

Original measured A6 source: `65d10e1c83950f97d0857b3145ba9f91fe848e2f`, complete tree SHA-256 `89a9af213b2c131854762a0e312108fbd90bba07bee9754a3a611327f4a60f39`. These are recorded historical identities, not checkout instructions after the authorized history cleanup.

| Executed check | Result and scope | Inspectable evidence |
|---|---|---|
| Fresh committed reproduction | PASS; all ten clean steps, fresh build cache on the same host with declared module cache | [command](verification/curated/20261012T170911-acbca9c7a0/run.json), [summary](verification/release-a6/clean-summary.json) |
| Complete Go JSON race tests | 201 passed test/subtest records, 108 top-level tests, 15 packages with tests; zero failures | [command](verification/curated/20261012T173406-6e96970fb7/run.json), [observations](verification/release-a6/test-summary.json) |
| Python tooling/security tests | 83 tests PASS | [command](verification/curated/20261012T173653-5e5eaec209/run.json) |
| Supported toolchain | Actual Go1.27.2 windows/amd64 PASS | [command](verification/curated/20261012T173919-435d881ee0/run.json) |
| Public content/history audit | PASS on its recorded candidate; each later export requires another audit | [command](verification/curated/20261012T174000-2fc097e0ec/run.json) |
| Four bounded fuzz targets | PASS, requested 3s each, parallel2; finite | [command](verification/curated/20261012T171840-9f315ad8c2/run.json) |
| Five kernel mutation controls | PASS / named semantic assertion FAIL / PASS, exact site and whole-copy byte restoration | [controls](verification/release-a6/kernel-mutations.json) |
| Skipped Store.Flush control | PASS / semantic durability FAIL / PASS; exact restoration | [control](verification/release-a6/target-mutation.json) |
| Online dependency/vulnerability check | PASS on actual pinned identities; finite configured scope | [command](verification/curated/20261012T172444-63e00446d0/run.json) |
| Three-way differential checking | 47,989 enumerated, 1,024 generated, nine named and 34 real-target histories; no definitive disagreement; UNKNOWN controls retained | [observations](verification/release-a6/test-summary.json) |
| Fault campaign | 1,000 requested/attempted/VALID, zero errors, unknown or unrun; finite schedules | [command](verification/curated/20261012T174559-dbbc24c104/run.json), [all outcomes](verification/release-a6/campaign-histories.jsonl) |
| Deliberately unsafe minority read | INVALID, ten matching replays; labelled diagnostic wrapper | [original trace](verification/release-a6/campaign-selected-unsafe-trace.json) |
| Replay and executable reduction | Safe VALID, unsafe INVALID; 11 to five actions, 24 candidate checks/four replays; deletion one-minimal | [command](verification/curated/20261012T175252-97054b8391/run.json), [reduction](verification/release-a6/demo/reduced/reduction.json) |
| Native production processes | Observed peak concurrency1/4/16, four distinct PIDs/profile, actual HTTP/synced journals and cleanup | [command](verification/curated/20261012T175046-f884daa970/run.json), [summary](verification/release-a6/native-summary.json) |
| Repeated benchmarks | All 91 samples, seven repetitions/13 groups, inclusive quartiles and spread | [command](verification/curated/20261012T174444-800c31eb52/run.json), [analysis](BENCHMARKS.md) |
| Browser inspection | Recorded360x800/1280x900 keyboard/scroll checks; unchanged report implementation, bounded scope | [receipt](verification/release/browser-qa.json) |
| Independent assessment | Genuine separate C5 review of all24 criteria, substantive replay/provenance/rejection checks and resolved findings | [authored assessment](verification/reviews/final-c5-assessment.json) |

The sole package-level skip is `internal/campaign [no test files]`, with no named Test field; system tests execute its behavior. No acceptance assertion was skipped. Budget exhaustion, cancellation, Porcupine timeout and unsupported timestamp projections preserve INCONCLUSIVE/UNKNOWN. No production mutant remains.

## Material failures retained

| Original failure | Resolution/evidence |
|---|---|
| First1,000 campaign:998 VALID/two ERROR at365/854 | General contingent-action generator fix, no seed exceptions. [Original](verification/curated/20261011T074443-a404a14eef/run.json); complete corrected campaign above. |
| CLI/version, native cancellation and Python lifecycle fixtures | Exact-version and causal-readiness/cancellation synchronization. [CLI](verification/curated/20261011T090707-f21da3639d/run.json), [native](verification/curated/20261011T091845-62e93e3dbc/run.json), [Python](verification/curated/20261011T091921-ebe46d7c0d/run.json). |
| Truncated response lost a possibly admitted write | Retain pending uncertain admission before protocol error. [Original](verification/curated/20261011T092626-23d4610787/run.json), [independent semantic control](verification/release/process-persistence-independent-uncertainty-control.json). |
| Unexpected child exit accepted as owned stop; subsequent pipe-drain failure | Validate actual exit and continue bounded drain until kill/Wait. [Exit](verification/curated/20261011T093351-f1932c3cd3/run.json), [drain](verification/curated/20261011T152808-29f57629ae/run.json), [semantic control](verification/release/native-exit-semantic-control.json). |
| Shallow history/cold CodeQL bootstrap assumptions | Complete-history checkout and explicit build preparation. [Original controls](verification/release/independent-ci-before-fix.json). |
| Graft metadata hid a synthetic historical token | Reject graft metadata/environment before trusting ancestry. [Original erroneous-PASS control](verification/release/independent-graft-before-fix.json), [regressions](verification/curated/20261011T165401-97875d41af/run.json). |
| Original mutation byte/restoration receipts were inaccurate | Original receipts remain rejected evidence; corrected byte-exact controls are above. [Original](verification/release/kernel-mutations.json), [independent before-fix comparison](verification/release-a4/fv04-before-fix.json). |
| Native path ordering changed the prepared module aggregate across OSes | Casefolded component order preserves every prepared byte and pin. [Original diagnosis](verification/reviews/linux-bootstrap-order-diagnosis.json), [semantic PASS/FAIL/PASS controls](verification/reviews/linux-bootstrap-repair-controls.json), [actual initial Linux failures](verification/publication/initial-c2/hosted-runs.json). |
| Linux directory test wrongly required regular-file0600 | Exact prospective [test-spec exception](TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND.md), immutable [kernel](verification/publication/c4/kernel-job-114840844595.txt)/[full](verification/publication/c4/full-job-114842092271.txt) failed logs. Locked criteria and owner-only protections unchanged. Later Linux jobs above passed. |

## Scope and reproduction

The selected records retain their original dates, sources, statuses, stream hashes and declared path/line-ending transformations. All benchmark repetitions and campaign outcomes remain public. Redundant earlier packets and operational setup/curation captures are archived off-repository; no failure is changed to PASS. The original authored independent judgment is unchanged. Three source-copy/syntax-only bodies omitted from its prerequisite list had explicit zero acceptance credit in that assessment.

Saved A6 traces are historical inspection artifacts. Full-source identity includes metadata, so the cleaned build must reject them. Generate current traces with `make demo`, replay/reduce them with the same identified binary, and run `make verify`/`make clean-verify` to execute current checks. Do not retag an old trace or claim the rewritten Git hash was the original execution hash.

Native repeated CPU/RSS/request p50/p95/p99, production scale/users, optional hosting and physical power-loss certification remain NOT RUN. Snapshots, compaction and dynamic membership are unsupported. Finite campaigns, fuzzing, scanners and static analysis establish no universal correctness/security guarantee. [Evidence index](verification/README.md), [test mapping](TEST_MATRIX.md) and [limitations](LIMITATIONS.md) give their precise scopes.
