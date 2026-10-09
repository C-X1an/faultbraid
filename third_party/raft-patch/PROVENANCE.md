# Instrumented etcd/raft provenance

- Module: `go.etcd.io/raft/v3`
- Version: `v3.7.0`
- Upstream: `https://github.com/etcd-io/raft`
- Tag commit: `b867cf13f6bc0dae21204302df97bc2355c3af55`
- Module checksum: `h1:BGzlwx07bLv8PW6OU5HObuz1y4hlPZUXA07pM1mPUh4=`
- Module-file checksum: `h1:6gX6T2X907DjnjsFLODnTxba77stjs84W9gTTI0GUNA=`
- Upstream entropy site: `raft.go:2054` in the locked archive
- Instrumentation: per-instance `Config.ElectionRandom func(n int) int`

`prepare.py` asks the Go command for the exact module JSON, validates both Go
checksums, validates the archive and patch SHA-256 values in `lock.json`, safely
extracts the module, checks and applies the patch with Git, then validates the
entire prepared source tree digest. Nil injection retains the upstream global
cryptographic random behavior. Injected selection never calls that source and
rejects results outside `[0,n)`.

The resulting target must be described as **instrumented etcd/raft v3.7.0**.
The generated source lives at `.local-runs/raft` and is intentionally ignored.
The full upstream test suite, including the added injection tests, is the
conformance check for the patched dependency.
