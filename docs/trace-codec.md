# Raft trace codec v1

This contract precedes the target trace implementation. A trace is a typed Go struct, encoded by encoding/json without indentation or a trailing newline. Hash it with the digest field set to the empty string. Struct fields have the order listed below; maps are excluded from the wire format. Arrays preserve recorded order, integers use decimal notation, strings use the standard Go JSON UTF-8 escaping (including HTML escapes). Pretty JSON is a transport representation; digesting means encoding the decoded, strictly validated struct. SHA256 provides consistency, not authenticity.

Envelope order: version, model, header, scenario, choices, events, history, result, failure, digest. Version is 1; model is raft-trace-v1. Empty ordered collections encode as [] rather than null. Failure is null when no definitive invariant violation is established.

Header order: source_tree_sha256, module_lock_sha256, patch_sha256, scheduler_version, entropy_version, seed, target_variant, initial. Source identity is the evidence runner's SHA256 of sorted [{path,sha256}] entries, excluding .git, .local-runs, bin and Python caches, rejecting symlinks. Module identity hashes go.mod, a zero byte, go.sum, a zero byte, dependency.lock.json. Patch identity hashes the exact applied patch. The reproducible build injects these identities; an unidentified build cannot claim target reproducibility.

Choice order: node, stream, sequence, bound, value. Election streams belong to one node and continue across its restarts; the node's generator and sequence are retained by the simulator while the target is recreated. They are distinct from workload generation. Replay validates node, stream, ordinal, bound, value and complete consumption; it does not silently resample edited choices.

Event order: event_id, parent_event_id, node_id, incarnation, virtual_time, kind, payload_sha256, detail. Event IDs are monotonically assigned on execution. Parent 0 is initialization. Payload is a typed action, immutable protobuf frame, boundary or reply, never an executable or path. A pre-crash frame retains both sender and recipient incarnation and is dropped on mismatch. Recorded barriers distinguish staging, durable flush, sending, application and response.

Scenario follows API_SPEC.md, with optional end_time (default last action plus 200 ticks) and target_variant (default raft-register-v1, the safe target). Action virtual times are monotone; insertion order breaks ties. A named unsafe-local-read variant is an intentional integration defect used only for negative controls and reduction. It remains explicitly identified in trace and failure fingerprint. Resource ceilings can make a run INCONCLUSIVE; they cannot produce VALID or INVALID from unfinished work.

Result is the bounded history checker result. Failure order: requirement, target_variant, kind, fingerprint. Fingerprint derives from those three fields and preserves the invariant class rather than every individual event. Reduction retains source, locks, seed, configuration and variant, reruns candidates and replays newly accepted traces. Invalid causality is an input rejection, not a counterexample.

Logical replay is a simulator property. The real-process lab shares target transitions but records actual PIDs, sockets and fsync; its scheduling and timestamps are not bitwise replay guarantees.
