#!/bin/sh
# Set GitHub repo description. Run: gh auth login   (once)
# Then: ./scripts/set_github_description.sh
set -e
gh repo edit bighogz/Cursor-Vibes --description "S&P 500 insider selling tracker with anomaly detection. Dual backend (Python/FastAPI + Go). Aggregates sells from FMP, SEC-API, EODHD; flags unusual activity via z-score; Yahoo for free quotes/trends."
echo "Description updated."
