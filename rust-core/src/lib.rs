//! Vibes core: anomaly detection and financial trend utilities.

mod anomaly;
mod models;
mod trend;

pub use anomaly::{compute_anomaly_signals, AnomalySignal};
pub use models::InsiderSellRecord;
pub use trend::{quarterly_trend, QuarterlyTrend};
