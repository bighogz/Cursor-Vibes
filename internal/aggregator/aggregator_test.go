package aggregator

import (
	"testing"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/models"
)

// keyFor deduplication was added after discovering that the same insider
// transaction from FMP and SEC-API appeared twice in the dashboard (same
// person, same date, same shares, different source). The key is:
//   TICKER|DATE|NAME|SHARES
// This test verifies the dedup logic directly.

func TestKeyFor_DedupsSameTransaction(t *testing.T) {
	name := "John Smith"
	r1 := models.InsiderSellRecord{
		Ticker:          "AAPL",
		InsiderName:     &name,
		TransactionDate: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		SharesSold:      10000,
		Source:          "fmp",
	}
	r2 := r1
	r2.Source = "sec"

	if keyFor(r1) != keyFor(r2) {
		t.Error("same transaction from different sources should have same key")
	}
}

func TestKeyFor_DifferentDates(t *testing.T) {
	name := "John Smith"
	r1 := models.InsiderSellRecord{
		Ticker:          "AAPL",
		InsiderName:     &name,
		TransactionDate: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		SharesSold:      10000,
		Source:          "fmp",
	}
	r2 := r1
	r2.TransactionDate = time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)

	if keyFor(r1) == keyFor(r2) {
		t.Error("different dates should produce different keys")
	}
}

func TestKeyFor_DifferentShares(t *testing.T) {
	name := "John Smith"
	r1 := models.InsiderSellRecord{
		Ticker:          "AAPL",
		InsiderName:     &name,
		TransactionDate: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		SharesSold:      10000,
		Source:          "fmp",
	}
	r2 := r1
	r2.SharesSold = 5000

	if keyFor(r1) == keyFor(r2) {
		t.Error("different share counts should produce different keys")
	}
}

func TestKeyFor_NilName(t *testing.T) {
	r := models.InsiderSellRecord{
		Ticker:          "AAPL",
		TransactionDate: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		SharesSold:      10000,
		Source:          "sec",
	}
	k := keyFor(r)
	if k == "" {
		t.Error("nil name should not produce empty key")
	}
}

func TestKeyFor_CaseNormalization(t *testing.T) {
	name := "Jane Doe"
	r1 := models.InsiderSellRecord{
		Ticker:          "aapl",
		InsiderName:     &name,
		TransactionDate: time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC),
		SharesSold:      1000,
	}
	r2 := r1
	r2.Ticker = "AAPL"

	if keyFor(r1) != keyFor(r2) {
		t.Error("ticker case should be normalized in key")
	}
}

func TestMeanStd_Empty(t *testing.T) {
	m, s := meanStd(nil)
	if m != 0 || s != 0 {
		t.Errorf("empty input should return 0,0; got %v,%v", m, s)
	}
}

func TestMeanStd_SingleValue(t *testing.T) {
	m, s := meanStd([]float64{42})
	if m != 42 {
		t.Errorf("mean of [42] should be 42, got %v", m)
	}
	if s != 0 {
		t.Errorf("std of single value should be 0, got %v", s)
	}
}

func TestMeanStd_KnownValues(t *testing.T) {
	m, s := meanStd([]float64{2, 4, 4, 4, 5, 5, 7, 9})
	if m != 5 {
		t.Errorf("mean should be 5, got %v", m)
	}
	if s < 1.99 || s > 2.01 {
		t.Errorf("std should be ~2.0, got %v", s)
	}
}
