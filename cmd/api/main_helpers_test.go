package main

import "testing"

func TestIsValidSector_Valid(t *testing.T) {
	valid := []string{
		"Information Technology",
		"Financials",
		"Health Care",
		"Energy",
		"Communication Services",
		"Consumer Discretionary",
		"Consumer Staples",
		"Industrials",
		"Materials",
		"Real Estate",
		"Utilities",
		"Unknown",
	}
	for _, s := range valid {
		if !isValidSector(s) {
			t.Errorf("isValidSector(%q) = false; want true", s)
		}
	}
}

func TestIsValidSector_CaseInsensitive(t *testing.T) {
	if !isValidSector("energy") {
		t.Error("isValidSector(\"energy\") should be true (case insensitive)")
	}
	if !isValidSector("FINANCIALS") {
		t.Error("isValidSector(\"FINANCIALS\") should be true (case insensitive)")
	}
}

func TestIsValidSector_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"Technology",
		"<script>alert(1)</script>",
		"'; DROP TABLE sectors; --",
		"Crypto",
	}
	for _, s := range invalid {
		if isValidSector(s) {
			t.Errorf("isValidSector(%q) = true; want false", s)
		}
	}
}

func TestClamp(t *testing.T) {
	cases := []struct {
		v, lo, hi, want int
	}{
		{50, 0, 100, 50},
		{-1, 0, 100, 0},
		{200, 0, 100, 100},
		{0, 0, 0, 0},
		{30, 30, 730, 30},
		{731, 30, 730, 730},
	}
	for _, tc := range cases {
		got := clamp(tc.v, tc.lo, tc.hi)
		if got != tc.want {
			t.Errorf("clamp(%d, %d, %d) = %d; want %d", tc.v, tc.lo, tc.hi, got, tc.want)
		}
	}
}

func TestClampFloat(t *testing.T) {
	cases := []struct {
		v, lo, hi, want float64
	}{
		{2.0, 1.0, 5.0, 2.0},
		{0.5, 1.0, 5.0, 1.0},
		{6.0, 1.0, 5.0, 5.0},
		{1.0, 1.0, 5.0, 1.0},
		{5.0, 1.0, 5.0, 5.0},
	}
	for _, tc := range cases {
		got := clampFloat(tc.v, tc.lo, tc.hi)
		if got != tc.want {
			t.Errorf("clampFloat(%v, %v, %v) = %v; want %v", tc.v, tc.lo, tc.hi, got, tc.want)
		}
	}
}

func TestParseInt_Default(t *testing.T) {
	if got := parseInt("", 42); got != 42 {
		t.Errorf("parseInt(\"\", 42) = %d; want 42", got)
	}
}

func TestParseInt_ValidInput(t *testing.T) {
	if got := parseInt("10", 42); got != 10 {
		t.Errorf("parseInt(\"10\", 42) = %d; want 10", got)
	}
}

func TestParseInt_InvalidInput(t *testing.T) {
	if got := parseInt("abc", 42); got != 42 {
		t.Errorf("parseInt(\"abc\", 42) = %d; want 42", got)
	}
}

func TestParseFloat_Default(t *testing.T) {
	if got := parseFloat("", 2.5); got != 2.5 {
		t.Errorf("parseFloat(\"\", 2.5) = %v; want 2.5", got)
	}
}

func TestParseFloat_ValidInput(t *testing.T) {
	if got := parseFloat("3.14", 2.5); got != 3.14 {
		t.Errorf("parseFloat(\"3.14\", 2.5) = %v; want 3.14", got)
	}
}

func TestParseFloat_InvalidInput(t *testing.T) {
	if got := parseFloat("xyz", 2.5); got != 2.5 {
		t.Errorf("parseFloat(\"xyz\", 2.5) = %v; want 2.5", got)
	}
}
