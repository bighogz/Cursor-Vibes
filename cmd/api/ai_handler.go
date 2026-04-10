package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/aggregator"
	"github.com/bighogz/Cursor-Vibes/internal/aiclient"
	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/dashboard"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

var aiClient *aiclient.Client

func initAIClient() {
	aiClient = aiclient.New(config.AIServiceURL)
}

// handleAIExplain proxies anomaly explanation requests to the Python AI sidecar.
//
//	GET /api/ai/explain-anomaly?ticker=MSFT
//
// 1. Looks up the company in the in-memory dashboard store.
// 2. Assembles a payload from dashboard data (no live API calls).
// 3. Forwards to the Python sidecar via aiclient.Client.
func handleAIExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	ticker := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("ticker")))
	if ticker == "" {
		w.WriteHeader(http.StatusBadRequest)
		jsonResponse(w, map[string]string{"error": "ticker query parameter required"})
		return
	}

	stored := dashStore.Load()
	if stored == nil {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
		jsonResponse(w, map[string]string{"error": "dashboard still building"})
		return
	}
	full := stored.(*dashboard.Result)

	company, sector := findCompany(full, ticker)
	if company == nil {
		w.WriteHeader(http.StatusNotFound)
		jsonResponse(w, map[string]string{"error": fmt.Sprintf("ticker %q not found in dashboard", ticker)})
		return
	}

	payload := buildAIPayload(company, sector, ticker)

	resp, err := aiClient.Explain(payload)
	if err != nil {
		log.Printf("ai sidecar error for %s: %v", ticker, err)
		w.WriteHeader(http.StatusBadGateway)
		jsonResponse(w, map[string]string{"error": "AI service unavailable"})
		return
	}

	jsonResponse(w, resp)
}

func findCompany(full *dashboard.Result, ticker string) (*models.Company, string) {
	for _, sec := range full.Sectors {
		for i := range sec.Companies {
			if strings.EqualFold(sec.Companies[i].Symbol, ticker) {
				return &sec.Companies[i], sec.Name
			}
		}
	}
	return nil, ""
}

func buildAIPayload(c *models.Company, sector, ticker string) *aiclient.ExplainRequest {
	req := &aiclient.ExplainRequest{
		Ticker:       ticker,
		CompanyName:  c.Name,
		RecentEvents: make([]aiclient.InsiderEvent, 0),
	}
	if sector != "" {
		req.Sector = &sector
	}

	if c.QuarterTrend != nil {
		ts := fmt.Sprintf("Quarterly price trend: %.2f%%", *c.QuarterTrend)
		req.TrendSummary = &ts
	}

	cw := fmt.Sprintf("%d days", config.BaselineDays+config.CurrentWindowDays)
	req.CoverageWindow = &cw

	// Compute real z-score from cached insider records (pure math, no API calls).
	cached := aggregator.LoadCachedRecords([]string{ticker})
	if len(cached) > 0 {
		signals := aggregator.ComputeAnomalySignals(
			cached,
			config.BaselineDays,
			config.CurrentWindowDays,
			config.AnomalyStdThreshold,
			time.Now(),
		)
		for _, s := range signals {
			if strings.EqualFold(s.Ticker, ticker) {
				req.AnomalyScore = s.ZScore
				req.ZScore = &s.ZScore
				break
			}
		}
	}

	if len(c.Sources) > 0 {
		parts := make([]string, 0, len(c.Sources))
		for k, v := range c.Sources {
			parts = append(parts, k+": "+v)
		}
		sn := strings.Join(parts, ", ")
		req.SourceNotes = &sn
	}

	now := time.Now().Format("2006-01-02")
	for _, ins := range c.TopInsiders {
		evt := aiclient.InsiderEvent{
			Date:        now,
			InsiderName: ins.Name,
		}
		if ins.Role != nil {
			evt.Role = ins.Role
		}
		if ins.Shares > 0 {
			s := ins.Shares
			evt.SharesSold = &s
		}
		if ins.Value != nil {
			evt.ValueUSD = ins.Value
		}
		req.RecentEvents = append(req.RecentEvents, evt)
	}

	return req
}
