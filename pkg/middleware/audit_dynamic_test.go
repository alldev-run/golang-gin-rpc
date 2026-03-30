package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/configcenter"
	"github.com/gin-gonic/gin"
)

func TestAuditMiddlewareDynamicUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := &captureAuditSink{}
	cfg := DefaultAuditConfig()
	cfg.Sink = sink
	cfg.Dynamic = NewDynamicAuditConfig(cfg)

	r := gin.New()
	r.Use(Audit(cfg))
	r.GET("/v1/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req1, _ := http.NewRequest(http.MethodGet, "/v1/users", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if len(sink.events) != 1 {
		t.Fatalf("expected first request to be audited")
	}

	cfg.Dynamic.Update(RuntimeAuditConfig{
		Enabled:       false,
		SkipPaths:     cfg.SkipPaths,
		SensitiveKeys: cfg.SensitiveKeys,
	})
	req2, _ := http.NewRequest(http.MethodGet, "/v1/users", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if len(sink.events) != 1 {
		t.Fatalf("expected disabled audit to skip events")
	}
}

func TestBindAuditConfigCenter(t *testing.T) {
	provider := configcenter.NewMemoryProvider()
	cc := configcenter.New(provider)
	defer cc.Close()

	base := DefaultAuditConfig()
	dyn := NewDynamicAuditConfig(base)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sub, err := BindAuditConfigCenter(ctx, cc, "governance", "audit/http", dyn)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	defer sub.Close()

	payload := RuntimeAuditConfig{
		Enabled:       false,
		SkipPaths:     []string{"/internal/health"},
		SensitiveKeys: []string{"authorization"},
	}
	raw, _ := json.Marshal(payload)
	if _, err := cc.Set(ctx, "governance", "audit/http", raw, nil); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		snap := dyn.Snapshot()
		if !snap.Enabled && len(snap.SkipPaths) == 1 && snap.SkipPaths[0] == "/internal/health" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dynamic config not updated in time: %+v", snap)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := cc.Delete(ctx, "governance", "audit/http"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	deadline = time.Now().Add(time.Second)
	for {
		snap := dyn.Snapshot()
		if snap.Enabled == base.Enabled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dynamic config not reset in time: %+v", snap)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
