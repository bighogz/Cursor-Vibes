#!/usr/bin/env python3
"""
Insider Selling Tracker - S&P 500.

Fetches insider sell data from FMP, SEC-API, EODHD, and Financial Datasets;
signals when selling for a company exceeds its normal baseline (algorithmic anomaly).
"""
from datetime import date, timedelta
import argparse
import sys
import os

_here = os.path.dirname(os.path.abspath(__file__))
if _here not in sys.path:
    sys.path.insert(0, _here)

from pathlib import Path
from dotenv import load_dotenv
load_dotenv(Path(_here) / ".env")

from src.config import (
    FMP_API_KEY,
    BASELINE_DAYS,
    CURRENT_WINDOW_DAYS,
    ANOMALY_STD_THRESHOLD,
)
from src.clients import FMPClient
from src.aggregator import (
    aggregate_insider_sells,
    compute_anomaly_signals,
    get_anomalous_tickers,
)


def main():
    parser = argparse.ArgumentParser(
        description="Track S&P 500 insider selling and flag anomalous selling."
    )
    parser.add_argument(
        "--baseline-days",
        type=int,
        default=BASELINE_DAYS,
        help="Days of history for baseline (default: 365)",
    )
    parser.add_argument(
        "--current-days",
        type=int,
        default=CURRENT_WINDOW_DAYS,
        help="Current window days to compare (default: 30)",
    )
    parser.add_argument(
        "--std-threshold",
        type=float,
        default=ANOMALY_STD_THRESHOLD,
        help="Z-score threshold to flag anomaly (default: 2.0)",
    )
    parser.add_argument(
        "--as-of",
        type=str,
        default=None,
        help="As-of date YYYY-MM-DD (default: today)",
    )
    parser.add_argument(
        "--list-all-signals",
        action="store_true",
        help="Print full signals table (not only anomalies)",
    )
    parser.add_argument(
        "--csv",
        type=str,
        default=None,
        help="Write anomaly signals to CSV path",
    )
    args = parser.parse_args()

    as_of = date.today()
    if args.as_of:
        as_of = date.fromisoformat(args.as_of)

    # 1) Get S&P 500 tickers (FMP API or free CSV fallback)
    fmp = FMPClient()
    tickers = fmp.get_sp500_tickers()
    if not tickers:
        print("Could not load S&P 500 constituents.", file=sys.stderr)
        sys.exit(1)
    print(f"Loaded {len(tickers)} S&P 500 tickers.")

    # 2) Date range: need baseline + current window
    total_days = args.baseline_days + args.current_days
    date_from = as_of - timedelta(days=total_days)
    date_to = as_of

    print(f"Fetching insider sells from {date_from} to {date_to}...")
    records = aggregate_insider_sells(tickers, date_from=date_from, date_to=date_to)
    print(f"Aggregated {len(records)} insider sell records (deduplicated across APIs).")

    # 3) Anomaly detection
    signals = compute_anomaly_signals(
        records,
        baseline_days=args.baseline_days,
        current_days=args.current_days,
        std_threshold=args.std_threshold,
        as_of=as_of,
    )
    anomalous = get_anomalous_tickers(
        records,
        baseline_days=args.baseline_days,
        current_days=args.current_days,
        std_threshold=args.std_threshold,
        as_of=as_of,
    )

    # 4) Output
    if args.list_all_signals:
        print("\nAll signals (current window vs baseline):")
        if signals.empty:
            print("  (No data)")
        else:
            print(signals.sort_values("z_score", ascending=False).to_string(index=False))
    else:
        print("\nAnomalous insider selling (above normal):")
        if not anomalous:
            print("  None detected.")
        else:
            subset = signals[signals["is_anomaly"]].sort_values("z_score", ascending=False)
            print(subset.to_string(index=False))

    if args.csv:
        if not signals.empty and "is_anomaly" in signals.columns:
            out_df = signals[signals["is_anomaly"]] if not args.list_all_signals else signals
            out_df.to_csv(args.csv, index=False)
            print(f"\nWrote {args.csv}.")


if __name__ == "__main__":
    main()
