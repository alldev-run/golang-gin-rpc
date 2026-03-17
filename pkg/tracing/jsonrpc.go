package tracing

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"github.com/gin-gonic/gin"
)

// JSONRPCInterceptor provides JSON-RPC tracing middleware
type JSONRPCInterceptor struct {
	tracer *TracerProvider
}

// NewJSONRPCInterceptor creates a new JSON-RPC tracing interceptor
func NewJSONRPCInterceptor(tracer *TracerProvider) *JSONRPCInterceptor {
	return &JSONRPCInterceptor{tracer: tracer}
}

// Middleware returns a Gin middleware for JSON-RPC tracing
func (i *JSONRPCInterceptor) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !i.tracer.IsEnabled() {
			c.Next()
			return
		}

		// Extract tracing context from headers
		ctx := c.Request.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, &headerCarrier{c.Request.Header})

		// Start span
		spanName := "jsonrpc.request"
		ctx, span := i.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			attribute.String("jsonrpc.method", c.Request.Method),
			attribute.String("jsonrpc.path", c.Request.URL.Path),
			attribute.String("jsonrpc.remote_addr", c.Request.RemoteAddr),
			attribute.String("jsonrpc.user_agent", c.Request.UserAgent()),
		))
		defer span.End()

		// Inject tracing context back to the request context
		c.Request = c.Request.WithContext(ctx)

		// Record start time
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Set span attributes based on response
		span.SetAttributes(
			attribute.Int("jsonrpc.status_code", c.Writer.Status()),
			attribute.String("jsonrpc.status_text", http.StatusText(c.Writer.Status())),
			attribute.Int64("jsonrpc.duration_ms", duration.Milliseconds()),
		)

		// Check if this is a JSON-RPC request
		if c.Request.URL.Path == "/rpc" || c.Request.URL.Path == "/jsonrpc" {
			i.handleJSONRPCSpecific(c, span)
		}

		// Set span status based on response code
		if c.Writer.Status() >= 400 {
			SetSpanError(span, fmt.Errorf("HTTP error: %d", c.Writer.Status()))
		} else {
			SetSpanOK(span)
		}
	}
}

// handleJSONRPCSpecific handles JSON-RPC specific tracing
func (i *JSONRPCInterceptor) handleJSONRPCSpecific(c *gin.Context, span trace.Span) {
	// Try to extract JSON-RPC method from request body or query
	method := "unknown"
	
	// For GET requests, check query parameters
	if c.Request.Method == "GET" {
		if methodParam := c.Query("method"); methodParam != "" {
			method = methodParam
		}
	} else if c.Request.Method == "POST" {
		// For POST requests, we could parse the body to extract method
		// but this would consume the request body, so we'll use a different approach
		// In a real implementation, you might want to use a request wrapper
		if methodParam := c.PostForm("method"); methodParam != "" {
			method = methodParam
		}
	}

	span.SetAttributes(
		attribute.String("jsonrpc.method", method),
		attribute.String("jsonrpc.version", "2.0"),
	)
}

// JSONRPCHandlerWrapper wraps a JSON-RPC handler with tracing
func (i *JSONRPCInterceptor) WrapHandler(methodName string, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !i.tracer.IsEnabled() {
			handler(c)
			return
		}

		ctx := c.Request.Context()
		
		// Start span for specific method
		spanName := fmt.Sprintf("jsonrpc.%s", methodName)
		ctx, span := i.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			attribute.String("jsonrpc.method", methodName),
			attribute.String("jsonrpc.service", extractJSONRPCService(methodName)),
		))
		defer span.End()

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		// Record start time
		start := time.Now()

		// Call handler
		handler(c)

		// Calculate duration
		duration := time.Since(start)

		// Set span attributes
		span.SetAttributes(
			attribute.Int64("jsonrpc.duration_ms", duration.Milliseconds()),
			attribute.Int("jsonrpc.status_code", c.Writer.Status()),
		)

		// Set span status
		if c.Writer.Status() >= 400 {
			SetSpanError(span, fmt.Errorf("JSON-RPC method %s failed with status %d", methodName, c.Writer.Status()))
		} else {
			SetSpanOK(span)
		}
	}
}

// headerCarrier implements propagation.TextMapCarrier for HTTP headers
type headerCarrier struct {
	header http.Header
}

func (c *headerCarrier) Get(key string) string {
	return c.header.Get(key)
}

func (c *headerCarrier) Set(key, value string) {
	c.header.Set(key, value)
}

func (c *headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.header))
	for k := range c.header {
		keys = append(keys, k)
	}
	return keys
}

// extractJSONRPCService extracts service name from JSON-RPC method
func extractJSONRPCService(method string) string {
	// JSON-RPC method format: service.method or just method
	if dot := indexByte(method, '.'); dot != -1 {
		return method[:dot]
	}
	return "default"
}

// indexByte is a helper function to find the first occurrence of a byte in a string
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
