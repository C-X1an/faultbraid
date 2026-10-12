# FaultBraid

Make a distributed-state failure small enough to understand.

FaultBraid is an experimental correctness workbench for a single integer register built on **instrumented etcd/raft v3.7.0**. It runs three fixed voters under explicit virtual faults, checks the collected history, re-executes saved traces, and reduces a failing scenario into a smaller executable reproducer. A separate native lab exercises the same integration code in real loopback child processes with checksummed disk journals.

**Implementation and release acceptance are separate.** Source-matched A6 local acceptance and clean reproduction passed on Go1.27.2 Windows. Historical C3 full approval and C4 bounded closure remain distinct. A fresh independent all24 assessment of the corrected test inputs passed on frozen C5. C4 publication and anonymous exact1376-file verification passed. Actual C4 Linux bootstrap and CodeQL succeeded, but kernel/full runtime failed at the supplemental directory permissions predicate. The exact reviewed correction preserves file0600/directory0700 and every locked criterion; corrected-source hosted runtime remains pending. See [VERIFICATION_REPORT](VERIFICATION_REPORT.md) and the [reviewed test exception](TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND.md).

## Run locally

Use the release toolchain pinned in [.go-version](.go-version): **Go 1.27.2**, Python 3.11+, Git, Make and GCC for race detection. Current local release execution uses Windows. Actual C4 Linux bootstrap and CodeQL succeeded; kernel/full runtime failed at the supplemental output-kind predicate, and corrected-source runtime CI remains pending. A compile check or an older Go 1.23.2 kernel run does not establish current Linux conformance.

From the repository root:

~~~sh
make bootstrap
make build
make verify-seed
make demo
~~~

On Windows, put the tools on PATH and use a Make distribution providing `sh.exe`. Binary examples below use the Unix spelling `./bin/faultbraid`; in PowerShell use `.\bin\faultbraid.exe`.

Bootstrap prepares the locked upstream Raft module and applies the checked-in entropy patch. The build injects measured source, module-lock and patch identities. Dependencies and the online vulnerability scan need network access; local execution needs no cloud account, API key, database service or administrator privilege.

`make demo` executes actual safe and deliberately unsafe Raft scenarios, verifies the unsafe trace ten times, reduces it, replays the reduction, and prints offline inspector paths under ignored `.local-runs/`. It also creates a separate history/disk-model kernel demo. The unsafe wrapper is labelled `unsafe-local-read` and is a diagnostic integration defect, not an assertion of an upstream etcd vulnerability.

## Reproduce a target failure

Create the parent `.local-runs/` directory if it does not already exist. Each output directory below must be new:

~~~sh
./bin/faultbraid simulate -scenario examples/raft-safe.json -outdir .local-runs/safe
./bin/faultbraid simulate -scenario examples/raft-unsafe.json -outdir .local-runs/unsafe
./bin/faultbraid replay -in .local-runs/unsafe/trace.json
./bin/faultbraid shrink-scenario -trace .local-runs/unsafe/trace.json -strategy ddmin -max-checks 1000 -outdir .local-runs/reduced
./bin/faultbraid replay -in .local-runs/reduced/trace.json
~~~

Open `report.html` in an output directory to inspect reproduction identity, operation intervals, scenario actions and causal records. Reports use escaped static HTML, a restrictive CSP and no product scripts or external resources. Tab focuses the scrollable tables; arrow keys inspect long or wide records.

Use the **same identified source build** for replay and reduction. Replay checks the digest and identities, executes the real target with recorded entropy, then executes it again from the declared seed. An edited result with a recomputed hash must still match those executions.

The checker uses three verdicts:

| Verdict | Meaning |
|---|---|
| VALID | A constructive sequential explanation exists for this history. |
| INVALID | Every admissible order was exhausted without a legal explanation. |
| INCONCLUSIVE | A budget, deadline or cancellation prevented a definitive answer. |

A response timeout preserves a pending operation. Pending writes may take effect or be omitted; a missing response does not prove that a write failed. Only affirmative rejection before admission can exclude a rejected-only request. A later rejected retry cannot erase an earlier uncertain admission.

`check` exits 1 for INVALID. A finished `simulate`, `report` or reduction command exits 0 for a definitive result and exposes its verdict in JSON. A successful `replay` exits 0 when execution matches the saved trace, even if that saved verdict is INVALID or INCONCLUSIVE. Inspect JSON rather than treating command success as a correctness verdict. Errors exit 2; a new check/run/reduction returning INCONCLUSIVE exits 3. [API_SPEC.md](API_SPEC.md) describes the command-specific details.

## Explore faults and native conformance

~~~sh
make system differential
make campaign
make real-lab
~~~

`make campaign` counts all 1,000 requested deterministic seeds, saves every attempted outcome, separately runs the labelled unsafe target, and replays each selected failure ten times. Its finite workload covers observed delay/drop/duplication/partition/crash effects; it does not prove every possible execution.

`make real-lab` measures the identified production executable at request-concurrency limits 1, 4 and 16. To run one native experiment:

~~~sh
./bin/faultbraid lab-real -scenario examples/raft-real.json -concurrency 4 -outdir .local-runs/native
~~~

The lab launches only its own built-in child command, binds to `127.0.0.1`, performs real HTTP transport, kills/restarts selected children, and retains PID, journal, entropy and cleanup evidence in `native.json`. Native mode supports invocation, crash and restart actions; virtual link faults and named-barrier crashes are rejected before launch. It has **no native replay** and provides **no physical power-loss guarantee**.

## Inspect the engineering

| Area | Source and contract |
|---|---|
| Strict history semantics and bounded search | `internal/history/`, [API_SPEC.md](API_SPEC.md) |
| Explicit election entropy and dependency patch | `internal/raftadapter/`, `third_party/raft-patch/` |
| Conservative persistence and quorum-safe reads | `internal/target/`, [ARCHITECTURE.md](ARCHITECTURE.md) |
| Virtual execution, replay and reduction | `internal/scenario/`, `internal/campaign/`, [trace codec](docs/trace-codec.md) |
| Actual-process adapter | `internal/labreal/`, [data ownership](DATA_MODEL.md) |
| Independent finite oracle and Porcupine checks | `tests/differential/` |
| Evidence, privacy and release gates | `scripts/`, [claims](CLAIMS_EVIDENCE.md), [benchmarks](BENCHMARKS.md) |

~~~sh
make test race
make fuzz mutation benchmark security-online
make public-audit
make clean-verify      # requires reviewed, committed source
make verify            # includes the separately gated release evidence
~~~

`make verify-seed` checks the implemented source tree; it does not approve full release. `make clean-verify` clones the committed tree and uses a fresh build cache on the same host, retaining its checkout and artifacts. It is neither an independent review nor a fresh Linux machine.

FaultBraid is a finite experiment, not a production database, new consensus algorithm, hostile-code sandbox or complete formal proof. Snapshots, compaction and dynamic membership are unsupported. Read [LIMITATIONS.md](LIMITATIONS.md), [THREAT_MODEL.md](THREAT_MODEL.md) and [SECURITY.md](SECURITY.md) before extending the trust boundary.

Start contributions with [CONTRIBUTING.md](CONTRIBUTING.md) and the [locked acceptance contract](ACCEPTANCE_SPEC.md). Original source is [MIT-licensed](LICENSE); dependencies and the instrumented Raft patch retain their upstream licenses and notices in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).


The [independent current all24 assessment](verification/reviews/final-c5-assessment.json) is a separate read-only review of all24 locked criteria, complete current packet and exact C5 export. Original C1/C2 and rejected B1 results remain historical with unchanged source identities. Actual live full verification, final integrated export and repaired hosted execution are separate pending gates.
