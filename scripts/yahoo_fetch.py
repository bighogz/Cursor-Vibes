#!/usr/bin/env python3
"""Fetch Yahoo Finance data via yfinance. Used by Go API when direct HTTP returns 401.
Usage:
  python scripts/yahoo_fetch.py quotes --symbols=GOOGL,AAPL,MSFT
  python scripts/yahoo_fetch.py hist --symbol=GOOGL --from=2024-01-01 --to=2024-04-01
  python scripts/yahoo_fetch.py news --symbol=GOOGL --limit=2
Outputs JSON to stdout."""
import argparse
import json
import sys
from typing import Optional

try:
    import yfinance as yf
except ImportError:
    sys.stderr.write("yfinance not installed\n")
    sys.exit(1)

try:
    import numpy as np
except ImportError:
    np = None


def _safe_float(v, default=0.0):
    try:
        return float(v) if v is not None else default
    except (TypeError, ValueError):
        return default


def yahoo_prices_batch(symbols: list[str]) -> dict[str, float]:
    """Batch fetch last close prices via yf.download (faster than per-ticker)."""
    symbols = [s.strip() for s in symbols[:100] if s and s.strip()]
    if not symbols:
        return {}
    try:
        df = yf.download(
            tickers=" ".join(symbols),
            period="5d",
            interval="1d",
            group_by="ticker",
            threads=True,
            auto_adjust=False,
            progress=False,
            timeout=30,
        )
        if df is None or df.empty:
            return {}
        out = {}
        for s in symbols:
            try:
                if len(symbols) == 1:
                    close = df["Close"]
                else:
                    close = df[s]["Close"]
                close = close.dropna()
                if len(close):
                    out[s] = float(close.iloc[-1])
            except Exception:
                pass
        return out
    except Exception:
        return {}


def _yahoo_quotes_from_batch(symbols: list[str]) -> list[dict]:
    """Use batch download for prices, compute chg_pct from 5d data."""
    symbols = [s.strip() for s in symbols[:100] if s and s.strip()]
    if not symbols:
        return []
    try:
        df = yf.download(
            tickers=" ".join(symbols),
            period="5d",
            interval="1d",
            group_by="ticker",
            threads=True,
            auto_adjust=False,
            progress=False,
            timeout=30,
        )
        if df is None or df.empty:
            return []
        out = []
        for s in symbols:
            try:
                if len(symbols) == 1:
                    close = df["Close"]
                else:
                    close = df[s]["Close"]
                close = close.dropna()
                if len(close) < 1:
                    continue
                price = float(close.iloc[-1])
                prev = float(close.iloc[-2]) if len(close) >= 2 else price
                chg_pct = round((price - prev) / prev * 100, 2) if prev and prev > 0 else 0
                out.append({"symbol": s, "price": price, "changesPercentage": chg_pct})
            except Exception:
                pass
        return out
    except Exception:
        return []


def fetch_quotes(symbols: list[str]) -> list[dict]:
    syms = [s.strip() for s in symbols[:100] if s and s.strip()]
    if len(syms) > 1:
        batch_result = _yahoo_quotes_from_batch(syms)
        if batch_result:
            return batch_result
    out = []
    for sym in syms:
        try:
            t = yf.Ticker(sym)
            info = getattr(t, "info", None) or {}
            info = info if isinstance(info, dict) else {}
            price = _safe_float(
                info.get("regularMarketPrice") or info.get("currentPrice") or info.get("lastPrice") or info.get("previousClose"),
                0,
            )
            if price <= 0:
                hist = t.history(period="1d", interval="1m")
                price = float(hist["Close"].iloc[-1]) if hist is not None and not hist.empty else 0.0
            prev = _safe_float(info.get("previousClose"), 1)
            chg_pct = round((price - prev) / prev * 100, 2) if prev and prev > 0 else 0
            if price > 0:
                out.append({"symbol": sym, "price": price, "changesPercentage": chg_pct})
        except Exception:
            pass
    return out


def fetch_hist(symbol: str, from_date: str, to_date: str) -> list[dict]:
    try:
        t = yf.Ticker(symbol)
        df = t.history(start=from_date, end=to_date, auto_adjust=True)
        if df is None or df.empty:
            return []
        out = []
        for idx, row in df.iterrows():
            dt = idx.strftime("%Y-%m-%d") if hasattr(idx, "strftime") else str(idx)[:10]
            close = _safe_float(row.get("Close", row["Close"]) if hasattr(row, "get") else row["Close"], 0)
            out.append({"date": dt, "close": close})
        return out
    except Exception:
        return []


def fetch_quarterly_trend(symbol: str) -> Optional[dict]:
    try:
        t = yf.Ticker(symbol)
        hist = t.history(period="6mo", interval="1d")
        if hist is None or hist.empty:
            return None

        close = hist["Close"].dropna()
        if len(close) < 30:
            return None

        last = float(close.iloc[-1])
        lookback = 63 if len(close) > 63 else max(1, len(close) // 2)
        prev = float(close.iloc[-lookback])

        q_return = (last / prev) - 1.0
        quarter_pct = round(q_return * 100, 2)

        slope = 0.0
        if np is not None:
            y = close.iloc[-lookback:].to_numpy()
            x = np.arange(len(y))
            slope = float(np.polyfit(x, y, 1)[0])

        return {
            "quarter_pct": quarter_pct,
            "q_return": q_return,
            "slope": slope,
            "last": last,
        }
    except Exception:
        return None


def fetch_news(symbol: str, limit: int = 5) -> list[dict]:
    try:
        t = yf.Ticker(symbol)
        items = getattr(t, "news", None) or []
        out = []
        for n in (items or [])[:limit]:
            if not isinstance(n, dict):
                continue
            content = n.get("content") or n
            title = str(
                content.get("title") or content.get("summary") or n.get("title") or n.get("link") or ""
            )[:200]
            link = ""
            for key in ("clickThroughUrl", "canonicalUrl", "url"):
                u = content.get(key) if isinstance(content.get(key), dict) else n.get(key)
                if isinstance(u, dict) and u.get("url"):
                    link = u["url"]
                    break
                if isinstance(u, str) and u.startswith("http"):
                    link = u
                    break
            link = link or str(n.get("link") or n.get("url") or "")
            publisher = content.get("provider", {})
            if isinstance(publisher, dict):
                publisher = publisher.get("displayName") or ""
            else:
                publisher = n.get("publisher") or ""
            published = (
                content.get("providerPublishTime") or content.get("pubDate") or content.get("displayTime")
                or n.get("providerPublishTime")
            )
            if title or link:
                out.append({
                    "title": title,
                    "url": link,
                    "link": link,
                    "publisher": publisher,
                    "published": published,
                })
        return out
    except Exception:
        return []


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("cmd", choices=["quotes", "hist", "news", "qtrend"])
    ap.add_argument("--symbols", help="Comma-separated for quotes")
    ap.add_argument("--symbol", help="Single symbol for hist/news")
    ap.add_argument("--from", dest="from_date", help="Start date YYYY-MM-DD")
    ap.add_argument("--to", dest="to_date", help="End date YYYY-MM-DD")
    ap.add_argument("--limit", type=int, default=5)
    args = ap.parse_args()

    if args.cmd == "quotes":
        syms = [s.strip() for s in (args.symbols or "").split(",") if s.strip()]
        data = fetch_quotes(syms)
    elif args.cmd == "hist":
        data = fetch_hist(args.symbol or "", args.from_date or "", args.to_date or "")
    elif args.cmd == "news":
        data = fetch_news(args.symbol or "", args.limit or 5)
    elif args.cmd == "qtrend":
        data = fetch_quarterly_trend(args.symbol or "")
    else:
        data = []

    print(json.dumps(data))


if __name__ == "__main__":
    main()
