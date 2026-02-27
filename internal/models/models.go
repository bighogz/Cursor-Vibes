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
	Source          string     `json:"source"`
}
