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

try:
    import yfinance as yf
except ImportError:
    sys.stderr.write("yfinance not installed\n")
    sys.exit(1)


def _safe_float(v, default=0.0):
    try:
        return float(v) if v is not None else default
    except (TypeError, ValueError):
        return default


def fetch_quotes(symbols: list[str]) -> list[dict]:
    out = []
    for sym in symbols[:100]:
        try:
            t = yf.Ticker(sym)
            info = getattr(t, "info", None) or {}
            if not isinstance(info, dict):
                continue
            price = _safe_float(
                info.get("regularMarketPrice") or info.get("currentPrice") or info.get("lastPrice") or info.get("previousClose"),
                0,
            )
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


def fetch_news(symbol: str, limit: int) -> list[dict]:
    try:
        t = yf.Ticker(symbol)
        items = getattr(t, "news", None) or []
        out = []
        for n in (items or [])[:limit]:
            if not isinstance(n, dict):
                continue
            content = n.get("content") or n
            title = str(content.get("title") or content.get("summary") or n.get("title") or n.get("link") or "")[:200]
            url = ""
            for key in ("clickThroughUrl", "canonicalUrl", "url"):
                u = content.get(key) if isinstance(content.get(key), dict) else n.get(key)
                if isinstance(u, dict) and u.get("url"):
                    url = u["url"]
                    break
                if isinstance(u, str) and u.startswith("http"):
                    url = u
                    break
            url = url or str(n.get("link") or n.get("url") or "")
            if title or url:
                out.append({"title": title, "url": url, "link": url})
        return out
    except Exception:
        return []


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("cmd", choices=["quotes", "hist", "news"])
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
    else:
        data = []

    print(json.dumps(data))


if __name__ == "__main__":
    main()
