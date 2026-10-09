# Explicit limitations
**Status: experimental kernel, not a complete distributed workbench or audited security boundary.**

Implemented: single-register history checking with at most24 operations and bounded search; optional pending writes; diagnostic delete-one reduction; stable event queue; a tiny volatile/durable disk-register model; replay; static HTML; CLI and verification tools. This is not Raft, a real filesystem or an agent sandbox.

Not implemented: pinned instrumented three-voter Raft integration, fault campaigns against that target, causal scenario reduction, Porcupine differential integration, same-target three-process loopback conformance, snapshots/compaction or dynamic membership. Full release is intentionally blocked.

Testing has finite scope. Enumerating short histories over a small value/interval domain is not a proof for arbitrary histories. Five selected semantic mutations are not a comprehensive mutation score. Short fuzz runs are not assurance against every malformed input. Race detection is useful but does not establish distributed correctness. No customer/production scale exists.

The research runtime has Go1.23.2. Source compatibility is separate from supported-release status. Go1.27.2 is pinned for release/CI after primary-source verification; it was not installed or run here. Online dependency/vulnerability scanning and hosted GitHub CI remain NOT RUN until executed in the appropriate environment.

Output creation refuses existing leaves and checks existing parent symlinks. The local operator and filesystem namespace are trusted; concurrent privileged parent-directory replacement is outside this protection. Files are not promised durable against power loss. The report is static and escapes untrusted text, but browser accessibility/visual inspection must be performed separately.

Trace hashes are not signatures. Evidence is machine-captured on an operator-controlled host, not remotely attested. Public scans are heuristic, not complete secret detectors. The exact inventory cannot detect harmful content in an approved file by itself. A full independent reviewer must still challenge contracts, code and evidence.

A new repo has not been published by this package. A local source archive, a clean local checkout and a GitHub repository are different deliverables. No badges assert unexecuted CI or full acceptance.
