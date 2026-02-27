//! CLI: stdin JSON -> stdout JSON. Used by Go for anomaly computation.
use chrono::NaiveDate;
use serde::{Deserialize, Serialize};
use std::io;
use vibes_core::{compute_anomaly_signals, AnomalySignal, InsiderSellRecord};

#[derive(Debug, Deserialize)]
struct Input {
    records: Vec<InsiderSellRecord>,
    params: Params,
}

#[derive(Debug, Deserialize)]
struct Params {
    baseline_days: i64,
    current_days: i64,
    std_threshold: f64,
    min_baseline_points: usize,
    as_of: String,
}

#[derive(Debug, Serialize)]
struct Output {
    signals: Vec<AnomalySignal>,
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let input: Input = serde_json::from_reader(io::stdin())?;
    let as_of = NaiveDate::parse_from_str(&input.params.as_of[..input.params.as_of.len().min(10)], "%Y-%m-%d")?;
    let signals = compute_anomaly_signals(
        &input.records,
        input.params.baseline_days,
        input.params.current_days,
        input.params.std_threshold,
        input.params.min_baseline_points,
        as_of,
    );
    serde_json::to_writer(io::stdout(), &Output { signals })?;
    Ok(())
}
