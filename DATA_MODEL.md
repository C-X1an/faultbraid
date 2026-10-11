# Data model and ownership

FaultBraid stores experiment files, not a service database. The schemas separate input histories, virtual executions, native observations and verification evidence so that one cannot silently stand in for another.

| Artifact | Owned data and meaning |
|---|---|
| `history-v1` | Initial int64 register plus at most 24 unique logical operation intervals |
| Diagnostic reduction | Another history plus check counts, reduction status and deletion-minimality metadata |
| `disk-register-v1` | Separate model events, reconstructed volatile/durable steps and digest |
| Scenario-v1 | Fixed voters, seed, resource ceilings, target variant, horizon and ordered executable actions |
| `raft-trace-v1` | Source-bound virtual target execution with recorded entropy, causal events, history, verdict and fingerprint |
| `raft-campaign-v1` | Ordered per-seed accounting, observed coverage and independent workload choices |
| `native-loopback-conformance-v1` | Actual process/socket/FileStore observations and collected history; no replay guarantee |
| Verification record | Actual command, source/environment fingerprint, exit and output hashes; separate from a target verdict |

## Histories and collection

A write's `value` is its input; a completed read's `value` is its observed result. A pending read has no observed result. `call` and `return` are centrally ordered int64 ordinals, not synchronized machine clocks. A strict earlier return constrains a later invocation; ties overlap. `return` omitted/null means pending.

The checker owns transient operation masks, register values, predecessor constraints and fully explored failed states. A witness names a legal order. Omitting a pending write does not assert that it definitely failed. Diagnostic reduction copies the operation slice and rechecks deletions, retaining only definitive INVALID candidates.

Target collectors reconcile attempts by logical request ID. The original invocation and first completion bound the semantic interval. Retries retain operation kind and write payload. Every attempt remains causal/native evidence, while only requests with possible admission or an observed completion enter the history. An affirmative rejected-only request has no optional effect; later rejection cannot erase earlier uncertainty.

## Target and store ownership

Each target serially owns its RawNode, current register, applied index, pending requests and replicated write-deduplication map. The map holds at most 4,096 write identities without eviction; committed command replay reconstructs it and the register after restart. Safe read contexts contain node, term and sequence and wait for application through the quorum read index. Read results are not durable deduplication records.

`Store.Stage` owns copied proposed entries and HardState. `Store.Flush` changes the durable view only on success. `Recover` discards staging and validates the complete durable committed bootstrap prefix. Simulation retains a MemoryStore across target lifetimes; snapshots and dynamic membership are rejected.

FileStore journals begin with `FBRAFTJ1`. Each record has an unsigned big-endian payload length, its bitwise inverse, a 32-byte SHA-256 payload digest and a versioned batch payload containing deterministic protobuf data. A complete record must pass framing, protobuf and state-transition validation. An incomplete final header/payload retains the complete prefix and is truncated/synced; a complete corrupt record fails closed. Journal bytes have an independent bound of at most 64 MiB. Successful File.Sync counts are per handle, include initialization/recovery truncation when applicable, and reset on reopen.

## Virtual execution identity

The scenario contains no filesystem path or executable. Its arrays preserve order; explicit action time plus scheduler insertion sequence determines execution. The virtual runner owns node/incarnation state, directed link policies, copied queued frames, recorded choices and event ordinals.

The trace header contains full source-tree, module-lock and patch hashes, scheduler/entropy versions, seed, variant and initial register. Source identity hashes sorted public-relative file paths and exact file-byte hashes, excluding generated/cache directories. Module identity hashes `go.mod`, `go.sum` and `dependency.lock.json` with zero-byte separators. The reproducible build injects these identities.

Every entropy choice records node, stream, sequence, bound and result. Virtual node generators continue across target restarts. Separate workload-generation choices belong to the campaign index. Each trace event has a monotone ID, earlier causal parent, node/incarnation, virtual time, kind, payload digest and detail.

A canonical digest hashes the fixed typed envelope with its digest blank. Replay rebuilds the full envelope twice: first from exact saved choices, then from the scenario seed. A source change, including changed identified file bytes, requires the matching source checkout/build for an old trace. Digests provide consistency, not authenticity.

Scenario reduction preserves identities, seed, limits, variant and resolved horizon while deleting actions. Its accepted trace is a newly executed and replayed experiment, not a filtered projection of the original history. Failure fingerprints identify requirement, target variant and invariant kind; different causes can share an invariant fingerprint.

## Native observations and evidence

Native results retain scenario and history alongside actual binary/source identities, PID/incarnation/address records, per-lifetime entropy seeds/choices, attempt admission classifications, frame-delivery outcomes, journal byte hashes, successful sync observations and cleanup state. Declared request concurrency and observed peak are distinct fields.

Native action time denotes elapsed scheduling in 25 ms ticks. The parent assigns history ordinals centrally; native wall time is measured separately. Per-lifetime entropy is explicit, but the operating system can change which choices are consumed. Native records are not `raft-trace-v1` and `replay` rejects them.

Generated runs belong in ignored `.local-runs/`. Verification tooling captures actual commands and preserves failures. Public curation must keep synthetic inputs, source identities and relevant engineering evidence inspectable while excluding confidential histories, credentials, personal data and raw orchestration logs. Schema conversions require a new output file retaining original provenance; damaged traces are not silently repaired.

See [API_SPEC.md](API_SPEC.md) for exact schemas and ceilings and [docs/trace-codec.md](docs/trace-codec.md) for encoding.
