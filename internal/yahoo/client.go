package yahoo

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	yfc "github.com/wnjoon/go-yfinance/pkg/client"
	"github.com/wnjoon/go-yfinance/pkg/models"
	"github.com/wnjoon/go-yfinance/pkg/ticker"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
)

const yahooUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Shared go-yfinance client (handles TLS fingerprint + crumb/cookie auth).
var (
	sharedYFC    *yfc.Client
	yfcInitOnce  sync.Once
	yfcInitErr   error
	quoteSem     = make(chan struct{}, 8)
)

func ensureYFC() (*yfc.Client, error) {
	yfcInitOnce.Do(func() {
		sharedYFC, yfcInitErr = yfc.New()
		if yfcInitErr != nil {
			log.Printf("yahoo: go-yfinance client init failed: %v", yfcInitErr)
		}
	})
	return sharedYFC, yfcInitErr
}

func newTicker(sym string) (*ticker.Ticker, error) {
	c, err := ensureYFC()
	if err != nil {
		return nil, err
	}
	return ticker.New(sym, ticker.WithClient(c))
}

type Client struct{}

func New() *Client { return &Client{} }

// --- Symbol normalization ---

func toYahooSymbol(sym string) string {
	switch sym {
	case "BRK.B":
		return "BRK-B"
	case "BF.B":
		return "BF-B"
	default:
		return sym
	}
}

func fromYahooSymbol(sym string) string {
	switch sym {
	case "BRK-B":
		return "BRK.B"
	case "BF-B":
		return "BF.B"
	default:
		return sym
	}
}

func ToYahooSymbol(s string) string  { return toYahooSymbol(strings.TrimSpace(s)) }
func FromYahooSymbol(s string) string { return fromYahooSymbol(strings.TrimSpace(s)) }

// --- Quotes (batch via concurrent ticker.Quote) ---

func (c *Client) GetQuote(symbols []string) []map[string]interface{} {
	if len(symbols) == 0 {
		return nil
	}
	norm := make([]string, 0, len(symbols))
	for _, s := range symbols {
		if s = strings.TrimSpace(s); s != "" {
			norm = append(norm, toYahooSymbol(s))
		}
	}
	if len(norm) > 100 {
		norm = norm[:100]
	}

	if _, err := ensureYFC(); err == nil {
		results := c.batchQuoteYFC(norm)
		if len(results) > 0 {
			return results
		}
	}

	return c.quoteHTTPFallback(norm)
}

func (c *Client) batchQuoteYFC(symbols []string) []map[string]interface{} {
	type result struct {
		data map[string]interface{}
	}
	ch := make(chan result, len(symbols))
	var wg sync.WaitGroup

	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			quoteSem <- struct{}{}
			defer func() { <-quoteSem }()

			t, err := newTicker(s)
			if err != nil {
				return
			}
			defer t.Close()

			q, err := t.Quote()
			if err != nil {
				log.Printf("yahoo quote %s: %v", s, err)
				return
			}
			if q.RegularMarketPrice == 0 {
				return
			}
			ch <- result{data: map[string]interface{}{
				"symbol":            fromYahooSymbol(q.Symbol),
				"price":             q.RegularMarketPrice,
				"changesPercentage": q.RegularMarketChangePercent,
			}}
		}(sym)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	out := make([]map[string]interface{}, 0, len(symbols))
	for r := range ch {
		out = append(out, r.data)
	}
	return out
}

func (c *Client) quoteHTTPFallback(symbols []string) []map[string]interface{} {
	symStr := strings.Join(symbols, ",")
	req, err := http.NewRequest("GET", "https://query1.finance.yahoo.com/v7/finance/quote?symbols="+url.QueryEscape(symStr), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := httpclient.Default.Do(req)
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
			"symbol":            fromYahooSymbol(sym),
			"price":             price,
			"changesPercentage": chgPct,
		})
	}
	return out
}

// --- Historical data ---

func (c *Client) GetHistoricalRange(tickerSym, fromDate, toDate string) []map[string]interface{} {
	if tickerSym == "" {
		return nil
	}
	tickerSym = toYahooSymbol(strings.TrimSpace(tickerSym))

	if _, err := ensureYFC(); err == nil {
		if result := c.histYFC(tickerSym, fromDate, toDate); len(result) > 0 {
			return result
		}
	}

	return c.histHTTPFallback(tickerSym, fromDate, toDate)
}

func (c *Client) histYFC(sym, fromDate, toDate string) []map[string]interface{} {
	t, err := newTicker(sym)
	if err != nil {
		return nil
	}
	defer t.Close()

	start, err1 := time.Parse("2006-01-02", fromDate)
	end, err2 := time.Parse("2006-01-02", toDate)
	if err1 != nil || err2 != nil {
		return nil
	}

	bars, err := t.History(models.HistoryParams{
		Start:      &start,
		End:        &end,
		Interval:   "1d",
		AutoAdjust: true,
	})
	if err != nil {
		log.Printf("yahoo hist %s: %v", sym, err)
		return nil
	}

	out := make([]map[string]interface{}, 0, len(bars))
	for _, b := range bars {
		if b.Close > 0 {
			out = append(out, map[string]interface{}{
				"date":  b.Date.Format("2006-01-02"),
				"close": b.Close,
			})
		}
	}
	return out
}

func (c *Client) histHTTPFallback(sym, fromDate, toDate string) []map[string]interface{} {
	period1 := int64(0)
	period2 := int64(9999999999)
	if t, err := time.Parse("2006-01-02", fromDate); err == nil {
		period1 = t.Unix()
	}
	if t, err := time.Parse("2006-01-02", toDate); err == nil {
		period2 = t.Unix()
	}
	u := "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(sym) +
		"?interval=1d&period1=" + strconv.FormatInt(period1, 10) +
		"&period2=" + strconv.FormatInt(period2, 10)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var data struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
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

// --- News (HTTP only; Yahoo search doesn't need crumb) ---

func (c *Client) GetNews(tickerSym string, limit int) []map[string]interface{} {
	if tickerSym == "" {
		return nil
	}
	tickerSym = toYahooSymbol(strings.TrimSpace(tickerSym))
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	u := "https://query1.finance.yahoo.com/v1/finance/search?q=" +
		url.QueryEscape(tickerSym) + "&quotesCount=0&newsCount=" + strconv.Itoa(limit)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := httpclient.Default.Do(req)
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

// --- Helpers ---

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

// --- Provider Health ---

func ProviderHealth() map[string]interface{} {
	status := map[string]interface{}{
		"provider":        "go-yfinance (native Go)",
		"yfc_init_ok":     false,
		"yahoo_quotes_ok": false,
		"yahoo_hist_ok":   false,
		"config": map[string]interface{}{
			"fmp_free_tier": config.FMPFreeTier,
			"has_fmp_key":   config.FMPAPIKey != "",
		},
	}

	c, err := ensureYFC()
	if err != nil {
		status["yfc_init_error"] = err.Error()
		return status
	}
	status["yfc_init_ok"] = c != nil

	// Smoke test: single quote
	t, err := newTicker("AAPL")
	if err != nil {
		status["yahoo_quotes_error"] = err.Error()
		return status
	}
	defer t.Close()

	q, err := t.Quote()
	if err != nil {
		status["yahoo_quotes_error"] = err.Error()
	} else {
		status["yahoo_quotes_ok"] = q.RegularMarketPrice > 0
		status["yahoo_quotes_sample"] = map[string]interface{}{
			"symbol": q.Symbol,
			"price":  q.RegularMarketPrice,
		}
	}

	// Smoke test: historical
	start := time.Now().AddDate(0, 0, -30)
	end := time.Now()
	bars, err := t.HistoryRange(start, end, "1d")
	if err != nil {
		status["yahoo_hist_error"] = err.Error()
	} else {
		status["yahoo_hist_ok"] = len(bars) > 0
		status["yahoo_hist_bars"] = len(bars)
	}

	return status
}
