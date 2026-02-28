# Security Controls — NIST 800-53 Inspired

Curated subset of NIST SP 800-53r5 controls applicable to this project.

## RA-5: Vulnerability Management

**Status**: Implemented

- CI pipeline runs on every push/PR to `main`:
  - **semgrep**: SAST across Go, Rust, JS/TS (p/default + p/security-audit + p/owasp-top-ten)
  - **govulncheck**: Go-specific vulnerability scanner (symbol-level reachability)
  - **osv-scanner**: SCA across go.mod, Cargo.lock, package-lock.json
  - **cargo-audit**: Rust crate advisory database
  - **gitleaks**: Secret detection in working tree + git history
- **Triage SLAs**:
  - Critical: fix within 7 days
  - High: fix within 14 days
  - Medium: fix within 30 days
  - Low: next planned release
- **Current status**: 0 known vulnerabilities (last scan: see CI run)

## SA-10 / SA-15: Secure Development & Configuration

**Status**: Implemented

- **Config loading**: All environment variables loaded via `config.Load()` with `sync.Once` — no scattered `os.Getenv` calls
- **Build integrity**: `make build` produces deterministic binaries; CI verifies build succeeds
- **Code review focus areas** (enforced by convention):
  - `os/exec` usage: only in `internal/rustclient/`, binary path validated under project root
  - HTTP clients: all use shared client with timeouts (`internal/httpclient/`)
  - Auth: `ADMIN_API_KEY` checked via constant-time comparison (see improvement below)
  - Input parsing: all query params go through `clamp`/`parseInt`/`parseFloat` with bounds

### Threat Model

```
┌──────────────────────────────────────────────────────────────┐
│                     TRUST BOUNDARIES                         │
│                                                              │
│  [Browser] ──HTTP──▶ [Go API :8000]                         │
│                        │                                     │
│                        ├──▶ [Yahoo Finance API] (external)   │
│                        ├──▶ [FMP API] (external, keyed)      │
│                        ├──▶ [Rust binary] (local subprocess) │
│                        └──▶ [Cache file] (local disk)        │
│                                                              │
│  Secrets: .env (FMP_API_KEY, ADMIN_API_KEY)                  │
│  Assets: S&P 500 data, insider transaction records           │
└──────────────────────────────────────────────────────────────┘
```

**Key threats and mitigations**:

| # | Threat (STRIDE) | Vector | Mitigation |
|---|----------------|--------|------------|
| T1 | Spoofing | Unauthenticated access to scan/refresh | `ADMIN_API_KEY` middleware |
| T2 | Tampering | Malicious cache file injection | Cache written with 0600 perms, validated JSON |
| T3 | Info Disclosure | API key leakage in logs | Config values never logged; `.env` gitignored |
| T4 | Info Disclosure | Error messages exposing internals | Structured JSON errors, no stack traces |
| T5 | Denial of Service | Slowloris / resource exhaustion | Server timeouts (Read/Write/Idle), rate limiter |
| T6 | Denial of Service | Excessive API calls to Yahoo/FMP | Concurrency semaphore (cap 8), 150ms sleep between batches |
| T7 | Elevation | Path traversal to read arbitrary files | `safeStaticPath()` + `..` rejection |
| T8 | Tampering | Rust binary replaced with malicious one | `VIBES_ANOMALY_BIN` validated under project root |
| T9 | Injection | Query param injection | Params parsed as int/float with bounds; no shell construction |
| T10 | Spoofing | XSS via API data in frontend | React JSX auto-escapes; legacy `innerHTML` files deprecated |

## CM-10(1): Open-Source Dependency Governance

**Status**: Implemented

- **Go**: Modules pinned in `go.mod` with checksums in `go.sum`
- **Rust**: Crates pinned in `Cargo.lock`
- **JS/TS**: Packages pinned in `package-lock.json`
- **CI scanning**: osv-scanner checks all three ecosystems on every push
- **Update process**: `go get <pkg>@latest && go mod tidy` for Go; `cargo update` for Rust; `npm update` for JS

## SI-10: Input Validation

**Status**: Implemented

- `sector`: trimmed string, matched case-insensitively against known GICS sectors
- `limit`: parsed as int, clamped to [1, 600], further capped by actual ticker count
- `baseline_days`: clamped to [30, 730]
- `current_days`: clamped to [7, 90]
- `std_threshold`: clamped to [1.0, 5.0]
- `as_of`: strict `YYYY-MM-DD` format parsing; defaults to today on failure
- No shell string construction anywhere; subprocess uses `exec.Command` with explicit args
- JSON response body size not explicitly capped (acceptable: data is bounded by S&P 500 ~503 companies)

## SI-11: Error Handling

**Status**: Implemented

- All errors returned as structured JSON: `{"error": "message"}`
- No stack traces or internal paths exposed to clients
- Provider failures logged server-side, not leaked to responses
- `stderr` from Rust subprocess captured separately and logged (not mixed with `stdout`)
- Health endpoints (`/api/health`, `/api/health/providers`) prove provider availability

## AU-12 / AU-11: Audit Logging

**Status**: Implemented (basic)

- Request logging middleware with method, path, status, duration, request ID
- Provider status logged (rate-limited, fallback used)
- Dashboard build metrics logged (ticker count, quote count, trend count)
- **Retention**: stdout/stderr (container or systemd journal in prod)
- **Not logged**: API keys, request bodies, user data

## AC-17(2) / SC-28(1): Encryption

**Status**: Documented (pre-deployment)

- **In transit**: Server listens on plain HTTP; TLS termination expected via reverse proxy (nginx, caddy, cloud LB)
- **At rest**: Cache file contains public market data only (no secrets). `.env` contains API keys — should use OS-level disk encryption or vault in production.
- **Secrets flow**: `.env` → `config.Load()` → Go variables. Never written to cache, logs, or responses.

## Checklist Summary

| Control | ID | Status |
|---------|-----|--------|
| Automated vuln scanning in CI | RA-5 | Done |
| Secure SDLC + integrity checks | SA-10/SA-15 | Done |
| Dependency hygiene (pin + scan) | CM-10(1) | Done |
| Input validation + injection prevention | SI-10 | Done |
| Robust error handling | SI-11 | Done |
| Structured logs + retention | AU-12/AU-11 | Done |
| Encrypt in transit/at rest | AC-17/SC-28 | Documented |
