//! CLI: stdin JSON -> stdout JSON. Used by Go for anomaly + trend computation.
//!
//! Usage:
//!   echo '{"records":[...], "params":{...}}' | vibes-anomaly anomaly
//!   echo '{"closes":[100.0, 101.5, ...]}' | vibes-anomaly trend
use chrono::NaiveDate;
use serde::{Deserialize, Serialize};
use std::{env, io};
use vibes_core::{compute_anomaly_signals, quarterly_trend, AnomalySignal, InsiderSellRecord, QuarterlyTrend};

// --- Anomaly structs ---

#[derive(Debug, Deserialize)]
struct AnomalyInput {
    records: Vec<InsiderSellRecord>,
    params: AnomalyParams,
}

#[derive(Debug, Deserialize)]
struct AnomalyParams {
    baseline_days: i64,
    current_days: i64,
    std_threshold: f64,
    min_baseline_points: usize,
    as_of: String,
}

#[derive(Debug, Serialize)]
struct AnomalyOutput {
    signals: Vec<AnomalySignal>,
}

// --- Trend structs ---

#[derive(Debug, Deserialize)]
struct TrendInput {
    closes: Vec<f64>,
}

#[derive(Debug, Serialize)]
struct TrendOutput {
    trend: Option<QuarterlyTrend>,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = env::args().collect();
    let cmd = args.get(1).map(|s| s.as_str()).unwrap_or("anomaly");

    match cmd {
        "trend" => {
            let input: TrendInput = serde_json::from_reader(io::stdin())?;
            let trend = quarterly_trend(&input.closes);
            serde_json::to_writer(io::stdout(), &TrendOutput { trend })?;
        }
        _ => {
            let input: AnomalyInput = serde_json::from_reader(io::stdin())?;
            let as_of = NaiveDate::parse_from_str(
                &input.params.as_of[..input.params.as_of.len().min(10)],
                "%Y-%m-%d",
            )?;
            let signals = compute_anomaly_signals(
                &input.records,
                input.params.baseline_days,
                input.params.current_days,
                input.params.std_threshold,
                input.params.min_baseline_points,
                as_of,
            );
            serde_json::to_writer(io::stdout(), &AnomalyOutput { signals })?;
        }
    }
    Ok(())
}
