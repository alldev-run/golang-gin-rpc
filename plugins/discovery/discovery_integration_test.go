package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func requireDiscoveryIntegrationEnv(t *testing.T, key string) string {
	t.Helper()
	if os.Getenv("DISCOVERY_INTEGRATION") == "" {
		t.Skip("set DISCOVERY_INTEGRATION=1 to enable discovery integration tests")
	}
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("set %s to enable this discovery integration test", key)
	}
	return value
}

func waitForDiscoveredInstance(ctx context.Context, d Discovery, serviceName, instanceID string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		instances, err := d.GetService(ctx, serviceName)
		if err == nil {
			for _, inst := range instances {
				if inst != nil && inst.ID == instanceID {
					return nil
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("instance %s not discovered for service %s within timeout", instanceID, serviceName)
}

func waitForDeregisteredInstance(ctx context.Context, d Discovery, serviceName, instanceID string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		instances, err := d.GetService(ctx, serviceName)
		if err == nil {
			found := false
			for _, inst := range instances {
				if inst != nil && inst.ID == instanceID {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("instance %s still present for service %s after timeout", instanceID, serviceName)
}

func TestDiscoveryIntegration_Consul_RegisterGetDeregister(t *testing.T) {
	addr := requireDiscoveryIntegrationEnv(t, "DISCOVERY_CONSUL_ADDR")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	d, err := NewDiscovery(Config{Type: RegistryTypeConsul, Address: addr, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewDiscovery() error = %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create tcp listener: %v", err)
	}
	defer listener.Close()

	tcpAddr := listener.Addr().(*net.TCPAddr)
	instance := &ServiceInstance{
		ID:      fmt.Sprintf("consul-it-%d", time.Now().UnixNano()),
		Name:    fmt.Sprintf("svc-consul-it-%d", time.Now().UnixNano()),
		Address: tcpAddr.IP.String(),
		Port:    tcpAddr.Port,
		Payload: map[string]string{"env": "integration"},
	}

	if err := d.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer d.Deregister(context.Background(), instance)

	if err := waitForDiscoveredInstance(ctx, d, instance.Name, instance.ID); err != nil {
		t.Fatal(err)
	}

	if err := d.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if err := waitForDeregisteredInstance(ctx, d, instance.Name, instance.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryIntegration_Etcd_RegisterGetDeregister(t *testing.T) {
	addr := requireDiscoveryIntegrationEnv(t, "DISCOVERY_ETCD_ADDR")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	d, err := NewDiscovery(Config{Type: RegistryTypeEtcd, Address: addr, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewDiscovery() error = %v", err)
	}

	instance := &ServiceInstance{
		ID:      fmt.Sprintf("etcd-it-%d", time.Now().UnixNano()),
		Name:    fmt.Sprintf("svc-etcd-it-%d", time.Now().UnixNano()),
		Address: "127.0.0.1",
		Port:    18080,
		Payload: map[string]string{"env": "integration"},
	}

	if err := d.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	defer d.Deregister(context.Background(), instance)

	if err := waitForDiscoveredInstance(ctx, d, instance.Name, instance.ID); err != nil {
		t.Fatal(err)
	}

	if err := d.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if err := waitForDeregisteredInstance(ctx, d, instance.Name, instance.ID); err != nil {
		t.Fatal(err)
	}
}
