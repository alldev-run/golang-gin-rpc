package consul

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/api"
)

// newWatchStubServer returns 503 for every request so the watch plan created by
// NewServiceWatcher errors out immediately and retries with backoff instead of
// delivering results. This keeps watcher tests deterministic: the watch loop
// never invokes the handler, so tests drive plan.Handler directly.
func newWatchStubServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestServiceWatcher(t *testing.T) *ServiceWatcher {
	t.Helper()
	server := newWatchStubServer(t)
	watcher, err := NewServiceWatcher(context.Background(), server.URL, "api")
	if err != nil {
		t.Fatalf("NewServiceWatcher() error = %v", err)
	}
	t.Cleanup(func() {
		watcher.Stop()
		server.Close()
	})
	return watcher
}

func TestServiceWatcherListReturnsNilBeforeAnyUpdate(t *testing.T) {
	watcher := newTestServiceWatcher(t)
	if got := watcher.List(); got != nil {
		t.Fatalf("List() = %v, want nil before any handler invocation", got)
	}
}

func TestServiceWatcherHandlerIgnoresInvalidPayloads(t *testing.T) {
	watcher := newTestServiceWatcher(t)

	// nil payload must be ignored
	watcher.plan.Handler(1, nil)
	if got := watcher.List(); got != nil {
		t.Fatalf("List() after nil payload = %v, want nil", got)
	}

	// a payload of the wrong type must be ignored
	watcher.plan.Handler(1, "not-a-service-entry-list")
	if got := watcher.List(); got != nil {
		t.Fatalf("List() after wrong payload type = %v, want nil", got)
	}
}

func TestServiceWatcherHandlerStoresConvertedInstances(t *testing.T) {
	watcher := newTestServiceWatcher(t)

	entries := []*api.ServiceEntry{
		{Service: &api.AgentService{ID: "api-1", Service: "api", Address: "10.0.0.1", Port: 8080, Meta: map[string]string{"weight": "2"}}},
		{Service: &api.AgentService{ID: "api-2", Service: "api", Address: "10.0.0.2", Port: 8081}},
	}
	watcher.plan.Handler(2, entries)

	got := watcher.List()
	if len(got) != 2 {
		t.Fatalf("List() returned %d instances, want 2", len(got))
	}
	if got[0].ID != "api-1" || got[0].Name != "api" || got[0].Address != "10.0.0.1" ||
		got[0].Port != 8080 || got[0].Payload["weight"] != "2" {
		t.Fatalf("List()[0] = %+v", got[0])
	}
	if got[1].ID != "api-2" || got[1].Port != 8081 || got[1].Payload != nil {
		t.Fatalf("List()[1] = %+v", got[1])
	}
}

func TestServiceWatcherStopIsIdempotent(t *testing.T) {
	watcher := newTestServiceWatcher(t)
	watcher.Stop()
	// Calling Stop twice must not panic (plan.Stop and cancel are guarded).
	watcher.Stop()
}
