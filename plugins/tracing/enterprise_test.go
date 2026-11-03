package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestTracerTypeMetadataAndConfigValidation(t *testing.T) {
	for _, tracerType := range GetSupportedTypes() {
		if !tracerType.IsValid() || tracerType.String() == "" || tracerType.DisplayName() == "Unknown" {
			t.Errorf("invalid tracer type metadata for %q", tracerType)
		}
		if tracerType.DefaultPort() <= 0 {
			t.Errorf("DefaultPort(%q) = 0", tracerType)
		}
	}

	for input, want := range map[string]TracerType{
		" Zipkin ": Zipkin, "jaeger": Jaeger, "opentelemetry": OTLP,
		"aws-xray": AWSXRay, "google-cloud": GoogleCloudTrace, "app-insights": AzureAppInsights,
	} {
		got, err := ParseTracerType(input)
		if err != nil || got != want {
			t.Errorf("ParseTracerType(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ParseTracerType("unknown"); err == nil {
		t.Fatal("ParseTracerType() accepted an unknown backend")
	}

	for _, test := range []struct {
		name string
		cfg  Config
	}{
		{name: "unknown backend", cfg: Config{Type: "unknown", ServiceName: "svc"}},
		{name: "unimplemented backend", cfg: Config{Type: "prometheus", ServiceName: "svc"}},
		{name: "missing service", cfg: Config{Type: "zipkin"}},
		{name: "missing credentials", cfg: Config{Type: "xray", ServiceName: "svc"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateConfig(test.cfg); err == nil {
				t.Fatal("ValidateConfig() succeeded, want error")
			}
		})
	}
}

func TestTracingConfigURLAndFileLoading(t *testing.T) {
	if got := (Config{Host: "collector", Port: 9411, Endpoint: "/api/v2/spans"}).GetURL(); got != "http://collector:9411/api/v2/spans" {
		t.Fatalf("GetURL() = %q", got)
	}
	if got := (Config{Endpoint: "https://collector/spans"}).GetURL(); got != "https://collector/spans" {
		t.Fatalf("GetURL() = %q", got)
	}
	if got := (Config{Host: "collector", Port: 9411}).GetURL(); got != "http://collector:9411" {
		t.Fatalf("GetURL() without endpoint = %q", got)
	}

	path := filepath.Join(t.TempDir(), "tracing.yaml")
	if err := os.WriteFile(path, []byte("type: zipkin\nservice_name: readme-test\nenabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfigFromFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() error = %v", err)
	}
	if config.ServiceName != "readme-test" || config.Type != "zipkin" {
		t.Fatalf("loaded config = %+v", config)
	}
	if _, err := LoadConfigFromFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("LoadConfigFromFile() succeeded for a missing file")
	}
}

func TestHTTPMiddlewareRecordsResponseAndTraceHeaders(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	tp := &TracerProvider{provider: provider, config: Config{Enabled: true}}
	handler := NewHTTPMiddleware(tp).Wrap("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetTraceID(r.Context()) == "" {
			t.Error("request context has no trace ID")
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	request := httptest.NewRequest(http.MethodGet, "/upstream", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
	for _, header := range []string{"X-Trace-ID", "X-Span-ID", "X-Request-ID"} {
		if response.Header().Get(header) == "" {
			t.Errorf("response header %s is empty", header)
		}
	}
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestSpanHelpersAndParentLink(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	tp := &TracerProvider{provider: provider, config: Config{Enabled: true}}
	ctx, parent := tp.StartSpan(context.Background(), "parent")
	childCtx, child := tp.StartSpanWithParent(ctx, "child", parent)
	if !tp.IsEnabled() || GetTraceID(childCtx) == "" || GetSpanID(childCtx) == "" {
		t.Fatal("started span is missing trace context")
	}
	SetSpanAttributes(child, map[string]interface{}{
		"text": "value", "integer": 1, "integer64": int64(2), "float": 1.5,
		"boolean": true, "other": []string{"value"},
	})
	SetSpanError(child, context.Canceled)
	SetSpanOK(child)
	SetSpanAttributes(nil, nil)
	SetSpanError(nil, context.Canceled)
	SetSpanOK(nil)
	child.End()
	parent.End()
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	noop := &TracerProvider{config: Config{Enabled: false}}
	if noop.IsEnabled() {
		t.Fatal("disabled tracer reports enabled")
	}
	ctx, span := noop.StartSpan(context.Background(), "disabled")
	if ctx == nil || span == nil {
		t.Fatal("disabled tracer returned nil context or span")
	}
	ctx, span = noop.StartSpanWithParent(context.Background(), "disabled-child", trace.SpanFromContext(ctx))
	if ctx == nil || span == nil {
		t.Fatal("disabled tracer returned nil parent context or span")
	}
}

func TestGlobalTracerLifecycle(t *testing.T) {
	if err := InitGlobalTracer(Config{Type: "zipkin", ServiceName: "lifecycle", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if GlobalTracer().config.ServiceName != "lifecycle" {
		t.Fatal("InitGlobalTracer() did not replace the global tracer")
	}
	if err := ShutdownGlobalTracer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTracer(Config{Type: "zipkin", ServiceName: "compat", Enabled: false}); err != nil {
		t.Fatal(err)
	}
}
