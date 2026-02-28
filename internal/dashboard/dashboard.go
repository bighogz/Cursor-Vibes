package dashboard

import (
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/aggregator"
	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/models"
	"github.com/bighogz/Cursor-Vibes/internal/sp500"
	"github.com/bighogz/Cursor-Vibes/internal/trend"
	"github.com/bighogz/Cursor-Vibes/internal/yahoo"
)

// BuildOpts controls which companies get trend/news. Filter by sector first, then limit.
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
	// Collect sector names for frontend dropdown (before filtering)
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
	// Filter by sector first, then apply limit to that subset
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

	// When filtering by sector/limit, compute trend/news for all in subset (companies being viewed).
	// Otherwise sample first N from full CSV. Price is fetched for all; trend/news are per-symbol and
	// rate-limited, so only the first sample get them. "Trend/news still blank" for most rows is expected.
	trendNewsLimit := len(companies)
	if opts.Sector == "" && opts.Limit <= 0 {
		sample := 10
		if !config.FMPFreeTier {
			sample = min(50, len(companies))
		}
		trendNewsLimit = min(sample, len(companies))
	}
	insiderSample := 15
	if !config.FMPFreeTier {
		insiderSample = min(80, len(tickers))
	}

	quoteBySym := make(map[string]map[string]interface{})
	yahooClient := yahoo.New()
	fmpClient := fmp.New()
	providerStatus := make(map[string]string)

	// Use Yahoo for quotes when no FMP key or on FMP free tier (avoid rate limit).
	// Otherwise try FMP first; fall back to Yahoo on rate limit or error.
	useYahooForQuotes := config.FMPAPIKey == "" || config.FMPFreeTier
	batchSize := 100 // both fmpClient.GetQuote and yahooClient.GetQuote support up to 100

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
				// Store under both BRK.B and BRK-B so lookups work for either form
				if alt := yahoo.ToYahooSymbol(sym); alt != sym {
					quoteBySym[alt] = q
				}
			}
		}
		// a little breathing room; yfinance batch is heavy
		time.Sleep(150 * time.Millisecond)
	}
	log.Printf("dashboard Build: tickers=%d quoteBySym_keys=%d", len(tickers), len(quoteBySym))

	insiderTickers := tickers[:min(insiderSample, len(tickers))]
	var insiderRecords []models.InsiderSellRecord
	if config.FMPAPIKey != "" {
		insiderRecords = aggregator.AggregateInsiderSells(insiderTickers, dateFrom, dateTo)
	}
	topInsiders := topInsidersByTicker(insiderRecords)

	qStart := asOf.AddDate(0, 0, -92)
	qStartStr := qStart.Format("2006-01-02")
	qEndStr := asOf.Format("2006-01-02")

	histBySym := make(map[string]*float64)
	newsBySym := make(map[string][]map[string]interface{})

	for i := 0; i < trendNewsLimit; i++ {
		sym := companies[i].Symbol
		var hist []map[string]interface{}
		if useYahooForQuotes {
			hist = yahooClient.GetHistoricalRange(sym, qStartStr, qEndStr)
		} else {
			hist = fmpClient.GetHistoricalRange(sym, qStartStr, qEndStr)
			if len(hist) == 0 {
				hist = yahooClient.GetHistoricalRange(sym, qStartStr, qEndStr)
			}
		}
		if qt := quarterTrendFromHist(hist); qt != nil {
			histBySym[sym] = qt
		}
		if i < 3 {
			log.Printf("dashboard Build: hist ticker[%d]=%s hist_records=%d trend=%v", i, sym, len(hist), histBySym[sym] != nil)
		}
		time.Sleep(80 * time.Millisecond)
	}
	log.Printf("dashboard Build: trends_computed=%d", len(histBySym))
	for i := 0; i < trendNewsLimit; i++ {
		sym := companies[i].Symbol
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
		newsBySym[sym] = formatted
		time.Sleep(80 * time.Millisecond)
	}

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
		if len(topInsiders[sym]) > 0 {
			insiderSrc = "fmp"
		}
		bySector[sector] = append(bySector[sector], map[string]interface{}{
			"symbol":         sym,
			"name":           c.Name,
			"price":          pricePtr,
			"change_pct":     chgPtr,
			"quarter_trend":  histBySym[sym],
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
		"as_of":              asOf.Format("2006-01-02"),
		"total_companies":    len(companies),
		"sectors":            sectors,
		"available_sectors":  availableSectors,
	}
	if len(providerStatus) > 0 {
		out["provider_status"] = providerStatus
	}
	return out
}

func quarterTrendFromHist(hist []map[string]interface{}) *float64 {
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
	qt := trend.FromCloses(closes)
	if qt == nil {
		return nil
	}
	return &qt.QuarterPct
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
			"name":  ins,
			"role":  r.Role,
			"shares": r.SharesSold,
			"value": val,
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
