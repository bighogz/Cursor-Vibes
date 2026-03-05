package secapi

import (
	"testing"
)

// parseFloat was refactored from a custom implementation to strconv.ParseFloat
// after discovering that whitespace-padded values from SEC EDGAR XML
// (e.g. " 15000.0 ") caused parse failures and silent zero values in
// insider records. This test verifies the trimming + parsing behavior.

func TestParseFloat_Normal(t *testing.T) {
	if got := parseFloat("123.45"); got != 123.45 {
		t.Errorf("parseFloat(%q) = %v; want 123.45", "123.45", got)
	}
}

func TestParseFloat_WhitespacePadded(t *testing.T) {
	if got := parseFloat("  15000.0  "); got != 15000.0 {
		t.Errorf("parseFloat(%q) = %v; want 15000.0 (should trim whitespace)", "  15000.0  ", got)
	}
}

func TestParseFloat_Empty(t *testing.T) {
	if got := parseFloat(""); got != 0 {
		t.Errorf("parseFloat(%q) = %v; want 0", "", got)
	}
}

func TestParseFloat_NotANumber(t *testing.T) {
	if got := parseFloat("N/A"); got != 0 {
		t.Errorf("parseFloat(%q) = %v; want 0 (non-numeric should default to 0)", "N/A", got)
	}
}

func TestParseFloat_NegativeValue(t *testing.T) {
	if got := parseFloat("-500.25"); got != -500.25 {
		t.Errorf("parseFloat(%q) = %v; want -500.25", "-500.25", got)
	}
}

func TestParseFloat_IntegerString(t *testing.T) {
	if got := parseFloat("42"); got != 42 {
		t.Errorf("parseFloat(%q) = %v; want 42", "42", got)
	}
}
