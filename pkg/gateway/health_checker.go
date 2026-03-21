package gateway

import (
	"context"
	"net"
	"net/url"
	"time"

	"alldev-gin-rpc/pkg/health"
)

type upstreamHealthChecker struct {
	gw *Gateway
}

func (c *upstreamHealthChecker) Name() string {
	return "gateway_upstreams"
}

func (c *upstreamHealthChecker) Check(ctx context.Context) *health.CheckResult {
	start := time.Now()
	now := time.Now()

	type serviceStat struct {
		total   int
		healthy int
	}

	stats := map[string]*serviceStat{}

	c.gw.router.mu.Lock()
	for _, route := range c.gw.router.routes {
		healthyTargets := make([]string, 0, len(route.targets))
		for _, t := range route.targets {
			u, err := url.Parse(t)
			if err != nil {
				continue
			}
			host := u.Host
			if host == "" {
				continue
			}
			d := net.Dialer{}
			conn, err := d.DialContext(ctx, "tcp", host)
			if err != nil {
				continue
			}
			_ = conn.Close()
			healthyTargets = append(healthyTargets, t)
		}

		route.healthyTargets = healthyTargets
		route.lastHealthCheck = now

		st, ok := stats[route.config.Service]
		if !ok {
			st = &serviceStat{}
			stats[route.config.Service] = st
		}
		st.total += len(route.targets)
		st.healthy += len(healthyTargets)
	}
	c.gw.router.mu.Unlock()

	overallHealthyRoutes := 0
	c.gw.router.mu.RLock()
	for _, r := range c.gw.router.routes {
		if len(r.healthyTargets) > 0 {
			overallHealthyRoutes++
		}
	}
	totalRoutes := len(c.gw.router.routes)
	c.gw.router.mu.RUnlock()

	status := health.StatusHealthy
	msg := "ok"
	if totalRoutes == 0 {
		status = health.StatusDegraded
		msg = "no routes configured"
	} else if overallHealthyRoutes == 0 {
		status = health.StatusUnhealthy
		msg = "no healthy upstream"
	}

	details := map[string]any{
		"routes":         totalRoutes,
		"healthy_routes": overallHealthyRoutes,
		"services":       stats,
	}

	return &health.CheckResult{
		Name:        c.Name(),
		Status:      status,
		Message:     msg,
		Details:     details,
		Duration:    time.Since(start),
		Timestamp:   now,
		LastChecked: now,
	}
}
