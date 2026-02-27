package aggregator

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

type dailyVolume struct {
	ticker string
	date   time.Time
	shares float64
}

func AggregateInsiderSells(tickers []string, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	seen := make(map[string]bool)
	all := make([]models.InsiderSellRecord, 0)

	fmpLimit := len(tickers)
	if config.FMPFreeTier && fmpLimit > 25 {
		fmpLimit = 25
	}
	fmpTickers := tickers[:min(fmpLimit, len(tickers))]

	if config.FMPAPIKey != "" {
		client := fmp.New()
		for _, t := range fmpTickers {
			recs := client.GetInsiderSells(t, dateFrom, dateTo)
			for _, r := range recs {
				key := keyFor(r)
				if !seen[key] {
					seen[key] = true
					all = append(all, r)
				}
			}
		}
		recs := client.GetInsiderSells("", dateFrom, dateTo)
		for _, r := range recs {
			key := keyFor(r)
			if !seen[key] {
				seen[key] = true
				all = append(all, r)
			}
		}
	}

	return all
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