package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"github.com/google/uuid"
)

// HTTPMiddleware provides HTTP tracing middleware
type HTTPMiddleware struct {
	tracer *TracerProvider
}

// NewHTTPMiddleware creates a new HTTP tracing middleware
func NewHTTPMiddleware(tracer *TracerProvider) *HTTPMiddleware {
	return &HTTPMiddleware{tracer: tracer}
}

// Wrap wraps an HTTP handler with tracing
func (m *HTTPMiddleware) Wrap(handlerName string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.tracer.IsEnabled() {
			handler.ServeHTTP(w, r)
			return
		}

		// Extract context from incoming headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		
		// Start span
		spanName := handlerName
		if spanName == "" {
			spanName = r.URL.Path
		}
		
		ctx, span := m.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			trace.String("http.method", r.Method),
			trace.String("http.url", r.URL.String()),
			trace.String("http.host", r.Host),
			trace.String("http.scheme", r.URL.Scheme),
			trace.String("http.user_agent", r.UserAgent()),
			trace.String("http.remote_addr", r.RemoteAddr),
		))
		
		defer span.End()

		// Add request ID if not present
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		span.SetAttributes(trace.String("http.request_id", requestID))

		// Inject tracing context into response headers
		w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
		w.Header().Set("X-Span-ID", span.SpanContext().SpanID().String())
		w.Header().Set("X-Request-ID", requestID)

		// Create response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		// Call handler with tracing context
		handler.ServeHTTP(wrapped, r.WithContext(ctx))

		// Set response attributes
		span.SetAttributes(
			trace.Int("http.status_code", wrapped.statusCode),
			trace.String("http.status_text", http.StatusText(wrapped.statusCode)),
		)

		// Set error status if response indicates error
		if wrapped.statusCode >= 400 {
			span.SetStatus(trace.Status{
				Code:        trace.StatusCodeError,
				Description: http.StatusText(wrapped.statusCode),
			})
		} else {
			span.SetStatus(trace.Status{
				Code:        trace.StatusCodeOk,
				Description: "OK",
			})
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// InjectHeaders injects tracing context into HTTP headers
func InjectHeaders(ctx context.Context, headers http.Header) {
	if ctx == nil {
		return
	}
	
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractHeaders extracts tracing context from HTTP headers
func ExtractHeaders(ctx context.Context, headers http.Header) context.Context {
	if headers == nil {
		return ctx
	}
	
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}

// GetTraceID returns the trace ID from context
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// GetSpanID returns the span ID from context
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span == nil {
		return ""
	}
	return span.SpanContext().SpanID().String()
}
