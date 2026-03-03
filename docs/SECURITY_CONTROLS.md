# Security Controls Matrix

Unified mapping of **NIST SP 800-218 (SSDF v1.1)** practices to **NIST SP 800-53r5** controls, with implementation status and evidence artifacts for the Vibes project.

## Framework Alignment Overview

The Vibes project implements a defense-in-depth strategy by aligning two complementary NIST frameworks:

- **NIST SP 800-218 (SSDF)**: Defines *what* secure software development practices to follow across the SDLC.
- **NIST SP 800-53r5**: Defines *which* security and privacy controls protect the operational system.

Together, they ensure that security is embedded from code through deployment and operation.

---

## Controls Matrix

### PO: Prepare the Organization

| SSDF Practice | 800-53 Control | Status | Evidence / Artifact |
|---|---|---|---|
| **PO.1** Security requirements | SA-8 | Done | `SECURITY.md`, `docs/SECURITY_CONTROLS.md` (this file) |
| **PO.2** Roles and responsibilities | SA-3 | Done | `CODEOWNERS` assigns ownership for all paths; security-sensitive paths require explicit review |
| **PO.3** Toolchains | RA-5, SA-11 | Done | `.github/workflows/security.yml` (semgrep, govulncheck, osv-scanner, cargo-audit, gitleaks), `.github/workflows/ci.yml` (go vet, go test -race) |
| **PO.4** Check criteria | SA-15 | Done | Release criteria defined below in "Security Check Criteria" section |
| **PO.5** Secure environments | SA-8(23) | Done | `docs/SECURE_DEFAULTS.md`, `.env.tpl` for 1Password injection, `.gitignore` excludes secrets |

### PS: Protect the Software

| SSDF Practice | 800-53 Control | Status | Evidence / Artifact |
|---|---|---|---|
| **PS.1** Code access control | AC-3, AC-6 | Done | GitHub branch protection, `CODEOWNERS` review requirements |
| **PS.2** Release integrity | SA-10, SR-4 | Done | `Makefile` embeds version+commit via `-ldflags`; `make checksums` generates SHA256; `/api/health` exposes version |
| **PS.3** Archive and protect releases | SR-4 | Done | CI SBOM generation via `anchore/sbom-action` (CycloneDX); checksums published as build artifacts |

### PW: Produce Well-Secured Software

| SSDF Practice | 800-53 Control | Status | Evidence / Artifact |
|---|---|---|---|
| **PW.1** Secure design | SA-8 | Done | Threat model below; trust boundaries defined between Go API, Rust CLI, external APIs |
| **PW.4** Reuse vetted components | CM-10(1) | Done | `go.mod`, `Cargo.toml`, `package.json` pin dependencies; `osv-scanner` + `cargo-audit` + `govulncheck` verify |
| **PW.5** Secure coding practices | SI-10, SI-11 | Done | Input validation (GICS enum, bounds checking), structured JSON errors, no stack traces in responses |
| **PW.6** Build configuration | SA-10 | Done | `Makefile` with `-ldflags`, reproducible build targets, `VERSION` file |
| **PW.7** Code analysis | SA-11 | Done | `go vet ./...` in CI, `semgrep` SAST in security workflow, `go test -race` for data race detection |
| **PW.8** Testing | SA-11 | Done | `cmd/api/middleware_test.go`, `cmd/api/main_helpers_test.go`, `internal/config/config_test.go`, `cargo test` for Rust |
| **PW.9** Secure defaults | SA-8(23) | Done | `docs/SECURE_DEFAULTS.md` documents all config, timeouts, rate limits, and their security impact |

### RV: Respond to Vulnerabilities

| SSDF Practice | 800-53 Control | Status | Evidence / Artifact |
|---|---|---|---|
| **RV.1** Identify vulnerabilities | RA-5 | Done | Weekly + push security scans (`.github/workflows/security.yml`); `SECURITY.md` defines disclosure process |
| **RV.2** Assess and remediate | RA-5(2) | Done | Triage SLAs in `SECURITY.md`; `continue-on-error: true` allows advisory scan results |
| **RV.3** Root cause analysis | SA-11(4) | Done | Post-fix review process documented in `SECURITY.md`; structured audit logging supports investigation |

---

## 800-53 Controls Detail

### RA-5: Vulnerability Monitoring and Scanning

- **Implementation**: Automated scanning via CI (semgrep for SAST, govulncheck for Go dependencies, osv-scanner for SCA, cargo-audit for Rust, gitleaks for secrets)
- **Frequency**: Every push to `main` + weekly scheduled scan
- **Artifact**: `.github/workflows/security.yml`

### SA-3: System Development Life Cycle

- **Implementation**: Code ownership and review requirements via `CODEOWNERS`; security-sensitive paths (`internal/config/`, `cmd/api/middleware.go`, `internal/rustclient/`) require explicit review
- **Artifact**: `CODEOWNERS`

### SA-8: Security and Privacy Engineering Principles

- **Implementation**: Defense-in-depth (Go input validation + Rust computation isolation + frontend CSP), least privilege (rate limiting, admin-only endpoints), fail-secure defaults
- **Artifact**: `docs/SECURE_DEFAULTS.md`, `cmd/api/middleware.go`

### SA-10: Developer Configuration Management

- **Implementation**: Version + commit hash embedded in binaries via `-ldflags`; SHA256 checksums generated; `/api/health` exposes build provenance
- **Artifact**: `Makefile`, `VERSION`, `cmd/api/main.go`

### SA-11: Developer Testing and Evaluation

- **Implementation**: Unit tests for security-critical paths (path traversal, rate limiting, auth, input validation); `go vet` static analysis; `-race` data race detection; Rust `cargo test`
- **Artifact**: `cmd/api/middleware_test.go`, `cmd/api/main_helpers_test.go`, `internal/config/config_test.go`

### SA-15: Development Process, Standards, and Tools

- **Implementation**: CI enforces build + test + vet before merge; security scanners run in parallel workflow
- **Artifact**: `.github/workflows/ci.yml`, `.github/workflows/security.yml`

### SI-10: Information Input Validation

- **Implementation**: `sector` parameter validated against GICS enum; `limit` bounded to `[0, 600]`; `parseInt`/`parseFloat` with safe defaults; path traversal prevention in `safeStaticPath`
- **Artifact**: `cmd/api/main.go` (`isValidSector`, `clamp`, `clampFloat`), `cmd/api/middleware.go` (`safeStaticPath`)

### SI-11: Error Handling

- **Implementation**: Structured JSON error responses (`{"error": "..."}`) instead of raw error text; no stack traces or internal paths exposed; `WriteTimeout` prevents hung connections
- **Artifact**: `cmd/api/main.go` (handleDashboard, handleScan), `cmd/api/middleware.go`

### AU-12/AU-11: Audit Record Generation and Retention

- **Implementation**: Every request logged with unique `X-Request-Id`, method, path, status code, and duration via `securityHeaders` middleware; `crypto/rand` generated request IDs
- **Artifact**: `cmd/api/middleware.go` (`securityHeaders`, `generateRequestID`)

### AC-17(2): Remote Access — Protection of Confidentiality/Integrity

- **Implementation**: Admin endpoints protected by `ADMIN_API_KEY` with `crypto/subtle.ConstantTimeCompare` (timing-attack resistant); Bearer token and `X-Admin-Key` header both supported
- **Artifact**: `cmd/api/middleware.go` (`adminOrRateLimit`)

### SC-28(1): Protection of Information at Rest

- **Implementation**: Secrets injected at runtime via 1Password CLI (`op run --env-file=.env.tpl`); `.env` files gitignored; no secrets in source code
- **Artifact**: `.env.tpl`, `.gitignore`, `Makefile` (`go-run-op` target)

### CM-10(1): Software Usage Restrictions — Open-Source Software

- **Implementation**: All dependencies tracked in `go.mod`/`go.sum`, `Cargo.toml`/`Cargo.lock`, `package.json`/`package-lock.json`; SCA scanning via `osv-scanner`; SBOM generated per build
- **Artifact**: Dependency manifests, `.github/workflows/ci.yml` (sbom job), `.github/workflows/security.yml`

### SR-4: Provenance

- **Implementation**: CycloneDX SBOM generated in CI; version+commit embedded in binaries; SHA256 checksums for release artifacts
- **Artifact**: `.github/workflows/ci.yml` (sbom job), `Makefile` (checksums target)

---

## Threat Model

### Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│  BROWSER (untrusted)                                        │
│  React SPA ──── CSP enforced ────▶ Go API (:8000)           │
└───────────────────────────────────┬─────────────────────────┘
                                    │ HTTP (localhost only)
┌───────────────────────────────────▼─────────────────────────┐
│  GO API SERVER (trusted)                                    │
│  ├── Input validation (SI-10)                               │
│  ├── Rate limiting + admin auth (AC-17)                     │
│  ├── Audit logging (AU-12)                                  │
│  ├── Security headers / CSP                                 │
│  └── Subprocess boundary ──────────▶ Rust vibes-anomaly     │
│                                      (validated binary path)│
├─────────────────────────────────────────────────────────────┤
│  EXTERNAL APIs (semi-trusted, rate-limited)                 │
│  ├── FMP API (HTTPS, API key in env)                        │
│  ├── Yahoo Finance (HTTPS, no auth)                         │
│  └── SEC-API.io (HTTPS, API key in env)                     │
├─────────────────────────────────────────────────────────────┤
│  SECRETS (1Password vault or .env)                          │
│  └── Injected at runtime, never committed                   │
└─────────────────────────────────────────────────────────────┘
```

### Key Assets

1. API keys (FMP, SEC-API, EODHD, Financial Datasets)
2. Admin API key
3. Market data cache (non-sensitive, integrity matters)
4. Insider trading aggregation data

### Top Threats

| # | Threat (STRIDE) | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 1 | API key leakage via logs/repo | Medium | High | 1Password injection, `.gitignore`, gitleaks scan |
| 2 | Path traversal via static file serving | Low | High | `safeStaticPath()` with `filepath.Clean` + prefix check |
| 3 | Denial of service (resource exhaustion) | Medium | Medium | Rate limiting, HTTP timeouts, cache debouncing |
| 4 | Admin auth bypass (timing attack) | Low | High | `crypto/subtle.ConstantTimeCompare` |
| 5 | Dependency vulnerability | Medium | Medium | `govulncheck`, `osv-scanner`, `cargo-audit` weekly |
| 6 | Log injection | Low | Low | Structured logging with request ID prefix |
| 7 | SSRF via API endpoints | Low | Medium | Hardcoded external API base URLs in `httpclient` |
| 8 | XSS via dashboard data | Low | Medium | React JSX auto-escaping, CSP header |

---

## Security Check Criteria (PO.4.1)

The following must pass before any release:

- [ ] All CI jobs green (`go vet`, `go test -race`, `cargo test`, frontend build)
- [ ] Zero critical/high vulnerabilities in security scan (or documented risk acceptance)
- [ ] SBOM generated and archived as CI artifact
- [ ] SHA256 checksums generated for binary artifacts
- [ ] No secrets detected by gitleaks
- [ ] `SECURITY.md` and `SECURITY_CONTROLS.md` are up to date
- [ ] Version tag matches `VERSION` file
