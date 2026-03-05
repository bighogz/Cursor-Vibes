package dashboard

import (
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/aggregator"
	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/models"
	"github.com/bighogz/Cursor-Vibes/internal/rustclient"
	"github.com/bighogz/Cursor-Vibes/internal/sp500"
	"github.com/bighogz/Cursor-Vibes/internal/trend"
	"github.com/bighogz/Cursor-Vibes/internal/yahoo"
)

// BuildOpts controls the dashboard build. When Sector/Limit are empty,
// Build produces the full S&P 500 dataset (used by the background refresher).
type BuildOpts struct {
	Sector string
	Limit  int
	AsOf   time.Time
}

func Build(opts BuildOpts) map[string]interface{} {
	allCompanies := sp500.Load()
	if len(allCompanies) == 0 {
		return map[string]interface{}{"error": "Could not load S&P 500", "sectors": []interface{}{}}
	}
	seen := make(map[string]bool)
	availableSectors := make([]string, 0)
	for _, c := range allCompanies {
		s := c.Sector
		if s == "" {
			s = "Unknown"
		}
		if !seen[s] {
			seen[s] = true
			availableSectors = append(availableSectors, s)
		}
	}
	sort.Strings(availableSectors)
	companies := allCompanies
	if opts.Sector != "" {
		filtered := make([]sp500.Company, 0)
		for _, c := range companies {
			if strings.EqualFold(c.Sector, opts.Sector) {
				filtered = append(filtered, c)
			}
		}
		companies = filtered
	}
	if opts.Limit > 0 {
		companies = companies[:min(opts.Limit, len(companies))]
	}
	asOf := opts.AsOf
	tickers := make([]string, len(companies))
	for i, c := range companies {
		tickers[i] = c.Symbol
	}

	totalDays := 365 + 30
	dateFrom := asOf.AddDate(0, 0, -totalDays)
	dateTo := asOf

	// Trend/news fetches go through Yahoo (free) and are rate-limited by the
	// semaphore, not by FMP API budget. For the background full build we fetch
	// all companies so every sector has data. Build time ~2-3 min is fine for
	// the 30-min refresh cycle.
	trendNewsLimit := len(companies)
	quoteBySym := make(map[string]map[string]interface{})
	yahooClient := yahoo.New()
	fmpClient := fmp.New()
	providerStatus := make(map[string]string)

	useYahooForQuotes := config.FMPAPIKey == "" || config.FMPFreeTier
	batchSize := 100

	for i := 0; i < len(tickers); i += batchSize {
		batch := tickers[i:min(i+batchSize, len(tickers))]
		var quotes []map[string]interface{}
		if !useYahooForQuotes && config.FMPAPIKey != "" {
			quotes = fmpClient.GetQuote(batch)
			if len(quotes) == 0 {
				useYahooForQuotes = true
				providerStatus["fmp"] = "rate_limited"
				quotes = yahooClient.GetQuote(batch)
			}
		} else {
			quotes = yahooClient.GetQuote(batch)
		}
		for _, q := range quotes {
			if sym, ok := q["symbol"].(string); ok && sym != "" {
				sym = strings.TrimSpace(sym)
				quoteBySym[sym] = q
				if alt := yahoo.ToYahooSymbol(sym); alt != sym {
					quoteBySym[alt] = q
				}
			}
		}
	}
	log.Printf("dashboard Build: tickers=%d quoteBySym_keys=%d", len(tickers), len(quoteBySym))

	allTickers := make([]string, len(allCompanies))
	for i, c := range allCompanies {
		allTickers[i] = c.Symbol
	}
	var insiderRecords []models.InsiderSellRecord
	if config.FMPAPIKey != "" || config.EODHDAPIKey != "" || config.SECAPIKey != "" {
		insiderRecords = aggregator.AggregateInsiderSells(allTickers, dateFrom, dateTo)
		log.Printf("dashboard Build: insider_records=%d", len(insiderRecords))
	}
	topInsiders := topInsidersByTicker(insiderRecords)

	qStart := asOf.AddDate(0, 0, -92)
	qStartStr := qStart.Format("2006-01-02")
	qEndStr := asOf.Format("2006-01-02")

	histBySym := make(map[string]*trendDataResult)
	newsBySym := make(map[string][]map[string]interface{})

	// Concurrent hist + news fetching. A semaphore caps the number of
	// in-flight Yahoo/FMP calls to avoid hammering upstream APIs.
	// Each goroutine fetches both historical data and news for one ticker.
	var mu sync.Mutex
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i := 0; i < trendNewsLimit; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sym := companies[idx].Symbol

			var hist []map[string]interface{}
			if useYahooForQuotes {
				hist = yahooClient.GetHistoricalRange(sym, qStartStr, qEndStr)
			} else {
				hist = fmpClient.GetHistoricalRange(sym, qStartStr, qEndStr)
				if len(hist) == 0 {
					hist = yahooClient.GetHistoricalRange(sym, qStartStr, qEndStr)
				}
			}
			td := quarterTrendData(hist)

			var news []map[string]interface{}
			if useYahooForQuotes {
				news = yahooClient.GetNews(sym, 2)
			} else {
				news = fmpClient.GetNews(sym, 2)
				if len(news) == 0 {
					news = yahooClient.GetNews(sym, 2)
				}
			}
			formatted := make([]map[string]interface{}, 0)
			for j, n := range news {
				if j >= 2 {
					break
				}
				title := getStr(n, "title", "url")
				if len(title) > 80 {
					title = title[:80]
				}
				formatted = append(formatted, map[string]interface{}{
					"title": title,
					"url":   getStr(n, "url", "link"),
				})
			}

			mu.Lock()
			if td != nil {
				histBySym[sym] = td
			}
			newsBySym[sym] = formatted
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	log.Printf("dashboard Build: trends_computed=%d news_computed=%d", len(histBySym), len(newsBySym))

	bySector := make(map[string][]map[string]interface{})
	for _, c := range companies {
		sym := c.Symbol
		q := quoteBySym[sym]
		price := 0.0
		chg := 0.0
		if q != nil {
			price = getFloat(q, "price", "change")
			chg = getFloat(q, "changesPercentage")
		}
		var pricePtr *float64
		if price > 0 {
			pricePtr = &price
		}
		var chgPtr *float64
		if chg != 0 {
			chgPtr = &chg
		}
		sector := c.Sector
		if sector == "" {
			sector = "Unknown"
		}
		priceSrc := "yahoo"
		if !useYahooForQuotes {
			priceSrc = "fmp"
		}
		newsSrc := "none"
		if len(newsBySym[sym]) > 0 {
			newsSrc = priceSrc
		}
		insiderSrc := "none"
		if insiders := topInsiders[sym]; len(insiders) > 0 {
			sources := make(map[string]bool)
			for _, ins := range insiders {
				if s, ok := ins["source"].(string); ok && s != "" {
					sources[s] = true
				}
			}
			switch {
			case sources["fmp"] && sources["eodhd"]:
				insiderSrc = "fmp+eodhd"
			case sources["eodhd"]:
				insiderSrc = "eodhd"
			default:
				insiderSrc = "fmp"
			}
		}
		var qTrend *float64
		var qCloses []float64
		if td := histBySym[sym]; td != nil {
			qTrend = &td.Pct
			qCloses = td.Closes
		}
		bySector[sector] = append(bySector[sector], map[string]interface{}{
			"symbol":         sym,
			"name":           c.Name,
			"price":          pricePtr,
			"change_pct":     chgPtr,
			"quarter_trend":  qTrend,
			"quarter_closes": qCloses,
			"news":           newsBySym[sym],
			"top_insiders":   topInsiders[sym],
			"sources":        map[string]string{"price": priceSrc, "news": newsSrc, "insiders": insiderSrc},
		})
	}

	sectors := make([]map[string]interface{}, 0)
	sectorNames := make([]string, 0, len(bySector))
	for k := range bySector {
		sectorNames = append(sectorNames, k)
	}
	sort.Strings(sectorNames)
	for _, name := range sectorNames {
		sectors = append(sectors, map[string]interface{}{
			"name":      name,
			"companies": bySector[name],
		})
	}

	out := map[string]interface{}{
		"as_of":             asOf.Format("2006-01-02"),
		"total_companies":   len(companies),
		"sectors":           sectors,
		"available_sectors": availableSectors,
	}
	if len(providerStatus) > 0 {
		out["provider_status"] = providerStatus
	}
	return out
}

type trendDataResult struct {
	Pct    float64
	Closes []float64
}

// quarterTrendData computes the quarter return and samples ~13 weekly closes for sparklines.
// Prefers Rust binary when available; falls back to Go trend package.
func quarterTrendData(hist []map[string]interface{}) *trendDataResult {
	if len(hist) < 2 {
		return nil
	}
	sort.Slice(hist, func(i, j int) bool {
		return getStr(hist[i], "date") < getStr(hist[j], "date")
	})
	closes := make([]float64, 0, len(hist))
	for _, h := range hist {
		c := getFloat(h, "close", "Close")
		if c > 0 {
			closes = append(closes, c)
		}
	}

	var pct float64
	computed := false

	if rustclient.Available() {
		if rt, err := rustclient.ComputeTrend(closes); err == nil && rt != nil {
			pct = rt.QuarterPct
			computed = true
		}
	}
	if !computed {
		qt := trend.FromCloses(closes)
		if qt == nil {
			return nil
		}
		pct = qt.QuarterPct
	}

	weekly := sampleWeekly(closes)
	return &trendDataResult{Pct: pct, Closes: weekly}
}

func sampleWeekly(closes []float64) []float64 {
	weekly := make([]float64, 0, 14)
	step := 5
	if len(closes) <= 14 {
		step = 1
	}
	for i := 0; i < len(closes); i += step {
		weekly = append(weekly, closes[i])
	}
	if len(closes) > 0 && (len(closes)-1)%step != 0 {
		weekly = append(weekly, closes[len(closes)-1])
	}
	return weekly
}

func topInsidersByTicker(records []models.InsiderSellRecord) map[string][]map[string]interface{} {
	byTicker := make(map[string][]map[string]interface{})
	for _, r := range records {
		t := strings.ToUpper(r.Ticker)
		ins := "Unknown"
		if r.InsiderName != nil {
			ins = *r.InsiderName
		}
		var val *float64
		if r.ValueUSD != nil {
			val = r.ValueUSD
		}
		byTicker[t] = append(byTicker[t], map[string]interface{}{
			"name":   ins,
			"role":   r.Role,
			"shares": r.SharesSold,
			"value":  val,
			"source": r.Source,
		})
	}
	out := make(map[string][]map[string]interface{})
	for t, lst := range byTicker {
		sort.Slice(lst, func(i, j int) bool {
			return getFloat(lst[i], "shares") > getFloat(lst[j], "shares")
		})
		if len(lst) > 5 {
			lst = lst[:5]
		}
		out[t] = lst
	}
	return out
}

func getStr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return v
		}
		if v, ok := m[k].(int); ok {
			return float64(v)
		}
	}
	return 0
}
