# S&P 500 Insider Selling Tracker

Real-time tracking of insider selling across all S&P 500 companies with anomaly detection
and local LLM-powered anomaly explanations.
Built with **Go**, **Rust**, a **React** frontend, and a **Python** AI sidecar.

> **This is a portfolio project.** It's not deployed. The code is the deliverable.
> Run `make demo` for a self-contained local proof, or read on for the thinking behind it.

---

## Why This Problem

SEC Form 4 filings are one of the few non-synthetic signals of privileged access behavior
in public markets. Detecting anomalous insider selling — unusual volume, unusual timing,
unusual actors relative to a baseline — is structurally identical to detecting anomalous
privileged access in an enterprise environment. The data domain is finance; the mental
model is security: baseline a population, flag deviations, explain them without speculation.

---

## Design Principles

- **Fast feedback.** `make demo` builds, starts, samples, and writes output — one command,
  under a minute.
- **Security from the start.** Input validation, auth, logging, and dependency scanning
  ship with the first commit, not after. Retrofitting controls is how real vulnerabilities
  happen; the CI pipeline enforces SAST, SCA, secret scanning, and SBOM generation on
  every push.
- **Graceful degradation.** Rust wasm module missing? Subprocess fallback. Native binary
  missing? Go fallback. FMP rate-limited? Yahoo takes over. Cache stale? Rebuild on demand.

### Reading order

1. **This README** — architecture, quick start
2. **[docs/design.md](docs/design.md)** — tradeoffs, constraints, non-goals
3. **[docs/security/](docs/security/)** — threat model, controls matrix
4. **[docs/ops-lite.md](docs/ops-lite.md)** — deployment reasoning
5. **[docs/decisions/](docs/decisions/)** — ADRs: why Go, why Rust

---

## Quick Start

```bash
# Prerequisites: Go 1.23+, Node.js 18+, Rust (optional), Python 3.9+ & Ollama (optional, for AI)
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
│  ├─ DetailDrawer (price, sparkline, news, insiders,  │
│  │                AI anomaly explanation)             │
│  ├─ CommandPalette (⌘K fuzzy search)                 │
│  └─ Anomaly Scan page + Settings page                │
├──────────────────────────────────────────────────────┤
│  Go API  (cmd/api) — port 8000                       │
│  ├─ Dashboard builder → prices, trends, news         │
│  │  ├─ go-yfinance    → Yahoo Finance (native Go)    │
│  │  ├─ FMP client     → S&P 500 list, insider latest │
│  │  ├─ SEC-API client → Form 4 insider sells (EDGAR) │
│  │  ├─ EODHD client   → insider transactions (paid)  │
│  │  └─ HTTP fallback  → Yahoo v7/v8 (news)           │
│  ├─ AI proxy          → assembles payload from       │
│  │                       dashboard, forwards to       │
│  │                       Python sidecar               │
│  ├─ Rust engine       → anomaly + trend compute      │
│  │  ├─ Preferred: Wasm in-process (wazero, zero IPC) │
│  │  └─ Fallback: subprocess or pure-Go               │
│  ├─ Cache layer       → pluggable: FileStore, SQLite │
│  ├─ Observability     → OpenTelemetry traces (stdout) │
│  └─ Security          → timeouts, rate limits, CSP   │
├──────────────────────────────────────────────────────┤
│  Python AI sidecar  (ai/) — port 8001                │
│  ├─ FastAPI + Pydantic → POST /explain-anomaly       │
│  ├─ LangChain prompt   → structured JSON output      │
│  └─ Ollama (qwen3.5)   → local LLM, no cloud calls  │
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
| `ai/` | Python | FastAPI sidecar: LLM anomaly explanation via Ollama |
| `rust-core` | Rust | `vibes-anomaly`: anomaly + trend (native or wasm32-wasip1) |

For *why* this shape, see [docs/design.md](docs/design.md).

---

## Frontend

The UI is a Linear-inspired React SPA:

- **Sidebar** — Navigation + collapsible sector filter groups
- **DataTable** — Dense rows with sector headers, prices, sparklines, news, insider sellers
- **DetailDrawer** — Slide-in panel on row click (deep-linkable via `?stock=AAPL`), with "Explain Anomaly" button for LLM analysis
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

**No paid keys are required for insider data.** SEC EDGAR queries fetch Form 4 filings directly
from the SEC, covering 200+ companies on a fresh build. Coverage grows across builds via the
unified disk cache. Paid keys increase freshness but are optional. Yahoo Finance handles
prices, trends, and news without a key.

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

6. **AI anomaly explanations** (optional):

   ```bash
   ollama pull qwen3.5                  # one-time model download (~6.6 GB)
   python3 -m venv ai/.venv             # create virtualenv
   ai/.venv/bin/pip install -r ai/requirements.txt
   OLLAMA_MODEL=qwen3.5 ai/.venv/bin/uvicorn ai.app:app --port 8001
   ```

7. **LangSmith observability** (optional — traces every LLM call):

   ```bash
   export LANGSMITH_API_KEY="lsv2_pt_..."   # from https://smith.langchain.com/settings
   export LANGSMITH_TRACING=true
   export LANGSMITH_ENDPOINT=https://api.smith.langchain.com
   export LANGSMITH_PROJECT=500-sketchpad
   export OLLAMA_MODEL=qwen3.5
   ai/.venv/bin/uvicorn ai.app:app --port 8001
   ```

   Or with 1Password (no plaintext key on disk):

   ```bash
   export LANGSMITH_API_KEY="$(op read 'op://VAULT_ID/ITEM_ID/credential')"
   # ... remaining exports same as above
   ```

   Verify with: `ai/.venv/bin/python3 ai/test_trace.py`

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
| `AI_SERVICE_URL` | `http://localhost:8001` | Python AI sidecar URL |
| `OLLAMA_MODEL` | `qwen3.5` | Ollama model for anomaly explanations (sidecar-side) |
| `LANGSMITH_API_KEY` | *(optional)* | LangSmith API key for LLM tracing (`lsv2_pt_...`) |
| `LANGSMITH_TRACING` | `false` | Set `true` to enable LangSmith trace export |
| `LANGSMITH_PROJECT` | `500-sketchpad` | LangSmith project name for trace grouping |
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
| GET | `/api/ai/explain-anomaly?ticker=` | LLM anomaly explanation (proxied to Python sidecar) |
| POST | `/api/scan` | Run anomaly detection (admin key required if set) |

---

## Anomaly Detection

For each S&P 500 ticker:

1. **Baseline** — Historical window (default 365 days) of daily insider sell volume; compute mean and standard deviation.
2. **Current window** — Last *N* days (default 30); compute average daily sell volume.
3. **Signal** — Flag when current average exceeds `baseline_mean + Z × baseline_std` (default Z = 2.0).

Z-score over a few hundred tickers doesn't need Rust. The Rust binary exists to validate
the wazero WASM execution path and the subprocess fallback pattern under a real build
constraint — the value is the boundary, not the performance. The identical algorithm
runs in Go (`internal/aggregator`) as the default fallback.

```bash
# CLI scan
./bin/scan --baseline-days 365 --current-days 30 --std-threshold 2.5 --list-all-signals
```

---

## AI Anomaly Explanation

Clicking "Explain Anomaly" in the detail drawer produces a structured, non-speculative
explanation of insider activity for that company:

```
React → GET /api/ai/explain-anomaly?ticker=AAPL (Go :8000)
         → Go assembles payload from in-memory dashboard store
         → POST /explain-anomaly (Python sidecar :8001)
           → qwen3.5 via Ollama (local, no cloud API calls)
         ← { summary, drivers[], caveats[] }
```

**Why this shape:**

- **Go assembles the payload** from the in-memory dashboard store — company name, sector,
  quarterly trend, insider events, data sources. Zero live API calls; latency is pure LLM inference.
- **Python owns the LLM interaction.** FastAPI + LangChain + Pydantic handle prompt
  templating, model configuration, and response validation.
- **React only talks to Go.** The frontend has no knowledge of the Python service.
- **Everything runs locally.** Ollama serves qwen3.5 on-device. No data leaves the machine.
- **Observable via LangSmith.** Set `LANGSMITH_API_KEY` + `LANGSMITH_TRACING=true` to trace
  every LLM call — full prompt, raw output, latency, token counts.

**Guardrails:** The system prompt forbids speculation about illegality, insider intent,
future stock performance, and investment advice. Responses cite only the structured data
the Go backend provides.

**Running the AI service:**

```bash
# Terminal 1 — pull the model once, then start the sidecar
ollama pull qwen3.5
OLLAMA_MODEL=qwen3.5 ai/.venv/bin/uvicorn ai.app:app --port 8001

# Terminal 2 — Go API (sidecar URL defaults to http://localhost:8001)
./bin/api
```

**With LangSmith tracing** (export env vars before starting the sidecar):

```bash
export LANGSMITH_API_KEY="lsv2_pt_..."
export LANGSMITH_TRACING=true
export LANGSMITH_ENDPOINT=https://api.smith.langchain.com
export LANGSMITH_PROJECT=500-sketchpad
OLLAMA_MODEL=qwen3.5 ai/.venv/bin/uvicorn ai.app:app --port 8001
```

The AI sidecar is optional. If it's not running, the "Explain Anomaly" button returns a
502 error — the rest of the dashboard works normally.

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

SAST, dependency scanning, secret scanning, and SBOM generation run in CI on every push.
See [docs/decisions/0003-threat-model-scope.md](docs/decisions/0003-threat-model-scope.md) for the reasoning.

### Controls

| Category | What |
|---|---|
| **Input validation** | GICS enum check, numeric bounds, path traversal guard |
| **Auth** | `ADMIN_API_KEY` via `crypto/subtle.ConstantTimeCompare` |
| **Structured errors** | JSON error responses, no stack traces |
| **Security headers** | CSP, X-Frame-Options, X-Content-Type-Options via middleware |
| **Audit logging** | Unique `X-Request-Id`, method/path/status/duration per request |
| **Rate limiting** | Per-IP scan throttle, refresh debounce |
| **Secrets management** | 1Password CLI integration, `.env` gitignored |

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
ai/
  app.py                 FastAPI service: POST /explain-anomaly
  chains.py              LangChain prompt + Ollama chain (qwen3.5)
  schemas.py             Pydantic models: AnomalyInput, AnomalyExplanation
  test_trace.py          LangSmith tracing diagnostic
  requirements.txt       Pinned Python dependencies
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
cmd/api/                 Go HTTP server (dashboard, health, scan, AI proxy)
cmd/scan/                Go CLI (anomaly scanner)
internal/
  aiclient/              Go client for the Python AI sidecar
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

## Known Limitations

| Limitation | Impact | What would fix it |
|---|---|---|
| **10b5-1 plan trades** | A meaningful fraction of flagged anomalies are plan-governed sells — anomalous by volume, not by intent. The system cannot distinguish pre-scheduled liquidation from discretionary selling. | Cross-reference SEC Form 144 filings or a plan disclosure feed. Neither is publicly available in structured form. |
| **EDGAR coverage gaps** | Free EDGAR backfill covers ~200 of 503 S&P 500 companies per build. Companies with no recent Form 4 filings show no insider data. | Paid APIs (FMP, SEC-API, EODHD) fill the gaps. Coverage grows across builds via the unified disk cache. |
| **LLM latency** | Local Ollama inference on `qwen3.5` takes 1–3 minutes per explanation on consumer hardware. | A smaller model, quantization, or cloud inference would reduce latency at the cost of privacy or accuracy. |

---

## Disclaimer

For research and education only — not investment advice. Insider selling often has
benign explanations (10b5-1 plans, diversification). Do your own due diligence.
