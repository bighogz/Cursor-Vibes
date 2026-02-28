package yahoo

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
)

// User-Agent required: Yahoo blocks generic clients (401/429)
const yahooUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const quoteURL = "https://query1.finance.yahoo.com/v7/finance/quote"
const chartURL = "https://query1.finance.yahoo.com/v8/finance/chart"

type Client struct{}

func New() *Client {
	return &Client{}
}

var yfinancePython string

func init() {
	// Prefer venv Python (has yfinance)
	if p := os.Getenv("VIBES_YAHOO_PYTHON"); p != "" {
		yfinancePython = p
		return
	}
	cwd, _ := os.Getwd()
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	for _, base := range []string{cwd, execDir, filepath.Join(execDir, "..")} {
		venv := filepath.Join(base, ".venv", "bin", "python3")
		if _, err := os.Stat(venv); err == nil {
			yfinancePython = venv
			return
		}
	}
	yfinancePython = "python3"
}

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

// ToYahooSymbol converts S&P 500 symbols to Yahoo format: BRK.B -> BRK-B
func ToYahooSymbol(s string) string { return toYahooSymbol(strings.TrimSpace(s)) }

// FromYahooSymbol converts Yahoo symbols back: BRK-B -> BRK.B
func FromYahooSymbol(s string) string { return fromYahooSymbol(strings.TrimSpace(s)) }

// YfinanceAvailable returns true if the yfinance script can be used (preferred over HTTP).
func YfinanceAvailable() bool {
	return yfinanceScriptPath() != ""
}

func yfinanceScriptPath() string {
	if p := os.Getenv("VIBES_YAHOO_SCRIPT"); p != "" {
		return p
	}
	cwd, _ := os.Getwd()
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	for _, base := range []string{cwd, execDir, filepath.Join(execDir, "..")} {
		p := filepath.Join(base, "scripts", "yahoo_fetch.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (c *Client) GetQuote(symbols []string) []map[string]interface{} {
	if len(symbols) == 0 {
		return nil
	}
	norm := make([]string, 0, len(symbols))
	for _, s := range symbols[:min(100, len(symbols))] {
		norm = append(norm, toYahooSymbol(strings.TrimSpace(s)))
	}
	symStr := strings.Join(norm, ",")
	// Prefer yfinance (works when Yahoo HTTP API returns 401)
	if script := yfinanceScriptPath(); script != "" {
		cmd := exec.Command(yfinancePython, script, "quotes", "--symbols="+symStr)
		cmd.Dir = filepath.Dir(filepath.Dir(script))
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("yahoo quotes subprocess failed: %v; stderr/stdout: %s", err, string(out))
		} else {
			var data []map[string]interface{}
			if json.Unmarshal(out, &data) == nil && len(data) > 0 {
				for _, q := range data {
					if sym, ok := q["symbol"].(string); ok && sym != "" {
						q["symbol"] = fromYahooSymbol(sym)
					}
				}
				return data
			}
		}
	}
	// Fallback: direct HTTP (may return 401)
	req, err := http.NewRequest("GET", quoteURL+"?symbols="+url.QueryEscape(symStr), nil)
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

func (c *Client) GetHistoricalRange(ticker, fromDate, toDate string) []map[string]interface{} {
	if ticker == "" {
		return nil
	}
	ticker = toYahooSymbol(strings.TrimSpace(ticker))
	// Prefer yfinance
	if script := yfinanceScriptPath(); script != "" {
		cmd := exec.Command(yfinancePython, script, "hist", "--symbol="+ticker, "--from="+fromDate, "--to="+toDate)
		cmd.Dir = filepath.Dir(filepath.Dir(script))
		out, err := cmd.Output()
		if err == nil {
			var data []map[string]interface{}
			if json.Unmarshal(out, &data) == nil {
				return data
			}
		}
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
	ticker = toYahooSymbol(strings.TrimSpace(ticker))
	// Prefer yfinance
	if script := yfinanceScriptPath(); script != "" {
		cmd := exec.Command(yfinancePython, script, "news", "--symbol="+ticker, "--limit="+strconv.Itoa(min(10, limit)))
		cmd.Dir = filepath.Dir(filepath.Dir(script))
		out, err := cmd.Output()
		if err == nil {
			var data []map[string]interface{}
			if json.Unmarshal(out, &data) == nil {
				return data
			}
		}
	}
	u := "https://query1.finance.yahoo.com/v1/finance/search?q=" + url.QueryEscape(ticker) + "&quotesCount=0&newsCount=" + strconv.Itoa(min(10, limit))
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
