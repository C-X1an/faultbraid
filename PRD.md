# Product requirements - FaultBraid
Version 1.0. Status: locked behavioral baseline, implementation partial. Targets below are not measured results.

## Thesis and user
FaultBraid turns a bounded distributed-system failure into an inspectable, repeatable experiment. Its primary users are backend/platform engineers implementing storage/transport integration, and reviewers who need a small reproducer rather than a green badge. Agent-platform engineers can use the same methods for durable state; the tool does not evaluate language models or prompt injection.

## Jobs and user stories
An engineer loads a concurrent register history and asks whether any legal sequential explanation exists. They inspect a witness when it does, or reduce an invalid history for diagnosis. A target integrator runs a three-voter register under controlled faults, saves a trace, replays it, and reduces the scenario until a short reproducer still violates the same invariant. A reviewer runs the public demo without credentials and follows each claim to its exact test/evidence. A maintainer treats cancellation, ambiguous outcomes and unknown checker results as first-class states.

## Core versus delivered seed
The finishable full core is one data type (an integer register), one target (instrumented etcd/raft), one deterministic scheduler, one fault model, two independent checking methods, and one offline inspector. The seed implements a bounded checker, history reduction, queue/disk primitives, replay and CLI; it is NOT the distributed target. Full acceptance cannot be claimed without FR-009 through FR-011 and FR-013.

## Stable requirements
**FR-001** - Parse bounded versioned histories; reject unknown/duplicate fields, invalid intervals and duplicate operation IDs.

**FR-002** - Check a single signed 64-bit integer register against a sequential specification, respecting strict real-time precedence; return VALID, INVALID or INCONCLUSIVE.

**FR-003** - Model pending writes as optional operations that may take effect after invocation; pending reads may be omitted because no result is observed.

**FR-004** - Return a legal ordered witness for VALID; bound explored states and honor cancellation without converting incomplete search to VALID or INVALID.

**FR-005** - Reduce an INVALID history only by retaining candidates independently checked INVALID; distinguish diagnostic reduction from executable reproduction.

**FR-006** - Run a deterministic event queue ordered by virtual time then insertion sequence; reject time reversal and queue overflow.

**FR-007** - Represent volatile writes and completed durable flushes separately; a modeled crash discards only unflushed state.

**FR-008** - Generate and verify a versioned replay envelope with input and output digests; replay must re-execute the model, not trust recorded states.

**FR-009** - Provide a built-in, fixed-three-voter etcd/raft register target with an explicit, audited election-entropy hook; record instrumented dependency identity.

**FR-010** - Inject bounded delay, drop, duplication, partitions, process crashes, restart and interrupted persistence at named model boundaries.

**FR-011** - Reduce target scenarios by replaying each candidate and preserving the same invariant fingerprint; report budget exhaustion and non-minimal results honestly.

**FR-012** - Render an offline, escaped, keyboard-readable HTML inspection report; no external resources, script execution or active target controls.

**FR-013** - Provide a three-process loopback transport/disk conformance lab using the same target integration state machine; do not claim OS-level deterministic replay.

**SEC-001** - Treat input histories, traces and generated text as untrusted; do not execute code or paths supplied in them.

**SEC-002** - Prevent parser/search/report resource exhaustion through byte, operation, nesting, queue, state and output limits.

**SEC-003** - Keep secrets, personal source material and private orchestration outside the public repository and its history; scan export allowlist and sanitized evidence.

**SEC-004** - Untrusted PRs receive no write token, repository secrets or privileged host access; dependencies and action revisions are reviewed and recorded.

**REL-001** - A lost client response is an unknown operation outcome, not evidence that the mutation never happened; retries preserve logical request identity.

**REL-002** - For the Raft adapter, persist each Ready batch conservatively before releasing dependent messages; never acknowledge writes before applied committed state.

**REL-003** - Produce deterministic logical traces for identical source, dependency lock, seed, scenario and explicit entropy stream; detect replay mismatch.

**REL-004** - On budget, timeout, corrupted trace or unsupported schema, stop with an explicit non-success classification and preserve useful evidence.

**NFR-001** - Core commands run locally without cloud accounts, LLM API keys, production data or root privileges.

**NFR-002** - Use stable command exit codes, structured JSON output and reproducible examples; full project readiness is separate from seed readiness.

**NFR-003** - Capture verification commands, source identity, environment, output hashes and real exit codes; generated success prose alone is not evidence.

**NFR-004** - Require fresh-checkout setup and independently reviewed tests before release; documentation must not exceed measured evidence.

**PERF-001** - TARGET: check common 12-operation histories within 1 second on a documented ordinary development host; report state counts and variance, not a universal bound.

**PERF-002** - TARGET: simulate 10000 scheduled model events within 2 seconds with bounded queue/memory on the benchmark host; quantify rather than assume target performance.

**PERF-003** - TARGET: replay fixed scenarios identically across ten runs; execute at least 1000 seeded target scenarios for a campaign, with explicit coverage limits.

## Invariants and reliability model
A completed write precedes any operation invoked strictly after its return. Pending writes may have happened; pending reads have no observed effect. The checker accepts only if it constructs a legal order containing every completed operation and a permissible subset of pending writes. Exhausted search is INCONCLUSIVE. Time ties are treated as overlapping: real-process collection uses a central monotonically increasing event ordinal rather than comparing unsynchronized wall clocks.

The simulation owns all logical time, target election entropy and delivery decisions. A modeled flush commits the full staged disk image atomically at its specified boundary; interrupted flush outcomes are selected explicitly, not inferred from real hardware behavior. The more detailed Raft store models a checksummed prefix journal with named append/flush/rename boundaries. No model claims physical power-loss certification.

## Environments, limits and scale
Linux is the release/reference platform; the pure-Go seed should also compile on other Go-supported platforms, but cross-platform runtime behavior is not yet verified. The local seed has no third-party runtime dependencies and source compatibility with Go 1.23; that is NOT a recommendation to deploy an unsupported toolchain. Release work must resolve a currently supported stable Go patch and run vulnerability checks. The current research container only has Go 1.23.2 and cannot resolve external downloads.

History input: at most 1 MiB; at most 24 operations; integer values only; IDs 1-64 safe ASCII bytes. Default search budget 100000 states, configurable 1..1000000. Maximum JSON depth 8. Simulation scenario: at most 100000 events; queued events at most 100000; event-time overflow is rejected. Report size at most 8 MiB. Initial implementation may enforce stricter safe bounds, documented without silently weakening required behavior.

DESIGN CAPACITY is those bounds, not a throughput promise. TESTED/OBSERVED CAPACITY must come from run manifests. REAL USER/PRODUCTION SCALE: none established.

## Privacy, retention, portability and accessibility
Use synthetic integer/event inputs only in public examples. Histories may reveal application values; collect explicitly, do not upload automatically. Runs are local and ignored by Git. Default cleanup removes only tool-owned run directories after review; no automatic erasure of user data. Export a sanitized fixture deliberately. Offline HTML uses semantic headings/tables, text status labels, visible focus and no color-only meaning. JSON stays the portable source of truth.

## Failure cases
Reject malformed JSON, duplicate keys/IDs, negative timestamps, invalid intervals, unknown schema/kind, inconsistent read/write fields, excessive work budgets, time overflow and unsupported target variants. Files are created exclusively rather than overwriting existing output or following a supplied trace path. A partial/untrusted trace is not repaired silently. A target crash cannot turn a client timeout into a definite failure. The simulator and checker are trusted code but are themselves tested adversarially.

## Non-goals and optional scope
No production database, new consensus algorithm, Byzantine fault tolerance, dynamic membership, multi-key transactions, arbitrary native plugins, public upload service, Kubernetes, AI provider, real user analytics or hosted paid control plane. Later adapters for the owner's other projects and an SMT/TLA+ model are optional only after full acceptance. A hosted site, if used, serves reviewed static reports only.
