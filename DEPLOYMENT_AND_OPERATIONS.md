# Deployment and operations
## Local seed
Requirements: Go (supported stable recommended; source-compatible seed can be checked with1.23.2), Python3.11+, make, git and C compiler for race detector. No root, cloud, containers or credentials.
```
make bootstrap
make build
make verify-seed
make demo
make fuzz
make mutation
make benchmark
make clean-verify
```
`make verify` is the full release gate and intentionally refuses completion while full-system acceptance is missing. `make verify-seed` is the implemented kernel suite. `make security-online` requires network and installs/runs a resolved govulncheck version; failure to download is BLOCKED/FAIL, not success. `make teardown` removes only tool-owned ignored demo output with a marker; source and user-specified inputs are never removed.

## Full local lab
Implement `make target`, `make differential`, `make system`, `make campaign` as real commands during target phases. The three-process lab uses temporary loopback addresses and directories, actual journal IO, known built-in binaries, bounded request bodies, cancellation and owned-process cleanup. It is not safe to expose as an Internet service. Simulation is unprivileged; do not demand Docker just to display a cluster diagram.

## CI
Use standard GitHub-hosted Linux with contents:read, no inherited secrets, no privileged containers and no fork code running under pull_request_target. Run formatting, vet, build, tests/race, locked-spec integrity, public inventory, test-integrity scans and short fuzz campaigns. Once full target exists run its integration/differential tests on every PR; run longer fault/performance campaigns manually or on a weekly schedule with bounded execution. Store synthetic evidence with short retention. A badge is not a substitute for raw logs and coverage of requirements.

## Hosted option
Default is no hosted compute. A reviewed static HTML report can be attached to a release or a static site; never host private histories. Static publication is authorized only for reviewed public artifacts. An optional Linux VM can run trusted scheduled experiments, not arbitrary uploaded code. Cloud billing is a separate human action; no provision/apply command runs automatically.

## Observability and recovery
Run-local JSON counters: events, queue peak, states explored, verdict, unknown counts, faults injected, replay mismatches, reduction checks, duration and output bytes. Keep clocks used for measurement separate from logical simulation time. Log stable IDs, not secrets. No metrics server needed. Readiness means schema/dependency identity valid; a real lab node health endpoint does not mean quorum or data correctness.

Each run is immutable; write to a new directory. Stop on schema/digest corruption. Back up an entire reviewed run directory plus source/dependency lock and patch. Restore by validating identity and replaying; never silently rewrite damaged traces. Roll back binary and dependency lock together, retaining old traces and converters. No live DB migrations exist; new trace schemas require explicit non-destructive conversion.

## Failure recovery
Dependency install failure: inspect network/platform/version; record ENV-001 and keep running independent tasks. Flaky test: preserve failing evidence, reproduce a fixed seed repeatedly, find the cause; retry-until-green is not a fix. External API unavailable: no core API; optional deployment stays blocked. Journal/format failure: reproduce on clean store, test old/new formats, no manual undocumented state repair. CI failure: inspect actual job logs and runner differences. Benchmark instability: report variance and increase repetitions, never cherry-pick. Merge conflict: one integrator resolves and reruns affected tests. Weak executor output: reject/escalate rather than normalize it. Corrupted workspace: preserve diffs/evidence, use clean checkout, do not conceal corruption.

## Cost tiers (vendor snapshot, not a purchase)
| Tier | Services / assumptions | Fixed cost | Variable drivers / controls | Added evidence |
|---|---|---|---|---|
| LOCAL / FREE BASELINE | Existing machine + public repo standard hosted CI | $0 incremental service fees; own hardware/electricity excluded | Bound CI duration/artifacts; standard public runners only | Entire core and reproducibility path |
| CREDIBLE HOSTED DEMO | Optional Lightsail Linux IPv4 2GB bundle; static report preferred instead | Listed US$12/month bundle; taxes/region add-ons excluded | Transfer/storage beyond allowance, snapshots; verify selected region and current tariff before consent | Stable optional trusted lab, not needed for core |
| PREMIUM / SCALE TEST | Three optional Lightsail 8GB Linux IPv4 bundles | Listed 3 x US$44 = US$132/month equivalent; not a recommended spend | Billed running time, transfer, snapshots; hard budget and explicit teardown | Cross-host conformance and resource comparison, not production scale |

Sources: GitHub Actions billing and AWS Lightsail pricing, accessed2026-10-10 (SOURCE_LEDGER S22-S23). Pricing page examples sometimes show older configurations; the table uses current bundle rows, not marketing examples. Free trials are excluded from the budget. Exact chosen-region total and tax are UNKNOWN until account selection. No subscription, billing approval, domain or purchase is authorized.
