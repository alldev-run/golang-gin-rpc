package bootstrap

import (
	"context"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/plugins/tracing"
)

func TestInitializeTracingInstallsGlobalTracer(t *testing.T) {
	t.Cleanup(func() { tracing.SetGlobalTracer(nil) })

	cfg := &config.GlobalConfig{}
	cfg.App.Name = "bootstrap-tracing-test"
	cfg.Observability.Tracing.Enabled = true
	cfg.Observability.Tracing.Type = "zipkin"
	cfg.Observability.Tracing.Endpoint = "http://127.0.0.1:9411/api/v2/spans"
	cfg.Observability.Tracing.SampleRate = 1

	b := &Bootstrap{config: cfg, dependencies: map[string]interface{}{}}
	if err := b.InitializeTracing(); err != nil {
		t.Fatalf("InitializeTracing() error = %v", err)
	}
	if b.GetTracer() == nil || tracing.GlobalTracer() != b.GetTracer() {
		t.Fatal("InitializeTracing() should install the bootstrap tracer as the global tracer")
	}

	if err := b.shutdownTracer(context.Background()); err != nil {
		t.Fatalf("shutdownTracer() error = %v", err)
	}
	if tracing.GlobalTracer() == b.GetTracer() {
		t.Fatal("shutdownTracer() should clear the global tracer")
	}
}
