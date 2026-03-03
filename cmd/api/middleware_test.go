package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
)

func TestSafeStaticPath_Normal(t *testing.T) {
	got := safeStaticPath("static", "app.js")
	if got == "" {
		t.Fatal("expected non-empty path for valid input")
	}
}

func TestSafeStaticPath_Traversal(t *testing.T) {
	cases := []string{
		"../etc/passwd",
		"../../secret",
		"foo/../../etc/passwd",
	}
	for _, tc := range cases {
		got := safeStaticPath("static", tc)
		if got != "" {
			t.Errorf("safeStaticPath(%q) = %q; want empty (path traversal)", tc, got)
		}
	}
}

func TestSafeStaticPath_EmptyInput(t *testing.T) {
	got := safeStaticPath("static", "")
	if got != "" && got != "static" {
		t.Errorf("unexpected result for empty subpath: %q", got)
	}
}

func TestRateLimiter_AllowFirst(t *testing.T) {
	rl := newRateLimiter(1 * time.Second)
	if !rl.allow("10.0.0.1") {
		t.Fatal("first request should be allowed")
	}
}

func TestRateLimiter_BlockRepeat(t *testing.T) {
	rl := newRateLimiter(1 * time.Second)
	rl.allow("10.0.0.1")
	if rl.allow("10.0.0.1") {
		t.Fatal("second immediate request should be blocked")
	}
}

func TestRateLimiter_AllowAfterInterval(t *testing.T) {
	rl := newRateLimiter(10 * time.Millisecond)
	rl.allow("10.0.0.1")
	time.Sleep(15 * time.Millisecond)
	if !rl.allow("10.0.0.1") {
		t.Fatal("request after interval should be allowed")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := newRateLimiter(1 * time.Second)
	rl.allow("10.0.0.1")
	if !rl.allow("10.0.0.2") {
		t.Fatal("different key should be allowed")
	}
}

func TestAdminOrRateLimit_NoKeyConfigured(t *testing.T) {
	config.Load()
	origKey := config.AdminAPIKey
	config.AdminAPIKey = ""
	defer func() { config.AdminAPIKey = origKey }()

	called := false
	handler := adminOrRateLimit(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatal("handler should be called when no admin key is configured")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAdminOrRateLimit_ValidKey(t *testing.T) {
	config.Load()
	origKey := config.AdminAPIKey
	config.AdminAPIKey = "test-secret-key"
	defer func() { config.AdminAPIKey = origKey }()

	called := false
	handler := adminOrRateLimit(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Admin-Key", "test-secret-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatal("handler should be called with valid admin key")
	}
}

func TestAdminOrRateLimit_InvalidKey(t *testing.T) {
	config.Load()
	origKey := config.AdminAPIKey
	config.AdminAPIKey = "test-secret-key"
	defer func() { config.AdminAPIKey = origKey }()

	handler := adminOrRateLimit(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should NOT be called with invalid key")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Admin-Key", "wrong-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAdminOrRateLimit_BearerToken(t *testing.T) {
	config.Load()
	origKey := config.AdminAPIKey
	config.AdminAPIKey = "bearer-test"
	defer func() { config.AdminAPIKey = origKey }()

	called := false
	handler := adminOrRateLimit(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bearer-test")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatal("handler should be called with valid Bearer token")
	}
}

func TestSecurityHeaders_Present(t *testing.T) {
	handler := securityHeaders(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	headers := []string{
		"X-Request-Id",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
	}
	for _, h := range headers {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing security header: %s", h)
		}
	}
}

func TestSecurityHeaders_UniqueRequestIDs(t *testing.T) {
	handler := securityHeaders(func(w http.ResponseWriter, r *http.Request) {})

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		id := rec.Header().Get("X-Request-Id")
		if ids[id] {
			t.Fatalf("duplicate request ID after %d requests: %s", i, id)
		}
		ids[id] = true
	}
}

func TestMain(m *testing.M) {
	config.Load()
	os.Exit(m.Run())
}
