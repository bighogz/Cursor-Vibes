#!/usr/bin/env python3
"""Pre-populate dashboard cache. Run before first deploy or when cache is missing."""
import sys
from pathlib import Path

_here = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(_here))

from dotenv import load_dotenv
load_dotenv(_here / ".env")

from src.data.dashboard import build_dashboard
from src.data.cache import write_cache

if __name__ == "__main__":
    print("Building dashboard (this may take 2–3 min for full S&P 500)...")
    data = build_dashboard(limit=0)
    if data.get("error"):
        print("Error:", data["error"])
        sys.exit(1)
    write_cache(data)
    print(f"Done. Cached {data.get('total_companies', 0)} companies across {len(data.get('sectors', []))} sectors.")
