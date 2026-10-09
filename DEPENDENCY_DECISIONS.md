# Dependency decisions
Research as of 2026-10-10. Release resolution must use current official metadata and preserve exact lock files.

| Dependency | Purpose / license | Verified source and maintenance | Decision / alternatives | Security and cost |
|---|---|---|---|---|
| Go standard library | seed CLI, heap, JSON, search, HTML, SHA256; BSD-style Go license | Official Go docs; local installed1.23.2, retrieved release endpoint showed cached1.26.5 while1.27 notes remained draft | Prefer Go for inspectable systems code and direct Raft API; Rust is valid but not needed for this bounded trusted harness | No runtime fees. Do NOT call1.23.2 current/supported. Resolve latest supported stable patch at build runtime; record exact version |
| etcd/raft v3.7.0 observed baseline | mature consensus core; Apache-2.0 | Official API and release sources S18-S19; retrieved page warns newer module may exist | Reuse algorithm, implement transport/persistence integration; avoid own Raft clone | Review current patched3.7.x release and advisories before lock. Small election-entropy patch is explicit, licensed and conformance-tested |
| Porcupine | differential test oracle; MIT | Official repo/pkg docs; v1.3.0 observed but latest banner ambiguous | Keep out of runtime core. Independent comparison is more credible than only our own oracle | Resolve stable patch and retain go.sum. Unknown/timeout stays inconclusive |
| Python3 standard library | build/evidence/export scripts | Existing3.11+ available; no external Python package required | Simple file/process tooling; Go-only rewrite adds no core value | No secret dump or shell execution; user-visible scripts readable |
| govulncheck | reachable Go vulnerability scanning | Official Go vulnerability docs | Install a pinned resolved tool version at bootstrap; scanner DB remains current | Needs network; blocked download is not PASS |
| GitHub Actions official actions | supported toolchain CI | Official secure-use guidance | Use commit-SHA pinned checkout/setup-go; record version comments, Dependabot review | contents:read, no secrets in untrustedPR, no pull_request_target checkout |

No database, Redis, Kafka, Kubernetes, Terraform, LLM provider, paid SaaS, plotting framework or browser runtime is necessary in the core. Avoid vendoring entire large repositories without purpose. Retain module checksums, source licenses and the tiny explicit Raft patch when target work begins. Initial seed go.mod has no third-party requires and therefore no fabricated go.sum.

## Runtime dependency resolution policy
Probe installed toolchains and network. Fetch official Go release metadata; choose a supported stable patch, validate official archive checksum, record exact version and date in the local dependency resolution report. Resolve compatible maintained stable Raft/Porcupine versions through module metadata, inspect API diffs/advisories and commit go.mod/go.sum. Prefer the observed baseline where necessary for reproducibility, but never bypass vulnerability findings merely to retain a version. An unavailable network blocks dependency integration only; continue the independent standard-library kernel, tests, documentation and export audit. Fundamental interface changes require a small recorded deviation, not silent redesign.

## Final support-resolution update (2026-10-10)
Official Go release-history/rebuild search results resolve Go 1.27.2, released 2026-10-08 (S40). The supported verification/CI pin is 1.27.2. Earlier cached endpoint observations in this document are historical and superseded for version selection. The actual research container remains Go 1.23.2; no supported-toolchain execution is claimed. Resolve raft/Porcupine versions and source digests before adding dependencies; do not invent go.sum entries.
