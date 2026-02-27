use chrono::NaiveDate;
use serde::{Deserialize, Deserializer, Serialize};

fn deserialize_naive_date<'de, D>(d: D) -> Result<NaiveDate, D::Error>
where
    D: Deserializer<'de>,
{
    let s = String::deserialize(d)?;
    NaiveDate::parse_from_str(&s[..s.len().min(10)], "%Y-%m-%d")
        .map_err(serde::de::Error::custom)
}

fn deserialize_opt_naive_date<'de, D>(d: D) -> Result<Option<NaiveDate>, D::Error>
where
    D: Deserializer<'de>,
{
    let opt: Option<String> = Option::deserialize(d)?;
    Ok(opt.and_then(|s| {
        NaiveDate::parse_from_str(&s[..s.len().min(10)], "%Y-%m-%d").ok()
    }))
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InsiderSellRecord {
    pub ticker: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub company_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub insider_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
    #[serde(deserialize_with = "deserialize_naive_date")]
    pub transaction_date: NaiveDate,
    #[serde(default, skip_serializing_if = "Option::is_none", deserialize_with = "deserialize_opt_naive_date")]
    pub filing_date: Option<NaiveDate>,
    pub shares_sold: f64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub value_usd: Option<f64>,
    pub source: String,
}
