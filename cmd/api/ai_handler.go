package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/dashboard"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

type aiInsiderEvent struct {
	Date        string   `json:"date"`
	InsiderName string   `json:"insider_name"`
	Role        *string  `json:"role,omitempty"`
	SharesSold  *float64 `json:"shares_sold,omitempty"`
	ValueUSD    *float64 `json:"value_usd,omitempty"`
}

type aiRequest struct {
	Ticker         string           `json:"ticker"`
	CompanyName    string           `json:"company_name"`
	Sector         *string          `json:"sector,omitempty"`
	AnomalyScore   float64          `json:"anomaly_score"`
	ZScore         *float64         `json:"z_score,omitempty"`
	TrendSummary   *string          `json:"trend_summary,omitempty"`
	CoverageWindow *string          `json:"coverage_window,omitempty"`
	SourceNotes    *string          `json:"source_notes,omitempty"`
	RecentEvents   []aiInsiderEvent `json:"recent_events"`
}

// handleAIExplain proxies anomaly explanation requests to the Python AI sidecar.
//
//	GET /api/ai/explain-anomaly?ticker=MSFT
//
// 1. Looks up the company in the in-memory dashboard store.
// 2. Computes a quick anomaly signal for the ticker.
// 3. Assembles a payload and POSTs it to the sidecar.
// 4. Streams the sidecar response back to the caller.
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

	body, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		jsonResponse(w, map[string]string{"error": "failed to marshal AI payload"})
		return
	}

	sidecarURL := config.AIServiceURL + "/explain-anomaly"
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(sidecarURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("ai sidecar error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
		jsonResponse(w, map[string]string{"error": "AI service unavailable"})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
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

func buildAIPayload(c *models.Company, sector, ticker string) aiRequest {
	req := aiRequest{
		Ticker:       ticker,
		CompanyName:  c.Name,
		RecentEvents: make([]aiInsiderEvent, 0),
	}
	if sector != "" {
		req.Sector = &sector
	}

	// Trend summary from dashboard data
	if c.QuarterTrend != nil {
		ts := fmt.Sprintf("Quarterly price trend: %.2f%%", *c.QuarterTrend)
		req.TrendSummary = &ts
	}

	// Coverage window from config
	cw := fmt.Sprintf("%d days", config.BaselineDays+config.CurrentWindowDays)
	req.CoverageWindow = &cw

	// Anomaly score derived from insider activity volume in the dashboard.
	// For a precise z-score, the user should run /api/scan first.
	var totalShares float64
	for _, ins := range c.TopInsiders {
		totalShares += ins.Shares
	}
	if totalShares > 0 {
		req.AnomalyScore = totalShares / 10000 // rough magnitude indicator
	}

	// Source notes
	if len(c.Sources) > 0 {
		parts := make([]string, 0, len(c.Sources))
		for k, v := range c.Sources {
			parts = append(parts, k+": "+v)
		}
		sn := strings.Join(parts, ", ")
		req.SourceNotes = &sn
	}

	// Insider events from dashboard (already in memory — no API calls)
	now := time.Now().Format("2006-01-02")
	for _, ins := range c.TopInsiders {
		evt := aiInsiderEvent{
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
