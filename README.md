# S&P 500 Insider Selling Tracker

Real-time tracking of insider selling across all S&P 500 companies with anomaly detection. Built with **Go**, **Rust**, and a **React** frontend -- zero Python runtime dependency.

Stock prices and quarterly trends are fetched via [go-yfinance](https://github.com/wnjoon/go-yfinance) (native Go with TLS fingerprint spoofing and Yahoo crumb/cookie auth). Anomaly detection and trend computation use a Rust binary for performance, with Go fallbacks when the binary isn't available.

## Quick start

```bash
# Prerequisites: Go 1.23+, Node.js 18+, Rust (optional, rustup.rs)
cp .env.example .env        # add your FMP_API_KEY
make build                   # builds Go API + Rust binary + React frontend
./bin/api                    # http://localhost:8000
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  React SPA (frontend/)                              │
│  ├── AppShell (sidebar + topbar + content)          │
│  ├── DataTable (keyboard nav, sector groups)        │
│  ├── DetailDrawer (price, sparkline, news, insiders)│
│  ├── CommandPalette (⌘K fuzzy search)               │
│  └── Anomaly Scan page + Settings page              │
├─────────────────────────────────────────────────────┤
│  Go API (cmd/api) — port 8000                       │
│  ├── Dashboard builder  → prices, trends, news      │
│  │   ├── go-yfinance    → Yahoo Finance (native Go) │
│  │   ├── FMP client     → insider sells, S&P 500    │
│  │   └── HTTP fallback  → Yahoo v7/v8 (news)        │
│  ├── Rust binary        → anomaly + trend compute   │
│  │   └── Go fallback    → aggregator + trend pkg    │
│  ├── Cache layer        → file-based JSON, 24h TTL  │
│  └── Security           → timeouts, rate limits, CSP│
├─────────────────────────────────────────────────────┤
│  Rust CLI (rust-core/)                              │
│  └── vibes-anomaly: anomaly + trend subcommands     │
└─────────────────────────────────────────────────────┘
```

| Component | Language | Purpose |
|-----------|----------|---------|
| `frontend/` | React + TypeScript + Tailwind | Linear-inspired SPA with keyboard-first UX |
| `cmd/api` | Go | HTTP server, routes, middleware, security headers |
| `cmd/scan` | Go | CLI anomaly scanner |
| `internal/yahoo` | Go | Yahoo Finance client (go-yfinance + HTTP fallback) |
| `internal/fmp` | Go | Financial Modeling Prep API client |
| `internal/dashboard` | Go | Dashboard data assembly, sparkline sampling |
| `internal/trend` | Go | Quarterly trend (linear regression, fallback) |
| `internal/aggregator` | Go | Z-score anomaly detection (fallback) |
| `internal/rustclient` | Go | Subprocess bridge to Rust binary |
| `internal/cache` | Go | File-based dashboard cache |
| `internal/config` | Go | Environment loading via sync.Once |
| `rust-core` | Rust | `vibes-anomaly` binary: anomaly + trend subcommands |

## Frontend

The UI is a Linear-inspired React SPA with:

- **Sidebar**: Navigation + collapsible sector filter groups
- **DataTable**: Dense rows with sector headers, prices, sparklines, news, insider sellers
- **DetailDrawer**: Slide-in panel on row click (deep-linkable via `?stock=AAPL`)
- **CommandPalette**: `⌘K` / `Ctrl+K` fuzzy search across stocks and actions
- **Keyboard navigation**: `j`/`k` move focus, `Enter` opens drawer, `Esc` closes
- **Pages**: Dashboard, Anomaly Scan (with configurable params), Settings

Development (with hot reload):

```bash
# Terminal 1: Go API
./bin/api

# Terminal 2: Vite dev server (proxies /api to :8000)
cd frontend && npm run dev    # http://localhost:5173
```

## Data sources

| Provider | Data | Key required |
|----------|------|--------------|
| **Yahoo Finance** | Stock prices, historical closes, news | No (handled by go-yfinance) |
| **FMP** | S&P 500 list, insider sells, quotes (fallback) | Yes (`FMP_API_KEY`) |
| **SEC-API.io** | Form 3/4/5 insider transactions | Optional (`SEC_API_KEY`) |
| **Financial Datasets** | Form 4 insider trades | Optional (`FINANCIAL_DATASETS_API_KEY`) |
| **EODHD** | Insider transactions | Optional (`EODHD_API_KEY`) |

At minimum, `FMP_API_KEY` is required for the S&P 500 constituent list and insider data. Yahoo Finance handles prices, trends, and news with no API key.

## Setup

1. **Install Go** (1.23+): https://go.dev/doc/install

2. **Install Node.js** (18+): https://nodejs.org

3. **Install Rust** (optional, for faster anomaly/trend): https://rustup.rs

4. **Configure API keys** (choose one):

   **Option A — `.env` file** (simple):

   ```bash
   cp .env.example .env
   # Edit .env and set at least FMP_API_KEY
   ```

   **Option B — 1Password CLI** (no plaintext secrets on disk):

   ```bash
   brew install 1password-cli   # one-time
   op signin                    # authenticate
   # Edit .env.tpl to point op:// URIs to your vault items
   make go-run-op               # injects secrets at runtime
   ```

   If on FMP's free tier (250 calls/day), set `FMP_FREE_TIER=true` to prefer Yahoo for quotes and trends.

5. **Build and run**:

   ```bash
   make build       # Go + Rust + React frontend -> bin/ + frontend/dist/
   ./bin/api        # starts on http://localhost:8000
   # Or with 1Password:
   make go-run-op   # op run injects secrets, no .env needed
   ```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `FMP_API_KEY` | (required) | Financial Modeling Prep API key |
| `FMP_FREE_TIER` | `false` | When `true`, uses Yahoo for quotes/trends to stay under 250 calls/day |
| `ADMIN_API_KEY` | (optional) | Protects `/api/scan` and `/api/dashboard/refresh` |
| `VIBES_ANOMALY_BIN` | auto-discovered | Absolute path to `vibes-anomaly` binary |
| `SEC_API_KEY` | (optional) | SEC-API.io key for Form 3/4/5 data |
| `FINANCIAL_DATASETS_API_KEY` | (optional) | Financial Datasets API key |
| `EODHD_API_KEY` | (optional) | EODHD API key |

## API endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | React SPA (falls back to legacy dashboard if not built) |
| GET | `/api/dashboard` | Dashboard JSON (cached) |
| GET | `/api/dashboard?sector=...&limit=...` | On-demand build, bypasses cache |
| POST | `/api/dashboard/refresh` | Force cache rebuild |
| GET | `/api/dashboard/meta` | Metadata (available sectors) |
| GET | `/api/health` | Health check |
| GET | `/api/health/providers` | Provider diagnostics (Yahoo, FMP status) |
| POST | `/api/scan` | Run anomaly detection |

## Anomaly detection

For each S&P 500 ticker:

1. **Baseline**: Historical window (default 365 days) of daily insider sell volume -- compute mean and standard deviation.
2. **Current window**: Last N days (default 30) -- compute average daily sell volume.
3. **Signal**: Flag when current average exceeds `baseline_mean + Z * baseline_std` (default Z = 2.0).

The Rust binary (`vibes-anomaly anomaly`) handles this computation when available. Otherwise, the identical algorithm runs in Go (`internal/aggregator`).

```bash
# CLI scan
./bin/scan --baseline-days 365 --current-days 30 --std-threshold 2.5 --list-all-signals
```

## Dashboard

The dashboard shows all S&P 500 companies grouped by GICS sector, with:

- **Price**: Current stock price (Yahoo Finance)
- **Change**: Daily percentage change (green/red badge)
- **Quarterly Trend**: 13-week return with inline SVG sparkline
- **News**: Recent headlines from Yahoo search
- **Top Insider Sellers**: Name + shares sold from FMP insider data

Click any row to open the detail drawer with expanded sparkline, full news list, and insider table. Selection is preserved in the URL (`?stock=AAPL`).

## Project layout

```
frontend/               React + TypeScript + Tailwind SPA
  src/components/       AppShell, SidebarNav, DataTable, DetailDrawer, CommandPalette
  src/pages/            Dashboard, Scan, Settings
  src/lib/              API client, formatters
cmd/api/                Go HTTP server (dashboard, health, scan)
cmd/scan/               Go CLI (anomaly scanner)
internal/
  aggregator/           Z-score anomaly detection (Go fallback)
  cache/                File-based JSON cache
  config/               Environment loading (sync.Once)
  dashboard/            Dashboard builder
  fmp/                  FMP API client
  httpclient/           Shared HTTP client with timeouts
  models/               InsiderSellRecord struct
  rustclient/           Go<->Rust subprocess bridge
  sp500/                S&P 500 company data
  trend/                Quarterly trend math (Go fallback)
  yahoo/                Yahoo Finance client (go-yfinance)
rust-core/
  src/main.rs           CLI: anomaly + trend subcommands
  src/anomaly.rs        Z-score detection
  src/trend.rs          Quarterly trend (linear regression)
  src/models.rs         InsiderSellRecord
static/                 Legacy dashboard UI (fallback)
docs/
  SECURITY_CONTROLS.md  SSDF + 800-53 controls matrix, threat model
  SECURE_DEFAULTS.md    All config settings with security impact
CODEOWNERS              Code ownership and review requirements
SECURITY.md             Vulnerability disclosure policy and SLAs
LICENSE                 MIT license
.github/workflows/
  ci.yml                Build + test + SBOM (push/PR)
  security.yml          Security scanners (weekly + push)
```

## Security

Aligned to **NIST SP 800-218 (SSDF v1.1)** for secure development practices and **NIST SP 800-53r5** for operational security controls. The two frameworks complement each other to provide defense-in-depth from code through deployment.

- **[SECURITY.md](SECURITY.md)** — Vulnerability disclosure policy and response SLAs
- **[docs/SECURITY_CONTROLS.md](docs/SECURITY_CONTROLS.md)** — Full SSDF + 800-53 controls matrix, threat model, and release criteria
- **[docs/SECURE_DEFAULTS.md](docs/SECURE_DEFAULTS.md)** — All configuration settings with security impact analysis

### Governance (SSDF PO.1/PO.2)

- `CODEOWNERS` assigns review ownership; security-sensitive paths require explicit approval
- `LICENSE` (MIT) and `SECURITY.md` with disclosure process and triage SLAs
- Release criteria defined in `docs/SECURITY_CONTROLS.md` (PO.4.1)

### CI pipelines (SSDF PO.3, PW.7, PS.3)

**Build workflow** (`.github/workflows/ci.yml`) — every push/PR:

| Step | Purpose |
|------|---------|
| `go vet ./...` | Static analysis (PW.7) |
| `go test -race` | Unit tests with data race detection (PW.8) |
| `cargo test` | Rust tests (PW.8) |
| `npm run build` | Frontend build verification |
| SBOM generation | CycloneDX SBOM via `anchore/sbom-action` (PS.3/SR-4) |

**Security workflow** (`.github/workflows/security.yml`) — weekly + on push:

| Scanner | Scope |
|---------|-------|
| **semgrep** | SAST — Go, Rust, JS/TS (OWASP Top 10 + security-audit rules) |
| **govulncheck** | Go dependency vulns with symbol-level reachability |
| **osv-scanner** | SCA across `go.mod`, `Cargo.lock`, `package-lock.json` |
| **cargo-audit** | Rust crate advisory database |
| **gitleaks** | Secret detection in working tree + git history |

Triage SLAs: Critical 7d, High 14d, Medium 30d, Low next release.

### Release integrity (SSDF PS.2)

- Version + commit hash embedded in binaries via `-ldflags` and exposed at `/api/health`
- `make checksums` generates SHA256 sums for all binary artifacts
- CycloneDX SBOM generated and archived as a CI artifact per build

### Runtime hardening (800-53 controls)

- **HTTP server timeouts**: ReadHeader 10s, Read 30s, Write 120s, Idle 60s
- **Security headers**: CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
- **Request audit logging (AU-12)**: Every request gets a unique `X-Request-Id`; method, path, status, and duration are logged
- **Input validation (SI-10)**: `sector` validated against GICS enum; `limit` bounded `[0, 600]`; all numeric params clamped; bad input returns structured `400` JSON
- **Auth (SA-10)**: `ADMIN_API_KEY` checked via `crypto/subtle.ConstantTimeCompare` — timing-attack resistant
- **Rate limiting**: Scan 1 req/5s per IP, refresh debounce 5 min
- **Path traversal protection**: `safeStaticPath()` + `..` rejection
- **Binary validation**: `VIBES_ANOMALY_BIN` must resolve under project root
- **Concurrency control**: Yahoo request semaphore (cap 8)
- **Config isolation**: All config loaded via `config.Load()` with `sync.Once`
- **Dependency scanning**: 0 known vulnerabilities (govulncheck, cargo-audit, osv-scanner)

### Controls matrix summary

| SSDF Practice | 800-53 Control | Status |
|---|---|---|
| PO.1 Security requirements | SA-8 | Done |
| PO.2 Roles & ownership | SA-3 | Done |
| PO.3 Toolchains | RA-5, SA-11 | Done |
| PS.2 Release integrity | SA-10, SR-4 | Done |
| PS.3 Archive releases | SR-4 | Done |
| PW.5 Secure coding | SI-10, SI-11 | Done |
| PW.7 Code analysis | SA-11 | Done |
| PW.8 Testing | SA-11 | Done |
| PW.9 Secure defaults | SA-8(23) | Done |
| RV.1 Identify vulns | RA-5 | Done |
| AU-12 Audit logging | AU-12/AU-11 | Done |
| AC-17 Remote access auth | AC-17(2) | Done |
| SC-28 Protection at rest | SC-28(1) | Done |
| CM-10 Dependency governance | CM-10(1) | Done |

See [docs/SECURITY_CONTROLS.md](docs/SECURITY_CONTROLS.md) for the full matrix with file-level evidence and threat model.

## Build targets

```bash
make build          # Go + Rust + React frontend (full build)
make go-build       # Go binaries only (version+commit embedded via ldflags)
make rust-build     # Rust binary only
make frontend       # React frontend only (npm install + build)
make frontend-dev   # Vite dev server with hot reload
make test           # go vet + go test -race
make go-run         # Build Go + run API
make go-run-op      # Build Go + run API with 1Password secrets injection
make checksums      # SHA256 sums for bin/ artifacts
make deps           # go mod download + npm install
make clean          # Remove bin/, rust-core/target/, frontend/dist/
```

## Disclaimer

This is for research and education only. It is not investment advice. Insider selling can have many benign explanations (e.g., 10b5-1 plans, diversification). Always do your own due diligence.
