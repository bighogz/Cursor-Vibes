"""Financial Datasets API client - insider trades (Form 4)."""
import requests
from datetime import date
from typing import List, Optional

from ..config import FINANCIAL_DATASETS_API_KEY
from ..models import InsiderSellRecord


FD_BASE = "https://api.financialdatasets.ai"


def _parse_date(s: Optional[str]) -> Optional[date]:
    if not s:
        return None
    try:
        from datetime import datetime
        return datetime.strptime(s[:10], "%Y-%m-%d").date()
    except Exception:
        return None


class FinancialDatasetsClient:
    """Fetch insider trades from Financial Datasets."""

    def __init__(self, api_key: Optional[str] = None):
        self.api_key = api_key or FINANCIAL_DATASETS_API_KEY

    def get_insider_sells(
        self,
        ticker: str,
        date_from: Optional[date] = None,
        date_to: Optional[date] = None,
        limit: int = 500,
    ) -> List[InsiderSellRecord]:
        """Get insider trades for ticker; filter to dispositions/sales."""
        records: List[InsiderSellRecord] = []
        if not self.api_key or not ticker:
            return records

        url = f"{FD_BASE}/insider-trades"
        params: dict = {"ticker": ticker.upper(), "limit": min(limit, 1000)}
        if date_from:
            params["filing_date_gte"] = date_from.isoformat()
        if date_to:
            params["filing_date_lte"] = date_to.isoformat()

        headers = {"X-API-KEY": self.api_key}
        try:
            r = requests.get(url, params=params, headers=headers, timeout=30)
            r.raise_for_status()
            data = r.json()
        except Exception:
            return records

        items = data.get("insider_trades") or data.get("data") or data.get("trades") or (data if isinstance(data, list) else [])
        if not isinstance(items, list):
            return records

        for item in items:
            # Filter: disposition / sold
            acq_disp = (item.get("acquired_disposed") or item.get("acquiredDisposedCode") or item.get("transaction_type") or "").upper()
            if acq_disp not in ("D", "DISPOSED", "S", "SALE"):
                continue

            shares = float(item.get("shares") or item.get("shares_traded") or item.get("numberOfShares") or 0)
            if shares <= 0:
                continue

            tx_date = _parse_date(item.get("transaction_date") or item.get("periodOfReport") or item.get("filing_date"))
            if not tx_date:
                continue

            value = item.get("value") or item.get("value_usd") or item.get("valueUsd")
            if value is not None:
                try:
                    value = float(value)
                except (TypeError, ValueError):
                    value = None

            records.append(
                InsiderSellRecord(
                    ticker=ticker.upper(),
                    company_name=item.get("company_name") or item.get("issuer", {}).get("name"),
                    insider_name=item.get("insider_name") or item.get("reportingOwner", {}).get("name"),
                    role=item.get("title") or item.get("relationship") or item.get("officerTitle"),
                    transaction_date=tx_date,
                    filing_date=_parse_date(item.get("filing_date") or item.get("filedAt")),
                    shares_sold=shares,
                    value_usd=value,
                    source="financial_datasets",
                    raw=item,
                )
            )
        return records
