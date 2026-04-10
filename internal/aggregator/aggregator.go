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
	"github.com/bighogz/Cursor-Vibes/internal/edgar"
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

	// Seed from unified cache. Load ALL records for matching tickers regardless
	// of date — the user wants the most recent disclosure even if it's years old.
	// The cache accumulates across builds and sources, so it grows over time.
	cached := loadUnifiedCache()
	for _, r := range cached {
		t := strings.ToUpper(r.Ticker)
		if tickerSet[t] {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
			}
		}
	}
	if len(cached) > 0 {
		log.Printf("aggregator: loaded %d records from unified cache, %d matched tickers", len(cached), len(all))
	}

	// Track which tickers already have data so SEC-API can prioritize gaps.
	coveredTickers := make(map[string]bool)
	for _, r := range all {
		coveredTickers[strings.ToUpper(r.Ticker)] = true
	}

	if config.FMPAPIKey != "" {
		client := fmp.New()
		recs := client.GetInsiderSells(tickerSet, dateFrom, dateTo)
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
				coveredTickers[strings.ToUpper(r.Ticker)] = true
			}
		}
		log.Printf("aggregator: fmp contributed, total now %d", len(all))
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
				coveredTickers[strings.ToUpper(r.Ticker)] = true
				added++
			}
		}
		log.Printf("aggregator: eodhd contributed %d new records", added)
	}

	if config.SECAPIKey != "" {
		sClient := secapi.New()
		recs := sClient.GetInsiderSells(tickerSet, dateFrom, dateTo, coveredTickers)
		added := 0
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
				coveredTickers[strings.ToUpper(r.Ticker)] = true
				added++
			}
		}
		log.Printf("aggregator: sec-api contributed %d new records", added)
	}

	// EDGAR direct backfill: for tickers still missing insider data, query
	// SEC EDGAR directly (free, no API key). This fills gaps that SEC-API.io
	// and FMP didn't cover.
	var missing []string
	for _, t := range tickers {
		if !coveredTickers[strings.ToUpper(t)] {
			missing = append(missing, strings.ToUpper(t))
		}
	}
	if len(missing) > 0 {
		eClient := edgar.New()
		maxFetches := min(800, len(missing)*5)
		recs := eClient.GetInsiderSells(missing, dateFrom, maxFetches)
		added := 0
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
				coveredTickers[strings.ToUpper(r.Ticker)] = true
				added++
			}
		}
		log.Printf("aggregator: edgar backfill contributed %d records for %d missing tickers", added, len(missing))
	}

	// Persist ALL records to the unified cache (including old ones from previous
	// builds). This means coverage grows monotonically across restarts.
	if len(all) > 0 {
		saveUnifiedCache(all)
	}

	covered := len(coveredTickers)
	total := len(tickerSet)
	log.Printf("aggregator: %d deduplicated records covering %d/%d tickers", len(all), covered, total)
	return all
}

const unifiedCachePath = "data/insider_unified_cache.json"

// LoadCachedRecords returns insider records from the on-disk unified cache
// without making any API calls. Returns nil if the cache doesn't exist.
func LoadCachedRecords(tickers []string) []models.InsiderSellRecord {
	all := loadUnifiedCache()
	if len(tickers) == 0 {
		return all
	}
	set := make(map[string]bool, len(tickers))
	for _, t := range tickers {
		set[strings.ToUpper(t)] = true
	}
	filtered := make([]models.InsiderSellRecord, 0, len(tickers)*4)
	for _, r := range all {
		if set[strings.ToUpper(r.Ticker)] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

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
	BlackoutAdjusted   bool    `json:"blackout_adjusted,omitempty"`
}

// earningsBlackoutMonths approximates the months when most S&P 500 companies
// are in earnings blackout periods (insiders cannot trade). Q4 earnings are
// reported Jan-Feb, Q1 in Apr-May, Q2 in Jul-Aug, Q3 in Oct-Nov. The ~2 week
// quiet period before each means sell volume naturally drops in these months,
// then spikes after. A naive Z-score would flag routine post-earnings selling.
//
// This is a heuristic — precise blackout windows vary per company and are
// governed by each firm's insider trading policy. For a production system,
// you'd ingest actual 10b5-1 plan data or company-specific blackout calendars.
var earningsBlackoutMonths = map[time.Month]bool{
	time.January: true, time.April: true,
	time.July: true, time.October: true,
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

		// Blackout period adjustment: if the current window falls right after
		// an earnings blackout month, insiders cluster their (routine) sells
		// in the open window. Dampen the z-score to avoid false positives on
		// expected post-blackout activity.
		adjusted := false
		if isPostBlackout(currentStart) {
			z *= 0.6 // 40% dampening for expected post-blackout clustering
			adjusted = true
		}

		sig.BaselineMean = meanB
		sig.BaselineStd = stdB
		sig.ZScore = z
		sig.IsAnomaly = z >= stdThreshold && currentTotal > 0
		sig.BlackoutAdjusted = adjusted
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

// isPostBlackout returns true if the given date falls in the month immediately
// following a typical earnings blackout month.
func isPostBlackout(t time.Time) bool {
	// Post-blackout months: Feb (after Jan blackout), May (after Apr),
	// Aug (after Jul), Nov (after Oct).
	prev := t.Month() - 1
	if prev < 1 {
		prev = 12
	}
	return earningsBlackoutMonths[prev]
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