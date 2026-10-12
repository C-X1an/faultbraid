# Changelog

## Unreleased — experimental correctness workbench

Implemented instrumented three-voter etcd/raft integration with explicit election entropy, conservative Store.Flush ordering, quorum-safe reads, retry/uncertain-admission semantics, virtual network/crash faults, source-bound target replay and executable causal scenario reduction. Added a same-code native loopback process adapter with checksummed File.Sync journals and owned kill/Wait/socket cleanup.

Added linked Porcupine plus an independent permutation oracle, real-target differential checks, finite1,000-seed campaign accounting, ten selected unsafe replays, bounded codecs/limits, fuzzing and semantic mutation controls. Release fixes reject incomplete/shallow/grafted Git ancestry, nested tags/blobs, environment filename bypass and selected-subtest acceptance overcredit. Native uncertain-response and unexpected-exit/draining regressions retain actual before-fix failures and PASS/FAIL/PASS controls.

Applicable local acceptance, fresh committed reproduction and repeated synthetic measurements ran on pinned Go1.27.2 Windows. Independent final release assessment passed on C1. Initial C2 publication/anonymous remote proof passed; initial hosted Linux bootstrap failed before tests and current repair reruns are pending. See [VERIFICATION_REPORT](VERIFICATION_REPORT.md) for actual results and limits. This is experimental; no stable release, physical power-loss certification or production scale is asserted.

Published reviewed C2 (`c7ddd018`) to the authorized public repository and
verified all 963 remote files, the complete Git tree and archive anonymously.
The initial Linux kernel and CodeQL builds failed before tests because native
path ordering changed the prepared Raft tree digest. Made that ordering explicit
across platforms without changing pinned upstream/patch/prepared bytes or
acceptance assertions. Fresh repair verification and hosted reruns are pending;
the previous source's approval does not establish approval of this repair.

Mutation tooling now hashes actual bytes, restores original bytes and verifies the entire disposable tree. Preserved the rejected A3/B1 receipts; reran all applicable local commands on committed A4. Separate final review passed on C1; initial C2 publication passed; current repair hosted gates remain pending.


The [independent all 24 assessment](verification/reviews/final-c1-assessment.json) is a separate read-only review, scoped to all 24 locked criteria and the exact C1 packet/export. FV-01–04 are resolved; original rejected assessments and failures remain inspectable. Live full verification and hosted/publication results must be recorded separately.

Reran all applicable local commands on committed A5 after the portable prepared-tree ordering repair, retaining unchanged dependency and acceptance pins. Independent current all24 review passed on C3; live gates and repaired hosted reruns remain pending. Initial C2 publication and anonymous exact remote proof passed.


The [independent current all24 assessment](verification/reviews/final-c3-assessment.json) is a separate read-only review of all24 locked criteria, complete current packet and exact C3 export. Original C1/C2 and rejected B1 results remain historical with unchanged source identities. Actual live full verification, final integrated export and repaired hosted execution are separate pending gates.


Current candidate corrects a supplemental output-kind predicate under the [independently reviewed exception](TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND.md). Production and locked acceptance bytes are unchanged. Fresh corrected-source acceptance, all24 review and hosted runtime are pending; historical full approvals cover previous test inputs. Actual C4 Windows verification, publication/anonymous contents and CodeQL passed, while Linux kernel/full failed at the original predicate. [Original C4 observations](verification/publication/c4/index.json) retain those outcomes.
