package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestIPRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewIPRateLimiter(10, 10, testLogger())

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/validate", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
}

func TestIPRateLimiter_BlocksOverLimit(t *testing.T) {
	// 1 request per second, burst of 2
	rl := NewIPRateLimiter(1, 2, testLogger())

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 2 requests should succeed (burst).
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/validate", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("burst request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// Third request should be rate-limited.
	req := httptest.NewRequest(http.MethodPost, "/validate", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", rec.Code)
	}

	if rec.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After header, got %q", rec.Header().Get("Retry-After"))
	}
}

func TestIPRateLimiter_DifferentIPsIndependent(t *testing.T) {
	// 1 request per second, burst of 1
	rl := NewIPRateLimiter(1, 1, testLogger())

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP A uses its burst.
	reqA := httptest.NewRequest(http.MethodPost, "/validate", nil)
	reqA.RemoteAddr = "1.1.1.1:1111"
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)

	if recA.Code != http.StatusOK {
		t.Errorf("IP A first request: expected 200, got %d", recA.Code)
	}

	// IP A is now rate-limited.
	reqA2 := httptest.NewRequest(http.MethodPost, "/validate", nil)
	reqA2.RemoteAddr = "1.1.1.1:1111"
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)

	if recA2.Code != http.StatusTooManyRequests {
		t.Errorf("IP A second request: expected 429, got %d", recA2.Code)
	}

	// IP B should still be allowed.
	reqB := httptest.NewRequest(http.MethodPost, "/validate", nil)
	reqB.RemoteAddr = "2.2.2.2:2222"
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	if recB.Code != http.StatusOK {
		t.Errorf("IP B first request: expected 200, got %d", recB.Code)
	}
}

func TestIPRateLimiter_XForwardedFor(t *testing.T) {
	rl := NewIPRateLimiter(1, 1, testLogger())

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request with X-Forwarded-For.
	req1 := httptest.NewRequest(http.MethodPost, "/validate", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	req1.Header.Set("X-Forwarded-For", "203.0.113.50")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request same forwarded IP: should be limited.
	req2 := httptest.NewRequest(http.MethodPost, "/validate", nil)
	req2.RemoteAddr = "127.0.0.1:5678"
	req2.Header.Set("X-Forwarded-For", "203.0.113.50")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request same forwarded IP: expected 429, got %d", rec2.Code)
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"

	ip := extractIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got %q", ip)
	}
}

func TestExtractIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "172.16.0.1")

	ip := extractIP(req)
	if ip != "172.16.0.1" {
		t.Errorf("expected '172.16.0.1', got %q", ip)
	}
}

func TestExtractIP_XForwardedForTakesPrecedence(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	req.Header.Set("X-Real-IP", "172.16.0.1")

	ip := extractIP(req)
	if ip != "8.8.8.8" {
		t.Errorf("expected '8.8.8.8', got %q", ip)
	}
}
