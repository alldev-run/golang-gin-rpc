package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/audit"
	"github.com/gin-gonic/gin"
)

// AuditConfig defines audit middleware behavior.
type AuditConfig struct {
	Enabled          bool
	SkipPaths        []string
	SensitiveKeys    []string
	Dynamic          *DynamicAuditConfig
	Sink             audit.Sink
	ActionResolver   func(*gin.Context) audit.Action
	ResourceResolver func(*gin.Context) string
}

// DefaultAuditConfig returns default audit middleware settings.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:       true,
		SkipPaths:     []string{"/health", "/ready", "/metrics"},
		SensitiveKeys: []string{"password", "token", "authorization", "api_key", "secret"},
		Sink:          audit.LogSink{},
		ActionResolver: func(c *gin.Context) audit.Action {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return audit.ActionRead
			case http.MethodPost:
				return audit.ActionCreate
			case http.MethodPut, http.MethodPatch:
				return audit.ActionUpdate
			case http.MethodDelete:
				return audit.ActionDelete
			default:
				return audit.ActionCustom
			}
		},
		ResourceResolver: func(c *gin.Context) string { return c.FullPath() },
	}
}

// Audit emits structured audit events for HTTP requests.
func Audit(config ...AuditConfig) gin.HandlerFunc {
	cfg := DefaultAuditConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Dynamic == nil && !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	if cfg.Sink == nil {
		cfg.Sink = audit.LogSink{}
	}
	if cfg.ActionResolver == nil {
		cfg.ActionResolver = DefaultAuditConfig().ActionResolver
	}
	if cfg.ResourceResolver == nil {
		cfg.ResourceResolver = DefaultAuditConfig().ResourceResolver
	}

	staticMasker := audit.NewMasker(cfg.SensitiveKeys)
	return func(c *gin.Context) {
		runtimeEnabled := cfg.Enabled
		runtimeSkipPaths := cfg.SkipPaths
		runtimeMasker := staticMasker
		if cfg.Dynamic != nil {
			runtimeCfg := cfg.Dynamic.Snapshot()
			runtimeEnabled = runtimeCfg.Enabled
			runtimeSkipPaths = runtimeCfg.SkipPaths
			runtimeMasker = audit.NewMasker(runtimeCfg.SensitiveKeys)
		}

		if !runtimeEnabled {
			c.Next()
			return
		}

		if shouldSkipAuditPath(c.Request.URL.Path, runtimeSkipPaths) {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		event := audit.Event{
			Timestamp:  time.Now(),
			RequestID:  getStringFromContext(c, "request_id"),
			TraceID:    getStringFromContext(c, "trace_id"),
			TenantID:   getStringFromContext(c, "tenant_id"),
			UserID:     getStringFromContext(c, "user_id"),
			Username:   getStringFromContext(c, "username"),
			ClientIP:   c.ClientIP(),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			StatusCode: c.Writer.Status(),
			Action:     cfg.ActionResolver(c),
			Resource:   cfg.ResourceResolver(c),
			Result:     auditResult(c.Writer.Status()),
			DurationMS: time.Since(start).Milliseconds(),
			Metadata: map[string]interface{}{
				"query":   c.Request.URL.RawQuery,
				"headers": runtimeMasker.MaskMap(headersToAuditMap(c.Request.Header)),
			},
		}
		if len(c.Errors) > 0 {
			event.Message = c.Errors.String()
		}
		_ = cfg.Sink.Write(context.Background(), event)
	}
}

func shouldSkipAuditPath(path string, skipPaths []string) bool {
	for _, p := range skipPaths {
		if p == path {
			return true
		}
		if strings.HasSuffix(p, "/*") {
			prefix := strings.TrimSuffix(p, "/*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}

func headersToAuditMap(h http.Header) map[string]interface{} {
	out := make(map[string]interface{}, len(h))
	for k, values := range h {
		if len(values) == 1 {
			out[k] = values[0]
		} else {
			out[k] = strings.Join(values, ",")
		}
	}
	return out
}

func getStringFromContext(c *gin.Context, key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func auditResult(code int) string {
	if code >= 200 && code < 400 {
		return "success"
	}
	return "failure"
}
