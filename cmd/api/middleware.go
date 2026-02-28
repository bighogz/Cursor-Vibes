package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
)

// securityHeaders adds security-related HTTP headers.
func securityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; connect-src 'self'; img-src 'self' data:;")
		next(w, r)
	}
}

const rateLimiterMaxSize = 10000
const rateLimiterEvictAge = time.Hour

// rateLimit limits requests per IP (simple in-memory, per-endpoint). Map size is capped.
type rateLimiter struct {
	mu       sync.Mutex
	last     map[string]time.Time
	interval time.Duration
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{
		last:     make(map[string]time.Time),
		interval: interval,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if len(rl.last) >= rateLimiterMaxSize {
		for k, t := range rl.last {
			if now.Sub(t) > rateLimiterEvictAge {
				delete(rl.last, k)
			}
		}
	}
	if t, ok := rl.last[key]; ok && now.Sub(t) < rl.interval {
		return false
	}
	rl.last[key] = now
	return true
}

var scanLimiter = newRateLimiter(5 * time.Second) // 1 scan per 5s per IP

func rateLimitScan(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if f := r.Header.Get("X-Forwarded-For"); f != "" {
			ip = strings.TrimSpace(strings.Split(f, ",")[0])
		}
		if !scanLimiter.allow(ip) {
			http.Error(w, `{"error":"rate limit: try again in a few seconds"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// adminOrRateLimit protects /api/scan and /api/dashboard/refresh when ADMIN_API_KEY is set.
func adminOrRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if config.AdminAPIKey == "" {
			next(w, r)
			return
		}
		key := r.Header.Get("X-Admin-Key")
		if key == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if key != config.AdminAPIKey {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"admin key required"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// safeStaticPath prevents path traversal. Returns clean path under staticDir or empty.
func safeStaticPath(staticDir, requestPath string) string {
	base := filepath.Clean(staticDir)
	joined := filepath.Join(base, filepath.Clean(requestPath))
	if !strings.HasPrefix(joined, base+string(filepath.Separator)) && joined != base {
		return ""
	}
	return joined
}
