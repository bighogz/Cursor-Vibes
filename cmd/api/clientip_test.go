package main

import (
	"net/http/httptest"
	"testing"
)

// These tests were added after the initial rateLimitScan used a naive
// strings.Split(xff, ",")[0] which trusts the *leftmost* (client-supplied)
// entry. An attacker behind a proxy could spoof their IP by prepending a
// fake address to X-Forwarded-For. The fix: trust the rightmost entry
// (added by the last trusted proxy) and validate with net.ParseIP.

func TestClientIP_RemoteAddrOnly(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	if got := clientIP(req); got != "10.0.0.1" {
		t.Errorf("clientIP() = %q; want %q (port should be stripped)", got, "10.0.0.1")
	}
}

func TestClientIP_XForwardedFor_SingleHop(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	if got := clientIP(req); got != "203.0.113.50" {
		t.Errorf("clientIP() = %q; want %q", got, "203.0.113.50")
	}
}

func TestClientIP_XForwardedFor_MultiHop_TrustsRightmost(t *testing.T) {
	// Attacker sends: X-Forwarded-For: 1.1.1.1
	// Proxy appends real IP: X-Forwarded-For: 1.1.1.1, 203.0.113.50
	// We trust the rightmost entry (added by proxy), not the leftmost (spoofable).
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.50")
	if got := clientIP(req); got != "203.0.113.50" {
		t.Errorf("clientIP() = %q; want %q (should trust rightmost)", got, "203.0.113.50")
	}
}

func TestClientIP_XForwardedFor_InvalidIP_Fallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := clientIP(req); got != "10.0.0.1" {
		t.Errorf("clientIP() = %q; want %q (invalid XFF should fall back to RemoteAddr)", got, "10.0.0.1")
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Real-Ip", "198.51.100.7")
	if got := clientIP(req); got != "198.51.100.7" {
		t.Errorf("clientIP() = %q; want %q", got, "198.51.100.7")
	}
}

func TestClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:8080"
	if got := clientIP(req); got != "::1" {
		t.Errorf("clientIP() = %q; want %q (IPv6 port strip)", got, "::1")
	}
}

func TestClientIP_XForwardedFor_PreferredOverXRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-Ip", "203.0.113.2")
	if got := clientIP(req); got != "203.0.113.1" {
		t.Errorf("clientIP() = %q; want %q (XFF takes priority)", got, "203.0.113.1")
	}
}
