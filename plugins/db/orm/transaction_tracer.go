package orm

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// OTelTracer implements transaction tracing through the global OpenTelemetry provider.
type OTelTracer struct {
	tracer trace.Tracer
}

// NewOTelTracer creates a tracer using the application's global TracerProvider.
func NewOTelTracer(serviceName string) *OTelTracer {
	return &OTelTracer{tracer: otel.Tracer(serviceName)}
}

// JaegerTracer is retained as a source-compatible alias for OTelTracer.
type JaegerTracer = OTelTracer

// NewJaegerTracer creates an OpenTelemetry tracer for compatibility.
//
// Deprecated: use NewOTelTracer. Configure exporters through the global TracerProvider.
func NewJaegerTracer(serviceName string) *JaegerTracer {
	return NewOTelTracer(serviceName)
}

type spanBaggageContextKey struct{}

type otelTransactionSpan struct {
	span    trace.Span
	mu      sync.RWMutex
	baggage map[string]string
}

type otelTransactionSpanContext struct {
	baggage map[string]string
}

func (t *OTelTracer) StartSpan(ctx context.Context, operationName string) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, operationName)
	wrapped := &otelTransactionSpan{span: span, baggage: make(map[string]string)}
	return context.WithValue(ctx, spanBaggageContextKey{}, wrapped), wrapped
}

func (t *OTelTracer) FinishSpan(span Span) {
	if span != nil {
		span.Finish()
	}
}

func (t *OTelTracer) InjectContext(ctx context.Context, headers map[string]string) {
	if headers == nil {
		return
	}
	if span, ok := ctx.Value(spanBaggageContextKey{}).(*otelTransactionSpan); ok {
		span.mu.RLock()
		spanBaggage := make(map[string]string, len(span.baggage))
		for key, value := range span.baggage {
			spanBaggage[key] = value
		}
		span.mu.RUnlock()
		members := make([]baggage.Member, 0, len(spanBaggage)+len(baggage.FromContext(ctx).Members()))
		for _, member := range baggage.FromContext(ctx).Members() {
			if _, overridden := spanBaggage[member.Key()]; !overridden {
				members = append(members, member)
			}
		}
		for key, value := range spanBaggage {
			member, err := baggage.NewMember(key, value)
			if err == nil {
				members = append(members, member)
			}
		}
		if current, err := baggage.New(members...); err == nil {
			ctx = baggage.ContextWithBaggage(ctx, current)
		}
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
}

func (t *OTelTracer) ExtractContext(headers map[string]string) (context.Context, error) {
	return otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier(headers)), nil
}

func (s *otelTransactionSpan) SetTag(key string, value interface{}) {
	s.span.SetAttributes(attribute.String(key, fmt.Sprint(value)))
}

func (s *otelTransactionSpan) SetBaggageItem(key, value string) {
	s.mu.Lock()
	if s.baggage == nil {
		s.baggage = make(map[string]string)
	}
	s.baggage[key] = value
	s.mu.Unlock()
	s.span.SetAttributes(attribute.String("baggage."+key, value))
}

func (s *otelTransactionSpan) Finish() {
	s.span.End()
}

func (s *otelTransactionSpan) Context() SpanContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	baggage := make(map[string]string, len(s.baggage))
	for key, value := range s.baggage {
		baggage[key] = value
	}
	return &otelTransactionSpanContext{baggage: baggage}
}

func (c *otelTransactionSpanContext) ForeachBaggageItem(handler func(key, value string) string) {
	for key, value := range c.baggage {
		if handler(key, value) == "error" {
			return
		}
	}
}

var _ TransactionTracer = (*OTelTracer)(nil)
