# TEST_SPEC_EXCEPTION_AC-010_OUTPUT_KIND

Status: **APPROVED prospectively for the exact three-file assertion correction below.** Corrected-source release acceptance and hosted retests remain pending.

This exception documents an erroneous supplemental test predicate. It does not revise locked AC-010, any locked assertion, or the acceptance digest. The [separate exception assessment](verification/reviews/posix-output-kind-exception-assessment.json) binds the original source, proposed patch and these approved document bytes.

The reviewed original is published C4, commit `748ba503af3d9b219005c0a1677f4d13b0938c1a`, complete source SHA256 `5ab5285ed8018292f0769a0fd20442ce5cc497897b52b3bff9f34c95cf87d60c`. Both actual Linux [kernel](https://github.com/C-X1an/faultbraid/actions/runs/38257967709) and [full](https://github.com/C-X1an/faultbraid/actions/runs/38258329675) jobs failed in `TestExclusiveOwnedDirectoryAndAliasParents`, at `target_test.go:60`, with `output permissions`. Their original failures remain FAIL. Those logs do not report the observed mode or umask; no numeric mode observation is inferred.

The directory was passed to a shared POSIX helper requiring `0600` for either object kind. The production file implementation requests `0600`, while `NewDirectory` requests `0700`. The [pinned Go Mkdir contract](https://pkg.go.dev/os@go1.27.2#Mkdir) applies requested bits before umask. A protected directory needs owner search as well as read/write; a regular report file does not need execute permission. This mismatch is in the test interpretation, not the operational directory creation.

Approved patch SHA256: `eb9f7be04e23c3b37d1349c5079d93f8db166c7eb4177b4d27a67638f8b237d3`. The original proposed-document SHA256 `556305b5ada6e2e578e5a943aaa9cf1029342ce44c4ad1a31e21156c5a33a258` is retained separately; this approved document has its own hash in the assessment.

Only `internal/report/permissions_other_test.go`, `internal/report/permissions_windows_test.go` and the directory assertion call in `internal/report/target_test.go` change. The corrected assertions use [Lstat](https://pkg.go.dev/os@go1.27.2#Lstat), reject aliases and distinguish regular files from directories. POSIX regular files remain exactly `0600`; directories require exactly `0700`. Group and other grants are rejected for both. Windows retains the entire protected-DACL, sole-current-user, non-inherited-ACE assertion body and adds kind/alias checks for both objects.

The separate reviewer executed the exact proposal on an owned Windows copy: the complete report package passed, an intentional directory-to-file-helper substitution failed the named kind assertion, and the same command passed after exact full-source restoration. This is Windows file/DACL/kind evidence, not execution of the POSIX permission branch. C4 CodeQL separately completed successfully with its configured static queries; that does not establish runtime acceptance.

No production implementation, locked criterion/digest, test selection, skip, timeout, budget or failure-accounting policy is changed or waived. Fresh corrected-source affected and full verification, current all-24 independent assessment, exact public audit and actual hosted Linux retests remain required. Historical local approvals and this narrow exception cannot substitute for those results.
