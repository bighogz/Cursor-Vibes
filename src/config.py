"""Configuration and environment for insider selling tracker."""
import os
from pathlib import Path

from dotenv import load_dotenv
_root = Path(__file__).resolve().parent.parent
load_dotenv(_root / ".env")

def _get(key: str) -> str:
    return (os.getenv(key) or "").strip()


# API keys - empty string means skip that source
FMP_API_KEY = _get("FMP_API_KEY")
SEC_API_KEY = _get("SEC_API_KEY")
FINANCIAL_DATASETS_API_KEY = _get("FINANCIAL_DATASETS_API_KEY")
EODHD_API_KEY = _get("EODHD_API_KEY")

# Anomaly detection: flag when current-period selling exceeds baseline + (this many) standard deviations
ANOMALY_STD_THRESHOLD = 2.0

# Lookback for baseline: days of history to compute "normal" selling
BASELINE_DAYS = 365

# Current window: days to compare against baseline (e.g. last 30 days)
CURRENT_WINDOW_DAYS = 30

# Minimum number of baseline data points to compute std (otherwise no flag)
MIN_BASELINE_POINTS = 5

# Free tier mode: severely limits FMP API calls so you stay within 250/day
# Set to true if on FMP free plan to get some data without hitting rate limit
FMP_FREE_TIER = os.getenv("FMP_FREE_TIER", "").lower() in ("1", "true", "yes")
