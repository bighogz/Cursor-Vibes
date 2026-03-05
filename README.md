# S&P 500 Insider Selling Tracker

Real-time tracking of insider selling across all S&P 500 companies with anomaly detection.
Built with **Go**, **Rust**, and a **React** frontend — zero Python runtime dependency.

> **This is a portfolio project.** It's not deployed. The code is the deliverable.
> Run `make demo` for a self-contained local proof, or read on for the thinking behind it.

---

## How I Think About This

This repo is designed to communicate engineering judgment, not just working code.
A few principles that shaped every decision:

- **Tight feedback loops.** `make demo` builds everything, starts the server, runs a
  sample request, and writes output artifacts — in one command. If it breaks, you know
  in under a minute.
- **Treat security as requirements, not polish.** Input validation, auth, logging, and
  dependency scanning are implemented from the start, not bolted on. They're tested the
  same way features are tested.
- **Evidence over claims.** Every security control has a file path and a test.
  Every architectural choice has a written rationale. "I thought about it" is backed
  by artifacts, not hand-waving.
- **Graceful degradation everywhere.** Rust wasm module missing? Subprocess fallback.
  Native binary missing? Go fallback runs. FMP API rate-limited? Yahoo takes over.
  Cache stale? Rebuild on demand. No single failure should break the whole system.

### Guided tour (2–3 minutes)

If you're reviewing this repo, read in this order:

1. **This README** — mental model, architecture, quick start
2. **[docs/design.md](docs/design.md)** — tradeoffs, constraints, non-goals, what I'd change
3. **[docs/security/](docs/security/)** — threat model, controls matrix, secure defaults
4. **[docs/ops-lite.md](docs/ops-lite.md)** — how I'd deploy this (explicitly: not deployed)
5. **[docs/decisions/](docs/decisions/)** — ADRs: why Go, why Rust, why these frameworks

---

## Quick Start

```bash
# Prerequisites: Go 1.23+, Node.js 18+, Rust (optional — rustup.rs)
cp .env.example .env        # add your FMP_API_KEY
make demo                    # builds, starts, samples, writes ./out/
```

Or step by step:

```bash
make build                   # Go API + Rust binary + React frontend
./bin/api                    # http://localhost:8000
```

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  React SPA  (frontend/)                              │
│  ├─ AppShell  (sidebar + topbar + content)           │
│  ├─ DataTable (keyboard nav, sector groups)          │
│  ├─ DetailDrawer (price, sparkline, news, insiders)  │
│  ├─ CommandPalette (⌘K fuzzy search)                 │
│  └─ Anomaly Scan page + Settings page                │
├──────────────────────────────────────────────────────┤
│  Go API  (cmd/api) — port 8000                       │
│  ├─ Dashboard builder → prices, trends, news         │
│  │  ├─ go-yfinance    → Yahoo Finance (native Go)    │
│  │  ├─ FMP client     → S&P 500 list, insider latest  │
│  │  ├─ SEC-API client → Form 4 insider sells (EDGAR) │
│  │  ├─ EODHD client   → insider transactions (paid)  │
│  │  └─ HTTP fallback  → Yahoo v7/v8 (news)           │
│  ├─ Rust engine       → anomaly + trend compute      │
│  │  ├─ Preferred: Wasm in-process (wazero, zero IPC) │
│  │  └─ Fallback: subprocess or pure-Go               │
│  ├─ Cache layer       → pluggable: FileStore, SQLite  │
│  ├─ Observability     → OpenTelemetry traces (stdout) │
│  └─ Security          → timeouts, rate limits, CSP   │
├──────────────────────────────────────────────────────┤
│  Rust CLI  (rust-core/)                              │
│  └─ vibes-anomaly: anomaly + trend subcommands       │
└──────────────────────────────────────────────────────┘
```

| Component | Language | Purpose |
|---|---|---|
| `frontend/` | React + TypeScript + Tailwind | Linear-inspired SPA with keyboard-first UX |
| `cmd/api` | Go | HTTP server, routes, middleware, security headers |
| `cmd/scan` | Go | CLI anomaly scanner |
| `internal/yahoo` | Go | Yahoo Finance client (go-yfinance + HTTP fallback) |
| `internal/fmp` | Go | Financial Modeling Prep API client |
| `internal/dashboard` | Go | Dashboard data assembly, sparkline sampling |
| `internal/trend` | Go | Quarterly trend (linear regression, fallback) |
| `internal/aggregator` | Go | Z-score anomaly detection with blackout-period awareness |
| `internal/rustclient` | Go | Wasm (wazero) or subprocess bridge to Rust |
| `internal/cache` | Go | Pluggable: `DashboardStore` interface → FileStore or SQLiteStore |
| `internal/otel` | Go | OpenTelemetry tracing (stdout exporter, swap for OTLP) |
| `internal/config` | Go | Environment loading via `sync.Once` |
| `rust-core` | Rust | `vibes-anomaly`: anomaly + trend (native or wasm32-wasip1) |

For *why* this shape, see [docs/design.md](docs/design.md).

---

## Frontend

The UI is a Linear-inspired React SPA:

- **Sidebar** — Navigation + collapsible sector filter groups
- **DataTable** — Dense rows with sector headers, prices, sparklines, news, insider sellers
- **DetailDrawer** — Slide-in panel on row click (deep-linkable via `?stock=AAPL`)
- **CommandPalette** — `⌘K` / `Ctrl+K` fuzzy search across stocks and actions
- **Keyboard navigation** — `j`/`k` move focus, `Enter` opens drawer, `Esc` closes
- **Pages** — Dashboard, Anomaly Scan (with configurable params), Settings

Development (with hot reload):

```bash
# Terminal 1 — Go API
./bin/api

# Terminal 2 — Vite dev server (proxies /api to :8000)
cd frontend && npm run dev    # http://localhost:5173
```

---

## Data Sources

| Provider | Data | Key Required |
|---|---|---|
| **Yahoo Finance** | Stock prices, historical closes, news | No (handled by go-yfinance) |
| **FMP** | S&P 500 list, insider sells (latest feed), quotes (fallback) | Yes — `FMP_API_KEY` |
| **SEC-API.io** | Form 4 insider sells parsed from SEC EDGAR XML (400 XMLs/build) | Recommended — `SEC_API_KEY` |
| **SEC EDGAR** | Direct Form 4 backfill for uncovered companies (free) | No — public API |
| **Financial Datasets** | Form 4 insider trades | Optional — `FINANCIAL_DATASETS_API_KEY` |
| **EODHD** | Insider transactions (requires All-In-One plan) | Optional — `EODHD_API_KEY` |

At minimum, `FMP_API_KEY` is required for the S&P 500 constituent list.
`SEC_API_KEY` is recommended for high insider sell coverage — it queries up to 400 Form 4 XMLs per build, prioritizing companies not yet covered.
SEC EDGAR direct queries (free, no key) backfill companies that SEC-API.io missed. Coverage grows across builds via the unified cache.
Yahoo Finance handles prices, trends, and news with no API key.

---

## Setup

1. **Install Go** (1.23+) — <https://go.dev/doc/install>

2. **Install Node.js** (18+) — <https://nodejs.org>

3. **Install Rust** (optional, for faster anomaly/trend) — <https://rustup.rs>

4. **Configure API keys** — choose one method:

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

   > If on FMP's free tier (250 calls/day), set `FMP_FREE_TIER=true` to prefer Yahoo for quotes and trends.

5. **Build and run**:

   ```bash
   make demo        # full build + sample request + output artifacts
   # Or manually:
   make build       # Go + Rust + React frontend → bin/ + frontend/dist/
   ./bin/api        # http://localhost:8000
   ```

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `FMP_API_KEY` | *(required)* | Financial Modeling Prep API key |
| `FMP_FREE_TIER` | `false` | When `true`, prefers Yahoo for quotes/trends (stays under 250 calls/day) |
| `ADMIN_API_KEY` | *(optional)* | Protects `/api/scan` and `/api/dashboard/refresh` |
| `VIBES_ANOMALY_BIN` | auto-discovered | Absolute path to `vibes-anomaly` binary |
| `SEC_API_KEY` | *(optional)* | SEC-API.io key for Form 3/4/5 data |
| `FINANCIAL_DATASETS_API_KEY` | *(optional)* | Financial Datasets API key |
| `EODHD_API_KEY` | *(optional)* | EODHD API key |
| `PORT` | `8000` | HTTP listen port |

See [docs/security/secure-defaults.md](docs/security/secure-defaults.md) for the full security-impact analysis of every setting.

---

## API Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/` | React SPA (falls back to legacy dashboard if not built) |
| GET | `/api/dashboard` | Dashboard JSON (cached, 24 h TTL) |
| GET | `/api/dashboard?sector=...&limit=...` | On-demand build, bypasses cache |
| POST | `/api/dashboard/refresh` | Force cache rebuild (admin key required if set) |
| GET | `/api/dashboard/meta` | Cache metadata (last updated timestamp) |
| GET | `/api/health` | Health check — returns `version` and `commit` |
| GET | `/api/health/providers` | Provider diagnostics (Yahoo, FMP status) |
| POST | `/api/scan` | Run anomaly detection (admin key required if set) |

---

## Anomaly Detection

For each S&P 500 ticker:

1. **Baseline** — Historical window (default 365 days) of daily insider sell volume; compute mean and standard deviation.
2. **Current window** — Last *N* days (default 30); compute average daily sell volume.
3. **Signal** — Flag when current average exceeds `baseline_mean + Z × baseline_std` (default Z = 2.0).

The Rust binary (`vibes-anomaly anomaly`) handles this computation when available.
Otherwise, the identical algorithm runs in Go (`internal/aggregator`).

```bash
# CLI scan
./bin/scan --baseline-days 365 --current-days 30 --std-threshold 2.5 --list-all-signals
```

---

## Dashboard

The dashboard shows all S&P 500 companies grouped by GICS sector:

- **Price** — Current stock price (Yahoo Finance)
- **Change** — Daily percentage change (green/red badge)
- **Quarterly Trend** — 13-week return with inline SVG sparkline
- **News** — Recent headlines from Yahoo search
- **Top Insider Sellers** — Name + shares sold from SEC Form 4 filings (primary), FMP, and EODHD

Click any row to open the detail drawer with expanded sparkline, full news list,
and insider table. Selection is preserved in the URL (`?stock=AAPL`).

---

## Security

These checks run in CI to demonstrate my default hygiene — SAST, dependency vulnerability
scanning, secret scanning, SBOM generation. They're habits I apply to every project,
not compliance theater for a portfolio repo.

For the full reasoning, see [docs/decisions/0003-threat-model-scope.md](docs/decisions/0003-threat-model-scope.md).

### What's implemented (and why)

| Category | What | Why it matters even here |
|---|---|---|
| **Input validation** | GICS enum check, numeric bounds, path traversal guard | Prevents bugs and demonstrates defense-in-depth thinking |
| **Auth** | `ADMIN_API_KEY` via `crypto/subtle.ConstantTimeCompare` | Shows I know timing attacks exist and how to prevent them |
| **Structured errors** | JSON error responses, no stack traces | Production habit — never leak internals |
| **Security headers** | CSP, X-Frame-Options, X-Content-Type-Options | Applied via middleware, costs nothing |
| **Audit logging** | Unique `X-Request-Id`, method/path/status/duration per request | Observable by default |
| **Rate limiting** | Per-IP scan throttle, refresh debounce | Prevents accidental self-DDoS during development |
| **Secrets management** | 1Password CLI integration, `.env` gitignored | API keys have real financial cost if leaked |

### CI pipelines

**Build workflow** (`.github/workflows/ci.yml`) — every push / PR:

| Step | Purpose |
|---|---|
| `go vet` + `go test -race` | Static analysis + tests with data race detection |
| `cargo test` | Rust tests |
| `npm run build` | Frontend build verification |
| SBOM generation | CycloneDX SBOM via `anchore/sbom-action` |

**Security workflow** (`.github/workflows/security.yml`) — weekly + on push:

| Scanner | What it checks |
|---|---|
| **semgrep** | SAST — OWASP Top 10 + security-audit rules across Go, Rust, JS/TS |
| **govulncheck** | Go dependency vulns with symbol-level reachability analysis |
| **osv-scanner** | SCA across `go.mod`, `Cargo.lock`, `package-lock.json` |
| **cargo-audit** | Rust crate advisory database |
| **gitleaks** | Secret detection in working tree + git history |

**Release workflow** (`.github/workflows/release.yml`) — on tag push:
builds cross-platform binaries, generates SHA256 checksums and SBOM, creates a GitHub Release.

### Deeper reading

- **[docs/security/threat-model.md](docs/security/threat-model.md)** — SSDF + 800-53 controls matrix, trust boundaries, top threats
- **[docs/security/secure-defaults.md](docs/security/secure-defaults.md)** — Every config setting with security impact analysis
- **[SECURITY.md](SECURITY.md)** — Vulnerability disclosure policy and response SLAs

---

## Decision Records

Architectural decisions are documented as ADRs in [docs/decisions/](docs/decisions/):

| ADR | Decision |
|---|---|
| [0001](docs/decisions/0001-why-go-api.md) | Why Go for the API (not Python, not Node) |
| [0002](docs/decisions/0002-why-rust-binary.md) | Why a Rust subprocess for anomaly detection |
| [0003](docs/decisions/0003-threat-model-scope.md) | Threat model scope and security framework choices |

---

## Project Layout

```
docs/
  design.md              Tradeoffs, constraints, non-goals
  ops-lite.md            How I'd deploy this (not deployed)
  decisions/             Architecture Decision Records (ADRs)
  security/
    threat-model.md      SSDF + 800-53 controls matrix, threat model
    secure-defaults.md   All config settings with security impact
frontend/                React + TypeScript + Tailwind SPA
  src/components/        AppShell, SidebarNav, DataTable, DetailDrawer, CommandPalette
  src/pages/             Dashboard, Scan, Settings
  src/lib/               API client, formatters
cmd/api/                 Go HTTP server (dashboard, health, scan)
cmd/scan/                Go CLI (anomaly scanner)
internal/
  aggregator/            Z-score anomaly detection (Go fallback)
  cache/                 File-based JSON cache
  config/                Environment loading (sync.Once)
  dashboard/             Dashboard builder
  edgar/                 SEC EDGAR direct Form 4 backfill (free, no key)
  eodhd/                 EODHD insider transactions client
  fmp/                   FMP API client
  httpclient/            Shared HTTP client with timeouts
  models/                InsiderSellRecord struct
  secapi/                SEC-API.io Form 4 query + EDGAR XML parser
  rustclient/            Go ↔ Rust subprocess bridge
  sp500/                 S&P 500 company data
  trend/                 Quarterly trend math (Go fallback)
  yahoo/                 Yahoo Finance client (go-yfinance)
rust-core/
  src/main.rs            CLI: anomaly + trend subcommands
  src/anomaly.rs         Z-score detection
  src/trend.rs           Quarterly trend (linear regression)
  src/models.rs          InsiderSellRecord
scripts/
  demo.sh               Local demo runner (make demo)
static/                  Legacy dashboard UI (fallback)
data/                    Runtime cache files (gitignored)
out/                     Demo output artifacts (gitignored)
CODEOWNERS               Code ownership and review requirements
SECURITY.md              Vulnerability disclosure policy and SLAs
LICENSE                  MIT license
.github/workflows/
  ci.yml                 Build + test + SBOM (push/PR)
  security.yml           Security scanners (weekly + push)
  release.yml            Tag → build → GitHub Release with artifacts
```

---

## Build Targets

```bash
make build          # Go + Rust + React frontend (full build)
make demo           # Build + start + sample request + output to ./out/
make go-build       # Go binaries only (version + commit embedded via ldflags)
make rust-build     # Rust native binary
make rust-wasm      # Rust → wasm32-wasip1 (in-process execution via wazero)
make frontend       # React frontend only (npm install + build)
make frontend-dev   # Vite dev server with hot reload
make test           # go vet + go test -race
make fuzz           # Fuzz test SEC XML/JSON parsers (30s + 10s + 10s)
make test-e2e       # Playwright end-to-end tests for React SPA
make go-run         # Build Go + run API
make go-run-op      # Build Go + run API with 1Password secrets injection
make checksums      # SHA256 sums for bin/ artifacts
make deps           # go mod download + npm install
make clean          # Remove bin/, rust-core/target/, frontend/dist/, out/
```

---

## Disclaimer

This is for **research and education only**. It is not investment advice.
Insider selling can have many benign explanations (e.g., 10b5-1 plans, diversification).
Always do your own due diligence.
