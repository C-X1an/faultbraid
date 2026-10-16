# CLI, history and trace contracts

Version 1 preserves the locked behavior in [ACCEPTANCE_SPEC.md](ACCEPTANCE_SPEC.md). Product verdicts VALID, INVALID and INCONCLUSIVE are distinct from command/evidence statuses such as PASS, FAIL and NOT RUN.

## Input, output and exit codes

Inputs are one UTF-8 JSON value of at most **1 MiB**, with nesting at most **8**. Duplicate or unknown keys, case variants of field names, trailing values, numeric null coercion, floats/exponents in integer fields and missing required fields are rejected before execution. Integer overflow is an input error. Pointer fields may be omitted or null where the schema permits; null arrays and null required scalar fields are rejected.

`-in -`, `-scenario -` and `-trace -` read stdin. File/stdin readers are operator-controlled local inputs, not a network slow-reader boundary. Flags irrelevant to a command and positional leftovers are errors. Input JSON never selects an executable, output path or arbitrary function.

Outputs are structured JSON on stdout. Error envelopes go to stderr:

~~~json
{"error":{"code":"INVALID_INPUT","message":"bounded explanation"}}
~~~

Codes include `INVALID_INPUT`, `UNSUPPORTED_VERSION`, `IO_ERROR`, `REPLAY_MISMATCH` and `LIMIT_EXCEEDED`. Raw inputs and stack traces are not echoed. Existing output leaves and directories are refused. Parents must already exist and must be trusted regular directories without symlink/reparse aliases. A failed multi-artifact write may leave an inspectable partial directory; there is no silent overwrite or all-files transaction.

| Command outcome | Exit |
|---|---:|
| `check`: VALID / INVALID / INCONCLUSIVE | 0 / 1 / 3 |
| Finished `minimize` or `shrink-scenario`: reduction status INVALID | 0 |
| Finished `simulate`, `report` or `lab-real`: definitive VALID or INVALID | 0 |
| Successful `replay` matching the saved trace, including an INVALID or INCONCLUSIVE target trace | 0 |
| Completed `campaign` with no inconclusive/error/unrun outcomes | 0 |
| Semantic budget, deadline or cancellation classified INCONCLUSIVE | 3 |
| Input, unsupported version, replay mismatch, I/O or protocol failure | 2 |

A successful campaign can contain INVALID histories: its outcome index records them. A replay PASS establishes trace agreement, including agreement with a saved INCONCLUSIVE result; it does not establish a definitive register verdict. Some I/O/protocol failures retain an INCONCLUSIVE history but exit 2 because the command itself failed. Parse the result and command status separately.

History, report, replay, simulate and reduction commands use a 10-second execution context; campaign uses 5 minutes; `lab-real` has a 60-second outer deadline plus its bounded adapter schedule. Resource exhaustion never establishes VALID or INVALID.

## History commands

~~~sh
faultbraid check -in history.json -max-states 100000
faultbraid minimize -in history.json -max-states 100000 -max-checks 1000
faultbraid report -in history.json -max-states 100000 -out report.html
faultbraid replay -in trace.json
faultbraid demo -outdir new-directory
~~~

`check`, `minimize` and `report` default to stdin and 100,000 search states; the allowed state budget is 1 through 1,000,000. History minimization permits 1 through 100,000 checks, default 1,000. The direct `demo` subcommand creates the separate kernel demo; `make demo` additionally executes and inspects the instrumented Raft target.

### Register history-v1

~~~json
{"version":1,"initial":0,"operations":[
  {"id":"w1","kind":"write","value":7,"call":1,"return":2},
  {"id":"r1","kind":"read","value":7,"call":3,"return":4}
]}
~~~

`version` is exactly 1, `initial` and `value` are int64. There are at most 24 operations. An ID is unique, 1–64 ASCII bytes from `[A-Za-z0-9._:-]`. A kind is `write` or `read`. `value` is the write input or completed read result. `call` is a nonnegative int64; `return` is either an int64 strictly greater than call, or null/omitted for pending.

For two different operations, a return **strictly before** another call creates precedence; equal timestamps overlap. A pending write may be selected or omitted from a legal completion. A pending read has no observed result and may be omitted. Histories contain logical requests once; retries must be reconciled by their collector.

Result fields are `status`, `witness`, `states` and `reason`. A VALID witness includes every completed operation and any selected pending writes in a legal order. INVALID has no witness. A bounded incomplete search is INCONCLUSIVE.

`minimize` deletes operation records and independently rechecks candidates. Its result contains `history`, `checks`, `one_minimal`, `inconclusive_candidates`, `classification:"diagnostic_only"`, `status` and `reason`. Budget or cancellation may return INCONCLUSIVE while retaining a previously proven INVALID history. `one_minimal` requires a definitive final delete-one pass. It is a diagnostic projection, not an executable target scenario.

### Separate disk-register-v1 trace

The disk model has `version:1`, `model:"disk-register-v1"`, `events:[{at,kind,value}]`, `steps:[{at,sequence,kind,volatile,durable,observed}]` and `digest`. Kinds are write, sync, crash and read; observations occur only on read. Time is nonnegative uint64, and insertion sequence breaks ties.

The digest hashes the fixed-order Go Trace struct with an empty digest field, encoded by `encoding/json` without indentation or a trailing newline. Replay reconstructs all steps from events and compares the complete trace. This model is not Raft, a physical filesystem or a native fsync experiment.

## Virtual target commands

~~~sh
faultbraid simulate -scenario scenario.json -outdir new-run
faultbraid shrink-scenario -trace run/trace.json -strategy ddmin -max-checks 1000 -outdir smaller-run
faultbraid campaign -seed-start 0 -count 1000 -outdir new-campaign
~~~

These commands require an identified build produced by `make build`. For these commands and `lab-real`, `FAULTBRAID_RUN_DIR` supplies `-outdir` only when the flag is absent. It must still name a new trusted directory.

`simulate` writes `scenario.json`, `trace.json`, `history.json`, `result.json`, `summary.json` and `report.html`. `shrink-scenario` writes the reduced equivalents plus `reduction.json`. Scenario, trace, history and ordinary summary/result JSON are bounded to 1 MiB; HTML, campaign index and reduction JSON are bounded to 8 MiB.

### Scenario-v1

Required fields:

~~~json
{"version":1,"target":"raft-register-v1","seed":0,"voters":[1,2,3],
 "event_limit":100000,"queue_limit":100000,"log_limit_bytes":1048576,
 "checker_state_limit":100000,"actions":[],"end_time":60,
 "target_variant":"raft-register-v1"}
~~~

`end_time` and `target_variant` are optional. The safe variant is `raft-register-v1`; `unsafe-local-read` is a labelled negative control. The register starts at 0. Voters are exactly `[1,2,3]` in that order.

| Limit | Range |
|---|---|
| `event_limit`, `queue_limit` | 1–100,000 |
| `log_limit_bytes` | 256 bytes–64 MiB |
| `checker_state_limit` | 1–1,000,000 |
| Actions / distinct logical request IDs | At most 1,024 / 24 |
| Action time and resolved end time | 0–10,000 virtual ticks |
| Link delay | 0–10,000 ticks, with overflow checks |
| Fault count / barrier occurrence | 1–100,000 |

Each action has a unique `action_id`, nondecreasing `virtual_time` and one kind from the following table. Node/link endpoints are integer voters 1–3; from and to must differ. IDs use the same bounded grammar as history IDs. Unrelated fields are rejected.

| Kind | Additional fields |
|---|---|
| `invoke_write` | `node`, `request_id`, `value` |
| `invoke_read` | `node`, `request_id` |
| `crash`, `restart` | `node` |
| `partition`, `heal` | `from`, `to` |
| `delay_link` | `from`, `to`, `ticks` |
| `drop_next`, `duplicate_next` | `from`, `to`, `count` |
| `crash_at` | `node`, `boundary`, `occurrence` |

A partition is directed; isolate both directions explicitly. Link policies apply to complete protobuf messages, not individual bytes. Delays replace the link delay, initially one tick. Crash/restart and partition/heal prerequisites must be executable in the current state; invalid causality is an execution/input error, not a counterexample.

An omitted end time resolves to the last action time plus 200 ticks, or 200 for an empty action list, within the 10,000-tick ceiling. Reduction retains that resolved horizon. Deleting an action that would change an implicit horizon is outside its deletion domain.

Barrier names are `before-append`, `after-stage-before-flush`, `after-flush-before-send`, `after-send-before-apply`, `after-apply-before-response` and `after-response`. `occurrence` counts matching barriers reached after the crash is armed; arming a barrier does not prove it will be reached.

Retries keep the same request ID, operation kind and write payload. Replicated write deduplication holds at most 4,096 IDs per target and reconstructs them from committed journal entries on restart. It does not evict old IDs. A retried old write returns its original result without reverting a later register value. Read results are not a durable deduplication cache.

### Raft trace and replay

`raft-trace-v1` contains `version`, `model`, `header`, `scenario`, `choices`, `events`, `history`, `result`, `failure` and `digest`. The [codec contract](docs/trace-codec.md) fixes field order, integer encoding and hashing; loosely equivalent input JSON is not itself the digest format.

Headers record measured source-tree, module-lock and patch digests, scheduler/entropy versions, seed, target variant and initial value. Choices record node, stream, sequence, bound and value. Events record causal parent ID, node/incarnation, virtual time, kind, payload digest and detail. Pre-crash frames with stale sender or recipient incarnation are dropped after restart.

Replay first requires the current build identity and envelope digest to match. It executes the actual target with exact recorded entropy, then independently regenerates choices from the declared seed and executes again. Both complete traces must match the saved envelope. A hash is a consistency check, not a signature.

A definitive INVALID target history receives a failure fingerprint of requirement, variant and invariant kind. `shrink-scenario` supports `ddmin` or `greedy` action deletion, 1–100,000 candidate evaluations, default 1,000. It replays the baseline, executes each eligible candidate with unchanged identities/seed/limits/variant/horizon, preserves the same fingerprint, and replays every accepted candidate.

Reduction accounting distinguishes candidate checks, actual executions, replay checks, causal/fingerprint rejections and unknown candidates. INCONCLUSIVE results retain the last verified failing trace but clear `one_minimal`. A completed final deletion pass proves only 1-minimality under `fixed-horizon-action-deletion-v1`, never a global minimum.

### Campaign index

Campaign count is 1–1,000 and the entire sequential seed range must fit uint64. `index.json` records requested, attempted, unrun, VALID, INVALID, INCONCLUSIVE, execution-error and callback-error totals, plus ordered per-seed entries, observed coverage and separate workload choices/version. Each seed streams a trace/history to its own directory. Execution errors retain partial traces and independent seeds continue; artifact-callback failure stops and counts the remaining seeds as unrun.

The CLI campaign generates the safe target workload. `make campaign` additionally executes the labelled unsafe scenario and performs ten actual replay commands for each selected INVALID trace. Missing outcomes and unknowns are never discarded from the index.

## Native process conformance

~~~sh
faultbraid lab-real -scenario scenario.json -concurrency 4 -outdir new-lab
~~~

`-concurrency` is 1–16, default 4. Native mode accepts only read/write invocation, crash and restart actions, a horizon at most 1,000 ticks and at most 64 process lifetimes. Unsupported faults and invalid crash/restart causality are rejected before output creation or launch. Each tick means **25 ms of native elapsed scheduling**, not a deterministic virtual event.

Native action times are elapsed-time lower bounds, not acknowledgement promises: blocking restart work can make later deadlines overdue. The optional `-completion-plan plan.json` adds explicit causal requirements without rebasing that clock or changing the default schedule. Plans are separate from replayable scenarios, use version 1, and are limited to 64 KiB. The adapter also bounds plan counts and validates every reference. Strict decoding rejects unknown or duplicate keys, null fields, invalid versions and trailing input. Invalid predecessor/filter/request references are rejected before output or process creation.

A plan has `completion_dependencies` and `required_completions` arrays. A dependency supplies `action_id` and a previously invoked logical `request_id`. Optional `attempt_actions` narrows acknowledgement to a complete/admitted response for one of those strictly earlier actions for the same request. This distinguishes a later retry from the logical request's initial completion. Pending, rejected and transport-unknown outcomes cannot satisfy a dependency. Actual observations continue to drain while waiting; invocation/response ordinals and the earliest logical completion/value are retained. Native results record the copied configuration and each satisfied barrier's actual acknowledgement action and receipt ordinal.

Configured non-request actions also record one `action_dispatches` observation at actual action entry, after all waits and before crash or restart side effects. Each observation contains `action_id`, `kind`, and `observed_ordinal`: an atomic snapshot of the existing common counter, which includes invocations and responses. This is a counter floor, not a dispatch timestamp or a newly assigned receipt. A floor at least equal to the matched acknowledgement ordinal proves that event already existed before action entry; equality is valid. The recipe matches observations to the scenario and rejects missing, duplicate, unknown, wrongly typed or out-of-order evidence. Request dependencies still require the actual acknowledgement ordinal to be strictly less than the client invocation ordinal. Recording these observations does not increment the counter or change existing receipt stamps.

`required_completions` declares terminal logical acknowledgements to await after the unchanged nominal scenario end. This is opt-in and remains within the original native run context and caller deadline; no timer is reset or extended. Cancellation or a missing acknowledgement interrupts the run and preserves INCONCLUSIVE/pending outcomes with owned cleanup. Native scheduling and these results remain non-replayable.

The production conformance recipe uses `examples/raft-real-completions.json`: the first read follows write7 acknowledgement, old-write retries follow write9 acknowledgement, and the final read follows an actual completed retry as well as write9. It also requires final-read completion. Reproduce with `faultbraid lab-real -scenario examples/raft-real.json -completion-plan examples/raft-real-completions.json -concurrency 4 -outdir new-lab`. The existing write/read, durable-restart, process, journal and cleanup assertions remain in the recipe.

The parent launches only its own current executable's internal `lab-node` command. The scenario cannot choose a binary or path. Children must match the parent build and actual executable SHA before workload admission. Output contains `native.json`, `history.json`, `result.json`, `history.html`, per-node journals, bounded protocol/stdout/stderr records and ownership metadata. `history.html` displays the collected history; `native.json` carries native process evidence.

Each child binds to `127.0.0.1:0`:

| Endpoint | Behavior |
|---|---|
| `GET /healthz` | Readiness only, not quorum or correctness |
| `POST /v1/propose` | Exact JSON `{request_id,kind,value}`, Content-Type `application/json`, at most 1,024 bytes |
| `POST /v1/message` | A serialized raftpb.Message, Content-Type `application/octet-stream`, 1–65,536 bytes |
| `GET /v1/diagnostics` | Bounded current target/FileStore counters |

Proposal responses carry `request_id`, `status`, `admission`, `value` and `code`. Complete/admitted responses establish completion; affirmative `rejected-before-admission` responses exclude rejected-only calls. Timeouts, unavailable nodes and uncertain transport outcomes remain pending. Malformed responses preserve the uncertain attempt and interrupt the run. The parent stamps invocation and received proposal outcomes with a shared atomic integer ordinal, rather than child wall-clock timestamps. Buffered collection cannot move an already received completion past a later invocation; unfinished requests remain overlapping. Retries retain the earliest received completion, and attempts are emitted in ordinal order.

The child actor queue is bounded to 128, connections to 64, headers to 8 KiB, wire lines to 128 KiB, and each output file to 16 MiB with a 64 MiB aggregate output budget. HTTP parsing and requests have deadlines; proxying and redirects are disabled. Journal growth is independently bounded. The parent kills and waits for owned children and verifies socket closure. Windows children are assigned to kill-on-close jobs while suspended before resuming; non-Windows children use owned process groups.

Native records have `mode:"native-loopback-conformance-v1"`, actual PIDs/addresses, source/binary identities, per-lifetime entropy choices, successful File.Sync counters, journal byte hashes, transport outcomes, declared/observed concurrency and cleanup state. `target_status` diagnostics currently retain the exported Go field names such as `Value` and `Commit`.

Native execution is **not accepted by `replay`**. Process kill and successful File.Sync calls do not certify loss-of-power behavior, parent-directory persistence or hardware write ordering. Loopback is not authentication, and this lab is not an Internet service.
