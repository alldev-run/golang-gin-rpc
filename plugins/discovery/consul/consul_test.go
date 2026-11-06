package consul

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/api"
)

const fakeHealthPayload = `[{"Node":{"Node":"node-1","Address":"127.0.0.1"},"Service":{"ID":"api-1","Service":"api","Address":"127.0.0.1","Port":8080,"Meta":{"weight":"3"}},"Checks":[{"CheckID":"serfHealth","Status":"passing"}]}]`

// newFakeConsulServer starts an HTTP server that mimics the Consul agent and
// health API endpoints exercised by the registry. Unknown endpoints (including
// watch long-polls) fail fast with 503 so background watch loops exit
// deterministically instead of polling forever.
func newFakeConsulServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/agent/service/register",
			r.Method == http.MethodPut && r.URL.Path == "/v1/agent/service/deregister/api-1":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/v1/health/service/api":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fakeHealthPayload))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestConsulRegistry(t *testing.T, serverURL string) *Registry {
	t.Helper()
	registry, err := NewRegistry(serverURL)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func TestRegistryRegisterGetDeregister(t *testing.T) {
	server := newFakeConsulServer(t)
	registry := newTestConsulRegistry(t, server.URL)
	ctx := context.Background()

	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	instances, err := registry.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("GetService() returned %d instances, want 1", len(instances))
	}
	got := instances[0]
	if got.ID != "api-1" || got.Name != "api" || got.Address != "127.0.0.1" || got.Port != 8080 ||
		got.Payload["weight"] != "3" {
		t.Fatalf("GetService() = %+v, want the fake payload fields", got)
	}

	if err := registry.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
}

func TestRegistryRegisterSendsTCPCheck(t *testing.T) {
	var registration api.AgentServiceRegistration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/agent/service/register" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("reading register body: %v", err)
			}
			if err := json.Unmarshal(body, &registration); err != nil {
				t.Errorf("decoding register body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registry := newTestConsulRegistry(t, server.URL)
	if err := registry.Register(context.Background(), &ServiceInstance{
		ID: "api-1", Name: "api", Address: "10.0.0.5", Port: 9090,
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if registration.ID != "api-1" || registration.Name != "api" ||
		registration.Address != "10.0.0.5" || registration.Port != 9090 {
		t.Fatalf("registration = %+v, want mirrored instance fields", registration)
	}
	if registration.Check == nil || registration.Check.TCP != "10.0.0.5:9090" {
		t.Fatalf("registration check = %+v, want TCP check for 10.0.0.5:9090", registration.Check)
	}
}

func TestRegistryGetServiceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health/service/api" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registry := newTestConsulRegistry(t, server.URL)
	if _, err := registry.GetService(context.Background(), "api"); err == nil {
		t.Fatal("GetService() succeeded despite the server returning 500")
	}
}
