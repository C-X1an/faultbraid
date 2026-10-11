# Reviewer demonstration

This demonstration uses synthetic inputs and executes the built-in instrumented Raft target. It keeps the separate history/disk-model kernel visibly distinct.

## Generate the artifacts

From a prepared repository:

~~~sh
make demo
~~~

The script builds an identified CLI, creates a kernel demo, executes `examples/raft-safe.json` and `examples/raft-unsafe.json`, checks their actual VALID/INVALID verdicts, runs ten replay commands against the unsafe trace, reduces it with the scenario reducer and replays the accepted result. Any unexpected verdict or command failure is retained as a failure.

The final JSON prints three local view paths: a kernel history report, the actual unsafe Raft inspector and its reduced inspector. Outputs are under ignored `.local-runs/`. Each path is an artifact from execution, not a manufactured live cluster.

## Inspect the counterexample

Open the actual Raft `report.html`. Read the variant, invariant fingerprint, source/module/patch identities and scope before interpreting the verdict. The labelled `unsafe-local-read` wrapper can return a minority leader's stale value after the majority has acknowledged a newer write. It deliberately removes quorum read confirmation; it does not imply an upstream etcd vulnerability.

Use the History link to compare completed operation intervals. Calls/returns are central ordinals, and a strictly completed write before a stale read rules out a legal register explanation. Inspect Actions and Causal records to follow partitions, target transitions, entropy decisions and named barriers. Tab focuses each scroll region; arrow keys move through long/wide tables.

Compare the reduced inspector with the original. Its scenario contains fewer executable actions, and the accepted trace is freshly executed and replayed with the same source/seed/configuration/horizon and invariant fingerprint. `one_minimal` describes only the final action-deletion domain; it is not a unique root cause or global minimum.

## Challenge the result

~~~sh
./bin/faultbraid check -in examples/valid.json
./bin/faultbraid check -in examples/pending.json
./bin/faultbraid check -in examples/invalid.json
./bin/faultbraid check -in examples/valid.json -max-states 1
~~~

Use `.\bin\faultbraid.exe` in Windows PowerShell. The invalid history exits 1; the exhausted budget exits 3 and reports INCONCLUSIVE. Pending writes may be included or omitted, so response loss cannot be presented as a definite failure.

Replay a saved target trace with its matching source build. Edit a copy of the trace, retain the original, and run `replay` against the copy to inspect mismatch rejection. Recomputing an edited envelope's digest is insufficient because replay executes recorded entropy and the declared seed.

`make real-lab` is a separate native conformance experiment with actual child PIDs, loopback transport, journal File.Sync and kill/restart/cleanup records. Its `native.json` is not replayable. Its `history.html` explains the collected register history, while the native JSON contains process evidence.

The kernel `disk-trace.json` demonstrates unflushed loss versus completed modeled sync only. It is not a physical filesystem experiment or substitute for the Raft/native modes.

Screenshots are supplementary: the source, exact inputs, trace identities, command records and re-execution are the engineering evidence. `make teardown` removes only owned kernel demo directories and preserves target/native records.
