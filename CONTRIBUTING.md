# Contributing
Start with AGENTS.md and the locked acceptance contract. Use a supported Go toolchain (the release pin is in .go-version), Python3.11+, Git, Make and a C compiler. The core needs no service account or API key.

Run `make verify-seed`, then `make fuzz mutation`. For performance changes run `make benchmark` and retain every repetition. Commit reviewed changes before `make clean-verify`. Full distributed changes must also pass system/differential/campaign/security checks through `make verify`.

Include the affected requirement/AC IDs, source changes, real positive/negative tests, measured evidence and changed limitations. Do not hide integration under mocks or weaken locked assertions. Public tests are first-class source. Security-sensitive reports should follow SECURITY.md rather than exposing secrets in an issue.

Update the reviewed PUBLIC_FILES.txt only after inspecting every new public file. Keep generated binaries, raw runs, credentials and private data out of Git. No force-push or destructive history rewrite as a shortcut. CI is unprivileged and uses no project secrets.
