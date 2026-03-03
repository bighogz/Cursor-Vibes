# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | Yes       |
| < 1.0   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, email **security@bighogz.dev** (or contact [@bighogz](https://github.com/bighogz) directly via GitHub) with:

1. A description of the vulnerability
2. Steps to reproduce
3. Affected component (Go API, Rust binary, React frontend, CI/CD)
4. Potential impact assessment

## Response SLAs

| Severity | Initial Response | Fix Target |
|----------|-----------------|------------|
| Critical | 24 hours        | 7 days     |
| High     | 48 hours        | 14 days    |
| Medium   | 5 business days | 30 days    |
| Low      | 10 business days| Next release|

## Vulnerability Management Process

This project follows a structured vulnerability response process aligned with NIST SP 800-218 (SSDF) practice RV.1-RV.3 and NIST SP 800-53r5 controls RA-5, SA-11.

1. **Identification (RV.1)**: Automated scanning via CI (semgrep, govulncheck, osv-scanner, cargo-audit, gitleaks) runs weekly and on every push. Manual reports accepted per the process above.
2. **Triage (RV.2.1)**: Each vulnerability is analyzed for exploitability and impact using the component's threat model (see `docs/security/threat-model.md`).
3. **Remediation (RV.2.2)**: Fixes are developed, tested, and released according to the SLA table above. Security advisories are published for any fix that changes external behavior.
4. **Root Cause Analysis (RV.3)**: After remediation, the root cause is documented and the SDLC is reviewed for process improvements to prevent recurrence.

## Scope

The following components are in scope:

- Go API server (`cmd/api/`)
- Go internal packages (`internal/`)
- Rust anomaly engine (`rust-core/`)
- React frontend (`frontend/`)
- CI/CD pipelines (`.github/workflows/`)
- Configuration and secrets management (`.env`, `.env.tpl`)

## Security Controls

This project implements controls from:

- **NIST SP 800-53r5**: RA-5, SA-3, SA-8, SA-10, SA-11, SA-15, SI-10, SI-11, AU-12, CM-10(1), AC-17, SC-28
- **NIST SP 800-218 (SSDF v1.1)**: PO.1-PO.5, PS.1-PS.3, PW.1-PW.9, RV.1-RV.3

See `docs/security/threat-model.md` for the full controls matrix.
