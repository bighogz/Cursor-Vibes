"""Unified price fetcher: try FMP first, fall back to Yahoo on rate limit or error."""
from typing import Optional, Tuple


from ..config import FMP_API_KEY
from .fmp import FMPClient
from .yahoo import YahooClient


class RateLimited(Exception):
    """Raised when FMP returns 429 (API limit reached)."""
    pass


def _fmp_price_raw(symbol: str) -> Optional[float]:
    """Fetch price from FMP. Returns None on error. Raises RateLimited on 429."""
    if not FMP_API_KEY or not symbol:
        return None
    client = FMPClient()
    data = client._get("/quote", {"symbol": symbol})
    if isinstance(data, dict) and data.get("_error"):
        if data.get("_error") == "rate_limit" or "limit" in str(data.get("message", "")).lower():
            raise RateLimited("FMP API rate limit reached")
        return None
    items = data if isinstance(data, list) else []
    for item in items:
        if isinstance(item, dict) and (item.get("symbol") or "").strip().upper() == symbol.upper():
            p = item.get("price") or item.get("change")
            if p is not None:
                try:
                    return float(p)
                except (TypeError, ValueError):
                    pass
            break
    return None


def fmp_price(symbol: str) -> Optional[float]:
    """Fetch price from FMP. Raises RateLimited on 429."""
    return _fmp_price_raw(symbol)


def yahoo_price(symbol: str) -> Optional[float]:
    """Fetch price from Yahoo (yfinance)."""
    client = YahooClient()
    if not client._available():
        return None
    quotes = client.get_quote([symbol])
    for q in quotes:
        if (q.get("symbol") or "").strip().upper() == symbol.upper():
            p = q.get("price") or 0
            if p and float(p) > 0:
                return float(p)
    return None


def get_price(symbol: str) -> Tuple[Optional[float], str]:
    """Try FMP first; fall back to Yahoo on rate limit or error. Returns (price, source)."""
    try:
        p = fmp_price(symbol)
        if p is not None:
            return p, "fmp"
    except RateLimited:
        p = yahoo_price(symbol)
        return p, "yahoo"
    except Exception:
        p = yahoo_price(symbol)
        return p, "yahoo"
    p = yahoo_price(symbol)
    return p, "yahoo"
