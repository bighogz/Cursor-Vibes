package yahoo

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const quoteURL = "https://query1.finance.yahoo.com/v7/finance/quote"
const chartURL = "https://query1.finance.yahoo.com/v8/finance/chart"

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) GetQuote(symbols []string) []map[string]interface{} {
	if len(symbols) == 0 {
		return nil
	}
	symStr := strings.Join(symbols[:min(100, len(symbols))], ",")
	u := quoteURL + "?symbols=" + url.QueryEscape(symStr)
	resp, err := http.Get(u)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var data struct {
		QuoteResponse struct {
			Result []map[string]interface{} `json:"result"`
		} `json:"quoteResponse"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0)
	for _, r := range data.QuoteResponse.Result {
		sym, _ := r["symbol"].(string)
		price := getFloat(r, "regularMarketPrice", "previousClose")
		prev := getFloat(r, "previousClose")
		chgPct := 0.0
		if prev > 0 {
			chgPct = (price - prev) / prev * 100
		}
		out = append(out, map[string]interface{}{
			"symbol":            sym,
			"price":             price,
			"changesPercentage": chgPct,
		})
	}
	return out
}

func (c *Client) GetHistoricalRange(ticker, fromDate, toDate string) []map[string]interface{} {
	if ticker == "" {
		return nil
	}
	period1 := int64(0)
	period2 := int64(9999999999)
	if t, err := time.Parse("2006-01-02", fromDate); err == nil {
		period1 = t.Unix()
	}
	if t, err := time.Parse("2006-01-02", toDate); err == nil {
		period2 = t.Unix()
	}
	u := chartURL + "/" + url.PathEscape(ticker) + "?interval=1d&period1=" + strconv.FormatInt(period1, 10) + "&period2=" + strconv.FormatInt(period2, 10)
	resp, err := http.Get(u)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var data struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64   `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Close []float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	if len(data.Chart.Result) == 0 {
		return nil
	}
	r := data.Chart.Result[0]
	if len(r.Indicators.Quote) == 0 {
		return nil
	}
	closes := r.Indicators.Quote[0].Close
	out := make([]map[string]interface{}, 0)
	for i, ts := range r.Timestamp {
		if i >= len(closes) {
			break
		}
		t := time.Unix(ts, 0)
		out = append(out, map[string]interface{}{
			"date":  t.Format("2006-01-02"),
			"close": closes[i],
		})
	}
	return out
}

func (c *Client) GetNews(ticker string, limit int) []map[string]interface{} {
	if ticker == "" {
		return nil
	}
	u := "https://query1.finance.yahoo.com/v1/finance/search?q=" + url.QueryEscape(ticker) + "&quotesCount=0&newsCount=" + strconv.Itoa(min(10, limit))
	resp, err := http.Get(u)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var data struct {
		News []struct {
			Title string `json:"title"`
			Link  string `json:"link"`
		} `json:"news"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0)
	for i, n := range data.News {
		if i >= limit {
			break
		}
		title := n.Title
		if len(title) > 80 {
			title = title[:80]
		}
		out = append(out, map[string]interface{}{
			"title": title,
			"url":   n.Link,
			"link":  n.Link,
		})
	}
	return out
}

func getFloat(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case float64:
				return x
			case int:
				return float64(x)
			}
		}
	}
	return 0
}
