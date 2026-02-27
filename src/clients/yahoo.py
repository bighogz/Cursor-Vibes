"""Yahoo Finance client via yfinance. No API key required."""
from typing import List

try:
    import yfinance as yf
except ImportError:
    yf = None


def _safe_float(v, default=0.0):
    try:
        return float(v) if v is not None else default
    except (TypeError, ValueError):
        return default


class YahooClient:
    """Uses yfinance to fetch quotes, historical prices, and news from Yahoo Finance."""

    def _available(self) -> bool:
        return yf is not None

    def get_quote(self, symbols: List[str]) -> List[dict]:
        """Batch quote for symbols. Returns FMP-style: symbol, price, changesPercentage."""
        if not self._available() or not symbols:
            return []
        out = []
        for sym in symbols[:100]:
            try:
                t = yf.Ticker(sym)
                info = t.fast_info if hasattr(t, "fast_info") else {}
                if not info:
                    info = t.info or {}
                price = _safe_float(info.get("lastPrice") or info.get("regularMarketPrice") or info.get("previousClose"), 0)
                prev = _safe_float(info.get("previousClose"), 1)
                chg_pct = round((price - prev) / prev * 100, 2) if prev and prev > 0 else 0
                out.append({
                    "symbol": sym,
                    "price": price,
                    "changesPercentage": chg_pct,
                })
            except Exception:
                pass
        return out

    def get_historical_range(self, ticker: str, from_date: str, to_date: str) -> List[dict]:
        """Historical prices for quarterly trend. Returns list of {date, close}."""
        if not self._available() or not ticker:
            return []
        try:
            t = yf.Ticker(ticker)
            df = t.history(start=from_date, end=to_date, auto_adjust=True)
            if df is None or df.empty:
                return []
            out = []
            for idx, row in df.iterrows():
                dt = idx.strftime("%Y-%m-%d") if hasattr(idx, "strftime") else str(idx)[:10]
                close = _safe_float(row.get("Close") if hasattr(row, "get") else row["Close"], 0)
                out.append({"date": dt, "close": close})
            return out
        except Exception:
            return []

    def get_news(self, ticker: str, limit: int = 5) -> List[dict]:
        """Stock news for ticker. Returns list of {title, url}."""
        if not self._available() or not ticker:
            return []
        try:
            t = yf.Ticker(ticker)
            items = getattr(t, "news", None) or []
            out = []
            for n in (items or [])[:limit]:
                if isinstance(n, dict):
                    out.append({
                        "title": (n.get("title") or n.get("link") or "")[:200],
                        "url": n.get("link") or n.get("url") or "",
                    })
            return out
        except Exception:
            return []
