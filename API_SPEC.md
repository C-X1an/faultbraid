# CLI, history and trace contracts
Version1. JSON only; no implicit coercion, comments, trailing values, duplicate object keys, unknown fields, NaN, floats or unbounded strings. Input <=1MiB, nesting<=8. Output errors go to stderr as JSON; user data is not echoed wholesale.

## Seed executable commands
```
faultbraid check -in history.json -max-states 100000
faultbraid minimize -in history.json -max-states 100000 -max-checks 1000
faultbraid replay -in trace.json
faultbraid report -in history.json -out report.html
faultbraid demo -outdir new-directory
```
`-in -` means stdin. `check` exit0=VALID,1=INVALID,2=input/IO/usage error,3=INCONCLUSIVE. Other successful commands return0; minimization success does not mean the underlying history is valid. Semantic search deadlines default10seconds; exhausted semantic search returns3. Local file/stdin reads are operator-controlled and are not a network slow-reader timeout boundary. Output files/directories must not exist: no silent overwrite. No input JSON contains an executable or output filesystem path.

## Register history-v1
```json
{"version":1,"initial":0,"operations":[
 {"id":"w1","kind":"write","value":7,"call":1,"return":2},
 {"id":"r1","kind":"read","value":7,"call":3,"return":4}
]}
```
`version` exactly1. `initial` int64. Operations at most24. `id` unique 1..64 bytes in `[A-Za-z0-9._:-]`. `kind` read or write. `value` int64 is write input or completed read result. `call` nonnegative int64. `return` either an int64 strictly greater than call or null/omitted for pending. Pending reads may be omitted by the oracle; their value is unobserved and ignored. All non-pointer fields are required; return may be null/omitted. Field names are case-sensitive, null cannot coerce to numeric zero, and input must be valid UTF-8. All normal emitted histories include every field. No float timestamp conversion. Time ties across operations count as overlapping. The collector assigns ordered events centrally.

Result: `{ "status":"VALID|INVALID|INCONCLUSIVE", "witness":["id"], "states":17, "reason":"bounded text" }`. The witness contains every completed operation and selected pending writes in a legal order; it may omit pending reads and unused pending writes. Invalid histories have no witness; no minimal-counterexample claim is implied.

Reduction result: `{ "history":{...}, "checks":12, "one_minimal":true, "inconclusive_candidates":0, "classification":"diagnostic_only" }`. `one_minimal` is false if budget/unknown prevents a complete definitive final pass.

## Seed disk trace-v1
A separate model, not Raft or a real disk. Shape: `version:1`, `model:"disk-register-v1"`, `events:[{at,kind,value}]`, `steps:[{at,sequence,kind,volatile,durable,observed}]`, `digest:<hex SHA256>`.
Event kinds: write, sync, crash, read. Sequence is insertion order starting0. Times are nonnegative uint64. `observed` is present only for read. Events are run by `(at,sequence)`. Digest is SHA256 of Go encoding/json serialization of the fixed-order Trace struct with `digest` set to the empty string and no trailing newline. This is a project codec, NOT general JSON canonicalization. Replayer reconstructs steps from events and compares the entire expected trace. Unknown versions/models and altered steps/digests are errors. Hashes are not signatures or tamper-proof evidence.

## Full target commands (required, not implemented by the seed)
```
faultbraid simulate -scenario scenario.json -outdir new-run
faultbraid shrink-scenario -trace run/trace.json -outdir smaller-run
faultbraid campaign -seed-start 0 -count 1000 -outdir campaign
faultbraid lab-real -scenario scenario.json -outdir local-lab
```
Full trace format is `raft-trace-v1`, deliberately distinct from disk-register-v1. Header includes `source_tree_sha256`, `module_lock_sha256`, `patch_sha256`, `scheduler_version`, `entropy_version`, `seed`, `target_variant` and initial state. Ordered records have `event_id`, `parent_event_id`, `node_id`, `incarnation`, `virtual_time`, `kind`, payload digest and choices. Canonical byte encoding is specified in a checked-in codec schema and golden fixtures before implementation; use struct field order, sorted maps as arrays, decimal integers and standard escaped UTF-8. Digest the codec bytes, not loosely equivalent JSON.

Scenario: version1, target `raft-register-v1`, seed uint64, voters[1,2,3], event_limit100000, queue_limit100000, log_limit_bytes1048576, checker_state_limit100000, and an ordered action array. Actions: invoke_write(request_id,node,value), invoke_read(request_id,node), partition(from,to), heal(from,to), crash(node), restart(node), delay_link(from,to,ticks), drop_next(from,to,count), duplicate_next(from,to,count), and crash_at(node,boundary,occurrence). Every action has nonnegative virtual_time and stable action_id. No arbitrary function names or filesystem paths. Invocation timeout records pending; retries use request identity and identical payload.

## In-process target contract
`Target.Step(Event) ([]Effect,error)` advances exactly one event. Effects are Persist(batch), Send(to,bytes), Reply(requestID,value), Schedule(kind,delay) or Fail(code). `Store.Stage(batch)`, `Store.Flush()`, `Store.Recover()` expose explicit durability. `Entropy.Draw(stream,bound)` returns a recorded bounded choice. Workloads/targets never call wall time, network or disk directly in simulation. A typed enum defines events/effects; unsupported events are errors, not ignored.

## Real-process local protocol
Child nodes expose an explicitly loopback-bound HTTP server chosen by parent on port0. `POST /v1/propose` JSON `{request_id,kind,value}`; `POST /v1/message` carries a length-bounded serialized raftpb.Message (Content-Type application/octet-stream, <=64KiB); `GET /healthz` exposes process readiness only. Parent records every invoke/return event with a global monotonic ordinal. Client timeouts are unknown; HTTP failures before admission can be recorded as definitely rejected only with explicit admission evidence. Request retry identity is replicated. No public Internet binding, TLS claim, bearer token or service discovery is part of this lab.

## Error envelope and compatibility
`{"error":{"code":"INVALID_INPUT|UNSUPPORTED_VERSION|IO_ERROR|REPLAY_MISMATCH|LIMIT_EXCEEDED","message":"bounded explanation"}}`. Avoid embedding raw input or stack traces. No automatic retries for replay mismatch/corruption. Unsupported major schemas require explicit conversion in a new output file retaining original provenance. `FAULTBRAID_RUN_DIR` affects trusted CLI output location only; no secrets or credentials are needed. The full CLI may add stricter limits but not reinterpret version1 histories.
