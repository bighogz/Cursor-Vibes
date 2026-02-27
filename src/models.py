"""Insider selling tracker for S&P 500 - data models and shared types."""
from dataclasses import dataclass
from datetime import date
from typing import Optional


@dataclass
class InsiderSellRecord:
    """Single normalized insider sell record (any source)."""
    ticker: str
    company_name: Optional[str]
    insider_name: Optional[str]
    role: Optional[str]
    transaction_date: date
    filing_date: Optional[date]
    shares_sold: float
    value_usd: Optional[float]
    source: str  # "fmp" | "sec_api" | "eodhd" | "financial_datasets"
    raw: Optional[dict] = None
