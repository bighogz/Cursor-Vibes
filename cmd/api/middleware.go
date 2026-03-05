package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bighogz/Cursor-Vibes/internal/config"
	viotel "github.com/bighogz/Cursor-Vibes/internal/otel"
	"go.opentelemetry.io/otel/attribute"
)

// securityHeaders adds security-related HTTP headers + request ID logging (AU-12).
func securityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := viotel.StartSpan(r.Context(), "HTTP "+r.Method+" "+r.URL.Path)
		defer span.End()
		r = r.WithContext(ctx)

		reqID := generateRequestID()
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.path", r.URL.Path),
			attribute.String("request.id", reqID),
		)

		w.Header().Set("X-Request-Id", reqID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; connect-src 'self'; img-src 'self' data:;")

		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: 200}
		next(rw, r)
		elapsed := time.Since(start)
		span.SetAttributes(
			attribute.Int("http.status_code", rw.status),
			attribute.Int64("http.duration_ms", elapsed.Milliseconds()),
		)
		log.Printf("[%s] %s %s %d %s", reqID, r.Method, r.URL.Path, rw.status, elapsed.Round(time.Millisecond))
	}
}

// requestIDCounter is a monotonic fallback when crypto/rand fails.
var requestIDCounter uint64

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback: timestamp-based ID. This path only triggers on entropy
		// exhaustion, which is effectively impossible on modern kernels, but
		// failing open with a predictable-but-unique ID is safer than panicking.
		counter := atomic.AddUint64(&requestIDCounter, 1)
		return fmt.Sprintf("%x-%x", time.Now().UnixNano(), counter)
	}
	return hex.EncodeToString(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wrote {
		sw.status = code
		sw.wrote = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wrote {
		sw.wrote = true
	}
	return sw.ResponseWriter.Write(b)
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
		if !scanLimiter.allow(clientIP(r)) {
			http.Error(w, `{"error":"rate limit: try again in a few seconds"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// clientIP extracts the client's IP address from the request. When behind a
// reverse proxy, the proxy appends the real client IP to X-Forwarded-For. We
// trust the *rightmost* entry (added by the closest trusted proxy), not the
// leftmost (which the client can spoof). Falls back to RemoteAddr with the
// port stripped.
//
// For production behind a known proxy count, you'd take xff[len(xff)-trustedHops]
// instead. Single-proxy (nginx → app) means trustedHops=1, which is the default.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		// Trust rightmost entry: the one added by our reverse proxy.
		// In a multi-hop setup, you'd subtract the number of trusted proxies.
		ip := strings.TrimSpace(parts[len(parts)-1])
		if parsed := net.ParseIP(ip); parsed != nil {
			return parsed.String()
		}
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		if parsed := net.ParseIP(strings.TrimSpace(xri)); parsed != nil {
			return parsed.String()
		}
	}
	// RemoteAddr is "ip:port" — strip the port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
		if subtle.ConstantTimeCompare([]byte(key), []byte(config.AdminAPIKey)) != 1 {
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
