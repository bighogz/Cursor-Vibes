package main

import (
	"testing"

	"github.com/bighogz/Cursor-Vibes/internal/dashboard"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

// filterDashboard was originally written with map[string]interface{} and
// required type-switching between []map[string]interface{} (live build) and
// []interface{} (JSON-deserialized cache). This caused a bug where cold-start
// cache data silently produced empty sectors. The typed refactor eliminates
// this class of bug entirely.

func sampleResult() *dashboard.Result {
	p1, p2 := 150.0, 200.0
	return &dashboard.Result{
		AsOf:           "2026-03-04",
		TotalCompanies: 3,
		Sectors: []models.SectorGroup{
			{
				Name: "Technology",
				Companies: []models.Company{
					{Symbol: "AAPL", Name: "Apple", Price: &p1},
					{Symbol: "MSFT", Name: "Microsoft", Price: &p2},
				},
			},
			{
				Name: "Financials",
				Companies: []models.Company{
					{Symbol: "JPM", Name: "JPMorgan"},
				},
			},
		},
		AvailableSectors: []string{"Financials", "Technology"},
	}
}

func TestFilterDashboard_NoFilter(t *testing.T) {
	r := sampleResult()
	out := filterDashboard(r, "", 0)
	if out != r {
		t.Error("no filter should return the same pointer")
	}
}

func TestFilterDashboard_BySector(t *testing.T) {
	out := filterDashboard(sampleResult(), "Technology", 0)
	if len(out.Sectors) != 1 {
		t.Fatalf("expected 1 sector, got %d", len(out.Sectors))
	}
	if out.Sectors[0].Name != "Technology" {
		t.Errorf("expected Technology sector, got %s", out.Sectors[0].Name)
	}
	if out.TotalCompanies != 2 {
		t.Errorf("expected 2 companies, got %d", out.TotalCompanies)
	}
}

func TestFilterDashboard_BySector_CaseInsensitive(t *testing.T) {
	out := filterDashboard(sampleResult(), "financials", 0)
	if len(out.Sectors) != 1 || out.Sectors[0].Name != "Financials" {
		t.Error("sector filter should be case-insensitive")
	}
}

func TestFilterDashboard_ByLimit(t *testing.T) {
	out := filterDashboard(sampleResult(), "", 2)
	if out.TotalCompanies != 2 {
		t.Errorf("expected 2 companies (limit=2), got %d", out.TotalCompanies)
	}
}

func TestFilterDashboard_LimitAcrossSectors(t *testing.T) {
	out := filterDashboard(sampleResult(), "", 3)
	total := 0
	for _, s := range out.Sectors {
		total += len(s.Companies)
	}
	if total != 3 {
		t.Errorf("expected 3 companies across sectors, got %d", total)
	}
}

func TestFilterDashboard_UnknownSector(t *testing.T) {
	out := filterDashboard(sampleResult(), "Crypto", 0)
	if len(out.Sectors) != 0 {
		t.Error("unknown sector should return empty sectors")
	}
	if out.TotalCompanies != 0 {
		t.Error("unknown sector should have 0 companies")
	}
}

func TestFilterDashboard_PreservesMetadata(t *testing.T) {
	out := filterDashboard(sampleResult(), "Technology", 0)
	if out.AsOf != "2026-03-04" {
		t.Error("filter should preserve AsOf")
	}
	if len(out.AvailableSectors) != 2 {
		t.Error("filter should preserve AvailableSectors")
	}
}
