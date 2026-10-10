# ADR 009: portable source identity and archive preparation

Source identity lists relative POSIX paths in ascending UTF-8 byte order on
every OS. Each item is `{path,sha256}`, serialized as compact JSON with ASCII
Unicode escapes and sorted object keys. The tree digest hashes those bytes.
Go's encoder matches Python's `ensure_ascii=True`, including DEL and surrogate
pairs. No filesystem or locale case folding participates in this order.

A separate reader found a DEL escaping discrepancy and a Windows Unicode
case-fold ordering discrepancy. The public ASCII tree was unaffected, but
valid boundary filenames could yield different Go/Python identities. Concrete
fixtures now compare the two implementations. Previously captured manifests
retain their original ordered file lists and hashes; the gate verifies those
bytes and compares execution inputs by path. New commands capture the current
algorithm and require fresh execution evidence after source changes.

The reader also found that a source archive without `.git` inherited Windows
`core.autocrlf=true` during Raft patch application. Both `git apply --check` and
`git apply` now explicitly pass command-scoped `core.autocrlf=false`; no global
configuration changes. A non-Git zip fixture performs real patch application
under an inherited `true` setting. The locked upstream zip, patch and prepared
tree digests remain unchanged. Actual upstream archive preparation is checked
separately from this synthetic regression.
