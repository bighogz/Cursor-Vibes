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

// Result is exported so callers (main.go, tests) can use the typed return.
type Result = models.DashboardResult

func Build(opts BuildOpts) *Result {
	allCompanies := sp500.Load()
	if len(allCompanies) == 0 {
		return &Result{Error: "Could not load S&P 500"}
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

	// Insider data uses a 3-year window to capture the most recent disclosure
	// for every company — many S&P 500 insiders only sell once every 1-2 years.
	// Trend/news uses a separate 92-day window (qStart below).
	insiderFrom := asOf.AddDate(-3, 0, 0)
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
	insiderRecords := aggregator.AggregateInsiderSells(allTickers, insiderFrom, dateTo)
	log.Printf("dashboard Build: insider_records=%d", len(insiderRecords))
	topInsiders := topInsidersByTicker(insiderRecords)

	anomalySignals := aggregator.ComputeAnomalySignals(
		insiderRecords,
		config.BaselineDays,
		config.CurrentWindowDays,
		config.AnomalyStdThreshold,
		asOf,
	)
	anomalyByTicker := make(map[string]*aggregator.AnomalySignal, len(anomalySignals))
	for i := range anomalySignals {
		anomalyByTicker[anomalySignals[i].Ticker] = &anomalySignals[i]
	}
	scored := 0
	for _, s := range anomalySignals {
		if s.CompositeScore != 0 {
			scored++
		}
	}
	log.Printf("dashboard Build: anomaly_signals=%d scored=%d", len(anomalySignals), scored)

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
			td := computeAllTrends(hist)

			var news []map[string]interface{}
			if useYahooForQuotes {
				news = yahooClient.GetNews(sym, 3)
			} else {
				news = fmpClient.GetNews(sym, 3)
				if len(news) == 0 {
					news = yahooClient.GetNews(sym, 3)
				}
			}
			formatted := make([]map[string]interface{}, 0)
			for j, n := range news {
				if j >= 3 {
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

	bySector := make(map[string][]models.Company)
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
			srcSet := make(map[string]bool)
			for _, ins := range insiders {
				srcSet[ins.Source] = true
			}
			var parts []string
			for _, s := range []string{"fmp", "eodhd", "sec", "edgar"} {
				if srcSet[s] {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				insiderSrc = strings.Join(parts, "+")
			}
		}
		var qTrend *float64
		var qCloses []float64
		var trends map[string]*models.TrendPeriod
		if td := histBySym[sym]; td != nil && td.Periods != nil {
			if q := td.Periods["quarterly"]; q != nil {
				qTrend = &q.Pct
				qCloses = q.Closes
			}
			trends = td.Periods
		}

		newsItems := make([]models.NewsItem, 0, len(newsBySym[sym]))
		for _, n := range newsBySym[sym] {
			newsItems = append(newsItems, models.NewsItem{
				Title: getStr(n, "title"),
				URL:   getStr(n, "url"),
			})
		}

		co := models.Company{
			Symbol:        sym,
			Name:          c.Name,
			Price:         pricePtr,
			ChangePct:     chgPtr,
			QuarterTrend:  qTrend,
			QuarterCloses: qCloses,
			Trends:        trends,
			News:          newsItems,
			TopInsiders:   topInsiders[sym],
			Sources:       map[string]string{"price": priceSrc, "news": newsSrc, "insiders": insiderSrc},
		}
		if sig := anomalyByTicker[sym]; sig != nil && sig.CompositeScore != 0 {
			co.AnomalyScore = &sig.CompositeScore
			co.VolumeZScore = &sig.VolumeZScore
			co.BreadthZScore = &sig.BreadthZScore
			co.AccelerationScore = &sig.AccelerationScore
			ui := sig.UniqueInsiders
			co.UniqueInsiders = &ui
		}
		bySector[sector] = append(bySector[sector], co)
	}

	sectors := make([]models.SectorGroup, 0, len(bySector))
	sectorNames := make([]string, 0, len(bySector))
	for k := range bySector {
		sectorNames = append(sectorNames, k)
	}
	sort.Strings(sectorNames)
	for _, name := range sectorNames {
		sectors = append(sectors, models.SectorGroup{
			Name:      name,
			Companies: bySector[name],
		})
	}

	return &Result{
		AsOf:             asOf.Format("2006-01-02"),
		TotalCompanies:   len(companies),
		Sectors:          sectors,
		AvailableSectors: availableSectors,
		ProviderStatus:   providerStatus,
	}
}

type trendDataResult struct {
	Periods map[string]*models.TrendPeriod
}

// computeAllTrends produces daily/weekly/monthly/quarterly trends from a
// single set of daily closes (fetched over ~92 calendar days). Each period
// has a return percentage and a sparkline-appropriate slice of closes.
func computeAllTrends(hist []map[string]interface{}) *trendDataResult {
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
	if len(closes) < 2 {
		return nil
	}

	periods := make(map[string]*models.TrendPeriod, 4)

	// Helper: compute return from N trading days ago (clamped to available data).
	pctFrom := func(lookback int) float64 {
		idx := len(closes) - lookback
		if idx < 0 {
			idx = 0
		}
		if closes[idx] <= 0 {
			return 0
		}
		return (closes[len(closes)-1]/closes[idx] - 1) * 100
	}

	// 1D: previous close → last close; sparkline = last 5 trading days
	periods["daily"] = &models.TrendPeriod{
		Pct:    pctFrom(2),
		Closes: tail(closes, 5),
	}

	// 1W: ~5 trading days return; sparkline = last 5 trading days
	periods["weekly"] = &models.TrendPeriod{
		Pct:    pctFrom(5),
		Closes: tail(closes, 5),
	}

	// 1M: ~22 trading days return; sparkline = last 22 trading days
	periods["monthly"] = &models.TrendPeriod{
		Pct:    pctFrom(22),
		Closes: tail(closes, 22),
	}

	// 3M: ~63 trading days; sparkline = weekly-sampled closes.
	// Prefer Rust binary for quarterly pct when available.
	var qPct float64
	qComputed := false
	if rustclient.Available() {
		if rt, err := rustclient.ComputeTrend(closes); err == nil && rt != nil {
			qPct = rt.QuarterPct
			qComputed = true
		}
	}
	if !qComputed {
		qt := trend.FromCloses(closes)
		if qt != nil {
			qPct = qt.QuarterPct
		} else {
			qPct = pctFrom(63)
		}
	}
	periods["quarterly"] = &models.TrendPeriod{
		Pct:    qPct,
		Closes: sampleWeekly(closes),
	}

	return &trendDataResult{Periods: periods}
}

// tail returns the last n elements of a slice (or the whole slice if shorter).
func tail(s []float64, n int) []float64 {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
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

func topInsidersByTicker(records []models.InsiderSellRecord) map[string][]models.InsiderEntry {
	byTicker := make(map[string][]models.InsiderEntry)
	for _, r := range records {
		t := strings.ToUpper(r.Ticker)
		name := "Unknown"
		if r.InsiderName != nil {
			name = *r.InsiderName
		}
		byTicker[t] = append(byTicker[t], models.InsiderEntry{
			Name:   name,
			Role:   r.Role,
			Shares: r.SharesSold,
			Value:  r.ValueUSD,
			TxType: r.TxType,
			Source: r.Source,
		})
	}
	out := make(map[string][]models.InsiderEntry)
	for t, lst := range byTicker {
		sort.Slice(lst, func(i, j int) bool {
			return lst[i].Shares > lst[j].Shares
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
