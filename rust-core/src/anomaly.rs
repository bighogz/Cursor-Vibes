//! Z-score based anomaly detection. No external deps, constant-time where possible.

use crate::models::InsiderSellRecord;
use chrono::NaiveDate;
use serde::Serialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize)]
pub struct AnomalySignal {
    pub ticker: String,
    pub current_shares_sold: f64,
    pub baseline_mean: f64,
    pub baseline_std: f64,
    pub z_score: f64,
    pub is_anomaly: bool,
}

/// Compute anomaly signals: baseline period vs current window, z-score threshold.
pub fn compute_anomaly_signals(
    records: &[InsiderSellRecord],
    baseline_days: i64,
    current_days: i64,
    std_threshold: f64,
    min_baseline_points: usize,
    as_of: NaiveDate,
) -> Vec<AnomalySignal> {
    let daily = daily_volume_by_ticker(records);
    if daily.is_empty() {
        return vec![];
    }

    let baseline_end = as_of - chrono::Duration::days(current_days);
    let baseline_start = baseline_end - chrono::Duration::days(baseline_days);
    let current_start = as_of - chrono::Duration::days(current_days);

    // ticker -> date -> shares
    let mut ticker_dates: HashMap<String, HashMap<NaiveDate, f64>> = HashMap::new();
    for d in &daily {
        ticker_dates
            .entry(d.ticker.clone())
            .or_default()
            .entry(d.date)
            .and_modify(|s| *s += d.shares)
            .or_insert(d.shares);
    }

    let num_days = (as_of - current_start).num_days().max(1) as f64;
    let mut results: Vec<AnomalySignal> = ticker_dates
        .into_iter()
        .map(|(ticker, by_date)| {
            let mut baseline_totals: Vec<f64> = Vec::new();
            let mut current_total = 0.0_f64;

            for (dt, shares) in by_date {
                if dt >= baseline_start && dt < baseline_end {
                    baseline_totals.push(shares);
                }
                if dt >= current_start && dt <= as_of {
                    current_total += shares;
                }
            }

            let (baseline_mean, baseline_std) = mean_std(&baseline_totals);
            let sig = if baseline_totals.len() < min_baseline_points {
                AnomalySignal {
                    ticker,
                    current_shares_sold: current_total,
                    baseline_mean: 0.0,
                    baseline_std: 0.0,
                    z_score: 0.0,
                    is_anomaly: false,
                }
            } else {
                let std_safe = if baseline_std <= 0.0 { 1e-9 } else { baseline_std };
                let current_avg_daily = current_total / num_days;
                let z = (current_avg_daily - baseline_mean) / std_safe;
                AnomalySignal {
                    ticker: ticker.clone(),
                    current_shares_sold: current_total,
                    baseline_mean,
                    baseline_std,
                    z_score: z,
                    is_anomaly: z >= std_threshold && current_total > 0.0,
                }
            };
            sig
        })
        .collect();

    results.sort_by(|a, b| b.z_score.partial_cmp(&a.z_score).unwrap_or(std::cmp::Ordering::Equal));
    results
}

struct DailyVolume {
    ticker: String,
    date: NaiveDate,
    shares: f64,
}

fn daily_volume_by_ticker(records: &[InsiderSellRecord]) -> Vec<DailyVolume> {
    records
        .iter()
        .map(|r| DailyVolume {
            ticker: r.ticker.to_uppercase(),
            date: r.transaction_date,
            shares: r.shares_sold,
        })
        .collect()
}

fn mean_std(vals: &[f64]) -> (f64, f64) {
    if vals.is_empty() {
        return (0.0, 0.0);
    }
    let n = vals.len() as f64;
    let sum: f64 = vals.iter().sum();
    let mean = sum / n;
    let sq_diff: f64 = vals.iter().map(|v| (v - mean).powi(2)).sum();
    let std = (sq_diff / n).sqrt();
    (mean, std)
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::NaiveDate;

    fn make_record(ticker: &str, date: &str, shares: f64) -> InsiderSellRecord {
        InsiderSellRecord {
            ticker: ticker.to_string(),
            company_name: None,
            insider_name: None,
            role: None,
            transaction_date: NaiveDate::parse_from_str(date, "%Y-%m-%d").unwrap(),
            filing_date: None,
            shares_sold: shares,
            value_usd: None,
            source: "test".to_string(),
        }
    }

    #[test]
    fn test_anomaly_detection() {
        let mut records = vec![
            make_record("AAPL", "2024-01-05", 1000.0),
            make_record("AAPL", "2024-01-06", 500.0),
            make_record("AAPL", "2024-02-01", 5000.0),
        ];
        let as_of = NaiveDate::parse_from_str("2024-02-02", "%Y-%m-%d").unwrap();
        let signals = compute_anomaly_signals(&records, 365, 30, 2.0, 2, as_of);
        assert!(!signals.is_empty());
    }
}
