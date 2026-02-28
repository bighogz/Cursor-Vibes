package fmp

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const baseURL = "https://financialmodelingprep.com/stable"

type Client struct {
	APIKey string
}

func New() *Client {
	return &Client{APIKey: config.FMPAPIKey}
}

func (c *Client) get(path string, params url.Values) (interface{}, error) {
	if c.APIKey == "" {
		return nil, nil
	}
	params.Set("apikey", c.APIKey)
	u := baseURL + path + "?" + params.Encode()
	resp, err := httpclient.Default.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return map[string]interface{}{"_error": "rate_limit"}, nil
	}
	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if m, ok := data.(map[string]interface{}); ok {
		if _, hasErr := m["_error"]; hasErr {
			return m, nil
		}
		if msg, ok := m["Error Message"].(string); ok && msg != "" {
			return map[string]interface{}{"_error": msg}, nil
		}
	}
	return data, nil
}

func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	s = s[:min(10, len(s))]
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (c *Client) GetSP500Tickers() []string {
	data, err := c.get("/sp500-constituent", url.Values{})
	if err == nil {
		if arr, ok := data.([]interface{}); ok && len(arr) > 0 {
			out := make([]string, 0)
			for _, v := range arr {
				if m, ok := v.(map[string]interface{}); ok {
					if sym, ok := m["symbol"].(string); ok && sym != "" {
						out = append(out, strings.TrimSpace(sym))
					}
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	// Fallback: free CSV
	resp, err := httpclient.Default.Get("https://raw.githubusercontent.com/datasets/s-and-p-500-companies/master/data/constituents.csv")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	reader := csv.NewReader(resp.Body)
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil
	}
	headers := rows[0]
	symIdx := -1
	for i, h := range headers {
		if strings.EqualFold(h, "Symbol") {
			symIdx = i
			break
		}
	}
	if symIdx < 0 {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, row := range rows[1:] {
		if symIdx >= len(row) {
			continue
		}
		sym := strings.TrimSpace(row[symIdx])
		if sym != "" && !seen[sym] {
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out
}

func (c *Client) GetInsiderSells(ticker string, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	params := url.Values{}
	if ticker != "" {
		params.Set("symbol", ticker)
	}
	params.Set("page", "0")
	params.Set("limit", "100")
	path := "/insider-trading/latest"
	if ticker != "" {
		path = "/insider-trading/search"
	}
	data, err := c.get(path, params)
	if err != nil || data == nil {
		return nil
	}
	var items []interface{}
	switch v := data.(type) {
	case []interface{}:
		items = v
	case map[string]interface{}:
		if _, hasErr := v["_error"]; hasErr {
			return nil
		}
		if d, ok := v["data"].([]interface{}); ok {
			items = d
		}
		if len(items) == 0 {
			if d, ok := v["insider_trading"].([]interface{}); ok {
				items = d
			}
		}
	}
	if len(items) == 0 {
		return nil
	}
	records := make([]models.InsiderSellRecord, 0)
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		transType := strings.ToUpper(str(m["transactionType"]) + str(m["type"]))
		acqDisp := strings.ToUpper(str(m["acquisitionOrDisposition"]) + str(m["acquiredDisposedCode"]))
		isSell := transType == "S" || transType == "D" || acqDisp == "D" || strings.Contains(strings.ToLower(str(m["transactionType"])), "sale")
		if !isSell {
			continue
		}
		tickerSym := strings.TrimSpace(strOr(m["symbol"], m["ticker"]) + ticker)
		if tickerSym == "" {
			continue
		}
		txDate, ok := parseDate(strOr(m["transactionDate"], m["periodOfReport"], m["filingDate"]))
		if !ok {
			continue
		}
		if !dateFrom.IsZero() && txDate.Before(dateFrom) {
			continue
		}
		if !dateTo.IsZero() && txDate.After(dateTo) {
			continue
		}
		shares := toFloat(m["numberOfShares"], m["shares"])
		if shares <= 0 {
			continue
		}
		var valueUSD *float64
		if v := toFloat(m["value"], m["valueUsd"]); v > 0 {
			valueUSD = &v
		}
		var filingDate *time.Time
		if fd, ok := parseDate(strOr(m["filingDate"], m["filedAt"])); ok {
			filingDate = &fd
		}
		var companyName, insiderName, role *string
		if s := strOr(m["companyName"]); s != "" {
			companyName = &s
		}
		if s := strOr(m["reportingName"], m["reportingOwner"]); s != "" {
			insiderName = &s
		}
		if s := strOr(m["typeOfOwner"]); s != "" {
			role = &s
		}
		records = append(records, models.InsiderSellRecord{
			Ticker:          tickerSym,
			CompanyName:     companyName,
			InsiderName:     insiderName,
			Role:            role,
			TransactionDate: txDate,
			FilingDate:      filingDate,
			SharesSold:      shares,
			ValueUSD:        valueUSD,
			Source:          "fmp",
		})
	}
	return records
}

func (c *Client) GetQuote(symbols []string) []map[string]interface{} {
	if c.APIKey == "" || len(symbols) == 0 {
		return nil
	}
	limit := min(100, len(symbols))
	syms := make([]string, limit)
	for i := 0; i < limit; i++ {
		s := symbols[i]
		if len(s) > 10 {
			s = s[:10]
		}
		syms[i] = s
	}
	params := url.Values{}
	params.Set("symbol", strings.Join(syms, ","))
	data, err := c.get("/quote", params)
	if err != nil {
		return nil
	}
	if m, ok := data.(map[string]interface{}); ok {
		if _, hasErr := m["_error"]; hasErr {
			return nil
		}
	}
	if arr, ok := data.([]interface{}); ok {
		out := make([]map[string]interface{}, 0)
		for _, v := range arr {
			if m, ok := v.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func (c *Client) GetNews(ticker string, limit int) []map[string]interface{} {
	if c.APIKey == "" || ticker == "" {
		return nil
	}
	params := url.Values{}
	params.Set("symbol", ticker)
	params.Set("limit", fmt.Sprintf("%d", limit))
	data, err := c.get("/stock-news", params)
	if err != nil {
		return nil
	}
	if m, ok := data.(map[string]interface{}); ok {
		if _, hasErr := m["_error"]; hasErr {
			return nil
		}
	}
	if arr, ok := data.([]interface{}); ok {
		out := make([]map[string]interface{}, 0)
		for i, v := range arr {
			if i >= limit {
				break
			}
			if m, ok := v.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func (c *Client) GetHistoricalRange(ticker, fromDate, toDate string) []map[string]interface{} {
	if c.APIKey == "" || ticker == "" {
		return nil
	}
	params := url.Values{}
	params.Set("symbol", ticker)
	params.Set("from", fromDate)
	params.Set("to", toDate)
	data, err := c.get("/historical-price-eod/full", params)
	if err != nil {
		return nil
	}
	if m, ok := data.(map[string]interface{}); ok {
		if _, hasErr := m["_error"]; hasErr {
			return nil
		}
		if hist, ok := m["historical"].([]interface{}); ok {
			out := make([]map[string]interface{}, 0)
			for _, v := range hist {
				if h, ok := v.(map[string]interface{}); ok {
					out = append(out, h)
				}
			}
			return out
		}
	}
	if arr, ok := data.([]interface{}); ok {
		out := make([]map[string]interface{}, 0)
		for _, v := range arr {
			if m, ok := v.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if m, ok := v.(map[string]interface{}); ok {
		if n, ok := m["name"].(string); ok {
			return n
		}
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func strOr(vals ...interface{}) string {
	for _, v := range vals {
		if s := str(v); s != "" {
			return s
		}
	}
	return ""
}

func toFloat(vals ...interface{}) float64 {
	for _, v := range vals {
		if v == nil {
			continue
		}
		switch x := v.(type) {
		case float64:
			return x
		case int:
			return float64(x)
		case string:
			f, _ := strconv.ParseFloat(x, 64)
			return f
		}
	}
	return 0
}
