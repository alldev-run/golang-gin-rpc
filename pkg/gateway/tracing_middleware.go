package gateway

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
)

// TracingMiddleware creates a tracing middleware for the gateway
func (g *Gateway) TracingMiddleware() gin.HandlerFunc {
	if !g.tracingEnabled() {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return tracing.GinMiddleware(g.config.ServiceName)
}

// tracingEnabled checks if tracing is enabled in configuration
func (g *Gateway) tracingEnabled() bool {
	// Check if tracing config exists and is enabled
	if g.config.Tracing != nil {
		return g.config.Tracing.Enabled
	}
	return false
}

// ProxyTracingMiddleware creates tracing middleware specifically for proxy requests
func (g *Gateway) ProxyTracingMiddleware() gin.HandlerFunc {
	if !g.tracingEnabled() {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	tracer := tracing.GlobalTracer()
	return func(c *gin.Context) {
		// Extract tracing context from incoming request
		ctx := tracing.ExtractHeaders(c.Request.Context(), c.Request.Header)
		c.Request = c.Request.WithContext(ctx)

		// Start proxy span
		spanName := "gateway.proxy"
		if route := c.GetString("route"); route != "" {
			spanName = "gateway.proxy." + route
		}

		ctx, span := tracer.StartSpan(ctx, spanName)
		defer span.End()

		// Set span attributes
		span.SetAttributes(
			attribute.String("gateway.service", g.config.ServiceName),
			attribute.String("gateway.method", c.Request.Method),
			attribute.String("gateway.path", c.Request.URL.Path),
			attribute.String("gateway.host", c.Request.Host),
			attribute.String("gateway.remote_addr", c.Request.RemoteAddr),
		)

		// Store span in context for downstream use
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// InjectTracingHeaders injects tracing context into outgoing HTTP request
func (g *Gateway) InjectTracingHeaders(req *http.Request, ctx context.Context) {
	if !g.tracingEnabled() {
		return
	}

	if ctx == nil {
		ctx = req.Context()
	}

	tracing.InjectHeaders(ctx, req.Header)
}

// GetTraceInfo extracts trace information from the context
func (g *Gateway) GetTraceInfo(c *gin.Context) (traceID, spanID string) {
	if !g.tracingEnabled() {
		return "", ""
	}

	ctx := c.Request.Context()
	traceID = tracing.GetTraceID(ctx)
	spanID = tracing.GetSpanID(ctx)
	return
}
