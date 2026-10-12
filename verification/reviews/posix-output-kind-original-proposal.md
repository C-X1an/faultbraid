# TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND

Status: PROPOSED; separate read-only approval and corrected-source execution are pending.

This corrects an erroneous supplemental test predicate. The locked AC-010
contract, every locked assertion and acceptance digest remain unchanged.

On published C4, both actual Linux kernel/full workflows failed in
`TestExclusiveOwnedDirectoryAndAliasParents` at `target_test.go:60` with
`output permissions`. The directory was passed to a shared POSIX helper
which required `0600` for both regular files and directories. The production
`NewDirectory` code requests `0700`; `openExclusive` requests `0600` for files.
The original job logs report the assertion failure, not the observed mode;
no actual mode observation is inferred from that log.

The [Go Mkdir contract](https://pkg.go.dev/os#Mkdir) applies the requested bits
before umask. Owner-only regular report files remain exactly `0600`. Owner-only
report directories are exactly `0700`, including owner search. Neither group
nor other permissions are accepted. The reviewed correction uses `Lstat` and
separate kind-specific assertions, so aliases, special file types and wrong
permission masks cannot satisfy the predicate. Windows retains every existing
protected-DACL/sole-current-user ACE assertion, with additional kind/alias checks.

Only three supplemental test files change. No production implementation,
locked criterion, test selection, skip, timeout or failure-accounting policy
changes. The original failures and original source remain inspectable in Git:
[C4 kernel failure](https://github.com/C-X1an/faultbraid/actions/runs/38257967709),
[C4 full failure](https://github.com/C-X1an/faultbraid/actions/runs/38258329675).
C4 CodeQL separately completed successfully; that does not establish runtime acceptance.

Before integration a separate reviewer must approve the exact proposed test
bytes and exception. Fresh corrected-source verification, current all24 review,
exact public audit and actual hosted Linux retests remain required. Local Windows
execution or emulated permission fixtures must not be labelled Linux runtime PASS.
