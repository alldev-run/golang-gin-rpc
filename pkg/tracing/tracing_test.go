package tracing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracerProvider(t *testing.T) {
	config := Config{
		Type:              "zipkin",
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		Environment:       "test",
		Enabled:           false, // Disabled for testing
		Endpoint:          "http://localhost:9411/api/v2/spans",
		SampleRate:        1.0,
		BatchTimeout:      5 * time.Second,
		MaxExportBatchSize: 512,
	}

	tp, err := NewTracerProvider(config)
	require.NoError(t, err)
	require.NotNil(t, tp)
	assert.False(t, tp.IsEnabled())

	// Test tracer creation
	tracer := tp.Tracer("test")
	assert.NotNil(t, tracer)

	// Test shutdown
	ctx := context.Background()
	err = tp.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestGlobalTracer(t *testing.T) {
	// Test default global tracer
	tracer := GlobalTracer()
	assert.NotNil(t, tracer)

	// Test initialization with config
	config := Config{
		Type:        "zipkin",
		ServiceName: "test-global",
		Enabled:     false,
	}

	err := InitGlobalTracer(config)
	require.NoError(t, err)

	globalTracer := GlobalTracer()
	assert.NotNil(t, globalTracer)
	assert.Equal(t, config.ServiceName, globalTracer.config.ServiceName)
}

func TestSpanOperations(t *testing.T) {
	config := Config{
		Type:        "zipkin",
		ServiceName: "test-spans",
		Enabled:     false,
	}

	err := InitGlobalTracer(config)
	require.NoError(t, err)

	tracer := GlobalTracer()
	ctx := context.Background()

	// Test span creation
	ctx, span := tracer.StartSpan(ctx, "test-operation")
	assert.NotNil(t, span)
	assert.NotNil(t, ctx)

	// Test setting attributes
	attrs := map[string]interface{}{
		"user_id":   123,
		"operation": "test",
		"success":   true,
		"amount":    99.99,
	}

	SetSpanAttributes(span, attrs)

	// Test error setting
	testErr := assert.AnError
	SetSpanError(span, testErr)

	// Test success setting
	SetSpanOK(span)

	span.End()
}

func TestHTTPTracing(t *testing.T) {
	config := Config{
		Type:        "zipkin",
		ServiceName: "test-http",
		Enabled:     false,
	}

	err := InitGlobalTracer(config)
	require.NoError(t, err)

	tracer := GlobalTracer()
	middleware := NewHTTPMiddleware(tracer)
	assert.NotNil(t, middleware)

	// Test header injection/extraction
	ctx := context.Background()
	headers := make(map[string][]string)

	InjectHeaders(ctx, headers)
	extractedCtx := ExtractHeaders(ctx, headers)
	assert.NotNil(t, extractedCtx)
}

func TestGRPCInterceptor(t *testing.T) {
	config := Config{
		Type:        "zipkin",
		ServiceName: "test-grpc",
		Enabled:     false,
	}

	err := InitGlobalTracer(config)
	require.NoError(t, err)

	tracer := GlobalTracer()
	interceptor := NewGRPCInterceptor(tracer)
	assert.NotNil(t, interceptor)

	// Test interceptors creation
	unaryInterceptor := interceptor.UnaryServerInterceptor()
	assert.NotNil(t, unaryInterceptor)

	streamInterceptor := interceptor.StreamServerInterceptor()
	assert.NotNil(t, streamInterceptor)

	clientUnaryInterceptor := interceptor.UnaryClientInterceptor()
	assert.NotNil(t, clientUnaryInterceptor)

	clientStreamInterceptor := interceptor.StreamClientInterceptor()
	assert.NotNil(t, clientStreamInterceptor)
}

func TestConfiguration(t *testing.T) {
	// Test default config
	config := DefaultConfig()
	assert.Equal(t, "alldev-gin-rpc", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "development", config.Environment)
	assert.False(t, config.Enabled)
	assert.Equal(t, 1.0, config.SampleRate)

	// Test production config
	err := InitForProduction("test-service", "http://localhost:9411/api/v2/spans")
	if err != nil {
		t.Logf("Production initialization failed (expected in testing): %v", err)
	}

	// Test development config
	err = InitForDevelopment("test-service")
	if err != nil {
		t.Logf("Development initialization failed (expected in testing): %v", err)
	}
}

func BenchmarkSpanCreation(b *testing.B) {
	config := Config{
		Type:        "zipkin",
		ServiceName: "benchmark",
		Enabled:     false,
	}

	tp, _ := NewTracerProvider(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tp.StartSpan(ctx, "benchmark-operation")
		span.End()
	}
}

func BenchmarkAttributeSetting(b *testing.B) {
	config := Config{
		Type:        "zipkin",
		ServiceName: "benchmark",
		Enabled:     false,
	}

	tp, _ := NewTracerProvider(config)
	ctx := context.Background()

	attrs := map[string]interface{}{
		"user_id":   123,
		"operation": "test",
		"success":   true,
		"amount":    99.99,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tp.StartSpan(ctx, "benchmark-operation")
		SetSpanAttributes(span, attrs)
		span.End()
	}
}
