package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/plugins/gateway"
	"github.com/gin-gonic/gin"
)

func TestBusinessRoutesSeeOuterRequestID(t *testing.T) {
	builder := NewRouterBuilder(gateway.DefaultConfig())
	builder.registry.GET("/api/rid", func(c *gin.Context) {
		id, _ := c.Get("request_id")
		c.String(http.StatusOK, "%v", id)
	}, "request id echo")
	handler := builder.Build()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/rid", nil))

	want := rec.Header().Get("X-Request-ID")
	if want == "" {
		t.Fatal("outer middleware did not set X-Request-ID")
	}
	if got := rec.Body.String(); got != want {
		t.Fatalf("business handler request_id = %q, want %q", got, want)
	}
}
