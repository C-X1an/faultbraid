# Architecture

FaultBraid implements one fixed-three-voter integer-register target, one bounded checker and one offline inspector. It uses instrumented etcd/raft v3.7.0 for consensus and owns the surrounding entropy, storage, transport, collection and evidence boundaries. Implementation is present; final release approval remains separately gated by source-matched verification and independent review.

## Components and ownership

~~~mermaid
flowchart LR
    I[Strict history or scenario JSON] --> C[CLI]
    C --> H[Register checker and diagnostic reducer]
    C --> S[Virtual scenario runner]
    S --> T[Instrumented Raft target]
    T --> D[Stage and flush store]
    T --> N[Virtual directed transport]
    S --> R[Logical trace and two-pass replay]
    R --> H
    R --> X[Executable scenario reducer]
    X --> S
    R --> V[Static escaped inspector]
    H --> V
    C --> L[Explicit native process lab]
    L --> T
    L --> F[Loopback HTTP and FileStore journals]
~~~

The simulation has a single scheduler owner. Each target owns one serialized `RawNode` and its state machine; callers do not mutate it concurrently. The native adapter serializes target transitions through one actor per child while its parent collects asynchronous transport/client observations. There is no runtime plugin loader, service database, web backend or model provider.

`internal/history` owns the history specification and bounded search; `internal/sim` contains the separate queue/disk-register kernel. `internal/scenario` combines the target with virtual time and faults; `internal/campaign` generates workloads and reduces executable scenarios. `internal/labreal` shares the target and adds actual process/HTTP/FileStore adapters. `internal/report` produces bounded static HTML.

## Checker and uncertain operations

A write changes the int64 register; a completed read is legal only if its observed value equals the current state. Histories use central invocation/return ordinals. A return strictly before another invocation creates precedence; timestamp ties overlap.

The checker explores `(selected-operation mask, register value)` using predecessor masks and a failed-state cache. It caches only fully explored failed subtrees. Every completed operation is required; pending writes are optional effects and pending reads may be omitted. VALID needs a constructive witness. Exhausting every admissible order yields INVALID. A state limit or cancellation yields INCONCLUSIVE.

The collector records one semantic operation per logical request and retains all causal attempts. A response timeout, unavailable node or uncertain transport leaves possible admission pending. Only affirmative rejection before admission can exclude a rejected-only request. Possible admission is monotone: a subsequent rejected retry cannot erase earlier uncertainty.

Independent differential tests express the register separately for Porcupine and a tiny exhaustive subset/permutation oracle. Their finite domain and representability/time budgets are explicit. Porcupine UNKNOWN stays unknown; it cannot become a definitive disagreement or successful proof. These checks challenge the checker and actual target histories, not every possible distributed execution.

## Deterministic simulation and entropy

The virtual heap orders events by `(virtual_time, insertion_sequence)`. Payload bytes are copied when admitted. Nodes and effects use explicit stable iteration order. Crashed nodes do not tick or consume target messages. Clock/sequence arithmetic, work, queue and trace growth are bounded.

Election randomness is injected through a small checked-in source patch adding `Config.ElectionRandom func(n int) int`. Nil retains upstream behavior; FaultBraid always supplies the explicit hook. This is **instrumented etcd/raft**, not unmodified upstream. The patch and reproducible prepare script validate the locked module checksums, archive SHA, patch SHA and prepared source-tree digest. Upstream licenses and notices remain intact.

Each virtual node owns a SplitMix64-v1 generator, seeded from the scenario seed and node ID. The simulator retains that generator and sequence across target restarts. Every successful draw records node, stream, sequence, bound and value. A fallible decorator captures entropy errors immediately after the upstream callback. Out-of-range, missing, reordered or leftover replay choices fail.

Workload generation has its own recorded stream, `splitmix64-v1/fault-workload-v2`, separate from election entropy. This deterministic PRNG is only for experiments, never secrets or keys. Actual native process lifetimes use separately derived, recorded seeds; OS scheduling still controls when elections occur.

## Raft integration and durability

Membership is fixed to voters 1, 2 and 3, with initial register 0. Configuration uses election tick 10, heartbeat tick 1, PreVote, CheckQuorum, ReadOnlySafe, disabled proposal forwarding and at most 64 in-flight messages. Serialized frames are at most 64 KiB; the upstream message/committed-batch size settings leave encoding headroom below that ceiling. The default log budget is 1 MiB, independently enforced by the target and store. Async storage writes, snapshots, compaction and dynamic membership are unsupported.

For each Ready batch, the adapter:

1. Encodes copied deterministic protobuf send effects.
2. Validates and stages the entries/HardState batch.
3. Completes `Store.Flush` before exposing the flushed storage view or any dependent send effect.
4. Releases send effects in order, then applies committed entries sequentially.
5. Replies to writes only after their replicated command is applied; replies to safe reads only after their quorum ReadIndex is applied.
6. Advances the RawNode after processing the Ready batch.

This is deliberately conservative. A send effect's position is an integration ordering boundary; actual native socket delivery occurs asynchronously afterward. A target failure retains already released effects while stopping subsequent mutation. Interrupted persistence does not invent a successful response.

The six injectable boundaries are `before-append`, `after-stage-before-flush`, `after-flush-before-send`, `after-send-before-apply`, `after-apply-before-response` and `after-response`. Crash occurrence counters apply only after arming. A conditional barrier might never be reached; a scenario cannot assume that arming it killed the node.

`MemoryStore` models staged and flushed durable state; recovery discards staging and validates the committed bootstrap prefix. Its lifetime outlives the recreated target. It is an explicit durability model, not `raft.MemoryStorage` treated as physical persistence.

`FileStore` uses the same validated state representation with an actual append journal. A nonempty batch is written with length, inverse length and SHA-256 framing, then `File.Sync` must succeed before the in-memory durable view changes. Empty Ready batches do not append records. Recovery accepts the complete validated prefix and truncates/syncs an incomplete EOF record. A complete malformed length, checksum, protobuf or state transition is corruption and fails closed. Journal growth is capped independently, at no more than 64 MiB.

Successful File.Sync counters are diagnostic observations per open handle and reset on reopen. Neither a successful call nor process kill proves physical power-loss survival or persistence of parent-directory creation.

## Request identity and safe reads

Replicated write commands encode version, kind, bounded request ID and int64 value; their hash covers the whole command. Applying an already seen ID with the same payload leaves the register unchanged and returns its original result. A conflicting payload is rejected or treated as corrupt replicated state. The target reconstructs deduplication by applying the durable committed prefix on restart.

Deduplication is bounded to 4,096 write IDs per target, without eviction. Repeating an old write(7) after write(9) cannot reset the register to 7. Reads use unique node/term/sequence ReadIndex contexts; safe reads wait for application through the returned index. Read results are not persisted in a deduplication cache. A follower rejects a new client proposal before admission; an already admitted request remains pending if that target later loses leadership.

`unsafe-local-read` deliberately substitutes a leader's local value for quorum confirmation. Its trace and invariant fingerprint identify it explicitly. It is a diagnostic wrapper defect, not a new consensus algorithm or an upstream-vulnerability claim.

## Network faults and causal trace

Directed links support delay, drop-next, duplicate-next, partition and heal. Policies affect complete protobuf frames. Partitions need both directions when isolating a voter. Crash stops the node and recovery discards only staged data; restart increments incarnation. A queued frame is dropped if the recipient is down or either endpoint incarnation no longer matches. Previously released traffic is not retroactively unsent.

The trace contains source/module/patch identities, scheduler/entropy versions, scenario, choices, causal events, collected history, checker result and optional invariant fingerprint. Transition payload hashes cover typed input, ordered effects and resulting status. Logical event IDs are monotone and refer only to earlier causal parents.

The digest uses the fixed typed [codec](docs/trace-codec.md), not general JSON canonicalization. Replay requires matching build identity, validates the digest, executes with recorded choices, then executes from the seed. Both full envelopes must equal the saved trace. Rehashing edited outputs alone cannot bypass execution. Hashes remain consistency checks under a trusted maintainer, not authentication or remote attestation.

`minimize` removes history records for diagnosis. `shrink-scenario` removes actual workload/fault actions while preserving source, locks, seed, limits, target variant and resolved horizon. It replays the baseline and every accepted candidate, and accepts only the same invariant fingerprint. Causal-invalid candidates are rejected, not counted as failing systems. A complete final deletion pass proves 1-minimality under that transform; budgets, cancellation and any unknown candidate clear the claim. No globally smallest or unique-root-cause claim follows.

Campaign indexing counts every attempt and every unrun seed. Execution errors are retained and independent seeds continue; artifact failure stops with explicit remaining-unrun accounting. Coverage counts observed effects rather than merely configured fault actions.

## Native lab and trust boundary

Native mode launches three actual children, with additional lifetimes on restart, from the parent's current identified executable only. Each child owns a loopback listener and FileStore journal and runs the same `Target.Step` code. Parent collection measures real HTTP/frame outcomes, PID/address identity, entropy choices, journal hashes, successful syncs, concurrency and cleanup.

The supported native actions are invoke_read, invoke_write, crash and restart. Virtual partitions, delay/drop/duplication and named-barrier fault injection are rejected before launch. Wall-clock action ticks and process scheduling cannot be replayed bitwise. Conformance can reveal integration/model omissions but is not a guarantee for all operating systems, hardware or failures.

Windows directory creation uses a protected owner-only inheritable DACL before lab bytes are written. Children start suspended and enter a kill-on-close job before resuming. Non-Windows children use owned process groups. All work has bounded queues, requests, output and lifetimes; cleanup kills/waits for children, closes files and checks socket closure, retaining partial evidence on interruption. An already exited child or an exit that does not match the owned termination is an execution error. Cancellation stops live observation delivery while continuing bounded pipe draining until kill/Wait, so cleanup does not close a child's output prematurely.

The local operator and filesystem namespace are trusted. Path checks reject existing aliases, but do not defeat a privileged concurrent path replacement. Loopback is not authentication or a tenant boundary. No TLS, Internet service, arbitrary executable or hostile-code sandbox is provided.

Reports use `html/template`, static CSS/CSP, semantic headings/captions and focusable bounded scroll regions. They display saved data, not a live cluster; rendering does not itself validate replay. Publication requires deliberate content review, an exact public inventory/history/archive audit and independent release verification.
