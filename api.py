#!/usr/bin/env python3
"""FastAPI server for insider selling tracker - web UI and API."""
from concurrent.futures import ThreadPoolExecutor
from datetime import date, timedelta
from typing import Optional
import sys
import os

_here = os.path.dirname(os.path.abspath(__file__))
if _here not in sys.path:
    sys.path.insert(0, _here)

from pathlib import Path
from dotenv import load_dotenv
load_dotenv(Path(_here) / ".env")

from fastapi import FastAPI, Query
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse

from src.config import BASELINE_DAYS, CURRENT_WINDOW_DAYS, ANOMALY_STD_THRESHOLD, FMP_FREE_TIER
from src.clients import FMPClient
from src.aggregator import aggregate_insider_sells, compute_anomaly_signals
from src.data.dashboard import build_dashboard
from src.data.cache import read_cache, write_cache, get_cached_at

app = FastAPI(title="Insider Selling Tracker", version="1.0.0")
_executor = ThreadPoolExecutor(max_workers=2)

STATIC_DIR = Path(_here) / "static"
STATIC_DIR.mkdir(exist_ok=True)


@app.get("/")
async def index():
    """Serve dashboard (Modal-style)."""
    dash_path = STATIC_DIR / "dashboard.html"
    if dash_path.exists():
        return FileResponse(dash_path)
    index_path = STATIC_DIR / "index.html"
    if index_path.exists():
        return FileResponse(index_path)
    return {"message": "Frontend not found."}


app.mount("/static", StaticFiles(directory=str(STATIC_DIR)), name="static")


def _do_dashboard(limit: int, as_of_date: date):
    return build_dashboard(limit=limit, as_of=as_of_date)


def _refresh_cache():
    """Build dashboard and write to cache (runs in background)."""
    try:
        data = build_dashboard(limit=0, as_of=date.today())
        if not data.get("error"):
            write_cache(data)
    except Exception:
        pass


@app.get("/api/dashboard")
async def get_dashboard():
    """Return cached dashboard. Data refreshes every 24h. No user action required."""
    cached = read_cache(allow_stale=True)
    if cached:
        # If stale, trigger background refresh for next visit
        fresh = read_cache(allow_stale=False)
        if not fresh:
            import asyncio
            asyncio.get_event_loop().run_in_executor(_executor, _refresh_cache)
        return cached
    return {"error": "Data is being prepared. Check back in a few minutes.", "sectors": []}


@app.on_event("startup")
async def startup_refresh():
    """On startup: if cache empty or stale, build in background."""
    import asyncio
    cached = read_cache()
    if not cached:
        asyncio.get_event_loop().run_in_executor(_executor, _refresh_cache)


@app.get("/api/dashboard/refresh")
async def trigger_refresh():
    """Manually trigger cache refresh (admin). Runs in background."""
    import asyncio
    asyncio.get_event_loop().run_in_executor(_executor, _refresh_cache)
    return {"status": "refresh started"}


@app.get("/api/dashboard/meta")
async def dashboard_meta():
    """Last updated time for display."""
    t = get_cached_at()
    return {"last_updated": t.isoformat() if t else None}


def _do_scan(limit: Optional[int], baseline_days: int, current_days: int, std_threshold: float, as_of_date: date):
    fmp = FMPClient()
    tickers = fmp.get_sp500_tickers()
    if not tickers:
        return {"error": "Could not load S&P 500 constituents", "tickers_count": 0}
    if limit is None:
        limit = 25 if FMP_FREE_TIER else 0
    if limit > 0:
        tickers = tickers[:limit]
    total_days = baseline_days + current_days
    date_from = as_of_date - timedelta(days=total_days)
    date_to = as_of_date
    records = aggregate_insider_sells(tickers, date_from=date_from, date_to=date_to)
    signals_df = compute_anomaly_signals(
        records, baseline_days=baseline_days, current_days=current_days,
        std_threshold=std_threshold, as_of=as_of_date,
    )
    signals_list = []
    if not signals_df.empty:
        signals_df = signals_df.sort_values("z_score", ascending=False)
        for _, row in signals_df.iterrows():
            signals_list.append({
                "ticker": str(row.get("ticker", "")),
                "current_shares_sold": float(row.get("current_shares_sold", 0)),
                "baseline_mean": float(row.get("baseline_mean", 0)),
                "baseline_std": float(row.get("baseline_std", 0)),
                "z_score": float(row.get("z_score", 0)),
                "is_anomaly": bool(row.get("is_anomaly", False)),
            })
    anomalies_list = [s for s in signals_list if s["is_anomaly"]]
    return {
        "tickers_count": len(tickers),
        "records_count": len(records),
        "anomalies_count": len(anomalies_list),
        "date_from": date_from.isoformat(),
        "date_to": date_to.isoformat(),
        "as_of": as_of_date.isoformat(),
        "params": {"baseline_days": baseline_days, "current_days": current_days, "std_threshold": std_threshold},
        "anomalies": anomalies_list,
        "all_signals": signals_list,
    }


@app.post("/api/scan")
async def run_scan(
    limit: Optional[int] = Query(default=None, ge=0, le=600, description="Max tickers (default: 25 if free tier, else all)"),
    baseline_days: int = Query(default=BASELINE_DAYS, ge=30, le=730),
    current_days: int = Query(default=CURRENT_WINDOW_DAYS, ge=7, le=90),
    std_threshold: float = Query(default=ANOMALY_STD_THRESHOLD, ge=1.0, le=5.0),
    as_of: str = Query(default=None, description="YYYY-MM-DD"),
):
    """Run insider selling scan and return anomalies."""
    import asyncio
    as_of_date = date.today() if not as_of else date.fromisoformat(as_of)
    loop = asyncio.get_event_loop()
    return await loop.run_in_executor(
        _executor,
        _do_scan,
        limit,
        baseline_days,
        current_days,
        std_threshold,
        as_of_date,
    )


@app.get("/api/health")
async def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
