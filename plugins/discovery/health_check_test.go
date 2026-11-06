package discovery

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

type healthCheckRegistry struct {
	registered   []string
	deregistered []string
}

func (r *healthCheckRegistry) Register(_ context.Context, instance *ServiceInstance) error {
	r.registered = append(r.registered, instance.ID)
	return nil
}
func (r *healthCheckRegistry) Deregister(_ context.Context, instance *ServiceInstance) error {
	r.deregistered = append(r.deregistered, instance.ID)
	return nil
}
func (r *healthCheckRegistry) GetService(context.Context, string) ([]*ServiceInstance, error) {
	return nil, nil
}

func TestPerformHealthCheckDeregistersAndReregistersRecoveredInstance(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	address := listener.Addr().(*net.TCPAddr)
	_ = listener.Close()

	instance := &ServiceInstance{ID: "unhealthy", Address: "127.0.0.1", Port: address.Port}
	registry := &healthCheckRegistry{}
	manager := &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true, Timeout: time.Second, HealthCheckTimeout: 100 * time.Millisecond},
		discovery: registry,
		instances: map[string]*ServiceInstance{instance.ID: instance},
	}
	manager.performHealthCheck()

	if len(registry.deregistered) != 1 || registry.deregistered[0] != instance.ID {
		t.Fatalf("deregistered = %v, want [%s]", registry.deregistered, instance.ID)
	}
	if manager.instances[instance.ID] != instance || !manager.unhealthy[instance.ID] {
		t.Fatal("unhealthy instance was not retained and marked unhealthy")
	}

	manager.performHealthCheck()
	if len(registry.deregistered) != 1 {
		t.Fatalf("repeated unhealthy check deregistered again: %v", registry.deregistered)
	}

	listener, err = net.Listen("tcp", net.JoinHostPort(instance.Address, strconv.Itoa(instance.Port)))
	if err != nil {
		t.Fatalf("net.Listen() to recover instance: %v", err)
	}
	defer listener.Close()
	manager.performHealthCheck()
	if len(registry.registered) != 1 || registry.registered[0] != instance.ID {
		t.Fatalf("registered after recovery = %v, want [%s]", registry.registered, instance.ID)
	}
	if manager.unhealthy[instance.ID] {
		t.Fatal("recovered instance remains marked unhealthy")
	}

	manager.performHealthCheck()
	if len(registry.registered) != 1 {
		t.Fatalf("healthy check registered again: %v", registry.registered)
	}
}

func TestPerformHealthCheckKeepsHealthyInstance(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()
	address := listener.Addr().(*net.TCPAddr)

	instance := &ServiceInstance{ID: "healthy", Address: "127.0.0.1", Port: address.Port}
	registry := &healthCheckRegistry{}
	manager := &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true, Timeout: time.Second, HealthCheckTimeout: 100 * time.Millisecond},
		discovery: registry,
		instances: map[string]*ServiceInstance{instance.ID: instance},
	}
	manager.performHealthCheck()

	if len(registry.deregistered) != 0 {
		t.Fatalf("healthy instance was deregistered: %v", registry.deregistered)
	}
}
