package dashboard

import (
	"testing"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/models"
)

// quarterTrendData previously panicked on nil/empty input before we added
// the len(hist) < 2 guard. These tests ensure the guard stays.

func TestQuarterTrendData_Nil(t *testing.T) {
	if quarterTrendData(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestQuarterTrendData_Empty(t *testing.T) {
	if quarterTrendData([]map[string]interface{}{}) != nil {
		t.Error("empty input should return nil")
	}
}

func TestQuarterTrendData_SinglePoint(t *testing.T) {
	hist := []map[string]interface{}{
		{"date": "2026-01-01", "close": 100.0},
	}
	if quarterTrendData(hist) != nil {
		t.Error("single data point is insufficient for trend — should return nil")
	}
}

func TestQuarterTrendData_ValidSeries(t *testing.T) {
	// trend.FromCloses requires >= 30 valid points. Generate a 60-day series
	// with a clear upward trend (100 → 160).
	hist := make([]map[string]interface{}, 60)
	for i := range hist {
		hist[i] = map[string]interface{}{
			"date":  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02"),
			"close": 100.0 + float64(i),
		}
	}
	td := quarterTrendData(hist)
	if td == nil {
		t.Fatal("60 valid points should compute a trend")
	}
	if td.Pct <= 0 {
		t.Errorf("trend should be positive for upward series, got %v", td.Pct)
	}
}

func TestQuarterTrendData_ZeroCloses_Filtered(t *testing.T) {
	hist := []map[string]interface{}{
		{"date": "2026-01-01", "close": 0.0},
		{"date": "2026-02-01", "close": 0.0},
		{"date": "2026-03-01", "close": 100.0},
	}
	// Zero closes are filtered out. After filtering: only 1 point → nil.
	td := quarterTrendData(hist)
	if td != nil {
		t.Error("all-zero closes except one should return nil (insufficient data)")
	}
}

func TestSampleWeekly_ShortSeries(t *testing.T) {
	closes := []float64{1, 2, 3}
	weekly := sampleWeekly(closes)
	if len(weekly) != 3 {
		t.Errorf("short series should return all points, got %d", len(weekly))
	}
}

func TestSampleWeekly_LongSeries(t *testing.T) {
	closes := make([]float64, 65)
	for i := range closes {
		closes[i] = float64(i)
	}
	weekly := sampleWeekly(closes)
	if len(weekly) < 13 || len(weekly) > 15 {
		t.Errorf("65-day series should produce ~13 weekly samples, got %d", len(weekly))
	}
	if weekly[len(weekly)-1] != 64 {
		t.Error("last sample should be the last close")
	}
}

// topInsidersByTicker was refactored from map[string]interface{} to typed
// InsiderEntry. This test ensures deduplication and top-5 cap work correctly.

func TestTopInsidersByTicker_DedupAndCap(t *testing.T) {
	name := func(s string) *string { return &s }
	records := make([]models.InsiderSellRecord, 0)
	for i := 0; i < 8; i++ {
		n := name("Insider" + string(rune('A'+i)))
		records = append(records, models.InsiderSellRecord{
			Ticker:          "AAPL",
			InsiderName:     n,
			SharesSold:      float64((i + 1) * 1000),
			TransactionDate: time.Now(),
			Source:          "sec",
		})
	}

	top := topInsidersByTicker(records)
	aapl := top["AAPL"]
	if len(aapl) != 5 {
		t.Fatalf("expected top 5, got %d", len(aapl))
	}
	// Should be sorted descending by shares
	if aapl[0].Shares != 8000 {
		t.Errorf("first insider should have 8000 shares, got %.0f", aapl[0].Shares)
	}
	if aapl[4].Shares != 4000 {
		t.Errorf("5th insider should have 4000 shares, got %.0f", aapl[4].Shares)
	}
}

func TestTopInsidersByTicker_Empty(t *testing.T) {
	top := topInsidersByTicker(nil)
	if len(top) != 0 {
		t.Error("nil input should return empty map")
	}
}

func TestTopInsidersByTicker_SourcePreserved(t *testing.T) {
	name := "Test User"
	records := []models.InsiderSellRecord{
		{Ticker: "MSFT", InsiderName: &name, SharesSold: 500, TransactionDate: time.Now(), Source: "sec"},
	}
	top := topInsidersByTicker(records)
	if top["MSFT"][0].Source != "sec" {
		t.Errorf("source should be 'sec', got %q", top["MSFT"][0].Source)
	}
}
