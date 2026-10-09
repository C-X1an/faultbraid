# Third-party notices
The delivered Go runtime source uses only the Go standard library; Python tooling uses its standard library. No third-party Go implementation has been vendored into this kernel. Go and Python retain their respective upstream licenses. Original project source and documentation use the MIT License.

The planned etcd/raft integration is Apache-2.0; the planned Porcupine differential checker is MIT. Those dependencies are not installed or bundled in this seed. Preserve exact upstream license/NOTICE files and record source/version/patch digests when integration is implemented. Do not claim this file is a license audit of absent dependencies.

Pinned GitHub Actions are separately fetched by GitHub when CI runs. Their source revisions and licenses must be reviewed on update. Current pins were resolved from official repositories on 2026-10-10; actual workflow execution has not occurred in this research container.
