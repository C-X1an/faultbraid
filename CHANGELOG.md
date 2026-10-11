# Changelog

## Unreleased — experimental correctness workbench

Implemented instrumented three-voter etcd/raft integration with explicit election entropy, conservative Store.Flush ordering, quorum-safe reads, retry/uncertain-admission semantics, virtual network/crash faults, source-bound target replay and executable causal scenario reduction. Added a same-code native loopback process adapter with checksummed File.Sync journals and owned kill/Wait/socket cleanup.

Added linked Porcupine plus an independent permutation oracle, real-target differential checks, finite1,000-seed campaign accounting, ten selected unsafe replays, bounded codecs/limits, fuzzing and semantic mutation controls. Release fixes reject incomplete/shallow/grafted Git ancestry, nested tags/blobs, environment filename bypass and selected-subtest acceptance overcredit. Native uncertain-response and unexpected-exit/draining regressions retain actual before-fix failures and PASS/FAIL/PASS controls.

Applicable local acceptance, fresh committed reproduction and repeated synthetic measurements ran on pinned Go1.27.2 Windows. Independent final release assessment, hosted Linux/CodeQL runtime CI and GitHub publication remain pending. See [VERIFICATION_REPORT](VERIFICATION_REPORT.md) for actual results and limits. This is experimental; no stable release, physical power-loss certification or production scale is asserted.
