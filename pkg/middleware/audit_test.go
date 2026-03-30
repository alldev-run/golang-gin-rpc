package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/audit"
	"github.com/gin-gonic/gin"
)

type captureAuditSink struct {
	events []audit.Event
}

func (c *captureAuditSink) Write(ctx context.Context, event audit.Event) error {
	c.events = append(c.events, event)
	return nil
}

func TestAuditMiddlewareEmitEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := &captureAuditSink{}
	cfg := DefaultAuditConfig()
	cfg.Sink = sink

	r := gin.New()
	r.Use(Audit(cfg))
	r.GET("/v1/users", func(c *gin.Context) {
		c.Set("user_id", "u-1")
		c.Set("tenant_id", "t-1")
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest(http.MethodGet, "/v1/users?page=1", nil)
	req.Header.Set("Authorization", "Bearer xyz")
	req.Header.Set("X-Request-ID", "req-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(sink.events))
	}
	e := sink.events[0]
	if e.Action != audit.ActionRead {
		t.Fatalf("expected read action, got %s", e.Action)
	}
	if e.UserID != "u-1" || e.TenantID != "t-1" {
		t.Fatalf("unexpected identity fields: %+v", e)
	}
	headers, _ := e.Metadata["headers"].(map[string]interface{})
	if headers["Authorization"] != "***" {
		t.Fatalf("expected authorization header to be masked")
	}
}
