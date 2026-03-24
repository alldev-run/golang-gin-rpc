package websocket

import (
	"context"
	"fmt"
	"net/http"

	pkgtracing "github.com/alldev-run/golang-gin-rpc/pkg/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type traceHeadersCarrier struct {
	headers map[string]string
}

func (c traceHeadersCarrier) Get(key string) string {
	if c.headers == nil {
		return ""
	}
	return c.headers[key]
}

func (c traceHeadersCarrier) Set(key, value string) {
	if c.headers == nil {
		return
	}
	c.headers[key] = value
}

func (c traceHeadersCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
		keys = append(keys, k)
	}
	return keys
}

func tracerOrGlobal(tp *pkgtracing.TracerProvider) *pkgtracing.TracerProvider {
	if tp != nil {
		return tp
	}
	return pkgtracing.GlobalTracer()
}

func startWebsocketSpan(ctx context.Context, tp *pkgtracing.TracerProvider, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := tracerOrGlobal(tp)
	if tracer == nil || !tracer.IsEnabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tracer.StartSpan(ctx, name, trace.WithAttributes(attrs...))
}

func injectTraceToHTTPHeaders(ctx context.Context, headers http.Header) {
	if ctx == nil || headers == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

func injectTraceToMapHeaders(ctx context.Context, headers map[string]string) {
	if ctx == nil || headers == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, traceHeadersCarrier{headers: headers})
}

func extractTraceFromHTTPHeaders(ctx context.Context, headers http.Header) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}

func websocketTraceAttrs(identity Identity, extra ...attribute.KeyValue) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "websocket"),
		attribute.String("websocket.connection_id", identity.ConnectionID),
		attribute.String("websocket.client_id", identity.ClientID),
		attribute.String("websocket.user_id", identity.UserID),
		attribute.String("websocket.tenant_id", identity.TenantID),
		attribute.String("websocket.path", identity.Path),
		attribute.String("websocket.remote_addr", identity.RemoteAddr),
	}
	attrs = append(attrs, extra...)
	return attrs
}

func endSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		pkgtracing.SetSpanError(span, err)
	} else {
		pkgtracing.SetSpanOK(span)
	}
	span.End()
}

func traceMessageName(direction string, isJSON bool) string {
	kind := "message"
	if isJSON {
		kind = "json"
	}
	return fmt.Sprintf("websocket.%s.%s", direction, kind)
}

func contextWithFallback(ctx context.Context, fallback context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	if fallback != nil {
		return fallback
	}
	return context.Background()
}
