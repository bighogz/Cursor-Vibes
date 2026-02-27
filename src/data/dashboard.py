"""Build full dashboard data: S&P 500 by sector with price, trend, news, top insiders."""
from datetime import date, timedelta
from collections import defaultdict
from typing import List, Dict, Optional
import time

from ..config import FMP_API_KEY, FMP_FREE_TIER, EODHD_API_KEY
from ..clients import FMPClient, EODHDClient
from ..aggregator import aggregate_insider_sells
from .sp500_sectors import load_sp500_with_sectors


def _quarter_dates(as_of: date):
    """Return (start, end) of ~3 months ago for quarterly trend."""
    end = as_of
    start = as_of - timedelta(days=92)
    return start.isoformat(), end.isoformat()


def _quarter_change(hist: List[dict]) -> Optional[float]:
    """Percent change from oldest to newest in historical list."""
    if not hist or len(hist) < 2:
        return None
    hist = sorted(hist, key=lambda x: x.get("date", ""))
    first = float(hist[0].get("close", 0) or 0)
    last = float(hist[-1].get("close", 0) or 0)
    if first <= 0:
        return None
    return round((last - first) / first * 100, 1)


def _top_insiders_by_ticker(records) -> Dict[str, List[dict]]:
    """Group by ticker, sort by shares_sold desc, take top 3 per ticker."""
    by_ticker = defaultdict(list)
    for r in records:
        by_ticker[r.ticker.upper()].append({
            "name": r.insider_name or "Unknown",
            "role": r.role,
            "shares": r.shares_sold,
            "value": r.value_usd,
        })
    out = {}
    for t, lst in by_ticker.items():
        sorted_lst = sorted(lst, key=lambda x: x["shares"], reverse=True)[:5]
        out[t] = sorted_lst
    return out


def build_dashboard(
    limit: int = 0,
    as_of: Optional[date] = None,
) -> Dict:
    """
    Load S&P 500, segment by sector, enrich with quote, quarter trend, news, top insiders.
    limit=0 means all 503; use 50-100 for faster runs.
    """
    as_of = as_of or date.today()
    companies = load_sp500_with_sectors()
    if not companies:
        return {"error": "Could not load S&P 500", "sectors": []}

    if limit > 0:
        companies = companies[:limit]

    tickers = [c["symbol"] for c in companies]
    total_days = 365 + 30
    date_from = as_of - timedelta(days=total_days)
    date_to = as_of

    fmp = FMPClient()
    quote_by_sym = {}
    news_by_sym = {}
    hist_by_sym = {}
    insider_records = []

    # Free tier: ~25 API calls total. Paid: full data.
    if FMP_FREE_TIER:
        quote_batch_limit = 1
        sample = 10
        insider_sample = 15
    else:
        quote_batch_limit = 20
        sample = min(50, len(companies))
        insider_sample = min(80, len(tickers))

    # Batch quotes: FMP first, EODHD as fallback when FMP rate limited
    batch_size = 50
    for i in range(0, min(len(tickers), quote_batch_limit * batch_size), batch_size):
        batch = tickers[i : i + batch_size]
        quotes = fmp.get_quote(batch)
        for q in quotes:
            s = (q.get("symbol") or "").strip()
            if s:
                quote_by_sym[s] = q
        time.sleep(0.2)

    # EODHD fallback for prices when FMP returned nothing (e.g. rate limited)
    if not quote_by_sym and EODHD_API_KEY:
        eod = EODHDClient()
        max_eod = 20 if FMP_FREE_TIER else 100  # EODHD free ≈20 calls/day
        for i in range(0, min(len(tickers), max_eod), 20):
            batch = tickers[i : i + 20]
            eq = eod.get_quote(batch)
            for q in eq:
                s = (q.get("symbol") or "").strip()
                if s:
                    quote_by_sym[s] = q
            time.sleep(0.15)

    # Insider sells
    insider_tickers = tickers[:insider_sample]
    if FMP_API_KEY:
        insider_records = aggregate_insider_sells(insider_tickers, date_from=date_from, date_to=date_to)
    top_insiders = _top_insiders_by_ticker(insider_records)

    q_start, q_end = _quarter_dates(as_of)
    for c in companies[:sample]:
        sym = c["symbol"]
        hist = fmp.get_historical_range(sym, q_start, q_end)
        hist_by_sym[sym] = _quarter_change(hist)
        time.sleep(0.05)

    for c in companies[:sample]:
        sym = c["symbol"]
        news = fmp.get_news(sym, limit=2)
        news_by_sym[sym] = [
            {"title": (n.get("title") or "")[:80], "url": n.get("url") or ""}
            for n in (news or [])[:2]
        ]
        time.sleep(0.05)

    # Group by sector
    by_sector = defaultdict(list)
    for c in companies:
        sym = c["symbol"]
        q = quote_by_sym.get(sym) or {}
        price = float(q.get("price") or q.get("change") or 0)
        chg = float(q.get("changesPercentage") or 0)

        by_sector[c["sector"]].append({
            "symbol": sym,
            "name": c["name"],
            "price": price if price > 0 else None,
            "change_pct": chg if chg else None,
            "quarter_trend": hist_by_sym.get(sym),
            "news": news_by_sym.get(sym, []),
            "top_insiders": top_insiders.get(sym, []),
        })

    sectors = []
    for sector in sorted(by_sector.keys()):
        sectors.append({
            "name": sector,
            "companies": by_sector[sector],
        })

    return {
        "as_of": as_of.isoformat(),
        "total_companies": len(companies),
        "sectors": sectors,
    }
