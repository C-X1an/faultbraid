# Third-party notices
Original project source and documentation use the MIT License. Python tooling uses its standard library. Go and Python retain their respective upstream licenses. The resolved full source depends on the modules listed below; it no longer has the seed's standard-library-only Go dependency boundary.

* go.etcd.io/raft/v3 v3.7.0: Apache-2.0. The instrumented source is fetched, verified and patched in an ignored build directory; preserve the source license/NOTICE and patch provenance in third_party/raft-patch.
* github.com/anishathalye/porcupine v1.3.1: MIT, differential-test dependency. Exact notice: third_party/licenses/porcupine.txt.
* golang.org/x/sys v0.49.0: BSD-3-Clause, linked by Windows access-control and process APIs. Exact notice: third_party/licenses/x-sys.txt.
* google.golang.org/protobuf v1.36.11: BSD-3-Clause, Raft message serialization. Exact notice: third_party/licenses/protobuf.txt.

dependency.lock.json and go.sum record actual fetched module identities. Upstream test dependencies retain their own notices in the prepared upstream tree. Development Go, GCC/w64devkit, GNU Make, 7-Zip and GitHub CLI are separately installed tools and are not bundled in the public repository. Their successful installation does not establish product or vulnerability acceptance.

Pinned GitHub Actions are separately fetched by GitHub when CI runs. Their source revisions and licenses must be reviewed on update. Current pins were resolved from official repositories on 2026-10-10; actual workflow execution has not occurred in this research container.
