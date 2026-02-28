package yahoo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
)

var ErrBadJSON = errors.New("yahoo: stdout is not valid JSON")

// scriptSem limits concurrent Python subprocess calls to avoid
// hammering yfinance when many dashboard requests arrive at once.
var scriptSem = make(chan struct{}, 4)

// runScript runs the yfinance Python script with a 20s timeout.
// Returns trimmed stdout bytes and error. Stderr is logged on failure
// but never mixed into the returned bytes.
func runScript(label string, args ...string) ([]byte, error) {
	script := yfinanceScriptPath()
	if script == "" {
		log.Printf("yahoo %s: script not found; set VIBES_YAHOO_SCRIPT or run from repo root", label)
		return nil, exec.ErrNotFound
	}
	// Acquire semaphore slot
	scriptSem <- struct{}{}
	defer func() { <-scriptSem }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, yfinancePython, append([]string{script}, args...)...)
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("yahoo %s failed: %v; stderr: %s", label, err, stderr.String())
		return nil, err
	}
	out := bytes.TrimSpace(stdout.Bytes())
	if len(out) == 0 || (out[0] != '[' && out[0] != '{') {
		preview := string(out)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		log.Printf("yahoo %s: stdout is not JSON: %s", label, preview)
		return nil, fmt.Errorf("%w: %s", ErrBadJSON, preview)
	}
	return out, nil
}

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
	return YfinanceScriptPath() != ""
}

// YfinanceScriptPath returns the path to yahoo_fetch.py, or empty if not found.
func YfinanceScriptPath() string {
	return yfinanceScriptPath()
}

// YfinancePythonPath returns the Python interpreter used for yfinance.
func YfinancePythonPath() string {
	return yfinancePython
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
	out, err := runScript("quotes", "quotes", "--symbols="+symStr)
	if err == nil {
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
	httpOut := make([]map[string]interface{}, 0)
	for _, r := range data.QuoteResponse.Result {
		sym, _ := r["symbol"].(string)
		price := getFloat(r, "regularMarketPrice", "previousClose")
		prev := getFloat(r, "previousClose")
		chgPct := 0.0
		if prev > 0 {
			chgPct = (price - prev) / prev * 100
		}
		httpOut = append(httpOut, map[string]interface{}{
			"symbol":            fromYahooSymbol(sym),
			"price":             price,
			"changesPercentage": chgPct,
		})
	}
	return httpOut
}

func (c *Client) GetHistoricalRange(ticker, fromDate, toDate string) []map[string]interface{} {
	if ticker == "" {
		return nil
	}
	ticker = toYahooSymbol(strings.TrimSpace(ticker))
	out, err := runScript("hist("+ticker+")", "hist", "--symbol="+ticker, "--from="+fromDate, "--to="+toDate)
	if err == nil {
		var data []map[string]interface{}
		if json.Unmarshal(out, &data) == nil {
			return data
		}
	}
	// Fallback: direct HTTP
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
	histOut := make([]map[string]interface{}, 0)
	for i, ts := range r.Timestamp {
		if i >= len(closes) {
			break
		}
		t := time.Unix(ts, 0)
		histOut = append(histOut, map[string]interface{}{
			"date":  t.Format("2006-01-02"),
			"close": closes[i],
		})
	}
	return histOut
}

func (c *Client) GetNews(ticker string, limit int) []map[string]interface{} {
	if ticker == "" {
		return nil
	}
	ticker = toYahooSymbol(strings.TrimSpace(ticker))
	out, err := runScript("news("+ticker+")", "news", "--symbol="+ticker, "--limit="+strconv.Itoa(min(10, limit)))
	if err == nil {
		var data []map[string]interface{}
		if json.Unmarshal(out, &data) == nil {
			return data
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
	newsOut := make([]map[string]interface{}, 0)
	for i, n := range data.News {
		if i >= limit {
			break
		}
		title := n.Title
		if len(title) > 80 {
			title = title[:80]
		}
		newsOut = append(newsOut, map[string]interface{}{
			"title": title,
			"url":   n.Link,
			"link":  n.Link,
		})
	}
	return newsOut
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

// ProviderHealth returns diagnostic info for /api/health/providers.
func ProviderHealth() map[string]interface{} {
	script := YfinanceScriptPath()
	py := YfinancePythonPath()
	if script == "" {
		return map[string]interface{}{
			"yfinance_script_path": "",
			"yfinance_python_path": py,
			"yfinance_import_ok":   false,
			"yahoo_quotes_ok":      false,
			"yahoo_hist_ok":        false,
			"hint":                 "Set VIBES_YAHOO_SCRIPT or run API from repo root so scripts/yahoo_fetch.py is found",
			"config": map[string]interface{}{
				"fmp_free_tier": config.FMPFreeTier,
				"has_fmp_key":   config.FMPAPIKey != "",
			},
		}
	}
	dir := filepath.Dir(filepath.Dir(script))

	// yfinance import check
	importOK := false
	imp := exec.Command(py, "-c", "import yfinance; print('ok')")
	imp.Dir = dir
	var impOut bytes.Buffer
	imp.Stdout = &impOut
	if imp.Run() == nil && strings.TrimSpace(impOut.String()) == "ok" {
		importOK = true
	}

	// quotes check (use runScript to avoid stderr mixing)
	quotesOK := false
	quotesOutput := ""
	outQuotes, errQuotes := runScript("health-quotes", "quotes", "--symbols=AAPL")
	quotesOK = errQuotes == nil
	quotesOutput = string(outQuotes)
	if len(quotesOutput) > 200 {
		quotesOutput = quotesOutput[:200] + "..."
	}

	// hist check
	histCloses := 0
	toDate := time.Now().Format("2006-01-02")
	fromDate := time.Now().AddDate(0, 0, -92).Format("2006-01-02")
	outHist, errHist := runScript("health-hist", "hist", "--symbol=AAPL", "--from="+fromDate, "--to="+toDate)
	if errHist == nil {
		var data []map[string]interface{}
		if json.Unmarshal(outHist, &data) == nil {
			histCloses = len(data)
		}
	}

	return map[string]interface{}{
		"yfinance_script_path": script,
		"yfinance_python_path": py,
		"yfinance_import_ok":   importOK,
		"yahoo_quotes_ok":     quotesOK,
		"yahoo_quotes_output": quotesOutput,
		"yahoo_hist_ok":       histCloses > 0,
		"yahoo_hist_closes":   histCloses,
		"config": map[string]interface{}{
			"fmp_free_tier": config.FMPFreeTier,
			"has_fmp_key":   config.FMPAPIKey != "",
		},
	}
}
