# Explicit limitations

**FaultBraid is an experimental distributed-state correctness workbench. Implemented source is not the same as completed release acceptance or an audited security boundary.**

Implemented components include the bounded register checker and diagnostic history reducer; queue/disk-model kernel; pinned instrumented three-voter Raft target; explicit election entropy; staged/flushed stores and named crash barriers; virtual faults, causal traces and two-pass replay; executable ddmin/greedy scenario reduction; independent Porcupine/finite-oracle differential checks; bounded seed campaigns; static inspectors; and the same-target native loopback/FileStore process lab.

Source-matched A6 local acceptance and clean reproduction passed on Go1.27.2 Windows. Historical C3 full approval and C4 bounded closure remain distinct. A fresh independent all24 assessment of the corrected test inputs passed on frozen C5. C4 publication and anonymous exact1376-file verification passed. Actual C4 Linux bootstrap and CodeQL succeeded, but kernel/full runtime failed at the supplemental directory permissions predicate. The exact reviewed correction preserves file0600/directory0700 and every locked criterion; recorded pre-cleanup C7 corrected-source hosted runtime passed. See [last verified pre-cleanup jobs](verification/publication/last-verified-ci.json). See [VERIFICATION_REPORT](VERIFICATION_REPORT.md) and the [reviewed test exception](TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND.md).

## Finite correctness and workload scope

The checked data type is one signed 64-bit integer register, initially 0 for the target. Membership is always voters 1, 2 and 3. There are at most 24 logical history operations and bounded search/queue/event/trace budgets. Snapshots, compaction, dynamic membership, multi-key transactions, arbitrary targets and unbounded workloads are unsupported.

VALID establishes a legal explanation for the supplied history, not correctness of every future execution. INVALID requires exhaustive rejection within that finite search. INCONCLUSIVE is neither result. Pending writes are optional effects; response loss does not prove non-admission. Histories from outside the built-in collector need correct logical retry reconciliation.

Differential enumeration is explicitly finite. Porcupine's API needs optional-write projections and artificial response ordinals for selected pending writes; representability or time limits stay UNKNOWN. Agreement with independently expressed models can reveal defects but is not a formal proof. Race detection, short bounded fuzzing and selected semantic mutation controls also have finite coverage; no universal security or mutation score is claimed.

The 1,000-seed campaign is a particular versioned workload generator, not exhaustive fault search. Coverage records effects actually observed. A safe campaign with no INVALID histories does not prove fault-free consensus. The separately labelled unsafe-local-read variant exercises an integration defect and reducer; it is not a naturally discovered safe-target failure or an upstream etcd vulnerability.

## Replay and reduction boundaries

Deterministic replay applies to virtual logical execution with matching source, dependency lock, patch, schema, scheduler, entropy, seed and scenario. It does not control the operating system. Recorded hash consistency is not a signature, remote attestation or protection against a privileged maintainer rewriting source and evidence together.

Source identity includes exact identified file bytes. Replaying old traces requires their matching source checkout/build; a current build with changed documentation or evidence bytes may have a different identity. The checked-in codec golden fixture is synthetic encoding evidence with unidentified source fields, not a runnable target trace.

History minimization is diagnostic only. Scenario reduction deletes actions at a fixed resolved horizon and preserves the same invariant fingerprint. Causal-invalid deletions are rejected. An implicit horizon may prevent deleting its determining final action. A completed final deletion pass proves 1-minimality under that transform, not the globally smallest scenario or a unique root cause. Budget, cancellation and unknown candidates prevent a minimality claim even when a previously verified INVALID trace is retained.

## Durability and native mode

MemoryStore is an explicit staged/flushed durability model. FileStore adds real checksummed journal writes and successful File.Sync calls. Recovery truncates an incomplete last record to a complete validated prefix; complete corrupted records fail closed. Errors stop further mutation rather than forging success.

A process kill is not physical power loss. File.Sync does not certify hardware caches, filesystem ordering, directory-entry persistence, every torn-write pattern or every device failure. Journal growth is bounded and compaction is absent. Replicated write deduplication retains at most 4,096 IDs without eviction; crossing the bound stops progress. Read results are not a durable deduplication cache.

The native lab supports read/write invocation, crash and restart only. Virtual link faults and named-barrier crashes are rejected before launch. It runs the same integration state machine in actual child processes with loopback HTTP and FileStore journals, while election/message timing depends on the OS. Native results have no deterministic replay contract and are not accepted by `replay`.

Configured and observed request concurrency differ. Cleanup evidence verifies owned child waits and socket closure for the run; it is not a general process sandbox guarantee. Loopback is not authentication or protection from other local processes. The lab must not be exposed as an Internet service or allowed to run arbitrary native executables.

## Runtime, output and publication

Go1.27.2 is the pinned release toolchain and actually ran on the Windows development host and recorded pre-cleanup C7 Linux runtime. Older kernel-only Go1.23.2/Linux records are historical compatibility evidence. Initial C2 bootstrap and C4 runtime failed; current corrected-source C6 hosted runtime passed. Cross-compilation is separate evidence and cannot substitute for runtime. Online vulnerability scanning is source/version/date specific and never a timeless security guarantee.

Outputs use exclusive creation and reject existing path aliases. Native lab directories use restricted access before content creation; report directories/files apply owner access controls. The local operator and filesystem namespace remain trusted. Concurrent privileged replacement of a checked parent path is outside that protection. Multi-file artifacts are not an atomic transaction and report files are not guaranteed durable against power loss.

Static reports escape supplied text, contain no product scripts or external resource loads, and offer focusable scroll regions. Browser rendering/accessibility observations are a separate check; HTML tests alone do not establish complete accessibility across browsers or assistive technologies. Report rendering does not execute a trace or prove its recorded verdict.

Fresh-checkout verification uses a new committed clone and build cache on the same host/toolchain, with a declared shared verified module cache. It is not a fresh virtual machine, independent review or hosted Linux execution.

The reviewed public inventory, staged tree, reachable Git history and deterministic export are audited for known private paths/content and malformed archives. Detection is heuristic and requires manual review; neither an allowlist nor a secret scanner proves approved content safe. Unreachable Git objects and external artifacts are outside a reachable-history audit. Raw runs and confidential/operator material must remain local and excluded.

No production scale, tenant isolation, live monitoring, TLS service, cloud deployment, complete formal proof or hardware power-loss certification is established. Repository publication and actual CI are separate deliverables; no badge represents an unexecuted job or substitutes for acceptance evidence.

Original source-bound results remain historical after authorized metadata/history cleanup. Executable inputs and locked assertions are unchanged. Generate fresh same-source traces with `make demo`; old recorded commit hashes are not checkout instructions. See [evidence contents](verification/README.md).
