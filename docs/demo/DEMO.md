# Reviewer demonstration
## Seed demo - actual implemented scope
Run `make demo`. It builds the CLI, checks an intentionally stale register history, reduces it for diagnosis, executes a disk model where unflushed state is lost but flushed state survives, and writes an offline report in a new `.local-runs/demo-*` directory. The disk experiment is explicitly a model, not a real database or fsync test.

30 seconds: open README, run demo, see a known-bad history receive INVALID and a useful reduced JSON artifact. Two minutes: compare a concurrent valid history, inspect a constructive witness, then force a state budget of1 to get INCONCLUSIVE. Five minutes: inspect interval ordering, pending write inclusion, mutate a trace, rerun replay and inspect the rejection. Follow the source/tests rather than trusting a screenshot.

## Full target demo - required future scope
Run a healthy three-voter register; complete write7 and quorum-safe read7. Partition the leader from the majority and issue operations: success must not be invented for the isolated side. Crash at a persistence boundary, heal, recover, check history, save/replay logical trace. Enable a named deliberately faulty wrapper variant, find a violation, shrink the scenario, replay the smaller trace and show the same invariant failing. Keep bad variants in test-only code and visibly label them; do not imply an upstream etcd vulnerability.

Capture screenshots only after real execution: CLI verdict and budget state, offline history/witness, trace replay success/mismatch, full target partition timeline and benchmark table with machine identity. Offline fallback is the actual recorded synthetic fixture plus raw logs, not a manufactured live demonstration. No large frontend or live LLM is necessary.
