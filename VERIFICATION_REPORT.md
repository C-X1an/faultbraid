# Verification report

The distributed workbench passed the applicable local acceptance commands on the pinned Windows toolchain. Independent final release assessment is pending this completed packet. Hosted Linux/CodeQL runtime CI and GitHub publication are NOT RUN.

Execution date: 2026-10-12 Asia/Singapore; original command timestamps use UTC on October11. Executed commit `007ef4ee5d47dddc8878d1c09607979b2b69c7fb`, complete source SHA256 `abad9b97745475faf3d7d26d48f9a77ee4c6b90edf190acb1de274a4fca0db8b` (276 files). Later prose/evidence curation does not rewrite that identity; execution inputs are compared by the release gate.

| Actual check | Result and scope | Evidence |
|---|---|---|
| Fresh committed clone, exact bytes and fresh build cache | PASS; same host, verified shared module cache; all10 clean steps, 589.566s | [20261011T165127-3ea1dec7ce](verification/curated/20261011T165127-3ea1dec7ce/run.json); [step summary](verification/release/clean-summary.json) |
| Complete all-package Go JSON race tests | PASS; 201 passed test records,108 top-level,15 tested packages,0 failing records | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json); [structured summary](verification/release/test-summary.json) |
| Python evidence/privacy/gate/teardown regressions | PASS;81 tests | [20261011T170233-b5771def95](verification/curated/20261011T170233-b5771def95/run.json) |
| Pinned runtime Go1.27.2 windows/amd64 | PASS | [20261011T170345-f9cecc1fdf](verification/curated/20261011T170345-f9cecc1fdf/run.json) |
| Exact working/index and complete reachable-history audit | PASS on execution candidate; final curation/export audit separate | [20261011T170348-a68d9c0feb](verification/curated/20261011T170348-a68d9c0feb/run.json) |
| Four timed fuzz targets | PASS;3s requested per target, parallel2; finite campaigns | [clean fuzz](verification/curated/20261011T165615-2155047943/run.json) |
| Five kernel semantic mutants plus skipped Store.Flush | Baseline PASS / semantic assertion FAIL / restored PASS; source restored | [kernel controls](verification/release/kernel-mutations.json), [target control](verification/release/target-mutation.json) |
| Patched upstream Raft tests | PASS;all6 packages; earlier120s timeout retained | [actual command](verification/curated/20261011T160335-b4e1550b35/run.json) |
| Independent finite oracle and linked Porcupine | 47,989 enumerated,1,024 generated,9 named controls,34 actual-target histories; no definitive disagreement | [20261011T170119-6f3f0950e8](verification/curated/20261011T170119-6f3f0950e8/run.json) |
| Current-source online vulnerability scan | PASS; actual resolved govulncheck and pinned module/patch identities; not a security certification | [clean scan](verification/curated/20261011T165910-094b376cd9/run.json) |
| Measured campaign |1,000 requested/attempted/VALID,0 INVALID/INCONCLUSIVE/ERROR/unrun; all histories/results and trace hashes retained | [20261011T170441-46975730ac](verification/curated/20261011T170441-46975730ac/run.json); [summary](verification/release/campaign-summary.json), [all histories](verification/release/campaign-histories.jsonl) |
| Deliberately unsafe minority read | INVALID;10/10 actual replays matched the recorded digest | [selected trace](verification/release/campaign-selected-unsafe-trace.json); campaign evidence |
| Actual target replay/reduction CLI | Safe VALID,unsafe INVALID;11 actions reduced to5,24 candidate checks/4 replay checks; retains INVALID and bounded1-minimality | [20261011T170547-4066dd239e](verification/curated/20261011T170547-4066dd239e/run.json); [reduction](verification/release/demo/reduced/reduction.json) |
| Production real-process conformance at concurrency1/4/16 | PASS; observed peaks1/4/16,each4 distinct PIDs including restart,HTTP,journals,Wait/socket cleanup | [20261011T170516-2d33cec802](verification/curated/20261011T170516-2d33cec802/run.json); [native summary](verification/release/native-summary.json) |
| Seven measured benchmark repetitions | All91 samples/13 groups exported; variability and uncertainty preserved | [20261011T170421-048ed5cb2f](verification/curated/20261011T170421-048ed5cb2f/run.json); [BENCHMARKS](BENCHMARKS.md) |
| Targeted browser report inspection | PASS on recorded earlier source:360x800/1280x900,table scrolling,Tab/Enter/arrows; authored HTML has no script/external resources | [engineering receipt](verification/release/browser-qa.json) |

The sole Go JSON skip is `internal/campaign` at package level with **[no test files]** and no Test field. Its behavior is exercised in `tests/system`; no acceptance assertion was skipped. Actual Porcupine timeout returned Unknown (~1.618ms), and an unrepresentable timestamp returned UNKNOWN. Budget-limited cases do not become passes for correctness. The 32-seed package smoke test is supplemented by the separate measured1,000-seed campaign, which observed635,550 events,17 pending writes and43 pending reads. Named barriers fired508 times across all six boundaries; these are observed finite schedules, not exhaustive coverage.

Native runs lasted6,092/6,030/6,020ms and recorded856/884/888 successful HTTP message posts and2,328/3,651/4,092 journal bytes. Exact journal bytes are hex-encoded alongside the original length/SHA256. Native integration runs the production executable and same Raft/Store code; the synthetic child lifecycle helper is a separately labelled regression fixture. Native traces have no replay or physical power-loss guarantee.

## Preserved failures and corrections

The A3 five kernel mutation controls below executed real PASS / semantic assertion FAIL / PASS, but their Windows text writes changed disposable source line endings. Their summary hashes describe normalized text, rather than the actual mutant files, and byte restoration was not established. These original receipts remain historical evidence with this limitation. The corrected runner uses byte writes and checks the entire disposable source identity; corrected source-matched controls and final review are pending.

| Observed failure/control | Resolution and evidence |
|---|---|
| Initial1,000 campaign:998 VALID,2 ERROR (seeds365/854) due to contingent crash/restart actions | General workload generator corrected; no seed exceptions. [Original](verification/curated/20261011T074443-a404a14eef/run.json), current full campaign above. |
| CLI version fixture and timing-dependent cancellation/descendant fixture | Correct complete-version fixtures and causal readiness/cancellation synchronization; locked semantics preserved. Originals [CLI](verification/curated/20261011T090707-f21da3639d/run.json), [native cancellation](verification/curated/20261011T091845-62e93e3dbc/run.json), [Python fixture](verification/curated/20261011T091921-ebe46d7c0d/run.json). |
| Truncated HTTP response lost a possibly admitted write | Pending uncertain write retained before protocol error. [Original](verification/curated/20261011T092626-23d4610787/run.json), [independent PASS/FAIL/PASS](verification/release/process-persistence-independent-uncertainty-control.json). |
| Unexpected child exit2 was accepted as owned stop; later strict stop exposed canceled-output pipe closure | Stop validates real exit status and keeps draining bounded protocol output until kill/Wait. [Original exit](verification/curated/20261011T093351-f1932c3cd3/run.json), [drain failure](verification/curated/20261011T152808-29f57629ae/run.json), [semantic control](verification/release/native-exit-semantic-control.json). |
| Shallow Git checkout truncated required audit; cold CodeQL build lacked bootstrap | Full history checkout and explicit bootstrap build, independently rerun on A3. [Before-fix controls](verification/release/independent-ci-before-fix.json). Hosted CodeQL remains NOT RUN. |
| Legacy Git graft hid a synthetic historical token and erroneously passed old audit | Graft metadata/environment rejected before ancestry trust; actual before-fix erroneous PASS is retained as a defect, not acceptance. [Before-fix control](verification/release/independent-graft-before-fix.json), [actual current regressions](verification/curated/20261011T165401-97875d41af/run.json). |
| Upstream test command reached120s timeout | Timeout remains FAIL; later actual unrestricted suite PASS. [Timeout](verification/curated/20261010T073853-9f60d1ae97/run.json). |

The [release index](verification/release/index.json) includes actual FAILs and expected negative controls. Command PASS is distinct from a target verdict. Review-driven audit/gate fixes also reject nested tag/blob disclosure, casefold environment filenames and restricted subtest overcredit. No production mutant or locked-test weakening is retained.

## NOT RUN and limits

Independent all24 final approval, live full release verification after that approval, exact final export audit and public GitHub publication remain pending. Hosted Linux native runtime and CodeQL are NOT RUN; Linux cross-compilation alone is not conformance. Native seven-repeat CPU/RSS/request p50/p95/p99 measurements, cloud deployment, production users/scale, snapshots, compaction, dynamic membership and physical crash certification are NOT RUN or unsupported. Optional paid hosting is not required.

## Evidence provenance

Selected engineering commands retain original statuses, exit codes, timing, tags, complete source file hashes and original raw stream hashes. Declared operator-path redactions and CRLF-to-LF text normalization produce new curated stream hashes and preserve original raw record/stream hashes. [Artifact provenance](verification/release/artifact-provenance.json) records original hashes for product artifacts; [extra receipts](verification/release/extra-artifact-provenance.json) cover targeted inspection/control observations. SHA256 establishes bundle integrity, not independent organizational attestation. Private research, profiles, prompts and agent session/chat logs are excluded. Legacy kernel evidence remains unchanged in [SEED_RESULTS](verification/SEED_RESULTS.md).
