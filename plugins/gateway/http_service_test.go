package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPServiceRouting(t *testing.T) {
	cfg := DefaultConfig()

	bizHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("biz"))
	})
	isBizPath := func(p string) bool { return strings.HasPrefix(p, "/debug/") }

	customMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "1")
			next.ServeHTTP(w, r)
		})
	}

	svc, err := NewHTTPServiceWithOptions(cfg, HTTPServiceOptions{
		BizHandler:     bizHandler,
		IsBusinessPath: isBizPath,
		Middlewares:    []Middleware{customMW},
	})
	if err != nil {
		t.Fatalf("NewHTTPService error: %v", err)
	}
	defer func() { _ = svc.Close() }()

	// Business path
	rw1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/debug/ok", nil)
	svc.Handler().ServeHTTP(rw1, req1)
	if rw1.Code != http.StatusOK {
		t.Fatalf("expected 200 for biz path, got %d", rw1.Code)
	}
	if got := rw1.Header().Get("X-Custom"); got != "1" {
		t.Fatalf("expected custom middleware header on biz path, got %q", got)
	}
	if got := strings.TrimSpace(rw1.Body.String()); got != "biz" {
		t.Fatalf("expected biz body, got %q", got)
	}

	// Gateway path
	rw2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	svc.Handler().ServeHTTP(rw2, req2)
	if rw2.Code != http.StatusOK {
		t.Fatalf("expected 200 for gateway path, got %d", rw2.Code)
	}
	if got := rw2.Header().Get("X-Custom"); got != "1" {
		t.Fatalf("expected custom middleware header on gateway path, got %q", got)
	}
	if !strings.Contains(rw2.Body.String(), "healthy") {
		t.Fatalf("expected gateway health response to contain 'healthy', got %q", rw2.Body.String())
	}
}
