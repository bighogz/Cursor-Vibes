"""Load S&P 500 with sector/industry from free CSV."""
import csv
import requests
from typing import List, Dict
from pathlib import Path


CSV_URL = "https://raw.githubusercontent.com/datasets/s-and-p-500-companies/master/data/constituents.csv"


def load_sp500_with_sectors() -> List[Dict]:
    """Return list of {symbol, name, sector, sub_industry}."""
    try:
        r = requests.get(CSV_URL, timeout=15)
        r.raise_for_status()
        reader = csv.DictReader(r.text.strip().split("\n"))
        rows = []
        seen = set()
        for row in reader:
            sym = (row.get("Symbol") or "").strip()
            if not sym or sym in seen:
                continue
            seen.add(sym)
            rows.append({
                "symbol": sym,
                "name": (row.get("Security") or "").strip(),
                "sector": (row.get("GICS Sector") or "").strip() or "Unknown",
                "sub_industry": (row.get("GICS Sub-Industry") or "").strip(),
            })
        return rows
    except Exception:
        return []
