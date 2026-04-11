package aiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// InsiderEvent is a single insider transaction sent to the AI sidecar.
type InsiderEvent struct {
	Date        string   `json:"date"`
	InsiderName string   `json:"insider_name"`
	Role        *string  `json:"role,omitempty"`
	SharesSold  *float64 `json:"shares_sold,omitempty"`
	ValueUSD    *float64 `json:"value_usd,omitempty"`
}

// ExplainRequest is the payload POSTed to the AI sidecar.
type ExplainRequest struct {
	Ticker         string         `json:"ticker"`
	CompanyName    string         `json:"company_name"`
	Sector         *string        `json:"sector,omitempty"`
	AnomalyScore   float64        `json:"anomaly_score"`
	ZScore         *float64       `json:"z_score,omitempty"`
	TrendSummary   *string        `json:"trend_summary,omitempty"`
	CoverageWindow *string        `json:"coverage_window,omitempty"`
	SourceNotes    *string        `json:"source_notes,omitempty"`
	RecentEvents   []InsiderEvent `json:"recent_events"`
}

// ExplainResponse is the structured explanation returned by the AI sidecar.
type ExplainResponse struct {
	Summary string   `json:"summary"`
	Drivers []string `json:"drivers"`
	Caveats []string `json:"caveats"`
}

// Client talks to the Python AI sidecar.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New creates a Client with sensible defaults.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 4 * time.Minute,
		},
	}
}

// Explain sends an anomaly payload to the sidecar and returns the structured
// explanation. Returns a non-nil error on network failure, non-2xx status, or
// malformed response JSON.
func (c *Client) Explain(req *ExplainRequest) (*ExplainResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("aiclient: marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(
		c.BaseURL+"/explain-anomaly",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("aiclient: POST failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("aiclient: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500]
		}
		return nil, fmt.Errorf("aiclient: HTTP %d: %s", resp.StatusCode, preview)
	}

	var out ExplainResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("aiclient: decode response: %w", err)
	}
	return &out, nil
}
