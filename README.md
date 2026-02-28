# S&P 500 Insider Selling Tracker

Real-time tracking of insider selling across all S&P 500 companies with anomaly detection. Built with **Go** and **Rust** -- zero Python runtime dependency.

Stock prices and quarterly trends are fetched via [go-yfinance](https://github.com/wnjoon/go-yfinance) (native Go with TLS fingerprint spoofing and Yahoo crumb/cookie auth). Anomaly detection and trend computation use a Rust binary for performance, with Go fallbacks when the binary isn't available.

## Quick start

```bash
# Prerequisites: Go 1.22+, Rust (rustup.rs)
cp .env.example .env        # add your FMP_API_KEY
make build                   # builds Go API + Rust binary -> bin/
./bin/api                    # http://localhost:8000
```

## Architecture

```
Go API (cmd/api)
├── Dashboard builder       → prices, trends, news, insiders
│   ├── go-yfinance         → Yahoo Finance (quotes, historical)
│   ├── FMP client          → insider sells, S&P 500 list
│   └── HTTP fallback       → Yahoo v7/v8 APIs (no auth needed for news)
├── Rust binary (optional)  → anomaly detection + trend computation
│   └── Go fallback         → internal/aggregator + internal/trend
├── Cache layer             → file-based JSON, 24h refresh
└── Static UI               → dashboard.html + dashboard.js
```

| Component | Language | Purpose |
|-----------|----------|---------|
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
| `static/` | HTML/JS | Dashboard UI with SVG sparklines |

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

1. **Install Go** (1.22+): https://go.dev/doc/install

2. **Install Rust** (optional, for faster anomaly/trend): https://rustup.rs

3. **Configure API keys**:

   ```bash
   cp .env.example .env
   ```

   Edit `.env` and set at least `FMP_API_KEY`. If on FMP's free tier (250 calls/day), also set `FMP_FREE_TIER=true` to prefer Yahoo for quotes and trends.

4. **Build and run**:

   ```bash
   make build       # Go binaries + Rust binary -> bin/
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
| GET | `/` | Dashboard UI |
| GET | `/api/dashboard` | Dashboard JSON (cached) |
| GET | `/api/dashboard?sector=...&limit=...` | On-demand build, bypasses cache |
| POST | `/api/dashboard/refresh` | Force cache rebuild |
| GET | `/api/dashboard/meta` | Metadata (available sectors) |
| GET | `/api/health` | Health check |
| GET | `/api/health/providers` | Provider diagnostics (Yahoo, FMP status) |
| GET | `/api/scan` | Run anomaly detection |

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
- **Quarterly Trend**: 13-week return with inline SVG sparkline
- **News**: Recent headlines from Yahoo search
- **Top Insider Sellers**: From FMP insider transaction data

Data is pre-built on server startup and cached for 24 hours. The frontend always sends `limit=50` by default to trigger an on-demand build with fresh data.

## Project layout

```
cmd/api/            Go HTTP server (dashboard, health, scan)
cmd/scan/           Go CLI (anomaly scanner)
internal/
  aggregator/       Z-score anomaly detection (Go fallback)
  cache/            File-based JSON cache
  config/           Environment loading (sync.Once)
  dashboard/        Dashboard builder
  fmp/              FMP API client
  httpclient/       Shared HTTP client
  models/           InsiderSellRecord struct
  rustclient/       Go<->Rust subprocess bridge
  sp500/            S&P 500 company data
  trend/            Quarterly trend math (Go fallback)
  yahoo/            Yahoo Finance client (go-yfinance)
rust-core/
  src/main.rs       CLI: anomaly + trend subcommands
  src/anomaly.rs    Z-score detection
  src/trend.rs      Quarterly trend (linear regression)
  src/models.rs     InsiderSellRecord
static/             Dashboard UI (HTML/JS/CSS)
docs/               Security audit, API call docs
```

## Security

- HTTP security headers (CSP, X-Frame-Options, X-Content-Type-Options)
- Rate limiting on scan endpoint
- Path traversal protection on static file serving
- Cache files written with mode 0600
- 5-minute refresh debounce
- Optional `ADMIN_API_KEY` for protected endpoints
- `VIBES_ANOMALY_BIN` path validation (must be under project root)
- Concurrent Yahoo request limiter (semaphore, cap 8)
- All config loaded via explicit `config.Load()` with `sync.Once`

See [docs/SECURITY_PERFORMANCE_AUDIT.md](docs/SECURITY_PERFORMANCE_AUDIT.md) for the full audit.

## Build targets

```bash
make build        # Go + Rust binaries
make go-build     # Go only
make rust-build   # Rust only
make go-run       # Build + run API
make deps         # go mod download + tidy
make clean        # Remove bin/ and rust-core/target/
```

## Disclaimer

This is for research and education only. It is not investment advice. Insider selling can have many benign explanations (e.g., 10b5-1 plans, diversification). Always do your own due diligence.
