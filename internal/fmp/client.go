package fmp

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
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

// GetInsiderSells fetches insider sell transactions. Uses /insider-trading/latest
// (free-tier compatible) rather than /insider-trading/search (paid only).
// When tickerFilter is non-nil, only records matching those tickers are returned.
// GetInsiderSells fetches recent insider sells from /insider-trading/latest
// (1 page on free tier), merges with a local cache file to accumulate data
// over multiple runs, and returns all records matching the ticker filter.
func (c *Client) GetInsiderSells(tickerFilter map[string]bool, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	freshRecords := c.fetchLatestInsiderPage(tickerFilter, dateFrom, dateTo)
	cached := loadInsiderCache()
	merged := mergeInsiderRecords(cached, freshRecords)
	if len(freshRecords) > 0 {
		saveInsiderCache(merged)
	}
	// Filter merged set by ticker and date
	var out []models.InsiderSellRecord
	for _, r := range merged {
		t := strings.ToUpper(r.Ticker)
		if tickerFilter != nil && !tickerFilter[t] {
			continue
		}
		if !dateFrom.IsZero() && r.TransactionDate.Before(dateFrom) {
			continue
		}
		if !dateTo.IsZero() && r.TransactionDate.After(dateTo) {
			continue
		}
		out = append(out, r)
	}
	log.Printf("fmp insiders: fresh=%d cached=%d merged=%d filtered=%d", len(freshRecords), len(cached), len(merged), len(out))
	return out
}

func (c *Client) fetchLatestInsiderPage(tickerFilter map[string]bool, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	params := url.Values{}
	params.Set("page", "0")
	params.Set("limit", "100")
	params.Set("transactionType", "S-Sale")
	data, err := c.get("/insider-trading/latest", params)
	if err != nil || data == nil {
		return nil
	}
	return parseInsiderRecords(data, nil, dateFrom, dateTo)
}

const insiderCachePath = "data/insider_cache.json"

func loadInsiderCache() []models.InsiderSellRecord {
	f, err := os.Open(insiderCachePath)
	if err != nil {
		return nil
	}
	defer f.Close()
	var records []models.InsiderSellRecord
	json.NewDecoder(f).Decode(&records)
	return records
}

func saveInsiderCache(records []models.InsiderSellRecord) {
	os.MkdirAll("data", 0700)
	f, err := os.OpenFile(insiderCachePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(records)
}

func mergeInsiderRecords(existing, fresh []models.InsiderSellRecord) []models.InsiderSellRecord {
	seen := make(map[string]bool)
	var out []models.InsiderSellRecord
	add := func(r models.InsiderSellRecord) {
		key := strings.ToUpper(r.Ticker) + "|" + r.TransactionDate.Format("2006-01-02") + "|" + fmt.Sprintf("%.0f", r.SharesSold)
		if n := r.InsiderName; n != nil {
			key += "|" + *n
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
	}
	for _, r := range existing {
		add(r)
	}
	for _, r := range fresh {
		add(r)
	}
	return out
}

func parseInsiderRecords(data interface{}, tickerFilter map[string]bool, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
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
		acqDisp := strings.ToUpper(str(m["acquisitionOrDisposition"]) + str(m["acquiredDisposedCode"]))
		transType := strings.ToLower(str(m["transactionType"]))
		isSell := acqDisp == "D" || strings.Contains(transType, "sale") || strings.Contains(transType, "s-sale") || strings.HasPrefix(transType, "s") || acqDisp == "DD"
		if !isSell {
			continue
		}
		tickerSym := strings.TrimSpace(strings.ToUpper(strOr(m["symbol"], m["ticker"])))
		if tickerSym == "" {
			continue
		}
		if tickerFilter != nil && !tickerFilter[tickerSym] {
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
		shares := toFloat(m["securitiesTransacted"], m["numberOfShares"], m["shares"])
		if shares <= 0 {
			continue
		}
		var valueUSD *float64
		if v := toFloat(m["price"]); v > 0 {
			total := v * shares
			valueUSD = &total
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
