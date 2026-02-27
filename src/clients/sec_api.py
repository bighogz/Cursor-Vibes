"""SEC-API.io client - insider trading from Form 3/4/5."""
import requests
from datetime import date, datetime
from typing import List, Optional

from ..config import SEC_API_KEY
from ..models import InsiderSellRecord


SEC_API_BASE = "https://api.sec-api.io"


def _parse_date(s: Optional[str]) -> Optional[date]:
    if not s:
        return None
    try:
        return datetime.strptime(s[:10], "%Y-%m-%d").date()
    except Exception:
        return None


def _parse_sec_datetime(s: Optional[str]) -> Optional[date]:
    if not s:
        return None
    try:
        # "2022-08-09T21:23:00-04:00"
        return datetime.fromisoformat(s.replace("Z", "+00:00")).date()
    except Exception:
        return _parse_date(s)


def _extract_sells_from_form(txn: dict) -> List[InsiderSellRecord]:
    """Parse one Form 3/4/5 JSON into sell records (disposed)."""
    records: List[InsiderSellRecord] = []
    issuer = txn.get("issuer") or {}
    ticker = (issuer.get("tradingSymbol") or "").strip()
    company_name = issuer.get("name")
    reporting = txn.get("reportingOwner") or {}
    insider_name = reporting.get("name")
    rel = reporting.get("relationship") or {}
    role = None
    if rel.get("isDirector"):
        role = "Director"
    elif rel.get("isOfficer"):
        role = reporting.get("officerTitle") or "Officer"
    elif rel.get("isTenPercentOwner"):
        role = "10% Owner"
    elif rel.get("isOther"):
        role = rel.get("otherText") or "Other"

    filed_at = _parse_sec_datetime(txn.get("filedAt"))
    period = _parse_date(txn.get("periodOfReport"))

    def process_transactions(transactions: list, is_derivative: bool = False):
        for tr in transactions or []:
            coding = tr.get("coding") or {}
            amounts = tr.get("amounts") or {}
            acq_disp = (amounts.get("acquiredDisposedCode") or "").upper()
            if acq_disp != "D":
                continue
            shares = float(amounts.get("shares") or 0)
            if shares <= 0:
                continue
            price = amounts.get("pricePerShare")
            value = float(price * shares) if price is not None else None
            tx_date = _parse_date(amounts.get("transactionDate")) or period or filed_at
            if not tx_date:
                continue
            records.append(
                InsiderSellRecord(
                    ticker=ticker,
                    company_name=company_name,
                    insider_name=insider_name,
                    role=role,
                    transaction_date=tx_date,
                    filing_date=filed_at,
                    shares_sold=shares,
                    value_usd=value,
                    source="sec_api",
                    raw=None,
                )
            )

    nd = txn.get("nonDerivativeTable") or {}
    process_transactions(nd.get("transactions") or nd.get("holdings") or [], is_derivative=False)
    der = txn.get("derivativeTable") or {}
    for holding in der.get("holdings") or []:
        process_transactions(holding.get("transactions") or [], is_derivative=True)

    return records


class SecApiClient:
    """Fetch insider trading via SEC-API.io (Form 3/4/5)."""

    def __init__(self, api_key: Optional[str] = None):
        self.api_key = api_key or SEC_API_KEY

    def get_insider_sells(
        self,
        ticker: str,
        date_from: Optional[date] = None,
        date_to: Optional[date] = None,
        size: int = 50,
        from_offset: int = 0,
    ) -> List[InsiderSellRecord]:
        """Query insider trading for symbol; filter to disposed (sells)."""
        records: List[InsiderSellRecord] = []
        if not self.api_key or not ticker:
            return records

        # Lucene: issuer.tradingSymbol:TICKER and period in range; we filter sells in code
        query_parts = [f"issuer.tradingSymbol:{ticker.upper()}"]
        if date_from and date_to:
            query_parts.append(f"periodOfReport:[{date_from.isoformat()} TO {date_to.isoformat()}]")
        query = " AND ".join(query_parts)

        payload = {
            "query": query,
            "from": str(from_offset),
            "size": str(size),
            "sort": [{"filedAt": {"order": "desc"}}],
        }
        url = f"{SEC_API_BASE}/insider-trading"
        try:
            r = requests.post(
                url,
                json=payload,
                headers={"Authorization": self.api_key},
                params={"token": self.api_key},
                timeout=30,
            )
            r.raise_for_status()
            data = r.json()
        except Exception:
            return records

        transactions = data.get("transactions") or []
        for txn in transactions:
            for rec in _extract_sells_from_form(txn):
                if date_from and rec.transaction_date < date_from:
                    continue
                if date_to and rec.transaction_date > date_to:
                    continue
                records.append(rec)
        return records
