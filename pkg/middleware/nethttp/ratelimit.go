package nethttp

import (
	"net"
	"net/http"
	"strings"
	"time"

	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/ratelimit"
)

func RateLimitFromGatewayConfig(cfg *gateway.Config) Middleware {
	if cfg == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	rl := cfg.RateLimit
	if !rl.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	requests := rl.Requests
	window := parseDurationOrDefault(rl.Window, time.Minute)
	if requests <= 0 {
		requests = 100
	}

	limiter := ratelimit.NewMemoryFixedWindow(requests, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			if !limiter.Allow(ip) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("{\"error\":\"Rate limit exceeded\"}"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseDurationOrDefault(s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
