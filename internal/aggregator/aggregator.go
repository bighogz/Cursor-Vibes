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

type txRecord struct {
	ticker     string
	insider    string
	date       time.Time
	shares     float64
	dollarVal  float64
	hasDollar  bool
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

// AnomalySignal holds a composite anomaly score built from three sub-signals:
// volume (dollar-normalized weekly z-score), breadth (unique insider count),
// and acceleration (sell frequency ratio). The composite is a weighted sum.
type AnomalySignal struct {
	Ticker            string  `json:"ticker"`
	CompositeScore    float64 `json:"composite_score"`
	VolumeZScore      float64 `json:"volume_z_score"`
	BreadthZScore     float64 `json:"breadth_z_score"`
	AccelerationScore float64 `json:"acceleration_score"`
	IsAnomaly         bool    `json:"is_anomaly"`
	CurrentDollarVol  float64 `json:"current_dollar_vol"`
	CurrentSharesSold float64 `json:"current_shares_sold"`
	UniqueInsiders    int     `json:"unique_insiders"`
	BaselineMean      float64 `json:"baseline_mean"`
	BaselineStd       float64 `json:"baseline_std"`
	BlackoutAdjusted  bool    `json:"blackout_adjusted,omitempty"`
}

var earningsBlackoutMonths = map[time.Month]bool{
	time.January: true, time.April: true,
	time.July: true, time.October: true,
}

// isoWeek returns a "YYYY-WNN" key for weekly bucketing.
func isoWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

func ComputeAnomalySignals(records []models.InsiderSellRecord, baselineDays, currentDays int, stdThreshold float64, asOf time.Time) []AnomalySignal {
	txs := extractTxRecords(records)
	if len(txs) == 0 {
		return nil
	}

	baselineEnd := asOf.AddDate(0, 0, -currentDays)
	baselineStart := baselineEnd.AddDate(0, 0, -baselineDays)
	currentStart := asOf.AddDate(0, 0, -currentDays)

	// Group transactions by ticker.
	byTicker := make(map[string][]txRecord)
	for _, tx := range txs {
		byTicker[tx.ticker] = append(byTicker[tx.ticker], tx)
	}

	baselineWeeks := float64(baselineDays) / 7.0
	if baselineWeeks < 1 {
		baselineWeeks = 1
	}
	currentWeeks := float64(currentDays) / 7.0
	if currentWeeks < 1 {
		currentWeeks = 1
	}

	results := make([]AnomalySignal, 0, len(byTicker))
	for ticker, txs := range byTicker {
		// --- Partition into baseline and current windows ---
		var baselineTxs, currentTxs []txRecord
		for _, tx := range txs {
			if !tx.date.Before(baselineStart) && tx.date.Before(baselineEnd) {
				baselineTxs = append(baselineTxs, tx)
			}
			if !tx.date.Before(currentStart) && !tx.date.After(asOf) {
				currentTxs = append(currentTxs, tx)
			}
		}

		// --- Weekly dollar volume buckets (baseline) ---
		baselineWeeklyVol := make(map[string]float64)
		baselineWeeklyInsiders := make(map[string]map[string]bool)
		for _, tx := range baselineTxs {
			wk := isoWeek(tx.date)
			baselineWeeklyVol[wk] += tx.dollarVal
			if baselineWeeklyInsiders[wk] == nil {
				baselineWeeklyInsiders[wk] = make(map[string]bool)
			}
			baselineWeeklyInsiders[wk][tx.insider] = true
		}

		// --- Current window aggregates ---
		var currentDollarVol, currentShares float64
		currentInsiders := make(map[string]bool)
		for _, tx := range currentTxs {
			currentDollarVol += tx.dollarVal
			currentShares += tx.shares
			currentInsiders[tx.insider] = true
		}

		sig := AnomalySignal{
			Ticker:            ticker,
			CurrentDollarVol:  currentDollarVol,
			CurrentSharesSold: currentShares,
			UniqueInsiders:    len(currentInsiders),
		}

		// Need enough weekly data points to compute meaningful statistics.
		if len(baselineWeeklyVol) < config.MinBaselineWeeks {
			results = append(results, sig)
			continue
		}

		// --- Sub-signal 1: Volume z-score (weekly dollar buckets) ---
		weeklyVols := make([]float64, 0, len(baselineWeeklyVol))
		for _, v := range baselineWeeklyVol {
			weeklyVols = append(weeklyVols, v)
		}
		meanVol, stdVol := meanStd(weeklyVols)
		if stdVol <= 0 {
			stdVol = 1e-9
		}
		currentWeeklyAvg := currentDollarVol / currentWeeks
		volumeZ := clampZ((currentWeeklyAvg - meanVol) / stdVol)

		// --- Sub-signal 2: Breadth z-score (unique insiders per week) ---
		weeklyBreadth := make([]float64, 0, len(baselineWeeklyInsiders))
		for _, ins := range baselineWeeklyInsiders {
			weeklyBreadth = append(weeklyBreadth, float64(len(ins)))
		}
		meanBreadth, stdBreadth := meanStd(weeklyBreadth)
		if stdBreadth <= 0 {
			stdBreadth = 1e-9
		}
		currentBreadthPerWeek := float64(len(currentInsiders)) / currentWeeks
		breadthZ := clampZ((currentBreadthPerWeek - meanBreadth) / stdBreadth)

		// --- Sub-signal 3: Acceleration (frequency ratio) ---
		baselineFreqPerWeek := float64(len(baselineWeeklyVol)) / baselineWeeks
		currentFreqPerWeek := 0.0
		if len(currentTxs) > 0 {
			// Count distinct weeks with activity in the current window.
			cwWeeks := make(map[string]bool)
			for _, tx := range currentTxs {
				cwWeeks[isoWeek(tx.date)] = true
			}
			currentFreqPerWeek = float64(len(cwWeeks)) / currentWeeks
		}
		accelScore := 0.0
		if baselineFreqPerWeek > 0 {
			accelScore = currentFreqPerWeek / baselineFreqPerWeek
		}

		// --- Composite ---
		composite := config.VolumeWeight*volumeZ +
			config.BreadthWeight*breadthZ +
			config.AccelerationWeight*accelScore

		// Blackout dampening
		adjusted := false
		if isPostBlackout(currentStart) {
			composite *= 0.6
			volumeZ *= 0.6
			adjusted = true
		}

		sig.VolumeZScore = volumeZ
		sig.BreadthZScore = breadthZ
		sig.AccelerationScore = accelScore
		sig.CompositeScore = composite
		sig.BaselineMean = meanVol
		sig.BaselineStd = stdVol
		sig.IsAnomaly = composite >= stdThreshold && currentDollarVol > 0
		sig.BlackoutAdjusted = adjusted
		results = append(results, sig)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].CompositeScore > results[j].CompositeScore })
	return results
}

func extractTxRecords(records []models.InsiderSellRecord) []txRecord {
	out := make([]txRecord, 0, len(records))
	for _, r := range records {
		insider := ""
		if r.InsiderName != nil {
			insider = *r.InsiderName
		}
		dv := 0.0
		hasDollar := false
		if r.ValueUSD != nil && *r.ValueUSD > 0 {
			dv = *r.ValueUSD
			hasDollar = true
		} else {
			dv = r.SharesSold
		}
		out = append(out, txRecord{
			ticker:    strings.ToUpper(r.Ticker),
			insider:   insider,
			date:      r.TransactionDate,
			shares:    r.SharesSold,
			dollarVal: dv,
			hasDollar: hasDollar,
		})
	}
	return out
}

func isPostBlackout(t time.Time) bool {
	prev := t.Month() - 1
	if prev < 1 {
		prev = 12
	}
	return earningsBlackoutMonths[prev]
}

func clampZ(z float64) float64 {
	if z > 10 {
		return 10
	}
	if z < -10 {
		return -10
	}
	return z
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