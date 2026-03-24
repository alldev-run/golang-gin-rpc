package jsonrpc

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
)

// TracedClient wraps a JSON-RPC client with tracing capabilities
type TracedClient struct {
	client  *Client
	tracer  *tracing.TracerProvider
	baseURL string
}

// NewTracedClient creates a new traced JSON-RPC client
func NewTracedClient(config ClientConfig) *TracedClient {
	return &TracedClient{
		client:  NewClient(config),
		tracer:  tracing.GlobalTracer(),
		baseURL: config.URL,
	}
}

// Call makes a traced JSON-RPC method call
func (tc *TracedClient) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	return tc.CallWithID(ctx, method, params, result, nil)
}

// CallWithID makes a traced JSON-RPC method call with a specific ID
func (tc *TracedClient) CallWithID(ctx context.Context, method string, params interface{}, result interface{}, id interface{}) error {
	if !tc.tracer.IsEnabled() {
		return tc.client.CallWithID(ctx, method, params, result, id)
	}

	// Start span
	spanName := fmt.Sprintf("jsonrpc.%s", method)
	ctx, span := tc.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
		attribute.String("jsonrpc.method", method),
		attribute.String("jsonrpc.service", extractJSONRPCService(method)),
		attribute.String("jsonrpc.url", tc.baseURL),
		attribute.String("jsonrpc.version", "2.0"),
	))
	defer span.End()

	// Record start time
	start := time.Now()

	// Inject tracing context into headers
	otel.GetTextMapPropagator().Inject(ctx, &headerCarrier{})

	// Make the actual call
	err := tc.client.CallWithID(ctx, method, params, result, id)

	// Calculate duration
	duration := time.Since(start)

	// Set span attributes
	span.SetAttributes(
		attribute.Int64("jsonrpc.duration_ms", duration.Milliseconds()),
		attribute.String("jsonrpc.client", "github.com/alldev-run/golang-gin-rpc-jsonrpc"),
	)

	// Set span status based on result
	if err != nil {
		span.SetAttributes(
			attribute.String("jsonrpc.error", err.Error()),
			attribute.String("jsonrpc.status", "error"),
		)
		tracing.SetSpanError(span, err)
	} else {
		span.SetAttributes(
			attribute.String("jsonrpc.status", "success"),
		)
		tracing.SetSpanOK(span)
	}

	return err
}

// BatchCall makes a traced batch JSON-RPC call
func (tc *TracedClient) BatchCall(ctx context.Context, requests BatchRequest) (BatchResponse, error) {
	if !tc.tracer.IsEnabled() {
		return tc.client.CallBatch(ctx, requests)
	}

	// Start span for batch call
	spanName := "jsonrpc.batch"
	ctx, span := tc.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
		attribute.Int("jsonrpc.batch_size", len(requests)),
		attribute.String("jsonrpc.url", tc.baseURL),
	))
	defer span.End()

	// Record start time
	start := time.Now()

	// Inject tracing context
	otel.GetTextMapPropagator().Inject(ctx, &headerCarrier{})

	// Make the batch call
	response, err := tc.client.CallBatch(ctx, requests)

	// Calculate duration
	duration := time.Since(start)

	// Set span attributes
	span.SetAttributes(
		attribute.Int64("jsonrpc.duration_ms", duration.Milliseconds()),
		attribute.Int("jsonrpc.response_size", len(response)),
	)

	// Set span status
	if err != nil {
		tracing.SetSpanError(span, err)
	} else {
		tracing.SetSpanOK(span)
	}

	return response, err
}

// headerCarrier implements propagation.TextMapCarrier for HTTP headers
type headerCarrier struct {
	headers map[string]string
}

func (c *headerCarrier) Get(key string) string {
	return c.headers[key]
}

func (c *headerCarrier) Set(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

func (c *headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for k := range c.headers {
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
