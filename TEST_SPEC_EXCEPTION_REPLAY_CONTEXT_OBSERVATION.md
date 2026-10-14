# TEST_SPEC_EXCEPTION_REPLAY_CONTEXT_OBSERVATION

Status: **APPROVED for the exact scoped replacement reviewed on 2026-10-14.** This approves the erroneous supplemental caller predicate's replacement and the accompanying cancellation repair. Fresh combined-source acceptance, the all-criterion independent release review and repaired-head hosted CI remain required.

The reviewed staged V2 proposal is patch SHA256 `73d3aec1c4a99555dfc6c8801d2cd4a7a115e10596087eaaadcea75cdfab3ea0`, with proposed-document SHA256 `a4accbf53768b8437e8952444f2f3d192264c664b53071426f50c1c3de592bfc`. Its complete proposal source SHA256 is `ca275911fd4b4d000585ccfba0dea27f59a8d6f1947a5a4653a4cdec38f8ff13`. This approved document supersedes only the proposed document's approval status and explanation; installing it changes that source digest and requires fresh source-bound verification. The separate assessment binds both document versions and the exact four reviewed code/test hashes.

This exception removes only the requirement that a caller still observe a live context after `Replay` has successfully returned. Locked criteria and locked assertions retain their original digest. The real 300 ms replay context, cleanup, source identity equality, recorded-entropy execution, independent fresh-seed execution, complete trace comparisons, tamper rejection and uncancelled extra fresh-run equality remain. No timeout increases, skipped seeds or test-selection changes are approved.

The published original is `e7ce3f9da331f9e107a1b3f098ab93fdc27c1378`, source SHA256 `05047573078ae775f06275e934bbf3081709c5d5169f92453f601d1860c9fdb1`. Its actual [kernel job](https://github.com/C-X1an/faultbraid/actions/runs/38521875962) failed in `FuzzTargetTraceReplay/seed#0` at the combined post-return identity/cancellation assertion. [Full acceptance](https://github.com/C-X1an/faultbraid/actions/runs/38521989990) separately passed on the same source. The failed log does not establish the exact cancellation interleaving; neither a pre-return nor a post-return timer ordering is claimed as its proven cause.

A normal `WithCancel` control executes the actual target and completes Replay under a live context, then explicitly cancels it before applying the old caller predicate. That predicate fails although Replay had already completed. A callee cannot guarantee the future liveness of an independently cancellable context. Success is committed at Replay's own final cancellation observation, after its comparisons. Later cancellation cannot retroactively invalidate that earlier completion.

Separate labelled scheduling controls exposed genuine implementation gaps. The fixture delegates to a real cancellation context, snapshots a legitimate nil `Err()` observation and then cancels at a fixed poll. At legacy Replay checkpoint 570, the original returned nil despite cancellation during remaining validation; a terminal guard rejected the same fixed schedule. At Run checkpoint 285, the original retained `INVALID` and a failure record. V2 instead returns an `INCONCLUSIVE` trace with an empty witness and no semantic failure, preserving the actual history, events, entropy choices, source identity and measured checker-state count, and resealing and bounding the canonical digest. It retains Run's simulation result contract rather than converting cancellation into an input error.

The actual recorded controls include:

| Control | Actual outcome | Run ID |
|---|---|---|
| Original Replay, fixed checkpoint 570 | FAIL: nil replay answer after injected cancellation | `20261014T063657-bb8f9ed821` |
| Original Replay with only terminal guard, same checkpoint 570 | PASS | `20261014T063906-9c3858721c` |
| Ordinary successful Replay, then cancellation, old caller predicate | Intentional specification FAIL | `20261014T063751-7aa12905dc` |
| Original Run, fixed checkpoint 285 | FAIL: retained INVALID/failure | `20261014T064819-7f97c4ae97` |
| V2 pre-cancel, checkpoints 200/570/576, Run preservation, later cancellation | PASS | `20261014T070726-13d7f5024a` |
| Remove only V2 terminal Replay guard, checkpoint 576 | Intentional defect FAIL at the cancellation assertion | `20261014T071010-36a61b78a6` |
| Remove only retained Run checker-state count | Intentional defect FAIL at the state-accounting assertion | `20261014T071020-1d4be8e652` |
| Exact restored V2, affected security/scenario/CLI race tests | PASS | `20261014T071028-a434bbdcc7` |

The target, Raft integration, entropy, history, traces and comparisons in these controls are production code. The scheduling fixture is test machinery. These are actual Windows controls, not reproduction of the opaque Linux deadline interleaving, execution of all acceptance criteria or production deployment proof. The first proposal's independent rejection remains preserved: it erased observed states and lacked a mutation specific to the sole terminal Replay guard. V2 corrects both findings; the two mutation failures are retained alongside the restored PASS.

Replay now checks cancellation at entry, after digest validation, after each actual execution and after the final comparison. Pre-existing cancellation remains a typed cancellation error; observing cancellation before the final Replay checkpoint cannot yield affirmative replay. Unknown Run results retain their observations and correct digest. Missing or corrupted inputs, identity mismatches, changed events or entropy, and resource exhaustion retain their existing rejected or unknown classifications.

Approval applies to the precise V2 code/test bytes and this approved-document version. It does not authorize retagging old source results, deleting the original failing evidence, changing locked P00-P07 semantics or bypassing affected tests, negative/restored controls, clean reproduction, the current full evidence gate, all-criterion independent review, public/privacy audit or actual repaired-head hosted CI.
