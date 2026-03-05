package eodhd

import (
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const baseURL = "https://eodhd.com/api"

type Client struct {
	APIKey string
}

func New() *Client {
	return &Client{APIKey: config.EODHDAPIKey}
}

// GetInsiderSells fetches bulk insider sell transactions from the EODHD
// Insider Transactions API. Calling without a ticker code returns recent
// transactions across all US companies. The results are filtered to only
// "Sale" transactions within the given date range and matching tickerFilter.
func (c *Client) GetInsiderSells(tickerFilter map[string]bool, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	if c.APIKey == "" {
		return nil
	}

	params := url.Values{}
	params.Set("api_token", c.APIKey)
	params.Set("fmt", "json")
	params.Set("limit", "1000")
	if !dateFrom.IsZero() {
		params.Set("from", dateFrom.Format("2006-01-02"))
	}
	if !dateTo.IsZero() {
		params.Set("to", dateTo.Format("2006-01-02"))
	}

	u := baseURL + "/insider-transactions?" + params.Encode()
	resp, err := httpclient.Default.Get(u)
	if err != nil {
		log.Printf("eodhd insider-transactions: request error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 401 {
		log.Printf("eodhd insider-transactions: status %d (requires All-In-One or Fundamentals plan)", resp.StatusCode)
		return nil
	}
	if resp.StatusCode != 200 {
		log.Printf("eodhd insider-transactions: status %d", resp.StatusCode)
		return nil
	}

	var raw []eodhRecord
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		log.Printf("eodhd insider-transactions: decode error: %v", err)
		return nil
	}

	var out []models.InsiderSellRecord
	for _, r := range raw {
		if !isSaleCode(r.TransactionCode) {
			continue
		}
		ticker := normalizeCode(r.Code)
		if ticker == "" {
			continue
		}
		if tickerFilter != nil && !tickerFilter[strings.ToUpper(ticker)] {
			continue
		}
		txDate, ok := parseDate(r.Date)
		if !ok {
			continue
		}
		if !dateFrom.IsZero() && txDate.Before(dateFrom) {
			continue
		}
		if !dateTo.IsZero() && txDate.After(dateTo) {
			continue
		}
		shares := r.TransactionAmount
		if shares <= 0 {
			continue
		}
		var valueUSD *float64
		if r.TransactionPrice > 0 {
			total := r.TransactionPrice * shares
			valueUSD = &total
		}
		var filingDate *time.Time
		if fd, ok := parseDate(r.ReportDate); ok {
			filingDate = &fd
		}
		var name *string
		if r.OwnerName != "" {
			name = &r.OwnerName
		}
		var role *string
		if r.OwnerRelationship != "" {
			role = &r.OwnerRelationship
		} else if r.OwnerTitle != "" {
			role = &r.OwnerTitle
		}
		var companyName *string
		if r.CompanyName != "" {
			companyName = &r.CompanyName
		}

		out = append(out, models.InsiderSellRecord{
			Ticker:          strings.ToUpper(ticker),
			CompanyName:     companyName,
			InsiderName:     name,
			Role:            role,
			TransactionDate: txDate,
			FilingDate:      filingDate,
			SharesSold:      shares,
			ValueUSD:        valueUSD,
			Source:          "eodhd",
		})
	}
	log.Printf("eodhd insider-transactions: raw=%d sells=%d", len(raw), len(out))
	return out
}

type eodhRecord struct {
	Code              string  `json:"code"`
	Date              string  `json:"date"`
	OwnerCik          string  `json:"ownerCik"`
	OwnerName         string  `json:"ownerName"`
	OwnerRelationship string  `json:"ownerRelationship"`
	OwnerTitle        string  `json:"ownerTitle"`
	TransactionDate   string  `json:"transactionDate"`
	TransactionCode   string  `json:"transactionCode"`
	TransactionAmount float64 `json:"transactionAmount"`
	TransactionPrice  float64 `json:"transactionPrice"`
	PostTransAmount   float64 `json:"transactionAcquiredDisposed"`
	ReportDate        string  `json:"reportDate"`
	SecLink           string  `json:"secLink"`
	CompanyName       string  `json:"companyName"`
}

func isSaleCode(code string) bool {
	c := strings.ToUpper(strings.TrimSpace(code))
	return c == "S" || c == "S-SALE" || c == "D"
}

// normalizeCode strips the ".US" exchange suffix that EODHD appends.
func normalizeCode(code string) string {
	code = strings.TrimSpace(code)
	if idx := strings.LastIndex(code, "."); idx > 0 {
		return code[:idx]
	}
	return code
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, s[:min(len(layout), len(s))]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

