# S&P 500 Insider Selling Tracker

Tracks insider selling for all S&P 500 constituents using multiple data providers and flags companies where selling exceeds **normal** levels using a simple statistical baseline.

## Data sources

Insider sell data is pulled from:

- **Financial Modeling Prep (FMP)** – S&P 500 list + insider trading (latest and search by symbol)
- **SEC-API.io** – Form 3/4/5 insider transactions (SEC EDGAR)
- **Financial Datasets** – Form 4 insider trades
- **EODHD** – Insider transactions (Form 4)

You can enable any subset by setting the corresponding API keys. At least **FMP** is required to load the S&P 500 constituent list.

## Anomaly detection

- **Baseline**: For each ticker, we use a historical window (e.g. 365 days before the “current” window) and compute the **mean** and **standard deviation** of **daily** insider sell volume (shares).
- **Current window**: We take the last N days (e.g. 30) and compute the **average daily** sell volume.
- **Signal**: We flag a ticker when its current-window average daily selling is at least **Z** standard deviations above the baseline mean (default Z = 2.0). So selling “beyond normal” means: recent average daily selling > baseline_mean + Z × baseline_std.

Tunable via `.env` or CLI: `BASELINE_DAYS`, `CURRENT_WINDOW_DAYS`, `ANOMALY_STD_THRESHOLD`, `MIN_BASELINE_POINTS`.

## Setup

1. **Python**: 3.9+

2. **Install dependencies**

   ```bash
   pip install -r requirements.txt
   ```

3. **API keys**  
   Copy `.env.example` to `.env` and set at least `FMP_API_KEY`. Add others to pull from more sources:

   - [Financial Modeling Prep](https://site.financialmodelingprep.com/developer/docs) → `FMP_API_KEY`
   - [SEC-API.io](https://sec-api.io/) → `SEC_API_KEY`
   - [Financial Datasets](https://financialdatasets.ai/) → `FINANCIAL_DATASETS_API_KEY`
   - [EODHD](https://eodhd.com/) → `EODHD_API_KEY`

   Leave a key empty to skip that provider.

## Usage

From the project root:

```bash
python main.py
```

Options:

- `--baseline-days 365` – Days of history for baseline (default from config).
- `--current-days 30` – Current window length.
- `--std-threshold 2.0` – Z-score above which to flag (default 2.0).
- `--as-of YYYY-MM-DD` – As-of date (default today).
- `--list-all-signals` – Print full signals table; otherwise only anomalies.
- `--csv path.csv` – Write (anomaly or full) signals to CSV.

Example:

```bash
python main.py --current-days 30 --std-threshold 2.5 --csv signals.csv
```

## Web frontend

Run the API server with the built-in UI:

```bash
.venv/bin/python api.py
```

Or with uvicorn:

```bash
.venv/bin/uvicorn api:app --reload --host 0.0.0.0 --port 8000
```

Then open **http://localhost:8000** in your browser.

**Dashboard** (default): S&P 500 companies grouped by sector (GICS). Data is **pre-populated** — you see it immediately on load. Refreshes every 24 hours automatically. For each company: stock price, quarterly trend, recent news, and top insider sellers.

**First run:** On startup, the server builds the cache in the background (2–3 min). Until then, you'll see "Dashboard is being built." To pre-warm before deploy: `python scripts/prepopulate_cache.py`

**FMP free tier (250 calls/day):** If price, trend, news, and insiders show as "—", you've likely hit the rate limit. Set `FMP_FREE_TIER=true` in `.env`, delete `data/dashboard_cache.json`, and run `python scripts/prepopulate_cache.py` to rebuild with fewer API calls. You'll get data for ~50 companies instead of 503.

**EODHD fallback:** With `EODHD_API_KEY` set, the dashboard uses EODHD for stock prices when FMP is rate limited. EODHD also supplies insider data (with FMP, SEC-API, Financial Datasets). EODHD free plan: 20 calls/day.

**Anomaly Scan** (`/static/index.html`): Run anomaly detection to flag companies where insider selling exceeds normal.

## Project layout

- `main.py` – CLI entrypoint (load S&P 500, aggregate sells, run anomaly detection, print/export).
- `api.py` – FastAPI server and web UI (POST /api/scan, static frontend).
- `static/` – Frontend (dashboard.html, index.html, app.js, dashboard.js).
- `run.py` – Thin wrapper that calls `main.main()`.
- `src/config.py` – Env and tuning constants.
- `src/models.py` – `InsiderSellRecord` and shared types.
- `src/clients/` – API clients: `fmp`, `sec_api`, `eodhd`, `financial_datasets`.
- `src/aggregator.py` – Aggregation (dedupe across APIs) and anomaly logic (baseline vs current window).

## Disclaimer

This is for research and education only. It is not investment advice. Insider selling can have many benign explanations (e.g. 10b5-1 plans, diversification). Always do your own due diligence.
