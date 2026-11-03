package tracing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEnabledZipkinProviderExportsSpans(t *testing.T) {
	exported := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read exported span payload: %v", err)
		}
		exported <- string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	provider, err := NewTracerProvider(Config{
		Type: "zipkin", ServiceName: "coverage-test", ServiceVersion: "test",
		Environment: "test", Enabled: true, Endpoint: server.URL + "/api/v2/spans",
		SampleRate: 1, BatchTimeout: 10 * time.Millisecond, MaxExportBatchSize: 1,
	})
	if err != nil {
		t.Fatalf("NewTracerProvider() error = %v", err)
	}
	ctx, span := provider.StartSpan(context.Background(), "exported-operation")
	if !provider.IsEnabled() {
		t.Fatal("enabled configuration produced a disabled provider")
	}
	span.End()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	select {
	case payload := <-exported:
		if !strings.Contains(payload, "exported-operation") {
			t.Fatalf("exported payload = %q", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("Zipkin exporter did not send a span to the test server")
	}
}
