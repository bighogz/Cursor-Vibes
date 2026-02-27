"""Aggregate insider sell data from all providers and run anomaly detection."""
from datetime import date, timedelta
from typing import List, Optional

import numpy as np
import pandas as pd

from .config import (
    BASELINE_DAYS,
    CURRENT_WINDOW_DAYS,
    FMP_API_KEY,
    FMP_FREE_TIER,
    SEC_API_KEY,
    EODHD_API_KEY,
    FINANCIAL_DATASETS_API_KEY,
    ANOMALY_STD_THRESHOLD,
    MIN_BASELINE_POINTS,
)
from .models import InsiderSellRecord
from .clients import FMPClient, SecApiClient, EODHDClient, FinancialDatasetsClient


def _ensure_date(d: date) -> date:
    return d if isinstance(d, date) else date.fromisoformat(str(d)[:10])


def aggregate_insider_sells(
    tickers: List[str],
    date_from: date,
    date_to: date,
) -> List[InsiderSellRecord]:
    """Fetch insider sells from all configured APIs and merge (deduplicated by ticker/date/insider/shares)."""
    all_records: List[InsiderSellRecord] = []
    seen = set()

    # FMP: 1 API call per ticker. Free tier 250/day — cap to 25 tickers.
    fmp_tickers = tickers[:25] if FMP_FREE_TIER else tickers

    if FMP_API_KEY:
        fmp = FMPClient()
        for t in fmp_tickers:
            all_records.extend(fmp.get_insider_sells(ticker=t, date_from=date_from, date_to=date_to))
        # Also get latest batch for any symbol (in case tickers not in S&P yet)
        all_records.extend(
            fmp.get_insider_sells(ticker=None, page=0, limit=200, date_from=date_from, date_to=date_to)
        )

    if SEC_API_KEY:
        sec = SecApiClient()
        for t in tickers:
            all_records.extend(sec.get_insider_sells(ticker=t, date_from=date_from, date_to=date_to))

    if EODHD_API_KEY:
        eod = EODHDClient()
        for t in tickers:
            all_records.extend(eod.get_insider_sells(ticker=t, date_from=date_from, date_to=date_to))

    if FINANCIAL_DATASETS_API_KEY:
        fd = FinancialDatasetsClient()
        for t in tickers:
            all_records.extend(fd.get_insider_sells(ticker=t, date_from=date_from, date_to=date_to))

    # Dedupe by (ticker, transaction_date, insider_name, shares_sold) to avoid double-counting across APIs
    out: List[InsiderSellRecord] = []
    for r in all_records:
        key = (r.ticker.upper(), _ensure_date(r.transaction_date), (r.insider_name or ""), r.shares_sold)
        if key in seen:
            continue
        seen.add(key)
        out.append(r)
    return out


def _daily_sell_volume_by_ticker(records: List[InsiderSellRecord]) -> pd.DataFrame:
    """DataFrame: ticker, date, total_shares_sold, total_value_usd, num_transactions."""
    if not records:
        return pd.DataFrame(columns=["ticker", "date", "total_shares_sold", "total_value_usd", "num_transactions"])

    rows = []
    for r in records:
        rows.append({
            "ticker": r.ticker.upper(),
            "date": _ensure_date(r.transaction_date),
            "shares": r.shares_sold,
            "value": r.value_usd if r.value_usd is not None else float("nan"),
        })
    df = pd.DataFrame(rows)
    agg = df.groupby(["ticker", "date"]).agg(
        total_shares_sold=("shares", "sum"),
        total_value_usd=("value", "sum"),
        num_transactions=("shares", "count"),
    ).reset_index()
    agg["date"] = pd.to_datetime(agg["date"]).dt.date
    return agg


def compute_anomaly_signals(
    records: List[InsiderSellRecord],
    baseline_days: int = BASELINE_DAYS,
    current_days: int = CURRENT_WINDOW_DAYS,
    std_threshold: float = ANOMALY_STD_THRESHOLD,
    min_baseline_points: int = MIN_BASELINE_POINTS,
    as_of: Optional[date] = None,
) -> pd.DataFrame:
    """
    For each ticker, compare recent selling (current window) to historical baseline.
    Signal when current-period selling exceeds baseline mean + std_threshold * std.
    Returns DataFrame with columns: ticker, current_shares_sold, baseline_mean, baseline_std, z_score, is_anomaly.
    """
    as_of = as_of or date.today()
    df = _daily_sell_volume_by_ticker(records)
    if df.empty:
        return pd.DataFrame(
            columns=["ticker", "current_shares_sold", "baseline_mean", "baseline_std", "z_score", "is_anomaly"]
        )

    baseline_end = as_of - timedelta(days=current_days)
    baseline_start = baseline_end - timedelta(days=baseline_days)
    current_start = as_of - timedelta(days=current_days)

    results = []
    for ticker, grp in df.groupby("ticker"):
        grp = grp.sort_values("date")
        current = grp[(grp["date"] >= current_start) & (grp["date"] <= as_of)]
        baseline = grp[(grp["date"] >= baseline_start) & (grp["date"] < baseline_end)]

        current_total = current["total_shares_sold"].sum()
        # Use daily totals for baseline; compare current window's *average daily* to baseline daily mean/std
        baseline_daily = baseline.groupby("date")["total_shares_sold"].sum()
        if baseline_daily.shape[0] < min_baseline_points:
            results.append({
                "ticker": ticker,
                "current_shares_sold": current_total,
                "baseline_mean": np.nan,
                "baseline_std": np.nan,
                "z_score": np.nan,
                "is_anomaly": False,
            })
            continue

        mean_b = float(baseline_daily.mean())
        std_b = float(baseline_daily.std())
        if std_b <= 0:
            std_b = 1e-9
        # Current window: average daily sell volume (so comparable to baseline daily mean)
        num_days = max(1, (as_of - current_start).days + 1)
        current_avg_daily = current_total / num_days
        z = (current_avg_daily - mean_b) / std_b
        is_anomaly = z >= std_threshold and current_total > 0

        results.append({
            "ticker": ticker,
            "current_shares_sold": current_total,
            "baseline_mean": mean_b,
            "baseline_std": std_b,
            "z_score": z,
            "is_anomaly": is_anomaly,
        })

    return pd.DataFrame(results)


def get_anomalous_tickers(
    records: List[InsiderSellRecord],
    baseline_days: int = BASELINE_DAYS,
    current_days: int = CURRENT_WINDOW_DAYS,
    std_threshold: float = ANOMALY_STD_THRESHOLD,
    min_baseline_points: int = MIN_BASELINE_POINTS,
    as_of: Optional[date] = None,
) -> List[str]:
    """Return list of tickers where insider selling is flagged as anomalous."""
    signals = compute_anomaly_signals(
        records,
        baseline_days=baseline_days,
        current_days=current_days,
        std_threshold=std_threshold,
        min_baseline_points=min_baseline_points,
        as_of=as_of,
    )
    if signals.empty or "ticker" not in signals.columns:
        return []
    return signals[signals["is_anomaly"]]["ticker"].tolist()
