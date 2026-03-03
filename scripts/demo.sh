#!/usr/bin/env bash
set -euo pipefail

# Local demo: build, start, sample, capture output, shutdown.
# Run via: make demo

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
OUT_DIR="$PROJECT_ROOT/out"
PORT="${PORT:-8000}"
API="http://localhost:$PORT"
PID=""

cleanup() {
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    echo "  Stopping server (pid $PID)..."
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

header() { printf "\n\033[1;36m▸ %s\033[0m\n" "$1"; }

cd "$PROJECT_ROOT"

# -------------------------------------------------------------------
header "Building (Go + Rust + React frontend)"
make build 2>&1 | tail -5

# -------------------------------------------------------------------
header "Starting server on :$PORT"
./bin/api &
PID=$!
sleep 3

if ! kill -0 "$PID" 2>/dev/null; then
  echo "ERROR: server failed to start" >&2
  exit 1
fi

# -------------------------------------------------------------------
header "Health check"
curl -sf "$API/api/health" | python3 -m json.tool

# -------------------------------------------------------------------
header "Provider diagnostics"
curl -sf "$API/api/health/providers" | python3 -m json.tool || echo "(endpoint not available)"

# -------------------------------------------------------------------
header "Dashboard sample (limit=5)"
DASH=$(curl -sf "$API/api/dashboard?limit=5")
echo "$DASH" | python3 -m json.tool | head -80
echo "  ... (truncated)"

# -------------------------------------------------------------------
header "Generating report artifact → ./out/"
mkdir -p "$OUT_DIR"

echo "$DASH" | python3 -m json.tool > "$OUT_DIR/dashboard-sample.json"
curl -sf "$API/api/health" | python3 -m json.tool > "$OUT_DIR/health.json"

# Count populated fields
TOTAL=$(echo "$DASH" | python3 -c "
import json, sys
d = json.load(sys.stdin)
companies = d.get('companies', d.get('data', []))
n = len(companies)
with_price = sum(1 for c in companies if c.get('price') not in (None, 0, '', '—'))
with_trend = sum(1 for c in companies if c.get('quarterly_trend') not in (None, '', '—'))
print(f'Companies: {n}  |  With price: {with_price}  |  With trend: {with_trend}')
" 2>/dev/null || echo "(could not parse)")
echo "$TOTAL" | tee "$OUT_DIR/summary.txt"

# -------------------------------------------------------------------
header "Artifacts written"
ls -lh "$OUT_DIR"/

echo ""
echo "Done. Server shutting down."
