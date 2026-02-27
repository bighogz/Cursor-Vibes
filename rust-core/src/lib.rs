//! Vibes core: anomaly detection for insider selling.
//! Memory-safe, no heap allocations in hot path where possible.

mod anomaly;
mod models;

pub use anomaly::{compute_anomaly_signals, AnomalySignal};
pub use models::InsiderSellRecord;
