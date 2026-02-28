# S&P 500 Insider Selling Tracker

Real-time tracking of insider selling across all S&P 500 companies with anomaly detection. Built with **Go**, **Rust**, and a **React** frontend -- zero Python runtime dependency.

Stock prices and quarterly trends are fetched via [go-yfinance](https://github.com/wnjoon/go-yfinance) (native Go with TLS fingerprint spoofing and Yahoo crumb/cookie auth). Anomaly detection and trend computation use a Rust binary for performance, with Go fallbacks when the binary isn't available.

## Quick start

```bash
# Prerequisites: Go 1.25+, Node.js 18+, Rust (optional, rustup.rs)
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

1. **Install Go** (1.25+): https://go.dev/doc/install

2. **Install Node.js** (18+): https://nodejs.org

3. **Install Rust** (optional, for faster anomaly/trend): https://rustup.rs

4. **Configure API keys**:

   ```bash
   cp .env.example .env
   ```

   Edit `.env` and set at least `FMP_API_KEY`. If on FMP's free tier (250 calls/day), also set `FMP_FREE_TIER=true` to prefer Yahoo for quotes and trends.

5. **Build and run**:

   ```bash
   make build       # Go + Rust + React frontend -> bin/ + frontend/dist/
   ./bin/api        # starts on http://localhost:8000
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
docs/                   Security audit, API call docs
```

## Security

- **HTTP server hardening**: ReadHeaderTimeout (10s), ReadTimeout (30s), WriteTimeout (120s), IdleTimeout (60s)
- **Security headers**: CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
- **Rate limiting**: Scan endpoint (1 req/5s per IP), refresh debounce (5 min)
- **Path traversal protection**: Static file serving validates paths under allowed directories
- **Cache security**: Files written with mode 0600
- **Auth**: Optional `ADMIN_API_KEY` for protected endpoints (scan, refresh)
- **Binary validation**: `VIBES_ANOMALY_BIN` must be under project root
- **Concurrency control**: Yahoo request semaphore (cap 8)
- **Config isolation**: All config loaded via explicit `config.Load()` with `sync.Once`
- **Dependency scanning**: govulncheck, cargo-audit, osv-scanner -- 0 known vulnerabilities

See [docs/SECURITY_PERFORMANCE_AUDIT.md](docs/SECURITY_PERFORMANCE_AUDIT.md) for the full audit.

## Build targets

```bash
make build          # Go + Rust + React frontend (full build)
make go-build       # Go binaries only
make rust-build     # Rust binary only
make frontend       # React frontend only (npm install + build)
make frontend-dev   # Vite dev server with hot reload
make go-run         # Build Go + run API
make deps           # go mod download + npm install
make clean          # Remove bin/, rust-core/target/, frontend/dist/
```

## Disclaimer

This is for research and education only. It is not investment advice. Insider selling can have many benign explanations (e.g., 10b5-1 plans, diversification). Always do your own due diligence.
