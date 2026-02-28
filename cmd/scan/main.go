package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/aggregator"
	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/rustclient"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load(".env")
}

func main() {
	baselineDays := flag.Int("baseline-days", config.BaselineDays, "Days of history for baseline")
	currentDays := flag.Int("current-days", config.CurrentWindowDays, "Current window days")
	stdThreshold := flag.Float64("std-threshold", config.AnomalyStdThreshold, "Z-score threshold")
	asOfStr := flag.String("as-of", "", "As-of date YYYY-MM-DD")
	listAll := flag.Bool("list-all-signals", false, "Print all signals")
	csvPath := flag.String("csv", "", "Write to CSV")
	flag.Parse()

	asOf := time.Now()
	if *asOfStr != "" {
		var err error
		asOf, err = time.Parse("2006-01-02", *asOfStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid as-of date: %v\n", err)
			os.Exit(1)
		}
	}

	client := fmp.New()
	tickers := client.GetSP500Tickers()
	if len(tickers) == 0 {
		fmt.Fprintln(os.Stderr, "Could not load S&P 500 constituents.")
		os.Exit(1)
	}
	fmt.Printf("Loaded %d S&P 500 tickers.\n", len(tickers))

	totalDays := *baselineDays + *currentDays
	dateFrom := asOf.AddDate(0, 0, -totalDays)
	dateTo := asOf
	fmt.Printf("Fetching insider sells from %s to %s...\n", dateFrom.Format("2006-01-02"), dateTo.Format("2006-01-02"))

	records := aggregator.AggregateInsiderSells(tickers, dateFrom, dateTo)
	fmt.Printf("Aggregated %d insider sell records.\n", len(records))

	var signals []struct {
		Ticker            string
		CurrentSharesSold float64
		BaselineMean      float64
		BaselineStd       float64
		ZScore            float64
		IsAnomaly         bool
	}
	if rustclient.Available() {
		rustSignals, err := rustclient.ComputeAnomalySignals(records, *baselineDays, *currentDays, *stdThreshold, asOf.Format("2006-01-02"))
		if err == nil {
			for _, s := range rustSignals {
				signals = append(signals, struct {
					Ticker            string
					CurrentSharesSold float64
					BaselineMean      float64
					BaselineStd       float64
					ZScore            float64
					IsAnomaly         bool
				}{s.Ticker, s.CurrentSharesSold, s.BaselineMean, s.BaselineStd, s.ZScore, s.IsAnomaly})
			}
		}
	}
	if len(signals) == 0 {
		goSignals := aggregator.ComputeAnomalySignals(records, *baselineDays, *currentDays, *stdThreshold, asOf)
		for _, s := range goSignals {
			signals = append(signals, struct {
				Ticker            string
				CurrentSharesSold float64
				BaselineMean      float64
				BaselineStd       float64
				ZScore            float64
				IsAnomaly         bool
			}{s.Ticker, s.CurrentSharesSold, s.BaselineMean, s.BaselineStd, s.ZScore, s.IsAnomaly})
		}
	}

	if *listAll {
		fmt.Println("\nAll signals (current window vs baseline):")
		if len(signals) == 0 {
			fmt.Println("  (No data)")
		} else {
			for _, s := range signals {
				fmt.Printf("  %s  current=%.0f  mean=%.1f  std=%.1f  z=%.2f  anomaly=%v\n",
					s.Ticker, s.CurrentSharesSold, s.BaselineMean, s.BaselineStd, s.ZScore, s.IsAnomaly)
			}
		}
	} else {
		fmt.Println("\nAnomalous insider selling (above normal):")
		count := 0
		for _, s := range signals {
			if s.IsAnomaly {
				fmt.Printf("  %s  current=%.0f  mean=%.1f  std=%.1f  z=%.2f\n",
					s.Ticker, s.CurrentSharesSold, s.BaselineMean, s.BaselineStd, s.ZScore)
				count++
			}
		}
		if count == 0 {
			fmt.Println("  None detected.")
		}
	}

	if *csvPath != "" {
		f, err := os.Create(*csvPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create CSV: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w := csv.NewWriter(f)
		w.Write([]string{"ticker", "current_shares_sold", "baseline_mean", "baseline_std", "z_score", "is_anomaly"})
		for _, s := range signals {
			if *listAll || s.IsAnomaly {
				w.Write([]string{
					s.Ticker,
					fmt.Sprintf("%.0f", s.CurrentSharesSold),
					fmt.Sprintf("%.2f", s.BaselineMean),
					fmt.Sprintf("%.2f", s.BaselineStd),
					fmt.Sprintf("%.2f", s.ZScore),
					fmt.Sprintf("%v", s.IsAnomaly),
				})
			}
		}
		w.Flush()
		fmt.Printf("\nWrote %s.\n", *csvPath)
	}
}
