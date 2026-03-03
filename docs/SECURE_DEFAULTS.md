# Secure Defaults Configuration

All configurable settings for the Vibes application, their defaults, security relevance, and operational impact. Aligned with NIST SP 800-218 practice PW.9 (Configure Software to Have Secure Settings by Default) and NIST SP 800-53 SA-8(23).

---

## Environment Variables

| Variable | Default | Required | Security Relevance |
|---|---|---|---|
| `FMP_API_KEY` | (empty) | Yes | API key for Financial Modeling Prep. Leaking grants access to paid financial data API. Inject via 1Password, never commit. |
| `SEC_API_KEY` | (empty) | No | API key for SEC-API.io. Same secret handling as FMP. |
| `EODHD_API_KEY` | (empty) | No | API key for EODHD. Same secret handling. |
| `FINANCIAL_DATASETS_API_KEY` | (empty) | No | API key for Financial Datasets. Same secret handling. |
| `ADMIN_API_KEY` | (empty) | Recommended | Protects `/api/dashboard/refresh` and `/api/scan`. When empty, these endpoints are open. Set to a strong random value (32+ chars). Compared using `crypto/subtle.ConstantTimeCompare`. |
| `FMP_FREE_TIER` | `false` | No | When `true`, reduces API call volume to stay within free tier limits (250 calls/day). Affects insider data page count and scan limits. |
| `PORT` | `8000` | No | HTTP listen port. No security impact unless binding to privileged port (<1024). |

## HTTP Server Timeouts

Configured in `cmd/api/main.go` to prevent slowloris and resource exhaustion attacks.

| Setting | Value | Purpose |
|---|---|---|
| `ReadHeaderTimeout` | 10s | Limits time to read request headers. Prevents slowloris. |
| `ReadTimeout` | 30s | Limits total time to read the full request (headers + body). |
| `WriteTimeout` | 120s | Limits time to write the response. Set high to accommodate dashboard builds. |
| `IdleTimeout` | 60s | Limits keep-alive idle time. Prevents connection hoarding. |

## Rate Limiting

| Setting | Value | Scope | Purpose |
|---|---|---|---|
| Scan rate limit | 1 per 5s per IP | `/api/scan` | Prevents scan abuse. Uses in-memory map with eviction. |
| Rate limiter max size | 10,000 entries | Global | Caps memory usage of rate limiter map. |
| Rate limiter evict age | 1 hour | Global | Cleans stale entries from rate limiter. |
| Cache refresh debounce | 5 minutes | Dashboard | Prevents multiple concurrent refresh operations. |

## Security Headers

Applied to all responses via `securityHeaders` middleware in `cmd/api/middleware.go`.

| Header | Value | Purpose |
|---|---|---|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing. |
| `X-Frame-Options` | `DENY` | Prevents clickjacking via iframes. |
| `X-XSS-Protection` | `1; mode=block` | Legacy XSS filter (defense in depth). |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits referrer information leakage. |
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; connect-src 'self'; img-src 'self' data:;` | Restricts resource loading origins. Prevents XSS via script injection. |
| `X-Request-Id` | (16-char hex, crypto/rand) | Unique per-request correlation ID for audit logging. |

## Input Validation Bounds

Configured in `cmd/api/main.go`.

| Parameter | Valid Range | Default | Rejection Behavior |
|---|---|---|---|
| `sector` | GICS sector enum (11 values + "Unknown") | (none) | HTTP 400 with JSON error |
| `limit` | 0–600 | 50 (when sector specified) | HTTP 400 with JSON error |
| `baseline_days` | 30–730 | 365 | Clamped silently |
| `current_days` | 7–90 | 30 | Clamped silently |
| `std_threshold` | 1.0–5.0 | 2.0 | Clamped silently |
| `as_of` | `YYYY-MM-DD` format | today | Falls back to today on parse error |

## Static File Serving

| Setting | Value | Purpose |
|---|---|---|
| Path traversal guard | `safeStaticPath()` with `filepath.Clean` + prefix validation | Prevents `../` escape from `static/` and `frontend/dist/` directories. |
| SPA asset caching | `Cache-Control: public, max-age=31536000, immutable` | Vite content-hashed assets. Safe for long caching. |
| `..` literal check | `strings.Contains(subpath, "..")` | Early reject before path resolution. |

## Secrets Management

| Method | Usage | Security Level |
|---|---|---|
| **1Password CLI** (`op run --env-file=.env.tpl`) | Recommended for production | High — secrets never touch disk, injected per-process |
| **`.env` file** | Local development only | Medium — gitignored, but plaintext on disk |
| **Environment variables** | CI/CD | Medium — depends on CI secret storage |

## Build Provenance

| Setting | Value | Purpose |
|---|---|---|
| Version embedding | `-X main.version=$(VERSION)` | Traces binary to source version |
| Commit embedding | `-X main.commit=$(COMMIT)` | Traces binary to exact source commit |
| Health endpoint | `/api/health` returns `version` + `commit` | Runtime provenance verification |
| Checksums | `make checksums` generates `SHA256SUMS` | Binary integrity verification |
| SBOM | CycloneDX JSON generated in CI | Supply chain transparency |

## Rust Binary Invocation

| Setting | Value | Purpose |
|---|---|---|
| Binary path validation | `validateBinPath()` in `internal/rustclient/` | Prevents arbitrary binary execution |
| Hardcoded subcommands | `"anomaly"`, `"trend"` only | No user-controlled command injection |
| Data passing | JSON via stdin/stdout | No shell interpolation risk |
