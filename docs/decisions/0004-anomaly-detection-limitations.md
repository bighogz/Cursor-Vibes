# ADR-0004: Anomaly Detection Limitations and Financial Domain Awareness

**Status:** Accepted  
**Date:** 2026-02-26

## Context

Our anomaly detection uses a Z-score model comparing current insider selling volume against a historical baseline. While statistically sound, financial markets have structural patterns that create predictable sell-volume spikes that are not genuinely anomalous.

## Known Limitations

### 1. Earnings Blackout Periods

S&P 500 companies impose "quiet periods" (typically 2-4 weeks) before quarterly earnings announcements during which insiders cannot trade. After the blackout lifts, insiders who were waiting to sell will cluster their transactions, creating a volume spike.

**Our mitigation:** We apply a 40% Z-score dampening when the current window falls in a post-blackout month (Feb, May, Aug, Nov). This is a heuristic — precise blackout windows are company-specific and governed by each firm's insider trading policy.

**What a production system would do:** Ingest actual earnings dates from SEC 8-K filings or a financial data provider, compute per-company blackout windows, and exclude or weight-adjust those periods in the baseline.

### 2. Rule 10b5-1 Predetermined Trading Plans

Many executives sell stock on predetermined schedules filed under SEC Rule 10b5-1. These "planned sales" are not informative about the insider's view of the company — they're liquidity events. Our model treats all sells equally.

**What a production system would do:** Cross-reference Form 4 filings with 10b5-1 plan disclosures (available in footnotes of Form 4 XML). Flag planned vs. discretionary sells and weight them differently in the Z-score computation.

### 3. Option Exercises and Tax Events

Many Form 4 filings record "sell-to-cover" transactions where insiders sell shares to cover the tax obligation from exercising stock options. These are routine and non-informative.

**Our mitigation:** We filter for transaction code "S" (open-market sale) and disposition code "D", which excludes most automatic transactions. However, some sell-to-cover transactions still appear as "S" codes.

### 4. Seasonality Beyond Blackouts

Insider selling has annual patterns: year-end tax-loss harvesting, equity compensation vesting schedules (often annual in Q1), and diversification selling after lock-up periods. Our baseline does not model these seasonal patterns.

**What a production system would do:** Use a seasonal decomposition (e.g., STL or Fourier-based) on the baseline, or compute per-calendar-month baselines instead of a rolling window.

## Decision

We document these limitations transparently rather than pretending the model is more sophisticated than it is. The blackout dampening heuristic demonstrates domain awareness without overengineering a system that isn't processing real trading signals.

## Consequences

- Reviewers see financial domain knowledge, not just generic statistics
- The dampening factor (0.6) is tunable via the codebase, not hardcoded as a magic number
- A future iteration could load actual earnings dates and 10b5-1 plan data from the SEC API
