package secapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	"github.com/bighogz/Cursor-Vibes/internal/httpclient"
	"github.com/bighogz/Cursor-Vibes/internal/models"
)

const queryURL = "https://api.sec-api.io"

type Client struct {
	APIKey string
}

func New() *Client {
	return &Client{APIKey: config.SECAPIKey}
}

// GetInsiderSells queries SEC-API.io for recent Form 4 filings matching
// tickers in tickerFilter, then fetches and parses each Form 4 XML from
// SEC EDGAR to extract sale transactions. Queries multiple batches of
// tickers to maximize coverage while respecting rate limits.
func (c *Client) GetInsiderSells(tickerFilter map[string]bool, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	if c.APIKey == "" {
		return nil
	}

	// Collect all tickers and split into batches of 30 (query string limit)
	var allTickers []string
	for t := range tickerFilter {
		allTickers = append(allTickers, t)
	}
	if len(allTickers) == 0 {
		return nil
	}

	// Query up to 17 batches (~500 tickers) with 50 filings each.
	// SEC-API.io rate limit is generous; SEC EDGAR XML fetches are the bottleneck.
	maxBatches := 17
	batchSize := 30
	maxXMLFetchTotal := 75
	var allFilings []form4Filing

	for b := 0; b < maxBatches && b*batchSize < len(allTickers); b++ {
		end := min((b+1)*batchSize, len(allTickers))
		batch := allTickers[b*batchSize : end]
		filings := c.queryForm4Filings(batch, dateFrom)
		allFilings = append(allFilings, filings...)
	}

	if len(allFilings) == 0 {
		return nil
	}

	// Fetch and parse Form 4 XMLs (respect SEC EDGAR 10 req/sec rate limit)
	var all []models.InsiderSellRecord
	parsed := 0
	for _, f := range allFilings {
		if parsed >= maxXMLFetchTotal {
			break
		}
		if f.XMLUrl == "" {
			continue
		}
		recs := c.parseForm4XML(f, dateFrom, dateTo)
		all = append(all, recs...)
		parsed++
		time.Sleep(120 * time.Millisecond)
	}
	log.Printf("secapi: queried %d filings, parsed %d XMLs, got %d sell records", len(allFilings), parsed, len(all))
	return all
}

type form4Filing struct {
	Ticker      string
	CompanyName string
	FiledAt     time.Time
	PeriodOf    string
	XMLUrl      string
}

func (c *Client) queryForm4Filings(tickers []string, dateFrom time.Time) []form4Filing {
	tickerQuery := strings.Join(tickers, " OR ")
	dateStr := dateFrom.Format("2006-01-02")

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"query_string": map[string]interface{}{
				"query": `formType:"4" AND ticker:(` + tickerQuery + `) AND filedAt:[` + dateStr + ` TO *]`,
			},
		},
		"from": "0",
		"size": "50",
		"sort": []map[string]interface{}{
			{"filedAt": map[string]string{"order": "desc"}},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		log.Printf("secapi: query marshal error: %v", err)
		return nil
	}

	req, err := http.NewRequest("POST", queryURL, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.Default.Do(req)
	if err != nil {
		log.Printf("secapi: query error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("secapi: query status %d", resp.StatusCode)
		return nil
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Printf("secapi: gzip error: %v", err)
			return nil
		}
		defer gr.Close()
		reader = gr
	}

	var result struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Filings []struct {
			Ticker      string `json:"ticker"`
			CompanyName string `json:"companyName"`
			FiledAt     string `json:"filedAt"`
			PeriodOf    string `json:"periodOfReport"`
			DocFiles    []struct {
				DocURL string `json:"documentUrl"`
				Type   string `json:"type"`
				Desc   string `json:"description"`
			} `json:"documentFormatFiles"`
		} `json:"filings"`
	}

	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		log.Printf("secapi: decode error: %v", err)
		return nil
	}

	var out []form4Filing
	for _, f := range result.Filings {
		var xmlURL string
		for _, doc := range f.DocFiles {
			if doc.Type == "4" && strings.HasSuffix(doc.DocURL, ".xml") && !strings.Contains(doc.DocURL, "xsl") {
				xmlURL = doc.DocURL
				break
			}
		}
		if xmlURL == "" {
			continue
		}
		filed, _ := time.Parse("2006-01-02T15:04:05-07:00", f.FiledAt)
		out = append(out, form4Filing{
			Ticker:      strings.ToUpper(f.Ticker),
			CompanyName: f.CompanyName,
			FiledAt:     filed,
			PeriodOf:    f.PeriodOf,
			XMLUrl:      xmlURL,
		})
	}
	log.Printf("secapi: found %d Form 4 filings (total in SEC: %d)", len(out), result.Total.Value)
	return out
}

// SEC Form 4 XML structures
type ownershipDocument struct {
	XMLName  xml.Name `xml:"ownershipDocument"`
	Issuer   struct {
		Symbol string `xml:"issuerTradingSymbol"`
		Name   string `xml:"issuerName"`
	} `xml:"issuer"`
	Owner struct {
		ID struct {
			Name string `xml:"rptOwnerName"`
		} `xml:"reportingOwnerId"`
		Relationship struct {
			IsDirector       string `xml:"isDirector"`
			IsOfficer        string `xml:"isOfficer"`
			IsTenPctOwner    string `xml:"isTenPercentOwner"`
			OfficerTitle     string `xml:"officerTitle"`
		} `xml:"reportingOwnerRelationship"`
	} `xml:"reportingOwner"`
	NonDerivTable struct {
		Transactions []nonDerivTx `xml:"nonDerivativeTransaction"`
	} `xml:"nonDerivativeTable"`
	DerivTable struct {
		Transactions []derivTx `xml:"derivativeTransaction"`
	} `xml:"derivativeTable"`
}

type nonDerivTx struct {
	Date    txValue `xml:"transactionDate"`
	Coding  struct {
		Code string `xml:"transactionCode"`
	} `xml:"transactionCoding"`
	Amounts struct {
		Shares  txValue `xml:"transactionShares"`
		Price   txValue `xml:"transactionPricePerShare"`
		AcqDisp txValue `xml:"transactionAcquiredDisposedCode"`
	} `xml:"transactionAmounts"`
}

type derivTx struct {
	Date    txValue `xml:"transactionDate"`
	Coding  struct {
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

func (c *Client) parseForm4XML(f form4Filing, dateFrom, dateTo time.Time) []models.InsiderSellRecord {
	req, err := http.NewRequest("GET", f.XMLUrl, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "VibesApp/1.0 (contact@example.com)")

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
		ticker = f.Ticker
	}
	ownerName := strings.TrimSpace(doc.Owner.ID.Name)
	role := ownerRole(doc.Owner.Relationship.IsDirector, doc.Owner.Relationship.IsOfficer,
		doc.Owner.Relationship.OfficerTitle, doc.Owner.Relationship.IsTenPctOwner)

	companyName := strings.TrimSpace(doc.Issuer.Name)

	var out []models.InsiderSellRecord
	process := func(date, code, shares, price, acqDisp string) {
		if !isSaleTransaction(code, acqDisp) {
			return
		}
		txDate, ok := parseDate(date)
		if !ok {
			return
		}
		if !dateFrom.IsZero() && txDate.Before(dateFrom) {
			return
		}
		if !dateTo.IsZero() && txDate.After(dateTo) {
			return
		}
		sh := parseFloat(shares)
		if sh <= 0 {
			return
		}
		var valueUSD *float64
		if p := parseFloat(price); p > 0 {
			total := p * sh
			valueUSD = &total
		}
		var filingDate *time.Time
		if !f.FiledAt.IsZero() {
			filingDate = &f.FiledAt
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
			Source:          "sec",
		})
	}

	for _, tx := range doc.NonDerivTable.Transactions {
		process(tx.Date.Value, tx.Coding.Code, tx.Amounts.Shares.Value,
			tx.Amounts.Price.Value, tx.Amounts.AcqDisp.Value)
	}
	for _, tx := range doc.DerivTable.Transactions {
		process(tx.Date.Value, tx.Coding.Code, tx.Amounts.Shares.Value,
			tx.Amounts.Price.Value, tx.Amounts.AcqDisp.Value)
	}
	return out
}

func isSaleTransaction(code, acqDisp string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	return code == "S" || (code == "D" && strings.ToUpper(strings.TrimSpace(acqDisp)) == "D")
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
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s[:min(10, len(s))])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
