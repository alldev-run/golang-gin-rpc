package orm

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

func TestOTelTracerSpanAndBaggage(t *testing.T) {
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		otel.SetTextMapPropagator(previousPropagator)
	})

	tracer := NewOTelTracer("transaction-test")
	ctx, span := tracer.StartSpan(context.Background(), "transaction")
	span.SetTag("attempt", 2)
	span.SetBaggageItem("tenant", "acme")

	baggageItems := make(map[string]string)
	span.Context().ForeachBaggageItem(func(key, value string) string {
		baggageItems[key] = value
		return ""
	})
	if baggageItems["tenant"] != "acme" {
		t.Fatalf("span baggage = %v, want tenant=acme", baggageItems)
	}

	headers := make(map[string]string)
	tracer.InjectContext(ctx, headers)
	if headers["baggage"] != "tenant=acme" {
		t.Fatalf("injected baggage = %q, want tenant=acme", headers["baggage"])
	}
	extracted, err := tracer.ExtractContext(headers)
	if err != nil {
		t.Fatalf("ExtractContext() error = %v", err)
	}
	if got := baggage.FromContext(extracted).Member("tenant").Value(); got != "acme" {
		t.Errorf("extracted baggage tenant = %q, want acme", got)
	}

	tracer.FinishSpan(span)
}

func TestNewJaegerTracerCompatibilityAlias(t *testing.T) {
	var tracer TransactionTracer = NewJaegerTracer("compatibility")
	ctx, span := tracer.StartSpan(context.Background(), "operation")
	if ctx == nil || span == nil {
		t.Fatal("compatibility tracer returned a nil context or span")
	}
	tracer.FinishSpan(span)
}
