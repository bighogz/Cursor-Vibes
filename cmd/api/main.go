package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/aggregator"
	"github.com/bighogz/Cursor-Vibes/internal/cache"
	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/dashboard"
	"github.com/bighogz/Cursor-Vibes/internal/fmp"
	"github.com/bighogz/Cursor-Vibes/internal/rustclient"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load(".env")
}

func main() {
	http.HandleFunc("/", securityHeaders(serveIndex))
	http.HandleFunc("/static/", securityHeaders(serveStatic))
	http.HandleFunc("/api/dashboard", securityHeaders(handleDashboard))
	http.HandleFunc("/api/dashboard/refresh", securityHeaders(adminOrRateLimit(handleRefresh)))
	http.HandleFunc("/api/dashboard/meta", securityHeaders(handleMeta))
	http.HandleFunc("/api/scan", securityHeaders(adminOrRateLimit(rateLimitScan(handleScan))))
	http.HandleFunc("/api/health", securityHeaders(handleHealth))

	go startupRefresh()

	port := "8000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	http.ListenAndServe(":"+port, nil)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	dashPath := "static/dashboard.html"
	if _, err := os.Stat(dashPath); err == nil {
		http.ServeFile(w, r, dashPath)
		return
	}
	indexPath := "static/index.html"
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}
	jsonResponse(w, map[string]string{"message": "Frontend not found."})
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	subpath := strings.TrimPrefix(r.URL.Path, "/static/")
	subpath = strings.TrimPrefix(subpath, "/")
	if subpath == "" || strings.Contains(subpath, "..") {
		http.NotFound(w, r)
		return
	}
	path := safeStaticPath("static", subpath)
	if path == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	sector := strings.TrimSpace(q.Get("sector"))
	limit := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	// When sector or limit specified, build on-demand for companies being viewed
	if sector != "" || limit > 0 {
		opts := dashboard.BuildOpts{Sector: sector, Limit: limit, AsOf: time.Now()}
		if limit <= 0 {
			opts.Limit = 50
		}
		data := dashboard.Build(opts)
		jsonResponse(w, data)
		return
	}
	cached, ok := cache.Read(true)
	if ok {
		jsonResponse(w, cached)
		return
	}
	_, okFresh := cache.Read(false)
	if !okFresh {
		go refreshCache()
	}
	jsonResponse(w, map[string]interface{}{
		"error":   "Data is being prepared. Check back in a few minutes.",
		"sectors": []interface{}{},
	})
}

var refreshMu sync.Mutex
var lastRefreshAt time.Time
const refreshDebounce = 5 * time.Minute

func refreshCache() {
	refreshMu.Lock()
	defer refreshMu.Unlock()
	if !lastRefreshAt.IsZero() && time.Since(lastRefreshAt) < refreshDebounce {
		return
	}
	lastRefreshAt = time.Now()
	data := dashboard.Build(dashboard.BuildOpts{AsOf: time.Now()})
	if data["error"] == nil {
		cache.Write(data)
	}
}

func startupRefresh() {
	_, ok := cache.Read(false)
	if !ok {
		refreshCache()
	}
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	go refreshCache()
	jsonResponse(w, map[string]string{"status": "refresh started"})
}

func handleMeta(w http.ResponseWriter, r *http.Request) {
	t := cache.CachedAt()
	var s *string
	if t != nil {
		formatted := t.Format("2006-01-02T15:04:05Z07:00")
		s = &formatted
	}
	jsonResponse(w, map[string]interface{}{"last_updated": s})
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	limit := -1
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit < 0 {
		if config.FMPFreeTier {
			limit = 25
		} else {
			limit = 0
		}
	}
	baselineDays := clamp(parseInt(q.Get("baseline_days"), config.BaselineDays), 30, 730)
	currentDays := clamp(parseInt(q.Get("current_days"), config.CurrentWindowDays), 7, 90)
	stdThreshold := clampFloat(parseFloat(q.Get("std_threshold"), config.AnomalyStdThreshold), 1.0, 5.0)
	asOfStr := q.Get("as_of")
	asOf := time.Now()
	if asOfStr != "" {
		if t, err := time.Parse("2006-01-02", asOfStr); err == nil {
			asOf = t
		}
	}

	client := fmp.New()
	tickers := client.GetSP500Tickers()
	if len(tickers) == 0 {
		jsonResponse(w, map[string]interface{}{
			"error":         "Could not load S&P 500 constituents",
			"tickers_count": 0,
		})
		return
	}
	if limit > 0 {
		limit = min(clamp(limit, 1, 600), len(tickers))
		tickers = tickers[:limit]
	}
	totalDays := baselineDays + currentDays
	dateFrom := asOf.AddDate(0, 0, -totalDays)
	dateTo := asOf

	records := aggregator.AggregateInsiderSells(tickers, dateFrom, dateTo)

	var signalsList []map[string]interface{}
	var anomaliesList []map[string]interface{}

	// Use Rust anomaly engine when available; fall back to Go
	if rustclient.Available() {
		signals, err := rustclient.ComputeAnomalySignals(records, baselineDays, currentDays, stdThreshold, asOf.Format("2006-01-02"))
		if err == nil {
			for _, s := range signals {
				m := map[string]interface{}{
					"ticker":                s.Ticker,
					"current_shares_sold":   s.CurrentSharesSold,
					"baseline_mean":         s.BaselineMean,
					"baseline_std":          s.BaselineStd,
					"z_score":               s.ZScore,
					"is_anomaly":            s.IsAnomaly,
				}
				signalsList = append(signalsList, m)
				if s.IsAnomaly {
					anomaliesList = append(anomaliesList, m)
				}
			}
		}
	}
	if len(signalsList) == 0 {
		goSignals := aggregator.ComputeAnomalySignals(records, baselineDays, currentDays, stdThreshold, asOf)
		for _, s := range goSignals {
			m := map[string]interface{}{
				"ticker":                s.Ticker,
				"current_shares_sold":   s.CurrentSharesSold,
				"baseline_mean":         s.BaselineMean,
				"baseline_std":          s.BaselineStd,
				"z_score":               s.ZScore,
				"is_anomaly":            s.IsAnomaly,
			}
			signalsList = append(signalsList, m)
			if s.IsAnomaly {
				anomaliesList = append(anomaliesList, m)
			}
		}
	}

	jsonResponse(w, map[string]interface{}{
		"tickers_count":   len(tickers),
		"records_count":  len(records),
		"anomalies_count": len(anomaliesList),
		"date_from":       dateFrom.Format("2006-01-02"),
		"date_to":         dateTo.Format("2006-01-02"),
		"as_of":           asOf.Format("2006-01-02"),
		"params": map[string]interface{}{
			"baseline_days":  baselineDays,
			"current_days":   currentDays,
			"std_threshold":  stdThreshold,
		},
		"anomalies":   anomaliesList,
		"all_signals":  signalsList,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "ok"})
}

func jsonResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func parseFloat(s string, def float64) float64 {
	if s == "" {
		return def
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return f
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
