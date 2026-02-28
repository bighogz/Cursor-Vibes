package main

import (
	"encoding/json"
	"fmt"
	"log"
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
	"github.com/bighogz/Cursor-Vibes/internal/yahoo"
)

func main() {
	config.Load()
	http.HandleFunc("/api/dashboard", securityHeaders(handleDashboard))
	http.HandleFunc("/api/dashboard/refresh", securityHeaders(adminOrRateLimit(handleRefresh)))
	http.HandleFunc("/api/dashboard/meta", securityHeaders(handleMeta))
	http.HandleFunc("/api/scan", securityHeaders(adminOrRateLimit(rateLimitScan(handleScan))))
	http.HandleFunc("/api/health", securityHeaders(handleHealth))
	http.HandleFunc("/api/health/providers", securityHeaders(handleProviders))

	// Static assets from React build (JS/CSS/images)
	http.HandleFunc("/assets/", securityHeaders(serveSPAAssets))
	// Legacy static files
	http.HandleFunc("/static/", securityHeaders(serveStatic))
	// SPA catch-all: serves React index.html for all non-API routes
	http.HandleFunc("/", securityHeaders(serveSPA))

	go startupRefresh()

	port := "8000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

// hasSPABuild returns true when the React frontend has been built.
func hasSPABuild() bool {
	_, err := os.Stat("frontend/dist/index.html")
	return err == nil
}

func serveSPA(w http.ResponseWriter, r *http.Request) {
	if hasSPABuild() {
		// For root or any client-side route, serve the SPA shell
		path := "frontend/dist" + r.URL.Path
		if r.URL.Path == "/" || !fileExists(path) {
			http.ServeFile(w, r, "frontend/dist/index.html")
			return
		}
		http.ServeFile(w, r, path)
		return
	}
	// Fallback to legacy static dashboard
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
	jsonResponse(w, map[string]string{"message": "Frontend not found. Run: cd frontend && npm run build"})
}

func serveSPAAssets(w http.ResponseWriter, r *http.Request) {
	subpath := strings.TrimPrefix(r.URL.Path, "/assets/")
	if subpath == "" || strings.Contains(subpath, "..") {
		http.NotFound(w, r)
		return
	}
	path := safeStaticPath("frontend/dist/assets", subpath)
	if path == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 600 {
			w.WriteHeader(http.StatusBadRequest)
			jsonResponse(w, map[string]string{"error": "limit must be an integer between 0 and 600"})
			return
		}
		limit = n
	}
	if sector != "" && !isValidSector(sector) {
		w.WriteHeader(http.StatusBadRequest)
		jsonResponse(w, map[string]string{"error": fmt.Sprintf("unknown sector: %q", sector)})
		return
	}
	if sector != "" || limit > 0 {
		if limit <= 0 {
			limit = 50
		}
		opts := dashboard.BuildOpts{Sector: sector, Limit: limit, AsOf: time.Now()}
		data := dashboard.Build(opts)
		w.Header().Set("X-Served-From", "on-demand")
		jsonResponse(w, data)
		return
	}
	cached, ok := cache.Read(true)
	if ok {
		w.Header().Set("X-Served-From", "cache")
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

	if rustclient.Available() {
		signals, err := rustclient.ComputeAnomalySignals(records, baselineDays, currentDays, stdThreshold, asOf.Format("2006-01-02"))
		if err == nil {
			for _, s := range signals {
				m := map[string]interface{}{
					"ticker":              s.Ticker,
					"current_shares_sold": s.CurrentSharesSold,
					"baseline_mean":       s.BaselineMean,
					"baseline_std":        s.BaselineStd,
					"z_score":             s.ZScore,
					"is_anomaly":          s.IsAnomaly,
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
				"ticker":              s.Ticker,
				"current_shares_sold": s.CurrentSharesSold,
				"baseline_mean":       s.BaselineMean,
				"baseline_std":        s.BaselineStd,
				"z_score":             s.ZScore,
				"is_anomaly":          s.IsAnomaly,
			}
			signalsList = append(signalsList, m)
			if s.IsAnomaly {
				anomaliesList = append(anomaliesList, m)
			}
		}
	}

	jsonResponse(w, map[string]interface{}{
		"tickers_count":  len(tickers),
		"records_count":  len(records),
		"anomalies_count": len(anomaliesList),
		"date_from":      dateFrom.Format("2006-01-02"),
		"date_to":        dateTo.Format("2006-01-02"),
		"as_of":          asOf.Format("2006-01-02"),
		"params": map[string]interface{}{
			"baseline_days": baselineDays,
			"current_days":  currentDays,
			"std_threshold": stdThreshold,
		},
		"anomalies":  anomaliesList,
		"all_signals": signalsList,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"status": "ok"})
}

func handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jsonResponse(w, yahoo.ProviderHealth())
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

// SI-10: restrict sector param to known GICS sectors + "Unknown".
var validSectors = func() map[string]bool {
	m := make(map[string]bool)
	for _, s := range []string{
		"Communication Services", "Consumer Discretionary", "Consumer Staples",
		"Energy", "Financials", "Health Care", "Industrials",
		"Information Technology", "Materials", "Real Estate", "Utilities",
		"Unknown",
	} {
		m[strings.ToLower(s)] = true
	}
	return m
}()

func isValidSector(s string) bool {
	return validSectors[strings.ToLower(s)]
}
