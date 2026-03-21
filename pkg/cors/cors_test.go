package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyOptionsHandled(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "http://example.com/", nil)
	req.Header.Set("Origin", "http://foo.com")
	rw := httptest.NewRecorder()

	handled := Apply(rw, req, Config{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"*"},
		MaxAge:         600,
	})

	if !handled {
		t.Fatalf("expected handled preflight")
	}
	if rw.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rw.Code)
	}
	if got := rw.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected allow-origin '*', got %q", got)
	}
}

func TestApplyOptionsPassthrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "http://example.com/", nil)
	req.Header.Set("Origin", "http://foo.com")
	rw := httptest.NewRecorder()

	handled := Apply(rw, req, Config{
		AllowedOrigins:      []string{"*"},
		OptionsPassthrough: true,
	})
	if handled {
		t.Fatalf("expected not handled when passthrough enabled")
	}
}

func TestWildcardSubdomain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header.Set("Origin", "https://a.example.com")
	rw := httptest.NewRecorder()

	Apply(rw, req, Config{AllowedOrigins: []string{"*.example.com"}})
	if got := rw.Header().Get("Access-Control-Allow-Origin"); got != "https://a.example.com" {
		t.Fatalf("expected origin echoed, got %q", got)
	}
}
