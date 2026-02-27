"""EODHD API client - insider transactions (Form 4)."""
import requests
from datetime import date, datetime
from typing import List, Optional

from ..config import EODHD_API_KEY
from ..models import InsiderSellRecord


EODHD_BASE = "https://eodhd.com/api"


def _parse_date(s: Optional[str]) -> Optional[date]:
    if not s:
        return None
    try:
        return datetime.strptime(s[:10], "%Y-%m-%d").date()
    except Exception:
        return None


class EODHDClient:
    """Fetch insider transactions from EODHD."""

    def __init__(self, api_key: Optional[str] = None):
        self.api_key = api_key or EODHD_API_KEY

    def get_insider_sells(
        self,
        ticker: str,
        date_from: Optional[date] = None,
        date_to: Optional[date] = None,
        limit: int = 500,
    ) -> List[InsiderSellRecord]:
        """Get insider transactions for ticker; filter to sales (transactionCode 'S' or similar)."""
        records: List[InsiderSellRecord] = []
        if not self.api_key or not ticker:
            return records

        params: dict = {"api_token": self.api_key, "code": ticker, "limit": min(limit, 1000)}
        if date_from:
            params["date_from"] = date_from.isoformat()
        if date_to:
            params["date_to"] = date_to.isoformat()

        try:
            r = requests.get(f"{EODHD_BASE}/insider-transactions", params=params, timeout=30)
            r.raise_for_status()
            data = r.json()
        except Exception:
            return records

        items = data if isinstance(data, list) else (data.get("data") or data.get("transactions") or [])
        if not isinstance(items, list):
            return records

        for item in items:
            # EODHD: transactionCode "S" = sale, "P" = purchase, etc.
            code = (item.get("transactionCode") or item.get("code") or item.get("transactionType") or "").upper()
            if code not in ("S", "D", "C"):  # S=Sale, D=Disposed, C=Conversion (treat as disposal)
                continue

            shares = float(item.get("shares") or item.get("value") or item.get("amount") or 0)
            if shares <= 0:
                shares = float(item.get("share") or 0)
            if shares <= 0:
                continue

            tx_date = _parse_date(item.get("transactionDate") or item.get("date") or item.get("reportDate"))
            if not tx_date:
                continue

            value = item.get("value") or item.get("valueUsd")
            if value is not None:
                try:
                    value = float(value)
                except (TypeError, ValueError):
                    value = None

            records.append(
                InsiderSellRecord(
                    ticker=ticker.split(".")[0] if "." in ticker else ticker,
                    company_name=item.get("companyName") or item.get("issuer"),
                    insider_name=item.get("ownerName") or item.get("reportingName") or item.get("name"),
                    role=item.get("relationship") or item.get("typeOfOwner"),
                    transaction_date=tx_date,
                    filing_date=_parse_date(item.get("reportDate") or item.get("filingDate")),
                    shares_sold=shares,
                    value_usd=value,
                    source="eodhd",
                    raw=item,
                )
            )
        return records

    def get_quote(self, symbols: List[str]) -> List[dict]:
        """Live (delayed) prices. EODHD format: SYMBOL.US. Batch up to 15-20 for one request."""
        if not self.api_key or not symbols:
            return []
        symbols = symbols[:20]
        primary = symbols[0] + ".US"
        extra = ",".join((s + ".US" for s in symbols[1:]))
        url = f"{EODHD_BASE}/real-time/{primary}"
        params = {"api_token": self.api_key, "fmt": "json"}
        if extra:
            params["s"] = extra
        try:
            r = requests.get(url, params=params, timeout=30)
            r.raise_for_status()
            data = r.json()
            out = []
            if isinstance(data, list):
                for i, item in enumerate(data):
                    sym = symbols[i] if i < len(symbols) else (item.get("code") or "").replace(".US", "")
                    price = item.get("close") or item.get("price")
                    if sym and price is not None:
                        out.append({"symbol": sym.replace(".US", ""), "price": float(price)})
            elif isinstance(data, dict) and data.get("close") is not None:
                sym = (data.get("code") or symbols[0]).replace(".US", "")
                out.append({"symbol": sym, "price": float(data["close"])})
            return out
        except Exception:
            return []
