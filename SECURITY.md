# Security policy

[← Field guide](docs/README.md) · [Overview](README.md) · [Try the demo](docs/demo/DEMO.md) · [Verification](VERIFICATION_REPORT.md)
FaultBraid is experimental verification tooling, not a sandbox, production datastore or audited security boundary. Do not give it confidential histories or untrusted native executables. Ordinary checker/simulation/report commands are offline; explicit native lab mode opens bounded loopback HTTP listeners and executes only the current identified built-in target. Loopback is not authentication, and the lab must not be exposed as an Internet service.

See THREAT_MODEL.md and LIMITATIONS.md. Input/output limits, escaped reports and explicit unknown results are intended controls, not a blanket guarantee. Local hashes cannot defeat a privileged actor who rewrites source and evidence together.

Report suspected vulnerabilities privately through GitHub private vulnerability reporting when enabled for the eventual repository. Until that channel is enabled, do not post secrets, exploit payloads containing private data or sensitive histories in public issues. No response-time guarantee is established. Maintainers must enable a real private reporting channel before inviting sensitive disclosures.

Use supported patched Go releases, inspect module updates, run govulncheck, and use read-only CI permissions for fork contributions. Native process experiments must run only on a disposable authorized host. No cloud credentials are required by the core.
