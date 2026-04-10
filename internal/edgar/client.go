// Package edgar queries SEC EDGAR directly for Form 4 insider filings.
// No API key required — uses the free public EDGAR APIs:
//   - company_tickers.json   → ticker-to-CIK mapping
//   - submissions/CIK{}.json → recent filings per company
//   - Archives/...xml        → Form 4 XML for parsing
//
// Rate limit: 10 req/sec (enforced by 110ms sleep between requests).
// This serves as a backfill source for S&P 500 companies not covered by
// SEC-API.io or FMP.
package edgar

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const (
	tickerMapURL = "https://www.sec.gov/files/company_tickers.json"
	edgarAgent   = "VibesApp/1.0 (contact@vibes-demo.example)"
)

type Client struct{}

func New() *Client { return &Client{} }

// ticker→CIK cache (fetched once per process lifetime)
var (
	cikMapOnce sync.Once
	cikMap     map[string]int // "AAPL" → 320193
)

func loadCIKMap() {
	cikMapOnce.Do(func() {
		cikMap = make(map[string]int)
		req, err := http.NewRequest("GET", tickerMapURL, nil)
		if err != nil {
			log.Printf("edgar: CIK map request error: %v", err)
			return
		}
		req.Header.Set("User-Agent", edgarAgent)
		resp, err := httpclient.Default.Do(req)
		if err != nil {
			log.Printf("edgar: CIK map fetch error: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			log.Printf("edgar: CIK map status %d", resp.StatusCode)
			return
		}
		var raw map[string]struct {
			CIK    int    `json:"cik_str"`
			Ticker string `json:"ticker"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			log.Printf("edgar: CIK map decode error: %v", err)
			return
		}
		for _, entry := range raw {
			cikMap[strings.ToUpper(entry.Ticker)] = entry.CIK
		}
		log.Printf("edgar: loaded %d ticker→CIK mappings", len(cikMap))
	})
}

// GetInsiderSells finds the most recent Form 4 sale for each ticker in
// needTickers by querying EDGAR's company submissions API, then parsing
// the Form 4 XML. Returns all sell transactions found.
func (c *Client) GetInsiderSells(needTickers []string, dateFrom time.Time, maxXMLFetches int) []models.InsiderSellRecord {
	if len(needTickers) == 0 {
		return nil
	}
	if maxXMLFetches <= 0 {
		maxXMLFetches = 200
	}

	loadCIKMap()
	if len(cikMap) == 0 {
		log.Printf("edgar: no CIK map available, skipping")
		return nil
	}

	// Resolve tickers → CIKs
	type tickerCIK struct {
		Ticker string
		CIK    int
	}
	var resolved []tickerCIK
	for _, t := range needTickers {
		t = strings.ToUpper(t)
		if cik, ok := cikMap[t]; ok {
			resolved = append(resolved, tickerCIK{Ticker: t, CIK: cik})
		}
	}
	if len(resolved) == 0 {
		log.Printf("edgar: none of %d tickers matched CIK map", len(needTickers))
		return nil
	}
	log.Printf("edgar: resolved %d/%d tickers to CIKs", len(resolved), len(needTickers))

	// Phase 1: fetch submissions for each company and collect up to 15 recent
	// Form 4 refs per company. With broadened transaction types (S/D/M/F),
	// most filings will now yield a record, but we check multiple to maximize
	// the chance of finding at least one relevant transaction per company.
	var form4Refs []form4Ref
	companiesChecked := 0
	maxCompanies := min(len(resolved), 500)

	for i := 0; i < maxCompanies; i++ {
		tc := resolved[i]
		refs := c.findRecentForm4s(tc.CIK, tc.Ticker, dateFrom, 15)
		form4Refs = append(form4Refs, refs...)
		companiesChecked++
		time.Sleep(110 * time.Millisecond) // 10 req/sec
	}
	log.Printf("edgar: checked %d companies, found %d Form 4 filing refs", companiesChecked, len(form4Refs))

	if len(form4Refs) == 0 {
		return nil
	}

	// Phase 2: parse XMLs, stopping once we find a sale for each ticker or
	// hit the XML cap. Process refs grouped by ticker so we stop early for
	// tickers that already have a sale.
	var all []models.InsiderSellRecord
	xmlsParsed := 0
	tickerHasSale := make(map[string]bool)

	for _, ref := range form4Refs {
		if xmlsParsed >= maxXMLFetches {
			break
		}
		if tickerHasSale[ref.Ticker] {
			continue // Already found a sale for this ticker
		}
		recs := c.parseForm4XML(ref.CIK, ref.AccnNum, ref.PrimDoc, ref.Ticker, ref.Filed, dateFrom)
		if len(recs) > 0 {
			all = append(all, recs...)
			tickerHasSale[ref.Ticker] = true
		}
		xmlsParsed++
		time.Sleep(110 * time.Millisecond) // 10 req/sec
	}

	log.Printf("edgar: parsed %d Form 4 XMLs, got %d sell records", xmlsParsed, len(all))
	return all
}

type form4Ref struct {
	Ticker  string
	CIK     int
	AccnNum string
	PrimDoc string
	Filed   string
}

// findRecentForm4s queries the EDGAR submissions API for a company and returns
// up to maxRefs recent Form 4 filing references.
func (c *Client) findRecentForm4s(cik int, ticker string, dateFrom time.Time, maxRefs int) []form4Ref {
	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%010d.json", cik)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", edgarAgent)

	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	var sub struct {
		Filings struct {
			Recent struct {
				Form            []string `json:"form"`
				FilingDate      []string `json:"filingDate"`
				AccessionNumber []string `json:"accessionNumber"`
				PrimaryDocument []string `json:"primaryDocument"`
			} `json:"recent"`
		} `json:"filings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil
	}

	dateFromStr := dateFrom.Format("2006-01-02")
	recent := sub.Filings.Recent
	var refs []form4Ref
	for i, form := range recent.Form {
		if len(refs) >= maxRefs {
			break
		}
		if form != "4" {
			continue
		}
		if i >= len(recent.FilingDate) || i >= len(recent.AccessionNumber) || i >= len(recent.PrimaryDocument) {
			break
		}
		if recent.FilingDate[i] < dateFromStr {
			break // Filings are sorted newest-first; stop when too old
		}
		refs = append(refs, form4Ref{
			Ticker:  ticker,
			CIK:     cik,
			AccnNum: recent.AccessionNumber[i],
			PrimDoc: recent.PrimaryDocument[i],
			Filed:   recent.FilingDate[i],
		})
	}
	return refs
}

// parseForm4XML fetches a Form 4 XML from EDGAR and extracts sale transactions.
func (c *Client) parseForm4XML(cik int, accnNum, primDoc, ticker, filed string, dateFrom time.Time) []models.InsiderSellRecord {
	accnClean := strings.ReplaceAll(accnNum, "-", "")

	// If primary doc starts with "xslF345X05/", we need the actual XML
	xmlDoc := primDoc
	if idx := strings.LastIndex(primDoc, "/"); idx >= 0 {
		xmlDoc = primDoc[idx+1:]
	}

	url := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s",
		cik, accnClean, xmlDoc)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", edgarAgent)

	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var doc ownershipDocument
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil
	}

	sym := strings.ToUpper(strings.TrimSpace(doc.Issuer.Symbol))
	if sym == "" {
		sym = ticker
	}
	ownerName := strings.TrimSpace(doc.Owner.ID.Name)
	companyName := strings.TrimSpace(doc.Issuer.Name)

	role := ownerRole(
		doc.Owner.Relationship.IsDirector,
		doc.Owner.Relationship.IsOfficer,
		doc.Owner.Relationship.OfficerTitle,
		doc.Owner.Relationship.IsTenPctOwner,
	)

	filedTime, _ := time.Parse("2006-01-02", filed)

	var out []models.InsiderSellRecord
	for _, tx := range doc.NonDerivTable.Transactions {
		code := strings.ToUpper(strings.TrimSpace(tx.Coding.Code))
		acqDisp := strings.ToUpper(strings.TrimSpace(tx.Amounts.AcqDisp.Value))
		txType := classifyTxCode(code, acqDisp)
		if txType == "" {
			continue
		}
		txDateStr := strings.TrimSpace(tx.Date.Value)
		if len(txDateStr) > 10 {
			txDateStr = txDateStr[:10]
		}
		txDate, err := time.Parse("2006-01-02", txDateStr)
		if err != nil {
			continue
		}
		if txDate.Before(dateFrom) {
			continue
		}
		sh := parseFloatVal(tx.Amounts.Shares.Value)
		if sh <= 0 {
			continue
		}
		var valueUSD *float64
		if p := parseFloatVal(tx.Amounts.Price.Value); p > 0 {
			total := p * sh
			valueUSD = &total
		}
		var fp *time.Time
		if !filedTime.IsZero() {
			fp = &filedTime
		}
		var namePtr, rolePtr, compPtr *string
		if ownerName != "" {
			namePtr = &ownerName
		}
		if role != "" {
			rolePtr = &role
		}
		if companyName != "" {
			compPtr = &companyName
		}
		out = append(out, models.InsiderSellRecord{
			Ticker:          sym,
			CompanyName:     compPtr,
			InsiderName:     namePtr,
			Role:            rolePtr,
			TransactionDate: txDate,
			FilingDate:      fp,
			SharesSold:      sh,
			ValueUSD:        valueUSD,
			TxType:          txType,
			Source:          "edgar",
		})
	}
	return out
}

// Form 4 XML structures
type ownershipDocument struct {
	XMLName xml.Name `xml:"ownershipDocument"`
	Issuer  struct {
		Symbol string `xml:"issuerTradingSymbol"`
		Name   string `xml:"issuerName"`
	} `xml:"issuer"`
	Owner struct {
		ID struct {
			Name string `xml:"rptOwnerName"`
		} `xml:"reportingOwnerId"`
		Relationship struct {
			IsDirector    string `xml:"isDirector"`
			IsOfficer     string `xml:"isOfficer"`
			IsTenPctOwner string `xml:"isTenPercentOwner"`
			OfficerTitle  string `xml:"officerTitle"`
		} `xml:"reportingOwnerRelationship"`
	} `xml:"reportingOwner"`
	NonDerivTable struct {
		Transactions []nonDerivTx `xml:"nonDerivativeTransaction"`
	} `xml:"nonDerivativeTable"`
}

type nonDerivTx struct {
	Date   txValue `xml:"transactionDate"`
	Coding struct {
		Code string `xml:"transactionCode"`
	} `xml:"transactionCoding"`
	Amounts struct {
		Shares  txValue `xml:"transactionShares"`
		Price   txValue `xml:"transactionPricePerShare"`
		AcqDisp txValue `xml:"transactionAcquiredDisposedCode"`
	} `xml:"transactionAmounts"`
}

type txValue struct {
	Value string `xml:"value"`
}

func ownerRole(isDir, isOff, title, isTen string) string {
	var parts []string
	if isDir == "1" || strings.EqualFold(isDir, "true") {
		parts = append(parts, "Director")
	}
	if isOff == "1" || strings.EqualFold(isOff, "true") {
		if title != "" {
			parts = append(parts, title)
		} else {
			parts = append(parts, "Officer")
		}
	}
	if isTen == "1" || strings.EqualFold(isTen, "true") {
		parts = append(parts, "10% Owner")
	}
	return strings.Join(parts, ", ")
}

// classifyTxCode returns a human-readable transaction type label for Form 4
// transaction codes, or "" if the transaction should be skipped.
//   S = open-market sale, F = tax withholding on vesting,
//   M = option exercise, D+D = disposition
func classifyTxCode(code, acqDisp string) string {
	switch code {
	case "S":
		return "Sale"
	case "F":
		return "Tax"
	case "M":
		return "Exercise"
	case "D":
		if acqDisp == "D" {
			return "Disposition"
		}
	}
	return ""
}

func parseFloatVal(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
