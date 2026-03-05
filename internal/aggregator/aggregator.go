package aggregator

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"encoding/json"
	"os"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/eodhd"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/models"
	"github.com/bighogz/Cursor-Vibes/internal/secapi"
)

type dailyVolume struct {
	ticker string
	date   time.Time
	shares float64
}

func AggregateInsiderSells(tickers []string, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	seen := make(map[string]bool)
	all := make([]models.InsiderSellRecord, 0)

	tickerSet := make(map[string]bool, len(tickers))
	for _, t := range tickers {
		tickerSet[strings.ToUpper(t)] = true
	}

	// Seed from unified cache (accumulated across multiple builds and sources)
	cached := loadUnifiedCache()
	for _, r := range cached {
		t := strings.ToUpper(r.Ticker)
		if tickerSet[t] && !r.TransactionDate.Before(dateFrom) && !r.TransactionDate.After(dateTo) {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
			}
		}
	}
	if len(cached) > 0 {
		log.Printf("aggregator: loaded %d records from unified cache, %d matched filters", len(cached), len(all))
	}

	if config.FMPAPIKey != "" {
		client := fmp.New()
		recs := client.GetInsiderSells(tickerSet, dateFrom, dateTo)
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
			}
		}
		log.Printf("aggregator: fmp contributed %d insider records", len(all))
	}

	if config.EODHDAPIKey != "" {
		eClient := eodhd.New()
		recs := eClient.GetInsiderSells(tickerSet, dateFrom, dateTo)
		added := 0
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
				added++
			}
		}
		log.Printf("aggregator: eodhd contributed %d new insider records (%d total from eodhd)", added, len(recs))
	}

	if config.SECAPIKey != "" {
		sClient := secapi.New()
		recs := sClient.GetInsiderSells(tickerSet, dateFrom, dateTo)
		added := 0
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
				added++
			}
		}
		log.Printf("aggregator: sec-api contributed %d new insider records (%d total from sec)", added, len(recs))
	}

	// Persist all aggregated records to the unified insider cache. This means
	// SEC-API and EODHD records accumulate alongside FMP records across restarts.
	if len(all) > 0 {
		saveUnifiedCache(all)
	}

	log.Printf("aggregator: total deduplicated insider records=%d", len(all))
	return all
}

const unifiedCachePath = "data/insider_unified_cache.json"

func loadUnifiedCache() []models.InsiderSellRecord {
	f, err := os.Open(unifiedCachePath)
	if err != nil {
		return nil
	}
	defer f.Close()
	var records []models.InsiderSellRecord
	json.NewDecoder(f).Decode(&records)
	return records
}

func saveUnifiedCache(records []models.InsiderSellRecord) {
	os.MkdirAll("data", 0700)
	f, err := os.OpenFile(unifiedCachePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(records)
}

func keyFor(r models.InsiderSellRecord) string {
	ins := ""
	if r.InsiderName != nil {
		ins = *r.InsiderName
	}
	return strings.ToUpper(r.Ticker) + "|" + r.TransactionDate.Format("2006-01-02") + "|" + ins + "|" + fmt.Sprintf("%.0f", r.SharesSold)
}

type AnomalySignal struct {
	Ticker             string  `json:"ticker"`
	CurrentSharesSold  float64 `json:"current_shares_sold"`
	BaselineMean       float64 `json:"baseline_mean"`
	BaselineStd        float64 `json:"baseline_std"`
	ZScore             float64 `json:"z_score"`
	IsAnomaly          bool    `json:"is_anomaly"`
}

func ComputeAnomalySignals(records []models.InsiderSellRecord, baselineDays, currentDays int, stdThreshold float64, asOf time.Time) []AnomalySignal {
	daily := dailyVolumeByTicker(records)
	if len(daily) == 0 {
		return nil
	}
	baselineEnd := asOf.AddDate(0, 0, -currentDays)
	baselineStart := baselineEnd.AddDate(0, 0, -baselineDays)
	currentStart := asOf.AddDate(0, 0, -currentDays)

	tickerDates := make(map[string]map[string]float64) // ticker -> date -> total shares
	for _, d := range daily {
		if tickerDates[d.ticker] == nil {
			tickerDates[d.ticker] = make(map[string]float64)
		}
		dt := d.date.Format("2006-01-02")
		tickerDates[d.ticker][dt] += d.shares
	}

	results := make([]AnomalySignal, 0)
	for ticker, byDate := range tickerDates {
		var baselineTotals []float64
		var currentTotal float64
		for dtStr, shares := range byDate {
			dt, _ := time.Parse("2006-01-02", dtStr)
			if !dt.Before(baselineStart) && dt.Before(baselineEnd) {
				baselineTotals = append(baselineTotals, shares)
			}
			if !dt.Before(currentStart) && !dt.After(asOf) {
				currentTotal += shares
			}
		}
		sig := AnomalySignal{Ticker: ticker, CurrentSharesSold: currentTotal}
		if len(baselineTotals) < config.MinBaselinePoints {
			sig.IsAnomaly = false
			results = append(results, sig)
			continue
		}
		meanB, stdB := meanStd(baselineTotals)
		if stdB <= 0 {
			stdB = 1e-9
		}
		numDays := asOf.Sub(currentStart).Hours()/24 + 1
		if numDays < 1 {
			numDays = 1
		}
		currentAvgDaily := currentTotal / numDays
		z := (currentAvgDaily - meanB) / stdB
		sig.BaselineMean = meanB
		sig.BaselineStd = stdB
		sig.ZScore = z
		sig.IsAnomaly = z >= stdThreshold && currentTotal > 0
		results = append(results, sig)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ZScore > results[j].ZScore })
	return results
}

func dailyVolumeByTicker(records []models.InsiderSellRecord) []dailyVolume {
	out := make([]dailyVolume, 0)
	for _, r := range records {
		out = append(out, dailyVolume{
			ticker: strings.ToUpper(r.Ticker),
			date:   r.TransactionDate,
			shares: r.SharesSold,
		})
	}
	return out
}

func meanStd(vals []float64) (mean, std float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean = sum / float64(len(vals))
	var sqDiff float64
	for _, v := range vals {
		d := v - mean
		sqDiff += d * d
	}
	std = math.Sqrt(sqDiff / float64(len(vals)))
	return mean, std
}