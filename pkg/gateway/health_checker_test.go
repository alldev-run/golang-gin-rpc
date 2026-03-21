package gateway

import (
	"context"
	"net"
	"testing"
	"time"

	"alldev-gin-rpc/pkg/health"
)

func TestUpstreamHealthChecker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	cfg := DefaultConfig()
	cfg.Routes = []RouteConfig{{Path: "/api/*", Method: "*", Service: "svc", Targets: []string{"http://" + ln.Addr().String()}}}
	gw := NewGateway(cfg)
	checker := &upstreamHealthChecker{gw: gw}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res := checker.Check(ctx)
	if res == nil {
		t.Fatalf("expected result")
	}
	if res.Status != health.StatusHealthy {
		t.Fatalf("expected healthy, got %s", res.Status)
	}
	gw.router.mu.RLock()
	r := gw.router.routes[gw.routeKey(normalizeGinRoutePath("/api/*"), "*")]
	gw.router.mu.RUnlock()
	if r == nil || len(r.healthyTargets) == 0 {
		t.Fatalf("expected healthyTargets to be set")
	}
}
