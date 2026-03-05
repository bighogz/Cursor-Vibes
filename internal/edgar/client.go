// Package edgar queries SEC EDGAR directly for Form 4 insider filings.
// No API key required — uses the free public EDGAR EFTS search API and
// the company submissions endpoint. Rate limit: 10 req/sec.
//
// This serves as a backfill source for S&P 500 companies that SEC-API.io
// or FMP didn't cover. It queries the full-text search for recent Form 4
// filings and parses the XML just like the secapi client.
package edgar

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const (
	eftsURL    = "https://efts.sec.gov/LATEST/search-index"
	edgarAgent = "VibesApp/1.0 (contact@vibes-demo.example)"
)

// Client queries SEC EDGAR directly. No API key needed.
type Client struct{}

func New() *Client { return &Client{} }

// GetInsiderSells fetches Form 4 filings from EDGAR's full-text search for
// the given tickers, then parses the XML for sale transactions. Targets only
// tickers in needTickers (companies missing insider data).
func (c *Client) GetInsiderSells(needTickers []string, dateFrom time.Time, maxXMLFetches int) []models.InsiderSellRecord {
	if len(needTickers) == 0 {
		return nil
	}
	if maxXMLFetches <= 0 {
		maxXMLFetches = 200
	}

	// EFTS can search by ticker. Query in batches of 10 tickers.
	var allURLs []filingRef
	batchSize := 10
	for i := 0; i < len(needTickers) && i < 500; i += batchSize {
		end := min(i+batchSize, len(needTickers))
		batch := needTickers[i:end]
		refs := c.searchForm4Filings(batch, dateFrom)
		allURLs = append(allURLs, refs...)
		time.Sleep(110 * time.Millisecond) // 10 req/sec limit
	}
	if len(allURLs) == 0 {
		log.Printf("edgar: no Form 4 filings found for %d tickers", len(needTickers))
		return nil
	}

	var all []models.InsiderSellRecord
	parsed := 0
	for _, ref := range allURLs {
		if parsed >= maxXMLFetches {
			break
		}
		recs := c.parseForm4(ref, dateFrom)
		all = append(all, recs...)
		parsed++
		time.Sleep(110 * time.Millisecond) // 10 req/sec
	}
	log.Printf("edgar: found %d filing refs, parsed %d XMLs, got %d sell records", len(allURLs), parsed, len(all))
	return all
}

type filingRef struct {
	Ticker  string
	FileURL string
	Filed   time.Time
}

// searchForm4Filings uses EDGAR's EFTS search endpoint to find Form 4 filings.
func (c *Client) searchForm4Filings(tickers []string, dateFrom time.Time) []filingRef {
	// EFTS query: search for Form 4 filings matching tickers
	params := url.Values{}
	params.Set("q", strings.Join(tickers, " OR "))
	params.Set("dateRange", "custom")
	params.Set("startdt", dateFrom.Format("2006-01-02"))
	params.Set("enddt", time.Now().Format("2006-01-02"))
	params.Set("forms", "4")

	u := eftsURL + "?" + params.Encode()
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", edgarAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.Default.Do(req)
	if err != nil {
		log.Printf("edgar: EFTS search error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("edgar: EFTS search status %d", resp.StatusCode)
		return nil
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					FileDateD string `json:"file_date_d"`
					FileURL   string `json:"file_url"`
					Tickers   string `json:"tickers"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("edgar: EFTS decode error: %v", err)
		return nil
	}

	tickerSet := make(map[string]bool)
	for _, t := range tickers {
		tickerSet[strings.ToUpper(t)] = true
	}

	var refs []filingRef
	for _, hit := range result.Hits.Hits {
		fileURL := hit.Source.FileURL
		if fileURL == "" || !strings.HasSuffix(fileURL, ".xml") {
			continue
		}
		if !strings.HasPrefix(fileURL, "https://") {
			fileURL = "https://www.sec.gov/Archives/" + fileURL
		}
		filed, _ := time.Parse("2006-01-02", hit.Source.FileDateD[:min(10, len(hit.Source.FileDateD))])
		// Match to a requested ticker
		ticker := ""
		for _, t := range strings.Split(hit.Source.Tickers, ",") {
			t = strings.ToUpper(strings.TrimSpace(t))
			if tickerSet[t] {
				ticker = t
				break
			}
		}
		if ticker == "" {
			continue
		}
		refs = append(refs, filingRef{Ticker: ticker, FileURL: fileURL, Filed: filed})
	}
	return refs
}

// Form 4 XML structures (reused from secapi)
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

func (c *Client) parseForm4(ref filingRef, dateFrom time.Time) []models.InsiderSellRecord {
	req, err := http.NewRequest("GET", ref.FileURL, nil)
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

	ticker := strings.ToUpper(strings.TrimSpace(doc.Issuer.Symbol))
	if ticker == "" {
		ticker = ref.Ticker
	}
	ownerName := strings.TrimSpace(doc.Owner.ID.Name)
	companyName := strings.TrimSpace(doc.Issuer.Name)

	role := ownerRole(
		doc.Owner.Relationship.IsDirector,
		doc.Owner.Relationship.IsOfficer,
		doc.Owner.Relationship.OfficerTitle,
		doc.Owner.Relationship.IsTenPctOwner,
	)

	var out []models.InsiderSellRecord
	for _, tx := range doc.NonDerivTable.Transactions {
		code := strings.ToUpper(strings.TrimSpace(tx.Coding.Code))
		acqDisp := strings.ToUpper(strings.TrimSpace(tx.Amounts.AcqDisp.Value))
		if code != "S" && !(code == "D" && acqDisp == "D") {
			continue
		}
		txDate, err := time.Parse("2006-01-02", strings.TrimSpace(tx.Date.Value)[:min(10, len(strings.TrimSpace(tx.Date.Value)))])
		if err != nil {
			continue
		}
		if txDate.Before(dateFrom) {
			continue
		}
		sh := parseFloat(tx.Amounts.Shares.Value)
		if sh <= 0 {
			continue
		}
		var valueUSD *float64
		if p := parseFloat(tx.Amounts.Price.Value); p > 0 {
			total := p * sh
			valueUSD = &total
		}
		var filingDate *time.Time
		if !ref.Filed.IsZero() {
			filingDate = &ref.Filed
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
			Ticker:          ticker,
			CompanyName:     compPtr,
			InsiderName:     namePtr,
			Role:            rolePtr,
			TransactionDate: txDate,
			FilingDate:      filingDate,
			SharesSold:      sh,
			ValueUSD:        valueUSD,
			Source:          "edgar",
		})
	}
	return out
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

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
