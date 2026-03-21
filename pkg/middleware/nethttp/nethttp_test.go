package nethttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"alldev-gin-rpc/pkg/gateway"
)

func TestRequestIDHeader(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), RequestID())

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if got := rw.Header().Get("X-Request-ID"); got == "" {
		t.Fatalf("expected X-Request-ID in response")
	}
}

func TestRateLimitFromGatewayConfig(t *testing.T) {
	cfg := &gateway.Config{RateLimit: gateway.RateLimitConfig{Enabled: true, Requests: 1, Window: "1m"}}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	wrapped := Chain(inner, RateLimitFromGatewayConfig(cfg))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	rw1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rw1, req)
	if rw1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rw1.Code)
	}

	rw2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rw2, req)
	if rw2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rw2.Code)
	}
}

func TestRecovery(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	wrapped := Chain(inner, Recovery())

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rw := httptest.NewRecorder()
	wrapped.ServeHTTP(rw, req)

	if rw.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rw.Code)
	}
}
