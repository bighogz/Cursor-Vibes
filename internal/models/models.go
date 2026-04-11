package models

import "time"

type InsiderSellRecord struct {
	Ticker          string     `json:"ticker"`
	CompanyName     *string    `json:"company_name,omitempty"`
	InsiderName     *string    `json:"insider_name,omitempty"`
	Role            *string    `json:"role,omitempty"`
	TransactionDate time.Time  `json:"transaction_date"`
	FilingDate      *time.Time `json:"filing_date,omitempty"`
	SharesSold      float64    `json:"shares_sold"`
	ValueUSD        *float64   `json:"value_usd,omitempty"`
	TxType          string     `json:"tx_type,omitempty"`
	Source          string     `json:"source"`
}

// --- Typed dashboard domain ---

// DashboardResult is the top-level response from dashboard.Build().
// Replaces the previous map[string]interface{} to enforce structure at
// compile time and make the Go↔JSON boundary explicit.
type DashboardResult struct {
	AsOf             string            `json:"as_of"`
	TotalCompanies   int               `json:"total_companies"`
	Sectors          []SectorGroup     `json:"sectors"`
	AvailableSectors []string          `json:"available_sectors"`
	ProviderStatus   map[string]string `json:"provider_status,omitempty"`
	Error            string            `json:"error,omitempty"`
}

// SectorGroup contains all companies within a GICS sector.
type SectorGroup struct {
	Name      string    `json:"name"`
	Companies []Company `json:"companies"`
}

// TrendPeriod holds a return percentage and sparkline closes for one time horizon.
type TrendPeriod struct {
	Pct    float64   `json:"pct"`
	Closes []float64 `json:"closes"`
}

// Company is a single S&P 500 constituent with all enriched data.
type Company struct {
	Symbol            string                  `json:"symbol"`
	Name              string                  `json:"name"`
	Price             *float64                `json:"price"`
	ChangePct         *float64                `json:"change_pct"`
	QuarterTrend      *float64                `json:"quarter_trend"`
	QuarterCloses     []float64               `json:"quarter_closes"`
	Trends            map[string]*TrendPeriod `json:"trends,omitempty"`
	News              []NewsItem              `json:"news"`
	TopInsiders       []InsiderEntry          `json:"top_insiders"`
	Sources           map[string]string       `json:"sources"`
	AnomalyScore      *float64                `json:"anomaly_score,omitempty"`
	VolumeZScore      *float64                `json:"volume_z_score,omitempty"`
	BreadthZScore     *float64                `json:"breadth_z_score,omitempty"`
	AccelerationScore *float64                `json:"acceleration_score,omitempty"`
	UniqueInsiders    *int                    `json:"unique_insiders,omitempty"`
}

// NewsItem is a headline with URL.
type NewsItem struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// InsiderEntry is an insider transaction displayed on the dashboard.
type InsiderEntry struct {
	Name   string   `json:"name"`
	Role   *string  `json:"role,omitempty"`
	Shares float64  `json:"shares"`
	Value  *float64 `json:"value,omitempty"`
	TxType string   `json:"tx_type"`
	Source string   `json:"source"`
}
