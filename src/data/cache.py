"""File-based cache for dashboard data. Refreshes every 24 hours."""
import json
from pathlib import Path
from datetime import datetime, timedelta
from typing import Optional, Dict, Any

CACHE_FILE = Path(__file__).resolve().parent.parent.parent / "data" / "dashboard_cache.json"
MAX_AGE_HOURS = 24


def _ensure_dir():
    CACHE_FILE.parent.mkdir(parents=True, exist_ok=True)


def read_cache(allow_stale: bool = True) -> Optional[Dict[str, Any]]:
    """Return cached data. If allow_stale=True, returns even if older than 24h."""
    if not CACHE_FILE.exists():
        return None
    try:
        data = json.loads(CACHE_FILE.read_text(encoding="utf-8"))
        ts = data.get("_cached_at")
        if not ts:
            return None
        cached_at = datetime.fromisoformat(ts)
        if not allow_stale and datetime.utcnow() - cached_at > timedelta(hours=MAX_AGE_HOURS):
            return None
        return data
    except Exception:
        return None


def write_cache(data: Dict[str, Any]) -> None:
    """Write dashboard data to cache."""
    _ensure_dir()
    payload = dict(data)
    payload["_cached_at"] = datetime.utcnow().isoformat()
    CACHE_FILE.write_text(json.dumps(payload, indent=0), encoding="utf-8")


def get_cached_at() -> Optional[datetime]:
    """Return when cache was last updated, or None."""
    d = read_cache()
    if not d or not d.get("_cached_at"):
        return None
    try:
        return datetime.fromisoformat(d["_cached_at"])
    except Exception:
        return None
