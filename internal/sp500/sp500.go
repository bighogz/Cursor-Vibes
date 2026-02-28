package sp500

import (
	"encoding/csv"
	"strings"

	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
)

const csvURL = "https://raw.githubusercontent.com/datasets/s-and-p-500-companies/master/data/constituents.csv"

type Company struct {
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	Sector     string `json:"sector"`
	SubIndustry string `json:"sub_industry,omitempty"`
}

func Load() []Company {
	resp, err := httpclient.Default.Get(csvURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	reader := csv.NewReader(resp.Body)
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil
	}
	headers := rows[0]
	symIdx, nameIdx, sectorIdx, subIdx := -1, -1, -1, -1
	for i, h := range headers {
		switch strings.ToLower(h) {
		case "symbol":
			symIdx = i
		case "security":
			nameIdx = i
		case "gics sector":
			sectorIdx = i
		case "gics sub-industry":
			subIdx = i
		}
	}
	if symIdx < 0 {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]Company, 0)
	for _, row := range rows[1:] {
		if symIdx >= len(row) {
			continue
		}
		sym := strings.TrimSpace(row[symIdx])
		if sym == "" || seen[sym] {
			continue
		}
		seen[sym] = true
		c := Company{Symbol: sym, Sector: "Unknown"}
		if nameIdx >= 0 && nameIdx < len(row) {
			c.Name = strings.TrimSpace(row[nameIdx])
		}
		if sectorIdx >= 0 && sectorIdx < len(row) {
			if s := strings.TrimSpace(row[sectorIdx]); s != "" {
				c.Sector = s
			}
		}
		if subIdx >= 0 && subIdx < len(row) {
			c.SubIndustry = strings.TrimSpace(row[subIdx])
		}
		out = append(out, c)
	}
	return out
}
