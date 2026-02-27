# Why 1000+ FMP API Calls?

**FMP free tier: 250 calls/day.** Here's where calls go:

## Call breakdown

### 1. Insider aggregation (biggest cost)
`aggregate_insider_sells` makes **1 FMP call per ticker** (no batch API for insider search).

| Action | Tickers | FMP calls |
|--------|---------|-----------|
| Anomaly Scan (limit=0 = all 503) | 503 | **504** |
| Dashboard build (free tier) | 15 | 16 |
| Dashboard build (paid) | 80 | 81 |

### 2. Dashboard build
| Source | Free tier | Paid |
|--------|-----------|------|
| Quote batch (50 symbols each) | 1 batch | 11 batches |
| Historical (per ticker) | 10 | 50 |
| News (per ticker) | 10 | 50 |
| Insiders | 16 | 81 |
| **Total** | ~37 | ~192 |

### 3. Anomaly Scan
- **504 FMP calls** when run with limit=0 (all S&P 500)
- One "Run Scan" with default = full 503 tickers

## How you hit 1001

- **Run Scan** with all tickers: **504 calls**
- **Dashboard startup refresh** (no cache): **~200 calls**
- **Prepopulate** without FMP_FREE_TIER: **~200 calls**
- **Restart server** (triggers refresh): another **~200**
- Total: **~1100 calls**

## Fix: enforce free-tier caps

- `aggregate_insider_sells` caps FMP to 25 tickers when `FMP_FREE_TIER=true`
- Anomaly Scan default limit = 25 when free tier
- Dashboard already uses free tier limits when `FMP_FREE_TIER=true`
