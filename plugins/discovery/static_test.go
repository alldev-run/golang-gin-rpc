package discovery

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestStaticRegistryLifecycle(t *testing.T) {
	ctx := context.Background()
	registry := newStaticRegistry()

	instance := &ServiceInstance{
		ID:      "api-1",
		Name:    "api",
		Address: "127.0.0.1",
		Port:    8080,
		Payload: map[string]string{"version": "1.0.0"},
	}

	// Register takes effect immediately (registerSelf semantics).
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	instances, err := registry.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(instances) != 1 || instances[0].ID != instance.ID || instances[0].Port != 8080 {
		t.Fatalf("GetService() = %+v, want the registered instance", instances)
	}

	// GetService must return a deep copy: mutating the result must not affect
	// the registry, and the stored pointer must differ from the returned one.
	instances[0].Payload["version"] = "mutated"
	instances[0].Port = 9999
	fresh, err := registry.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("GetService() after mutation error = %v", err)
	}
	if fresh[0].Payload["version"] != "1.0.0" || fresh[0].Port != 8080 {
		t.Fatalf("GetService() leaked registry state: %+v", fresh[0])
	}

	// Updating an instance with the same ID replaces the previous registration.
	updated := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 9090}
	if err := registry.Register(ctx, updated); err != nil {
		t.Fatalf("re-register error = %v", err)
	}
	instances, _ = registry.GetService(ctx, "api")
	if len(instances) != 1 || instances[0].Port != 9090 {
		t.Fatalf("GetService() after re-register = %+v, want one updated instance", instances)
	}

	// Deregister removes the instance and leaves an empty, non-nil result.
	if err := registry.Deregister(ctx, updated); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	instances, err = registry.GetService(ctx, "api")
	if err != nil || instances == nil || len(instances) != 0 {
		t.Fatalf("GetService() after Deregister() = %v, %v; want empty slice", instances, err)
	}

	// Registering instances of different services keeps them isolated.
	other := &ServiceInstance{ID: "web-1", Name: "web", Address: "127.0.0.2", Port: 80}
	if err := registry.Register(ctx, other); err != nil {
		t.Fatalf("Register(other) error = %v", err)
	}
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("Register(api) error = %v", err)
	}
	if instances, _ := registry.GetService(ctx, "web"); len(instances) != 1 || instances[0].ID != "web-1" {
		t.Fatalf("GetService(web) = %+v", instances)
	}

	// Deregistering one instance must not affect the other service.
	if err := registry.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister(api) error = %v", err)
	}
	if instances, _ := registry.GetService(ctx, "web"); len(instances) != 1 {
		t.Fatalf("GetService(web) after unrelated Deregister() = %+v", instances)
	}
}

func TestStaticRegistryValidation(t *testing.T) {
	ctx := context.Background()
	registry := newStaticRegistry()

	if err := registry.Register(ctx, nil); err == nil {
		t.Fatal("Register(nil) succeeded, want error")
	}
	if err := registry.Deregister(ctx, nil); err != nil {
		t.Fatalf("Deregister(nil) error = %v", err)
	}
	instances, err := registry.GetService(ctx, "unknown")
	if err != nil || instances == nil || len(instances) != 0 {
		t.Fatalf("GetService(unknown) = %v, %v; want empty slice", instances, err)
	}
	if err := registry.Deregister(ctx, &ServiceInstance{ID: "ghost", Name: "ghost"}); err != nil {
		t.Fatalf("Deregister(unknown) error = %v", err)
	}

	var nilRegistry *staticRegistry
	if err := nilRegistry.Register(ctx, &ServiceInstance{ID: "x", Name: "y"}); err == nil {
		t.Fatal("nil registry Register() succeeded, want error")
	}
	if _, err := nilRegistry.GetService(ctx, "y"); err == nil {
		t.Fatal("nil registry GetService() succeeded, want error")
	}
	if err := nilRegistry.Deregister(ctx, &ServiceInstance{ID: "x", Name: "y"}); err != nil {
		t.Fatalf("nil registry Deregister() error = %v", err)
	}
}

func TestStaticRegistryConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	registry := newStaticRegistry()

	const workers = 8
	const iterations = 50

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(3)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				inst := &ServiceInstance{
					ID:      fmt.Sprintf("api-%d-%d", id, i),
					Name:    "api",
					Address: "127.0.0.1",
					Port:    8000 + id,
				}
				if err := registry.Register(ctx, inst); err != nil {
					t.Errorf("Register() error = %v", err)
				}
			}
		}(w)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if _, err := registry.GetService(ctx, "api"); err != nil {
					t.Errorf("GetService() error = %v", err)
				}
			}
		}(w)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = registry.Deregister(ctx, &ServiceInstance{ID: fmt.Sprintf("api-%d-%d", id, i), Name: "api"})
			}
		}(w)
	}
	wg.Wait()
}

func TestNewDiscoveryStaticRegistry(t *testing.T) {
	d, err := NewDiscovery(StaticConfig())
	if err != nil {
		t.Fatalf("NewDiscovery(static) error = %v", err)
	}
	registry, ok := d.(*staticRegistry)
	if !ok {
		t.Fatalf("NewDiscovery(static) = %T, want *staticRegistry", d)
	}

	ctx := context.Background()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := d.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	instances, err := d.GetService(ctx, "api")
	if err != nil || len(instances) != 1 || instances[0].ID != instance.ID {
		t.Fatalf("GetService() = %v, %v; want the registered instance", instances, err)
	}
	if err := d.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if _, ok := registry.services["api"]; ok {
		t.Fatal("Deregister() left the service bucket behind")
	}
}
