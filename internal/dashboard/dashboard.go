package dashboard

import (
	"sort"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/aggregator"
	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/models"
	"github.com/bighogz/Cursor-Vibes/internal/sp500"
	"github.com/bighogz/Cursor-Vibes/internal/yahoo"
)

func Build(limit int, asOf time.Time) map[string]interface{} {
	companies := sp500.Load()
	if len(companies) == 0 {
		return map[string]interface{}{"error": "Could not load S&P 500", "sectors": []interface{}{}}
	}
	if limit > 0 {
		companies = companies[:min(limit, len(companies))]
	}
	tickers := make([]string, len(companies))
	for i, c := range companies {
		tickers[i] = c.Symbol
	}

	totalDays := 365 + 30
	dateFrom := asOf.AddDate(0, 0, -totalDays)
	dateTo := asOf

	sample := 10
	insiderSample := 15
	if !config.FMPFreeTier {
		sample = min(50, len(companies))
		insiderSample = min(80, len(tickers))
	}

	quoteBySym := make(map[string]map[string]interface{})
	yahooClient := yahoo.New()
	useYahooForQuotes := config.FMPFreeTier || config.FMPAPIKey == ""

	// Free tier: use Yahoo for quotes (saves FMP calls). Paid: FMP first, Yahoo fallback.
	if useYahooForQuotes {
		for i := 0; i < min(len(tickers), 50); i += 10 {
			batch := tickers[i:min(i+10, len(tickers))]
			quotes := yahooClient.GetQuote(batch)
			for _, q := range quotes {
				if sym, ok := q["symbol"].(string); ok && sym != "" {
					quoteBySym[strings.TrimSpace(sym)] = q
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else {
		fmpClient := fmp.New()
		batchLimit, batchSize := 20, 50
		for i := 0; i < min(len(tickers), batchLimit*batchSize); i += batchSize {
			batch := tickers[i:min(i+batchSize, len(tickers))]
			quotes := fmpClient.GetQuote(batch)
			for _, q := range quotes {
				if sym, ok := q["symbol"].(string); ok && sym != "" {
					quoteBySym[strings.TrimSpace(sym)] = q
				}
			}
			time.Sleep(200 * time.Millisecond)
		}
		if len(quoteBySym) == 0 {
			for i := 0; i < min(len(tickers), 50); i += 10 {
				batch := tickers[i:min(i+10, len(tickers))]
				quotes := yahooClient.GetQuote(batch)
				for _, q := range quotes {
					if sym, ok := q["symbol"].(string); ok && sym != "" {
						quoteBySym[strings.TrimSpace(sym)] = q
					}
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

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
	fmpClient := fmp.New()

	for i := 0; i < sample && i < len(companies); i++ {
		sym := companies[i].Symbol
		var hist []map[string]interface{}
		if useYahooForQuotes {
			hist = yahooClient.GetHistoricalRange(sym, qStartStr, qEndStr)
		} else if config.FMPAPIKey != "" {
			hist = fmpClient.GetHistoricalRange(sym, qStartStr, qEndStr)
		}
		if chg := quarterChange(hist); chg != nil {
			histBySym[sym] = chg
		}
		time.Sleep(80 * time.Millisecond)
	}
	for i := 0; i < sample && i < len(companies); i++ {
		sym := companies[i].Symbol
		var news []map[string]interface{}
		if useYahooForQuotes {
			news = yahooClient.GetNews(sym, 2)
		} else if config.FMPAPIKey != "" {
			news = fmpClient.GetNews(sym, 2)
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
		bySector[sector] = append(bySector[sector], map[string]interface{}{
			"symbol":        sym,
			"name":          c.Name,
			"price":         pricePtr,
			"change_pct":    chgPtr,
			"quarter_trend": histBySym[sym],
			"news":          newsBySym[sym],
			"top_insiders":  topInsiders[sym],
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

	return map[string]interface{}{
		"as_of":           asOf.Format("2006-01-02"),
		"total_companies": len(companies),
		"sectors":         sectors,
	}
}

func quarterChange(hist []map[string]interface{}) *float64 {
	if len(hist) < 2 {
		return nil
	}
	sort.Slice(hist, func(i, j int) bool {
		return getStr(hist[i], "date") < getStr(hist[j], "date")
	})
	first := getFloat(hist[0], "close")
	last := getFloat(hist[len(hist)-1], "close")
	if first <= 0 {
		return nil
	}
	chg := (last - first) / first * 100
	return &chg
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
