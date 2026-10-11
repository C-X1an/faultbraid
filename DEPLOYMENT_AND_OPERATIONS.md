# Local execution and operations

FaultBraid runs as a local CLI. Its only network service is the explicitly invoked loopback process lab; it has no public service, production database, model provider or default background daemon.

## Setup and execution

Install the pinned Go 1.27.2 toolchain, Python 3.11+, Git, Make and GCC on an authorized development host. On Windows, Make uses `sh.exe`. `make bootstrap` checks tools and reproducibly prepares the locked instrumented Raft module; it does not silently certify a mismatched Go version as a release toolchain.

~~~sh
make bootstrap
make build
make verify-seed
make demo
make system differential
make campaign
make real-lab
~~~

Builds contain measured source/module/patch identities. Generated upstream source, binaries, build metadata and run artifacts are ignored. Initial dependency preparation and `make security-online` need network; ordinary local runs need no service credential or elevated privilege.

Every output directory must be new and have an existing trusted parent. Use an ignored run directory for experiments. Do not feed confidential histories into a tool whose evidence you intend to publish. Rebuild after changing identified source bytes; replay requires the build that matches the trace's source identity.

`make campaign` performs the bounded 1,000-seed virtual experiment, verifies accounting and retains every trace/error, then separately exercises the unsafe wrapper and selected-failure replays. `make real-lab` measures the actual identified production CLI at concurrency limits 1, 4 and 16; reported observed peaks may be lower. Native mode supports invocation, crash and restart only.

The process lab creates owned loopback children from the current executable, checks build/binary identity before workload admission, and retains actual protocol, PID, socket, FileStore and cleanup evidence. Its health endpoint indicates readiness only. It is not safe to expose as an Internet service, and local loopback is not authentication.

## Verification and evidence

~~~sh
make test race
make fuzz mutation benchmark
make security
make security-online
make public-audit
make clean-verify
make verify
~~~

`verify-seed` checks the implemented source tree. Full release evidence is a separate gate; a command exit or a file named “summary” is not sufficient proof. `make verify` requires checked release artifacts before running the supported-toolchain/system/differential/campaign/native/online-security steps.

Use `scripts/run_evidence.py` for actual execution:

~~~sh
python3 scripts/run_evidence.py --requirements AC-011 --timeout 600 -- go test -count=1 -v ./tests/system
~~~

The runner records command, UTC times, exact source files/dirty state, environment, actual exit and stream hashes. Product verdicts remain separate. Raw run directories are ignored; deliberately curate synthetic, sanitized public evidence while retaining original failures locally.

`make benchmark` records seven samples of checker, model-event and virtual-target/reducer benchmarks with allocations and workload-specific counters. Those measurements include the stated harness/checker/trace costs. They are not production database throughput or native tail-latency distributions. Report variability and misses of the documented 1-second checker and 2-second model-event targets rather than converting a target into a measured result.

`make security-online` resolves a concrete `golang.org/x/vuln` revision, saves the module metadata and executes that revision's govulncheck against current source. Download failure or a scan error is non-success; an older scan is not proof for a changed source/dependency tree.

`make clean-verify` requires clean, reviewed, committed source. It performs an actual non-hardlinked Git clone with exact byte/commit comparison and a fresh Go build cache, then runs implemented verification, fuzz, mutation, benchmark, online security, campaign, native lab, demo and teardown. It retains its checkout and command artifacts on failure or success. The host/toolchain and verified module-download cache are shared; this is not a fresh VM, independent review or Linux runtime CI.

## Cleanup and recovery

The native parent owns process lifetimes, cancellation, waits, files and socket-close checks. An interruption preserves partial native evidence and an explicit non-success outcome. A process kill plus completed File.Sync is conformance evidence, not a physical power-loss test.

`make teardown` removes only flat `.local-runs/demo-<UTC>-<suffix>` kernel demos with the exact synthetic ownership marker and known regular, single-link files. It rejects aliased parents, hardlinks, nested/unrecognized files and unsafe marked candidates before deletion. It preserves target demos, campaign/native records and verification evidence. There is no automatic recursive deletion of user directories.

On dependency failure, inspect the actual version/network/platform error and continue unrelated local checks. On a flaky result, retain the failure, reproduce its inputs and diagnose before rerunning. Do not retry until green or omit an error seed. On journal corruption, stop and retain bytes; only documented incomplete-EOF recovery is automatic. On trace mismatch, use the matching source checkout/build and compare inputs; do not edit headers or hashes to conceal a mismatch.

Back up source, locks, patch and entire reviewed run directories together. Schema upgrades require explicit new versions/conversion outputs retaining the originals. There is no live migration service. A rollback changes source and dependency records together and retains old experiments.

## Public export and CI

Review every public file before adding it to `PUBLIC_FILES.txt`. Audit the exact working/index contents, complete reachable Git refs and export bytes:

~~~sh
make public-audit
python3 scripts/public_audit.py --export ../faultbraid-reviewed.zip
python3 scripts/public_audit.py --archive ../faultbraid-reviewed.zip
~~~

The export destination must be new and outside the source tree. The deterministic ZIP contains exactly the reviewed inventory, with no binaries, ignored raw runs or private/operator material. Archive verification rejects path aliases, duplicate names, unsupported file types, resource excess and any content differing from the reviewed bytes. Detection is heuristic; retain manual content/history review and independent release approval.

Checked workflows use pinned action revisions, `contents: read` and checkout without persisted credentials. Fork changes must not receive repository secrets or privileged execution. CodeQL has a separate analysis-upload permission where the platform allows it. Configured workflows are not executed CI evidence: current Linux runtime jobs remain NOT RUN until actual hosted execution is observed.

No hosted compute is required for the core. A static report may be published only after its contents are reviewed; uploading private histories or starting paid infrastructure is outside normal local execution. Optional hosting costs require current provider/account verification and explicit consent; no purchase or paid deployment is implied.
