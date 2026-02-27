"""Financial Modeling Prep API client - S&P 500 list and insider trading."""
import csv
import requests
from datetime import date, datetime
from typing import List, Optional

from ..config import FMP_API_KEY
from ..models import InsiderSellRecord


FMP_BASE = "https://financialmodelingprep.com/stable"
# FMP stable API uses apikey query param (per common usage)
AUTH_PARAM = "apikey"


def _parse_date(s: Optional[str]) -> Optional[date]:
    if not s:
        return None
    try:
        return datetime.strptime(s[:10], "%Y-%m-%d").date()
    except Exception:
        return None


class FMPClient:
    """Fetch S&P 500 constituents and insider trading from Financial Modeling Prep."""

    def __init__(self, api_key: Optional[str] = None):
        self.api_key = api_key or FMP_API_KEY

    def _get(self, path: str, params: Optional[dict] = None) -> dict:
        if not self.api_key:
            return {}
        url = f"{FMP_BASE}{path}"
        p = dict(params or {})
        p[AUTH_PARAM] = self.api_key
        try:
            r = requests.get(url, params=p, timeout=30)
            if r.status_code == 429:
                return {"_error": "rate_limit", "message": "FMP API limit reached. Upgrade plan or wait for reset."}
            r.raise_for_status()
            data = r.json()
            if isinstance(data, dict) and "Error Message" in data:
                return {"_error": "api_error", "message": data["Error Message"]}
            return data
        except requests.exceptions.HTTPError as e:
            return {"_error": "http", "status": getattr(e.response, "status_code", None)}
        except Exception:
            return {}

    def get_sp500_tickers(self) -> List[str]:
        """Return list of S&P 500 ticker symbols. Uses FMP if available, else free CSV fallback."""
        data = self._get("/sp500-constituent")
        if isinstance(data, list) and data:
            return [item.get("symbol", "").strip() for item in data if item.get("symbol")]
        # Fallback: FMP free tier doesn't include sp500-constituent (402 Restricted).
        # Use free public CSV (updated periodically).
        try:
            r = requests.get(
                "https://raw.githubusercontent.com/datasets/s-and-p-500-companies/master/data/constituents.csv",
                timeout=15,
            )
            r.raise_for_status()
            lines = r.text.strip().split("\n")
            if len(lines) < 2:
                return []
            # CSV: Symbol,Security,GICS Sector,GICS Sub-Industry,Created Date,...
            reader = csv.DictReader(lines)
            return [row.get("Symbol", "").strip() for row in reader if row.get("Symbol")]
        except Exception:
            return []

    def get_insider_sells(
        self,
        ticker: Optional[str] = None,
        page: int = 0,
        limit: int = 100,
        date_from: Optional[date] = None,
        date_to: Optional[date] = None,
    ) -> List[InsiderSellRecord]:
        """
        Fetch insider transactions; filter to sells and normalize.
        If ticker is None, uses latest insider trading (all symbols).
        """
        records: List[InsiderSellRecord] = []
        if not self.api_key:
            return records

        if ticker:
            # Search by symbol: search insider trades
            data = self._get("/insider-trading/search", {"symbol": ticker, "page": page, "limit": limit})
        else:
            data = self._get("/insider-trading/latest", {"page": page, "limit": limit})

        items = data if isinstance(data, list) else (data.get("data") or data.get("insider_trading") or [])
        if not isinstance(items, list):
            return records

        for item in items:
            # FMP: transactionType often "S" for sale, or check acquisition/disposition
            trans_type = (item.get("transactionType") or item.get("type") or "").upper()
            acq_disp = (item.get("acquisitionOrDisposition") or item.get("acquiredDisposedCode") or "").upper()
            is_sell = trans_type in ("S", "D") or acq_disp == "D" or "sale" in (item.get("transactionType") or "").lower()
            if not is_sell:
                continue

            ticker_sym = (item.get("symbol") or item.get("ticker") or ticker or "").strip()
            tx_date = _parse_date(item.get("transactionDate") or item.get("periodOfReport") or item.get("filingDate"))
            if not tx_date:
                continue
            if date_from and tx_date < date_from:
                continue
            if date_to and tx_date > date_to:
                continue

            shares = float(item.get("numberOfShares") or item.get("shares") or 0)
            if shares <= 0:
                continue

            value = item.get("value") or item.get("valueUsd")
            if value is not None:
                try:
                    value = float(value)
                except (TypeError, ValueError):
                    value = None

            records.append(
                InsiderSellRecord(
                    ticker=ticker_sym,
                    company_name=item.get("companyName") or item.get("issuer", {}).get("name"),
                    insider_name=item.get("reportingName") or item.get("reportingOwner", {}).get("name"),
                    role=item.get("typeOfOwner") or None,
                    transaction_date=tx_date,
                    filing_date=_parse_date(item.get("filingDate") or item.get("filedAt")),
                    shares_sold=shares,
                    value_usd=value,
                    source="fmp",
                    raw=item,
                )
            )
        return records

    def get_quote(self, symbols: List[str]) -> List[dict]:
        """Batch quote for symbols. FMP allows comma-separated."""
        if not self.api_key or not symbols:
            return []
        sym_str = ",".join(s[:10] for s in symbols[:100])
        data = self._get("/quote", {"symbol": sym_str})
        if isinstance(data, dict) and data.get("_error"):
            return []
        return data if isinstance(data, list) else []

    def get_news(self, ticker: str, limit: int = 5) -> List[dict]:
        """Stock news for ticker."""
        if not self.api_key or not ticker:
            return []
        data = self._get("/stock-news", {"symbol": ticker, "limit": limit})
        if isinstance(data, dict) and data.get("_error"):
            return []
        return data if isinstance(data, list) else []

    def get_historical_range(self, ticker: str, from_date: str, to_date: str) -> List[dict]:
        """Historical prices for quarterly trend. Uses historical-price-eod/full."""
        if not self.api_key or not ticker:
            return []
        data = self._get("/historical-price-eod/full", {"symbol": ticker, "from": from_date, "to": to_date})
        if isinstance(data, dict) and data.get("_error"):
            return []
        hist = data.get("historical") if isinstance(data, dict) else (data if isinstance(data, list) else [])
        return hist if isinstance(hist, list) else []
