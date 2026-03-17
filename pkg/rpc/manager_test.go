package rpc

import (
	"context"
	"testing"
	"time"

	"alldev-gin-rpc/pkg/discovery"
)

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	// Check default servers
	if len(config.Servers) != 2 {
		t.Errorf("Expected 2 default servers, got %d", len(config.Servers))
	}

	// Check GRPC server config
	grpcConfig, exists := config.Servers["grpc"]
	if !exists {
		t.Error("Expected grpc server config")
	} else {
		if grpcConfig.Type != ServerTypeGRPC {
			t.Errorf("Expected GRPC server type, got %s", grpcConfig.Type)
		}
		if grpcConfig.Host != "localhost" {
			t.Errorf("Expected host 'localhost', got %s", grpcConfig.Host)
		}
		if grpcConfig.Port != 50051 {
			t.Errorf("Expected port 50051, got %d", grpcConfig.Port)
		}
		if grpcConfig.Network != "tcp" {
			t.Errorf("Expected network 'tcp', got %s", grpcConfig.Network)
		}
		if grpcConfig.Timeout != 30 {
			t.Errorf("Expected timeout 30, got %d", grpcConfig.Timeout)
		}
		if grpcConfig.MaxMsgSize != 4*1024*1024 {
			t.Errorf("Expected MaxMsgSize %d, got %d", 4*1024*1024, grpcConfig.MaxMsgSize)
		}
		if !grpcConfig.Reflection {
			t.Error("Expected reflection to be enabled")
		}
	}

	// Check JSONRPC server config
	jsonrpcConfig, exists := config.Servers["jsonrpc"]
	if !exists {
		t.Error("Expected jsonrpc server config")
	} else {
		if jsonrpcConfig.Type != ServerTypeJSONRPC {
			t.Errorf("Expected JSONRPC server type, got %s", jsonrpcConfig.Type)
		}
		if jsonrpcConfig.Host != "localhost" {
			t.Errorf("Expected host 'localhost', got %s", jsonrpcConfig.Host)
		}
		if jsonrpcConfig.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", jsonrpcConfig.Port)
		}
		if jsonrpcConfig.Network != "tcp" {
			t.Errorf("Expected network 'tcp', got %s", jsonrpcConfig.Network)
		}
		if jsonrpcConfig.Timeout != 30 {
			t.Errorf("Expected timeout 30, got %d", jsonrpcConfig.Timeout)
		}
	}

	// Check timeout settings
	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}
	if config.GracefulShutdownTimeout != 10*time.Second {
		t.Errorf("Expected graceful shutdown timeout 10s, got %v", config.GracefulShutdownTimeout)
	}
}

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("NewManager() returned nil")
	}

	// Test that servers are initialized
	servers := manager.ListServers()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Test that we can get specific servers
	grpcServer, exists := manager.GetServer("grpc")
	if !exists {
		t.Error("Expected grpc server to be available")
	}
	if grpcServer == nil {
		t.Error("Expected grpc server to not be nil")
	}

	jsonrpcServer, exists := manager.GetServer("jsonrpc")
	if !exists {
		t.Error("Expected jsonrpc server to be available")
	}
	if jsonrpcServer == nil {
		t.Error("Expected jsonrpc server to not be nil")
	}

	// Test getting non-existent server
	_, exists = manager.GetServer("non-existent")
	if exists {
		t.Error("Expected non-existent server to not exist")
	}
}

func TestManager_StartStop(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	// Test starting all servers
	err := manager.Start()
	if err != nil {
		t.Errorf("Failed to start manager: %v", err)
	}

	// Test that manager is started
	if !manager.IsStarted() {
		t.Error("Expected manager to be started")
	}

	// Test stopping all servers
	err = manager.Stop()
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}

	// Test that manager is stopped
	if manager.IsStarted() {
		t.Error("Expected manager to be stopped")
	}
}

func TestManager_RegisterService(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	// Create a mock service
	mockService := &MockService{
		name: "test-service",
	}

	// Register service
	err := manager.RegisterService(mockService)
	if err != nil {
		t.Errorf("Failed to register service: %v", err)
	}

	// Check that service was registered
	service, exists := manager.GetService("test-service")
	if !exists {
		t.Error("Expected service to be registered")
	}
	if service == nil {
		t.Error("Expected service to not be nil")
	}

	// Test registering duplicate service
	err = manager.RegisterService(mockService)
	if err == nil {
		t.Error("Expected error when registering duplicate service")
	}
}

func TestManager_GetStatus(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	// Get status before starting
	status := manager.GetStatus()
	if status == nil {
		t.Error("Expected status to not be nil")
	}

	// Start manager
	err := manager.Start()
	if err != nil {
		t.Errorf("Failed to start manager: %v", err)
	}

	// Get status after starting
	status = manager.GetStatus()
	if status == nil {
		t.Error("Expected status to not be nil")
	}

	// Stop manager
	err = manager.Stop()
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	// Test concurrent status checks
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			status := manager.GetStatus()
			if status == nil {
				t.Error("Expected status to not be nil")
			}
			done <- true
		}()
	}

	// Wait for all operations to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestManager_ConfigValidation(t *testing.T) {
	// Test with empty config
	config := ManagerConfig{}
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("NewManager() returned nil for empty config")
	}

	// Should still be able to create manager
	servers := manager.ListServers()
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers for empty config, got %d", len(servers))
	}
}

func TestManager_AddRemoveServer(t *testing.T) {
	config := ManagerConfig{}
	manager := NewManager(config)

	// Add a new server
	newServerConfig := Config{
		Type:    ServerTypeJSONRPC,
		Host:    "localhost",
		Port:    9090,
		Network: "tcp",
		Timeout: 30,
	}

	err := manager.AddServer("test-server", newServerConfig)
	if err != nil {
		t.Errorf("Failed to add server: %v", err)
	}

	// Check that server was added
	server, exists := manager.GetServer("test-server")
	if !exists {
		t.Error("Expected test server to be available")
	}
	if server == nil {
		t.Error("Expected test server to not be nil")
	}

	// Remove the server
	err = manager.RemoveServer("test-server")
	if err != nil {
		t.Errorf("Failed to remove server: %v", err)
	}

	// Check that server was removed
	_, exists = manager.GetServer("test-server")
	if exists {
		t.Error("Expected test server to be removed")
	}
}

func TestManager_AddRemoveServer_AfterStart(t *testing.T) {
	config := ManagerConfig{}
	manager := NewManager(config)

	// Start manager
	err := manager.Start()
	if err != nil {
		t.Errorf("Failed to start manager: %v", err)
	}

	// Try to add server after start
	newServerConfig := Config{
		Type:    ServerTypeJSONRPC,
		Host:    "localhost",
		Port:    9090,
		Network: "tcp",
		Timeout: 30,
	}

	err = manager.AddServer("test-server", newServerConfig)
	if err == nil {
		t.Error("Expected error when adding server after start")
	}

	// Try to remove server after start
	err = manager.RemoveServer("test-server")
	if err == nil {
		t.Error("Expected error when removing server after start")
	}

	// Stop manager
	err = manager.Stop()
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}
}

func TestManager_RegisterService_AfterStart(t *testing.T) {
	config := ManagerConfig{}
	manager := NewManager(config)

	// Start manager
	err := manager.Start()
	if err != nil {
		t.Errorf("Failed to start manager: %v", err)
	}

	// Try to register service after start
	mockService := &MockService{
		name: "test-service",
	}

	err = manager.RegisterService(mockService)
	if err == nil {
		t.Error("Expected error when registering service after start")
	}

	// Stop manager
	err = manager.Stop()
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}
}

func TestManager_ListMethods(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	// Test listing servers
	servers := manager.ListServers()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Test listing services
	services := manager.ListServices()
	if len(services) != 0 {
		t.Errorf("Expected 0 services initially, got %d", len(services))
	}

	// Add a service
	mockService := &MockService{
		name: "test-service",
	}

	err := manager.RegisterService(mockService)
	if err != nil {
		t.Errorf("Failed to register service: %v", err)
	}

	// Test listing services again
	services = manager.ListServices()
	if len(services) != 1 {
		t.Errorf("Expected 1 service after registration, got %d", len(services))
	}

	if services[0] != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", services[0])
	}
}

func TestManager_UnregisterService(t *testing.T) {
	config := ManagerConfig{}
	manager := NewManager(config)

	// Register a service
	mockService := &MockService{
		name: "test-service",
	}

	err := manager.RegisterService(mockService)
	if err != nil {
		t.Errorf("Failed to register service: %v", err)
	}

	// Unregister the service
	err = manager.UnregisterService("test-service")
	if err != nil {
		t.Errorf("Failed to unregister service: %v", err)
	}

	// Check that service was removed
	_, exists := manager.GetService("test-service")
	if exists {
		t.Error("Expected service to be removed")
	}

	// Test unregistering non-existent service
	err = manager.UnregisterService("non-existent")
	if err == nil {
		t.Error("Expected error when unregistering non-existent service")
	}
}

func TestManager_DiscoveryRegistrationLifecycle(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Servers: map[string]Config{
			"jsonrpc": {
				Type:    ServerTypeJSONRPC,
				Host:    "127.0.0.1",
				Port:    0,
				Network: "tcp",
				Timeout: 1,
			},
		},
		GracefulShutdownTimeout: time.Second,
	})
	mockDiscovery := &mockDiscoveryRegistrar{}

	err := manager.SetDiscoveryIntegration(mockDiscovery, DiscoveryRegistrationConfig{
		Enabled:        true,
		ServiceName:    "orders",
		ServiceAddress: "10.0.0.8",
		ServiceTags:    []string{"rpc", "jsonrpc"},
		Metadata:       map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("failed to set discovery integration: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}
	if len(mockDiscovery.registered) != 1 {
		t.Fatalf("expected 1 registered instance, got %d", len(mockDiscovery.registered))
	}
	if mockDiscovery.registered[0].Name != "orders-jsonrpc" {
		t.Fatalf("unexpected discovery service name: %s", mockDiscovery.registered[0].Name)
	}
	if mockDiscovery.registered[0].Address != "10.0.0.8" {
		t.Fatalf("unexpected discovery service address: %s", mockDiscovery.registered[0].Address)
	}
	if got := mockDiscovery.registered[0].Payload["protocol"]; got != "jsonrpc" {
		t.Fatalf("unexpected protocol metadata: %s", got)
	}

	if err := manager.Stop(); err != nil {
		t.Fatalf("failed to stop manager: %v", err)
	}
	if len(mockDiscovery.deregistered) != 1 {
		t.Fatalf("expected 1 deregistered instance, got %d", len(mockDiscovery.deregistered))
	}
}

func TestManager_SetDiscoveryIntegrationAfterStart_RegistersImmediately(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Servers: map[string]Config{
			"jsonrpc": {
				Type:    ServerTypeJSONRPC,
				Host:    "127.0.0.1",
				Port:    0,
				Network: "tcp",
				Timeout: 1,
			},
		},
		GracefulShutdownTimeout: time.Second,
	})
	mockDiscovery := &mockDiscoveryRegistrar{}

	if err := manager.Start(); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	err := manager.SetDiscoveryIntegration(mockDiscovery, DiscoveryRegistrationConfig{
		Enabled:        true,
		ServiceName:    "payments",
		ServiceAddress: "10.0.0.9",
		Metadata:       map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("failed to set discovery integration after start: %v", err)
	}
	if len(mockDiscovery.registered) != 1 {
		t.Fatalf("expected 1 registered instance after late integration, got %d", len(mockDiscovery.registered))
	}

	if err := manager.Stop(); err != nil {
		t.Fatalf("failed to stop manager: %v", err)
	}
}

// MockService for testing
type MockService struct {
	name string
}

func (m *MockService) Name() string {
	return m.name
}

func (m *MockService) Register(server interface{}) error {
	// Mock implementation
	return nil
}

type mockDiscoveryRegistrar struct {
	registered   []*discovery.ServiceInstance
	deregistered []*discovery.ServiceInstance
}

func (m *mockDiscoveryRegistrar) Register(ctx context.Context, instance *discovery.ServiceInstance) error {
	m.registered = append(m.registered, instance)
	return nil
}

func (m *mockDiscoveryRegistrar) Deregister(ctx context.Context, instance *discovery.ServiceInstance) error {
	m.deregistered = append(m.deregistered, instance)
	return nil
}
