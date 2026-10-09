# Security policy
FaultBraid is experimental verification tooling, not a sandbox, production datastore or audited security boundary. Do not give it confidential histories or untrusted native executables. The initial seed opens no network service and executes only built-in models.

See THREAT_MODEL.md and LIMITATIONS.md. Input/output limits, escaped reports and explicit unknown results are intended controls, not a blanket guarantee. Local hashes cannot defeat a privileged actor who rewrites source and evidence together.

Report suspected vulnerabilities privately through GitHub private vulnerability reporting when enabled for the eventual repository. Until that channel is enabled, do not post secrets, exploit payloads containing private data or sensitive histories in public issues. No response-time guarantee is established. Maintainers must enable a real private reporting channel before inviting sensitive disclosures.

Use supported patched Go releases, inspect module updates, run govulncheck, and use read-only CI permissions for fork contributions. Native process experiments must run only on a disposable authorized host. No cloud credentials are required by the core.
