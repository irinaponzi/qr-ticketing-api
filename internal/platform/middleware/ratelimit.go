package middleware

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter implements per-IP rate limiting using a token bucket algorithm.
// Each unique IP address gets its own limiter with the configured rate and burst.
// Stale entries are cleaned up periodically to avoid unbounded memory growth.
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rate     rate.Limit
	burst    int
	logger   *slog.Logger
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a new IP-based rate limiter.
//
// Parameters:
//   - rps: Maximum requests per second allowed per IP.
//   - burst: Maximum burst size (allows short spikes above the rate).
//   - logger: Structured logger for observability.
//
// Returns:
//   - *IPRateLimiter: A pointer to the newly created rate limiter.
func NewIPRateLimiter(rps float64, burst int, logger *slog.Logger) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rate.Limit(rps),
		burst:    burst,
		logger:   logger,
	}

	go rl.cleanup()

	return rl
}

// getLimiter returns the rate limiter for the given IP, creating one if needed.
func (rl *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[ip] = &rateLimiterEntry{limiter: limiter, lastSeen: time.Now()}

		return limiter
	}

	entry.lastSeen = time.Now()

	return entry.limiter
}

// cleanup removes stale IP entries every 5 minutes to prevent memory leaks.
func (rl *IPRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()

		for ip, entry := range rl.limiters {
			if time.Since(entry.lastSeen) > 10*time.Minute {
				delete(rl.limiters, ip)
			}
		}

		rl.mu.Unlock()
	}
}

// Middleware returns an HTTP middleware that enforces per-IP rate limiting.
// When a client exceeds the configured rate, the middleware responds with
// HTTP 429 Too Many Requests and a Retry-After header.
func (rl *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			rl.logger.Warn("rate limit exceeded", "ip", ip, "path", r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)

			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "rate limit exceeded, try again later",
			})

			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP returns the client IP from X-Forwarded-For, X-Real-IP,
// or falls back to RemoteAddr.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}
