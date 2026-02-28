//! Quarterly trend: q_return and linear regression slope from close prices.

use serde::Serialize;

/// Quarterly trend result: percent change and $/day slope.
#[derive(Debug, Clone, Serialize)]
pub struct QuarterlyTrend {
    pub quarter_pct: f64,
    pub q_return: f64,
    pub slope: f64,
    pub last: f64,
}

/// Compute quarterly return and slope from close prices (oldest to newest).
/// Uses ~63 trading day lookback when available, else half the data.
/// Returns None if insufficient data (< 30 points).
pub fn quarterly_trend(closes: &[f64]) -> Option<QuarterlyTrend> {
    let closes: Vec<f64> = closes.iter().copied().filter(|&c| c > 0.0).collect();
    if closes.len() < 30 {
        return None;
    }
    let last = *closes.last().unwrap();
    let lookback = if closes.len() > 63 { 63 } else { closes.len() / 2 };
    let lookback = lookback.max(1);
    let prev = closes[closes.len() - lookback];
    if prev <= 0.0 {
        return None;
    }
    let q_return = (last / prev) - 1.0;
    let quarter_pct = q_return * 100.0;

    let slope = linear_slope(&closes[closes.len() - lookback..]);

    Some(QuarterlyTrend {
        quarter_pct,
        q_return,
        slope,
        last,
    })
}

/// Simple linear regression slope (Ordinary Least Squares).
fn linear_slope(y: &[f64]) -> f64 {
    let n = y.len() as f64;
    if n < 2.0 {
        return 0.0;
    }
    let x_mean = (n - 1.0) / 2.0;
    let y_mean: f64 = y.iter().sum::<f64>() / n;
    let mut num = 0.0;
    let mut den = 0.0;
    for (i, &yi) in y.iter().enumerate() {
        let xi = i as f64;
        num += (xi - x_mean) * (yi - y_mean);
        den += (xi - x_mean) * (xi - x_mean);
    }
    if den.abs() < 1e-12 {
        return 0.0;
    }
    num / den
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_quarterly_trend() {
        let closes: Vec<f64> = (0..65).map(|i| 100.0 + i as f64 * 0.5).collect();
        let t = quarterly_trend(&closes).unwrap();
        assert!(t.quarter_pct > 0.0);
        assert!(t.slope > 0.0);
    }
}
