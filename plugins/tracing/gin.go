package tracing

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns a Gin middleware for HTTP tracing
func GinMiddleware(serviceName string) gin.HandlerFunc {
	tracer := GlobalTracer()
	httpMiddleware := NewHTTPMiddleware(tracer)
	
	return func(c *gin.Context) {
		// Convert gin.Context to http.Handler
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Next()
		})
		
		// Wrap with HTTP tracing middleware
		tracedHandler := httpMiddleware.Wrap(serviceName, handler)
		
		// Create a new request with the gin context
		req := c.Request
		c.Request = req.WithContext(c.Request.Context())
		
		// Create a response writer wrapper
		w := &ginResponseWriter{ResponseWriter: c.Writer, Context: c}
		
		// Call the traced handler
		tracedHandler.ServeHTTP(w, c.Request)
		
		// Set tracing headers in gin context
		if traceID := GetTraceID(c.Request.Context()); traceID != "" {
			c.Header("X-Trace-ID", traceID)
			c.Set("trace_id", traceID)
		}
		if spanID := GetSpanID(c.Request.Context()); spanID != "" {
			c.Header("X-Span-ID", spanID)
			c.Set("span_id", spanID)
		}
	}
}

// ginResponseWriter wraps gin.ResponseWriter to work with http.Handler
type ginResponseWriter struct {
	gin.ResponseWriter
	Context *gin.Context
}

func (w *ginResponseWriter) WriteHeader(code int) {
	w.Context.Status(code)
	w.ResponseWriter.WriteHeader(code)
}
