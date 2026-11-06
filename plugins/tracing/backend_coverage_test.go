package tracing

import (
	"context"
	"testing"
	"time"
)

func TestTracingPresetsAndOTLPBackends(t *testing.T) {
	if ProductionConfig("service").ServiceName != "service" ||
		DevelopmentConfig("service").ServiceName != "service" {
		t.Fatal("tracing presets did not retain the service name")
	}
	if !AWSXRay.IsCloudBased() || !OTLP.IsOpenSource() ||
		!AWSXRay.RequiresAuthentication() || OTLP.RequiresAuthentication() {
		t.Fatal("tracer type metadata is inconsistent")
	}
	if len(GetFutureTypes()) == 0 {
		t.Fatal("GetFutureTypes() returned no planned backends")
	}

	for _, tracerType := range []TracerType{Jaeger, OTLP} {
		t.Run(tracerType.String(), func(t *testing.T) {
			provider, err := CreateTracerProviderByType(tracerType, Config{
				ServiceName: "coverage-test", Enabled: true, Host: "127.0.0.1",
				Port: tracerType.DefaultPort(), SampleRate: 1,
				BatchTimeout: time.Millisecond, MaxExportBatchSize: 1,
			})
			if err != nil {
				t.Fatalf("CreateTracerProviderByType() error = %v", err)
			}
			if !provider.IsEnabled() {
				t.Fatal("provider is disabled")
			}
			if tracerType == Jaeger && provider.config.Port != 4317 {
				t.Fatalf("Jaeger port = %d, want OTLP gRPC port 4317", provider.config.Port)
			}
			if err := provider.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})
	}
}
