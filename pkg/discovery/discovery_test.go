package discovery

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockDiscovery implements Discovery interface for testing
type mockDiscovery struct {
	mu       sync.RWMutex
	services map[string][]*ServiceInstance
}

func newMockDiscovery() *mockDiscovery {
	return &mockDiscovery{
		services: make(map[string][]*ServiceInstance),
	}
}

func (m *mockDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.services[instance.Name] == nil {
		m.services[instance.Name] = []*ServiceInstance{}
	}
	m.services[instance.Name] = append(m.services[instance.Name], instance)
	return nil
}

func (m *mockDiscovery) Deregister(ctx context.Context, instance *ServiceInstance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	instances := m.services[instance.Name]
	if instances == nil {
		return nil
	}
	
	for i, inst := range instances {
		if inst.ID == instance.ID {
			m.services[instance.Name] = append(instances[:i], instances[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockDiscovery) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if instances, ok := m.services[serviceName]; ok {
		copied := make([]*ServiceInstance, len(instances))
		copy(copied, instances)
		return copied, nil
	}
	return []*ServiceInstance{}, nil
}

func TestNewDiscovery(t *testing.T) {
	// Test consul discovery
	consulConfig := Config{
		Type:    "consul",
		Address: "127.0.0.1:8500",
		Timeout: 5 * time.Second,
	}
	
	discovery, err := NewDiscovery(consulConfig)
	if err != nil {
		// Consul is not running, this is expected in test environment
		t.Logf("Consul not available, skipping test: %v", err)
	} else {
		if discovery == nil {
			t.Error("Expected discovery instance, got nil")
		}
	}

	// Test etcd discovery
	etcdConfig := Config{
		Type:    "etcd",
		Address: "127.0.0.1:2379",
		Timeout: 5 * time.Second,
	}
	
	discovery, err = NewDiscovery(etcdConfig)
	if err != nil {
		// etcd is not running, this is expected in test environment
		t.Logf("etcd not available, skipping test: %v", err)
	} else {
		if discovery == nil {
			t.Error("Expected discovery instance, got nil")
		}
	}

	// Test invalid type
	invalidConfig := Config{
		Type:    "invalid",
		Address: "127.0.0.1:1234",
		Timeout: 5 * time.Second,
	}
	
	_, err = NewDiscovery(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid discovery type")
	}
	if err.Error() != "unsupported discovery type: invalid" {
		t.Errorf("Expected 'unsupported discovery type: invalid' error, got %v", err)
	}

	// Test empty type
	emptyConfig := Config{
		Type:    "",
		Address: "127.0.0.1:1234",
		Timeout: 5 * time.Second,
	}
	
	_, err = NewDiscovery(emptyConfig)
	if err == nil {
		t.Error("Expected error for empty discovery type")
	}
	if err.Error() != "discovery type is required" {
		t.Errorf("Expected 'discovery type is required' error, got %v", err)
	}
}

func TestServiceInstance(t *testing.T) {
	instance := &ServiceInstance{
		ID:      "service-1",
		Name:    "user-service",
		Address: "127.0.0.1",
		Port:    8080,
		Payload: map[string]string{
			"version": "1.0.0",
			"region":  "us-west",
		},
	}

	if instance.ID != "service-1" {
		t.Errorf("Expected ID 'service-1', got '%s'", instance.ID)
	}
	if instance.Name != "user-service" {
		t.Errorf("Expected Name 'user-service', got '%s'", instance.Name)
	}
	if instance.Address != "127.0.0.1" {
		t.Errorf("Expected Address '127.0.0.1', got '%s'", instance.Address)
	}
	if instance.Port != 8080 {
		t.Errorf("Expected Port 8080, got %d", instance.Port)
	}
	if instance.Payload["version"] != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", instance.Payload["version"])
	}
	if instance.Payload["region"] != "us-west" {
		t.Errorf("Expected region 'us-west', got '%s'", instance.Payload["region"])
	}
}

func TestDiscoveryOperations(t *testing.T) {
	mock := newMockDiscovery()
	ctx := context.Background()

	// Test register service
	instance := &ServiceInstance{
		ID:      "service-1",
		Name:    "user-service",
		Address: "127.0.0.1",
		Port:    8080,
		Payload: map[string]string{
			"version": "1.0.0",
		},
	}

	err := mock.Register(ctx, instance)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Test get service
	instances, err := mock.GetService(ctx, "user-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1 instance, got %d", len(instances))
	}

	if instances[0].ID != "service-1" {
		t.Errorf("Expected instance ID 'service-1', got '%s'", instances[0].ID)
	}

	// Test get non-existent service
	instances, err = mock.GetService(ctx, "non-existent-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	if len(instances) != 0 {
		t.Errorf("Expected 0 instances for non-existent service, got %d", len(instances))
	}

	// Test deregister service
	err = mock.Deregister(ctx, instance)
	if err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	// Verify service is deregistered
	instances, err = mock.GetService(ctx, "user-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	if len(instances) != 0 {
		t.Errorf("Expected 0 instances after deregistration, got %d", len(instances))
	}
}

func TestDiscoveryMultipleServices(t *testing.T) {
	mock := newMockDiscovery()
	ctx := context.Background()

	// Register multiple instances of the same service
	instance1 := &ServiceInstance{
		ID:      "service-1",
		Name:    "user-service",
		Address: "127.0.0.1",
		Port:    8080,
	}

	instance2 := &ServiceInstance{
		ID:      "service-2",
		Name:    "user-service",
		Address: "127.0.0.2",
		Port:    8080,
	}

	instance3 := &ServiceInstance{
		ID:      "service-3",
		Name:    "order-service",
		Address: "127.0.0.3",
		Port:    9090,
	}

	// Register instances
	err := mock.Register(ctx, instance1)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err = mock.Register(ctx, instance2)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err = mock.Register(ctx, instance3)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Test get user-service (should have 2 instances)
	instances, err := mock.GetService(ctx, "user-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Expected 2 instances for user-service, got %d", len(instances))
	}

	// Test get order-service (should have 1 instance)
	instances, err = mock.GetService(ctx, "order-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1 instance for order-service, got %d", len(instances))
	}

	// Deregister one instance from user-service
	err = mock.Deregister(ctx, instance1)
	if err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}

	// Verify user-service now has 1 instance
	instances, err = mock.GetService(ctx, "user-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1 instance after deregistration, got %d", len(instances))
	}

	if instances[0].ID != "service-2" {
		t.Errorf("Expected remaining instance ID 'service-2', got '%s'", instances[0].ID)
	}
}

func TestConsulAdapter(t *testing.T) {
	// This test would require a running Consul instance
	// For now, we just test the adapter structure
	
	// Create a consul adapter with mock discovery
	adapter := &consulAdapter{registry: nil} // We can't easily mock the consul.Registry
	
	// Test that the adapter implements the Discovery interface
	var _ Discovery = adapter
	
	// In a real test environment with Consul running, we would test:
	// - Register service
	// - Get service
	// - Deregister service
	t.Log("Consul adapter structure test passed")
}

func TestEtcdAdapter(t *testing.T) {
	// This test would require a running etcd instance
	// For now, we just test the adapter structure
	
	// Create an etcd adapter with mock discovery
	adapter := &etcdAdapter{registry: nil} // We can't easily mock the etcd.Registry
	
	// Test that the adapter implements the Discovery interface
	var _ Discovery = adapter
	
	// In a real test environment with etcd running, we would test:
	// - Register service
	// - Get service
	// - Deregister service
	t.Log("etcd adapter structure test passed")
}

func TestConcurrentDiscoveryOperations(t *testing.T) {
	mock := newMockDiscovery()
	ctx := context.Background()

	const numGoroutines = 10
	const numInstances = 5

	// Concurrently register instances
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numInstances; j++ {
				instance := &ServiceInstance{
					ID:      fmt.Sprintf("service-%d-%d", id, j),
					Name:    "user-service",
					Address: fmt.Sprintf("127.0.0.%d", id),
					Port:    8080 + j,
				}
				mock.Register(ctx, instance)
			}
		}(i)
	}

	// Wait for all registrations to complete
	time.Sleep(100 * time.Millisecond)

	// Verify all instances are registered
	instances, err := mock.GetService(ctx, "user-service")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}

	expectedInstances := numGoroutines * numInstances
	if len(instances) != expectedInstances {
		t.Errorf("Expected %d instances, got %d", expectedInstances, len(instances))
	}
}

func TestDiscoveryConfig(t *testing.T) {
	config := Config{
		Type:    "consul",
		Address: "127.0.0.1:8500",
		Timeout: 5 * time.Second,
	}

	if config.Type != "consul" {
		t.Errorf("Expected Type 'consul', got '%s'", config.Type)
	}
	if config.Address != "127.0.0.1:8500" {
		t.Errorf("Expected Address '127.0.0.1:8500', got '%s'", config.Address)
	}
	if config.Timeout != 5*time.Second {
		t.Errorf("Expected Timeout 5s, got %v", config.Timeout)
	}
}

func TestConfigValidate_AppliesDefaultsInPlace(t *testing.T) {
	cfg := Config{}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if cfg.Type != RegistryTypeConsul {
		t.Fatalf("expected default type consul, got %s", cfg.Type)
	}
	if cfg.Address != "localhost:8500" {
		t.Fatalf("expected default consul address, got %s", cfg.Address)
	}
	if cfg.Namespace != "default" {
		t.Fatalf("expected default namespace, got %s", cfg.Namespace)
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("expected default timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.HealthCheckInterval != 30*time.Second {
		t.Fatalf("expected default health check interval 30s, got %v", cfg.HealthCheckInterval)
	}
	if cfg.DeregisterCriticalServiceAfter != 24*time.Hour {
		t.Fatalf("expected default deregister duration 24h, got %v", cfg.DeregisterCriticalServiceAfter)
	}
	if cfg.Options == nil {
		t.Fatal("expected options map to be initialized")
	}
}

func TestConfigValidate_StaticRegistryDoesNotRequireAddress(t *testing.T) {
	cfg := Config{Type: RegistryTypeStatic}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Address != "" {
		t.Fatalf("expected static registry to keep empty address, got %s", cfg.Address)
	}
}
