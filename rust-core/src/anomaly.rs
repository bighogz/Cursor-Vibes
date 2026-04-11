//! Composite anomaly detection: weekly-bucketed dollar volume z-score, breadth
//! z-score, and sell-frequency acceleration. Mirrors the Go implementation in
//! internal/aggregator/aggregator.go.

use crate::models::InsiderSellRecord;
use chrono::{Datelike, NaiveDate};
use serde::Serialize;
use std::collections::{HashMap, HashSet};

#[derive(Debug, Clone, Serialize)]
pub struct AnomalySignal {
    pub ticker: String,
    pub composite_score: f64,
    pub volume_z_score: f64,
    pub breadth_z_score: f64,
    pub acceleration_score: f64,
    pub is_anomaly: bool,
    pub current_dollar_vol: f64,
    pub current_shares_sold: f64,
    pub unique_insiders: usize,
    pub baseline_mean: f64,
    pub baseline_std: f64,
}

const VOLUME_WEIGHT: f64 = 0.4;
const BREADTH_WEIGHT: f64 = 0.3;
const ACCELERATION_WEIGHT: f64 = 0.3;

fn iso_week(d: NaiveDate) -> String {
    let iso = d.iso_week();
    format!("{}-W{:02}", iso.year(), iso.week())
}

pub fn compute_anomaly_signals(
    records: &[InsiderSellRecord],
    baseline_days: i64,
    current_days: i64,
    std_threshold: f64,
    min_baseline_weeks: usize,
    as_of: NaiveDate,
) -> Vec<AnomalySignal> {
    let txs = extract_tx_records(records);
    if txs.is_empty() {
        return vec![];
    }

    let baseline_end = as_of - chrono::Duration::days(current_days);
    let baseline_start = baseline_end - chrono::Duration::days(baseline_days);
    let current_start = as_of - chrono::Duration::days(current_days);

    let baseline_weeks_f = (baseline_days as f64) / 7.0_f64.max(1.0);
    let current_weeks_f = (current_days as f64) / 7.0_f64.max(1.0);

    let mut by_ticker: HashMap<String, Vec<&TxRecord>> = HashMap::new();
    for tx in &txs {
        by_ticker.entry(tx.ticker.clone()).or_default().push(tx);
    }

    let mut results: Vec<AnomalySignal> = by_ticker
        .into_iter()
        .map(|(ticker, txs)| {
            let mut baseline_txs: Vec<&TxRecord> = Vec::new();
            let mut current_txs: Vec<&TxRecord> = Vec::new();
            for tx in &txs {
                if tx.date >= baseline_start && tx.date < baseline_end {
                    baseline_txs.push(tx);
                }
                if tx.date >= current_start && tx.date <= as_of {
                    current_txs.push(tx);
                }
            }

            // Weekly dollar volume buckets (baseline).
            let mut baseline_weekly_vol: HashMap<String, f64> = HashMap::new();
            let mut baseline_weekly_insiders: HashMap<String, HashSet<String>> = HashMap::new();
            for tx in &baseline_txs {
                let wk = iso_week(tx.date);
                *baseline_weekly_vol.entry(wk.clone()).or_default() += tx.dollar_val;
                baseline_weekly_insiders
                    .entry(wk)
                    .or_default()
                    .insert(tx.insider.clone());
            }

            // Current window aggregates.
            let mut current_dollar_vol = 0.0_f64;
            let mut current_shares = 0.0_f64;
            let mut current_insiders: HashSet<String> = HashSet::new();
            for tx in &current_txs {
                current_dollar_vol += tx.dollar_val;
                current_shares += tx.shares;
                current_insiders.insert(tx.insider.clone());
            }

            let unique_insiders = current_insiders.len();

            if baseline_weekly_vol.len() < min_baseline_weeks {
                return AnomalySignal {
                    ticker,
                    composite_score: 0.0,
                    volume_z_score: 0.0,
                    breadth_z_score: 0.0,
                    acceleration_score: 0.0,
                    is_anomaly: false,
                    current_dollar_vol,
                    current_shares_sold: current_shares,
                    unique_insiders,
                    baseline_mean: 0.0,
                    baseline_std: 0.0,
                };
            }

            // Volume z-score.
            let weekly_vols: Vec<f64> = baseline_weekly_vol.values().cloned().collect();
            let (mean_vol, std_vol) = mean_std(&weekly_vols);
            let std_safe = if std_vol <= 0.0 { 1e-9 } else { std_vol };
            let current_weekly_avg = current_dollar_vol / current_weeks_f;
            let volume_z = clamp_z((current_weekly_avg - mean_vol) / std_safe);

            // Breadth z-score.
            let weekly_breadth: Vec<f64> = baseline_weekly_insiders
                .values()
                .map(|s| s.len() as f64)
                .collect();
            let (mean_breadth, std_breadth) = mean_std(&weekly_breadth);
            let std_breadth_safe = if std_breadth <= 0.0 { 1e-9 } else { std_breadth };
            let current_breadth_per_week = (unique_insiders as f64) / current_weeks_f;
            let breadth_z = clamp_z((current_breadth_per_week - mean_breadth) / std_breadth_safe);

            // Acceleration.
            let baseline_freq_per_week = (baseline_weekly_vol.len() as f64) / baseline_weeks_f;
            let current_freq_per_week = if !current_txs.is_empty() {
                let mut cw_weeks: HashSet<String> = HashSet::new();
                for tx in &current_txs {
                    cw_weeks.insert(iso_week(tx.date));
                }
                (cw_weeks.len() as f64) / current_weeks_f
            } else {
                0.0
            };
            let accel = if baseline_freq_per_week > 0.0 {
                current_freq_per_week / baseline_freq_per_week
            } else {
                0.0
            };

            let composite =
                VOLUME_WEIGHT * volume_z + BREADTH_WEIGHT * breadth_z + ACCELERATION_WEIGHT * accel;

            AnomalySignal {
                ticker,
                composite_score: composite,
                volume_z_score: volume_z,
                breadth_z_score: breadth_z,
                acceleration_score: accel,
                is_anomaly: composite >= std_threshold && current_dollar_vol > 0.0,
                current_dollar_vol,
                current_shares_sold: current_shares,
                unique_insiders,
                baseline_mean: mean_vol,
                baseline_std: std_vol,
            }
        })
        .collect();

    results.sort_by(|a, b| {
        b.composite_score
            .partial_cmp(&a.composite_score)
            .unwrap_or(std::cmp::Ordering::Equal)
    });
    results
}

struct TxRecord {
    ticker: String,
    insider: String,
    date: NaiveDate,
    shares: f64,
    dollar_val: f64,
}

fn extract_tx_records(records: &[InsiderSellRecord]) -> Vec<TxRecord> {
    records
        .iter()
        .map(|r| {
            let dollar_val = r.value_usd.unwrap_or(r.shares_sold);
            TxRecord {
                ticker: r.ticker.to_uppercase(),
                insider: r.insider_name.clone().unwrap_or_default(),
                date: r.transaction_date,
                shares: r.shares_sold,
                dollar_val,
            }
        })
        .collect()
}

fn clamp_z(z: f64) -> f64 {
    z.clamp(-10.0, 10.0)
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

    fn make_record(ticker: &str, date: &str, shares: f64, insider: &str) -> InsiderSellRecord {
        InsiderSellRecord {
            ticker: ticker.to_string(),
            company_name: None,
            insider_name: Some(insider.to_string()),
            role: None,
            transaction_date: NaiveDate::parse_from_str(date, "%Y-%m-%d").unwrap(),
            filing_date: None,
            shares_sold: shares,
            value_usd: Some(shares * 150.0),
            source: "test".to_string(),
        }
    }

    #[test]
    fn test_anomaly_detection_with_weekly_buckets() {
        let records = vec![
            make_record("AAPL", "2023-03-10", 1000.0, "Alice"),
            make_record("AAPL", "2023-06-15", 500.0, "Bob"),
            make_record("AAPL", "2023-09-20", 800.0, "Alice"),
            make_record("AAPL", "2024-01-25", 50000.0, "Charlie"),
        ];
        let as_of = NaiveDate::parse_from_str("2024-02-02", "%Y-%m-%d").unwrap();
        let signals = compute_anomaly_signals(&records, 730, 30, 2.0, 3, as_of);
        assert!(!signals.is_empty(), "expected at least one signal");
        assert!(signals[0].composite_score > 0.0, "composite_score={}", signals[0].composite_score);
    }

    #[test]
    fn test_insufficient_baseline_returns_zero() {
        let records = vec![
            make_record("MSFT", "2024-01-25", 1000.0, "Alice"),
        ];
        let as_of = NaiveDate::parse_from_str("2024-02-02", "%Y-%m-%d").unwrap();
        let signals = compute_anomaly_signals(&records, 730, 30, 2.0, 3, as_of);
        assert!(!signals.is_empty());
        assert_eq!(signals[0].composite_score, 0.0);
    }
}
