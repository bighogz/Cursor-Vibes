package secapi

import (
	"encoding/xml"
	"testing"
)

// Fuzz targets for SEC EDGAR Form 4 XML parsing. These prove the parser
// won't panic on malformed financial data — a real concern when parsing
// XML from 30,000+ SEC filings per year.
//
// Run: go test -fuzz=FuzzParseForm4XML -fuzztime=30s ./internal/secapi/

func FuzzParseForm4XML(f *testing.F) {
	// Seed corpus with realistic Form 4 XML fragments
	f.Add([]byte(`<?xml version="1.0"?>
<ownershipDocument>
  <issuer><issuerTradingSymbol>AAPL</issuerTradingSymbol><issuerName>Apple Inc</issuerName></issuer>
  <reportingOwner>
    <reportingOwnerId><rptOwnerName>Tim Cook</rptOwnerName></reportingOwnerId>
    <reportingOwnerRelationship><isOfficer>1</isOfficer><officerTitle>CEO</officerTitle></reportingOwnerRelationship>
  </reportingOwner>
  <nonDerivativeTable>
    <nonDerivativeTransaction>
      <transactionDate><value>2026-01-15</value></transactionDate>
      <transactionCoding><transactionCode>S</transactionCode></transactionCoding>
      <transactionAmounts>
        <transactionShares><value>50000</value></transactionShares>
        <transactionPricePerShare><value>185.50</value></transactionPricePerShare>
        <transactionAcquiredDisposedCode><value>D</value></transactionAcquiredDisposedCode>
      </transactionAmounts>
    </nonDerivativeTransaction>
  </nonDerivativeTable>
</ownershipDocument>`))

	// Minimal valid document
	f.Add([]byte(`<ownershipDocument><issuer><issuerTradingSymbol>X</issuerTradingSymbol></issuer></ownershipDocument>`))

	// Empty document
	f.Add([]byte(`<ownershipDocument></ownershipDocument>`))

	// Corrupt/truncated XML
	f.Add([]byte(`<ownershipDocument><issuer><issuerTrad`))

	// Garbage
	f.Add([]byte(`not xml at all {{{`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var doc ownershipDocument
		// Must not panic regardless of input
		xml.Unmarshal(data, &doc)
	})
}

func FuzzParseFloat(f *testing.F) {
	f.Add("123.45")
	f.Add("")
	f.Add("  -0.001  ")
	f.Add("NaN")
	f.Add("Infinity")
	f.Add("1e308")
	f.Add("0000000001")
	f.Add("$1,234.56")

	f.Fuzz(func(t *testing.T, s string) {
		// Must not panic
		parseFloat(s)
	})
}

func FuzzParseDate(f *testing.F) {
	f.Add("2026-01-15")
	f.Add("")
	f.Add("not-a-date")
	f.Add("2026-13-40")
	f.Add("2026-01-15T10:30:00")
	f.Add("01/15/2026")

	f.Fuzz(func(t *testing.T, s string) {
		// Must not panic
		parseDate(s)
	})
}

func FuzzIsSaleTransaction(f *testing.F) {
	f.Add("S", "D")
	f.Add("P", "A")
	f.Add("", "")
	f.Add("s", "d")
	f.Add("D", "D")

	f.Fuzz(func(t *testing.T, code, acqDisp string) {
		isSaleTransaction(code, acqDisp)
	})
}
