# Contributing to FaultBraid

**Small, evidence-backed changes are easier to trust.** Start with the [project overview](README.md), [acceptance contract](ACCEPTANCE_SPEC.md), [architecture](ARCHITECTURE.md), [CLI/API contract](API_SPEC.md) and [threat model](THREAT_MODEL.md).

## Before you change code

Use the pinned Go toolchain from [`.go-version`](.go-version), Python 3.11+, Git, Make and a C compiler. No service account or API key is required for local execution. Keep the public repository free of credentials, private fixtures, generated binaries and raw operational runs.

~~~sh
make bootstrap
make verify-seed
make fuzz mutation
~~~

For changes to distributed behavior, also run the relevant real target suites and full gate:

~~~sh
make system differential campaign
make verify
~~~

For performance changes, use `make benchmark` and retain **all** repetitions, not the fastest sample. Commit reviewed changes before `make clean-verify`; that command uses a fresh committed checkout and build cache on the same host.

## A reviewable pull request includes

- **Scope:** affected requirement and acceptance-criterion IDs, plus the behavior being changed.
- **Proof:** positive and negative tests against the real integration when the claim requires it; mocks are not system evidence.
- **Evidence:** captured commands, outcomes, source identities, and the relevant failure records.
- **Boundaries:** updated limitations, assumptions and security implications where applicable.

Do not weaken locked acceptance assertions, hide failures with skips, or treat `INCONCLUSIVE` as `VALID`. Read [verification methodology](VERIFICATION_REPORT.md) and the [claims-to-evidence map](CLAIMS_EVIDENCE.md).

## Publishing & security

Review every new public file before adding it to [`PUBLIC_FILES.txt`](PUBLIC_FILES.txt). Run `make public-audit`; the audit includes Git history and is a finite heuristic, not exhaustive leak detection. Keep secrets out of issues, commits, sample traces and CI. Do not force-push or rewrite history as a shortcut.

**Security-sensitive reports:** follow [SECURITY.md](SECURITY.md), not public issues. Third-party code and the checked-in Raft patch retain their licenses and attribution in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

---

[← Documentation hub](docs/README.md) · [Reviewer demo](docs/demo/DEMO.md) · [Project overview](README.md)
