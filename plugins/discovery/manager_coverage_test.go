package discovery

import (
	"context"
	"testing"
	"time"
)

func TestServiceDiscoveryManagerRegistryLifecycle(t *testing.T) {
	ctx := context.Background()
	backend := newMockDiscovery()
	manager := &ServiceDiscoveryManager{
		config: ManagerConfig{
			Enabled:             true,
			Timeout:             time.Second,
			HealthCheckInterval: time.Hour,
			AutoRegister:        false,
		},
		discovery: backend,
		instances: make(map[string]*ServiceInstance),
		stopCh:    make(chan struct{}),
	}

	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := manager.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := manager.GetRegisteredInstances()["api-1"]; got != instance {
		t.Fatalf("GetRegisteredInstances() = %v, want registered instance", got)
	}
	services, err := manager.GetAllServices(ctx)
	if err != nil {
		t.Fatalf("GetAllServices() error = %v", err)
	}
	if got := services["api"]; len(got) != 1 || got[0] != instance {
		t.Fatalf("GetAllServices() = %v, want the registered instance", got)
	}
	if got, err := manager.GetService(ctx, "api"); err != nil || len(got) != 1 {
		t.Fatalf("GetService() = %v, %v; want one instance", got, err)
	}
	if got := manager.GetConfig().HealthCheckInterval; got != time.Hour {
		t.Fatalf("GetConfig().HealthCheckInterval = %v, want 1h", got)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !manager.IsStarted() {
		t.Fatal("IsStarted() = false after Start()")
	}
	if err := manager.Start(); err == nil {
		t.Fatal("second Start() succeeded, want already-started error")
	}
	if err := manager.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if got := manager.GetRegisteredInstances(); len(got) != 0 {
		t.Fatalf("registered instances after Deregister() = %v, want empty", got)
	}
	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if manager.IsStarted() {
		t.Fatal("IsStarted() = true after Stop()")
	}
}

func TestDisabledServiceDiscoveryManager(t *testing.T) {
	manager, err := NewServiceDiscoveryManager(ManagerConfig{})
	if err != nil {
		t.Fatalf("NewServiceDiscoveryManager() error = %v", err)
	}
	if err := manager.Register(context.Background(), &ServiceInstance{ID: "x"}); err == nil {
		t.Fatal("Register() succeeded while discovery is disabled")
	}
	if _, err := manager.GetService(context.Background(), "api"); err == nil {
		t.Fatal("GetService() succeeded while discovery is disabled")
	}
	if services, err := manager.GetAllServices(context.Background()); err != nil || len(services) != 0 {
		t.Fatalf("GetAllServices() = %v, %v; want empty map", services, err)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
