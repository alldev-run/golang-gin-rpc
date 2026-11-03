package discovery

import (
	"context"
	"testing"
)

func TestLoadBalancerStrategiesAndRegistry(t *testing.T) {
	instances := []*ServiceInstance{
		{ID: "one", Name: "api", Payload: map[string]string{"weight": "1"}},
		{ID: "two", Name: "api", Payload: map[string]string{"weight": "3"}},
	}
	ctx := context.Background()
	lb := NewLoadBalancer(StrategyRoundRobin)
	if _, err := lb.NextInstance(ctx, ""); err == nil {
		t.Fatal("NextInstance() succeeded without instances")
	}
	lb.UpdateInstances(instances)
	if first, _ := lb.NextInstance(ctx, ""); first != instances[0] {
		t.Fatalf("round-robin first instance = %v", first)
	}
	if second, _ := lb.NextInstance(ctx, ""); second != instances[1] {
		t.Fatalf("round-robin second instance = %v", second)
	}
	if got := lb.GetInstances(); len(got) != 2 {
		t.Fatalf("GetInstances() = %v", got)
	}

	for _, strategy := range []LoadBalancerStrategy{
		StrategyRandom, StrategyWeightedRandom, StrategyLeastConn, StrategyIPHash, "unknown",
	} {
		lb.SetStrategy(strategy)
		instance, err := lb.NextInstance(ctx, "192.0.2.10")
		if err != nil || instance == nil {
			t.Errorf("NextInstance(%q) = %v, %v", strategy, instance, err)
		}
	}
	lb.SetStrategy(StrategyIPHash)
	first, _ := lb.NextInstance(ctx, "192.0.2.10")
	second, _ := lb.NextInstance(ctx, "192.0.2.10")
	if first != second {
		t.Fatal("IP hash selected different instances for the same client")
	}

	lb.IncrementConnections("one")
	lb.IncrementConnections("one")
	if got := lb.GetConnections("one"); got != 2 {
		t.Fatalf("GetConnections() = %d, want 2", got)
	}
	lb.DecrementConnections("one")
	lb.DecrementConnections("one")
	lb.DecrementConnections("one")
	if got := lb.GetConnections("one"); got != 0 {
		t.Fatalf("connection count after decrements = %d", got)
	}
	lb.UpdateInstances(instances[:1])
	if got := lb.GetConnections("two"); got != 0 {
		t.Fatalf("removed instance connection count = %d", got)
	}

	registry := NewServiceRegistry()
	balancer := registry.RegisterService("api", StrategyRoundRobin)
	registry.UpdateServiceInstances("api", instances)
	if got, ok := registry.GetLoadBalancer("api"); !ok || got != balancer {
		t.Fatal("GetLoadBalancer() did not return the registered balancer")
	}
	if names := registry.ListServices(); len(names) != 1 || names[0] != "api" {
		t.Fatalf("ListServices() = %v", names)
	}
	if info := registry.GetServiceInfo("api"); info == nil || info["instances"] != 2 {
		t.Fatalf("GetServiceInfo() = %v", info)
	}
	if all := registry.GetAllServiceInfo(); all["api"] == nil {
		t.Fatalf("GetAllServiceInfo() = %v", all)
	}
	registry.RemoveService("api")
	if _, ok := registry.GetLoadBalancer("api"); ok {
		t.Fatal("RemoveService() left the service registered")
	}
}

func TestServiceSelectorAndConnectionTracker(t *testing.T) {
	backend := newMockDiscovery()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := backend.Register(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	manager := &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true},
		discovery: backend,
		instances: map[string]*ServiceInstance{instance.ID: instance},
	}
	selector := NewServiceSelector(manager, StrategyRoundRobin)
	selected, tracker, err := selector.SelectInstance(context.Background(), "api", "127.0.0.1")
	if err != nil || selected.ID != instance.ID || tracker == nil {
		t.Fatalf("SelectInstance() = %v, %v, %v", selected, tracker, err)
	}
	if got, ok := selector.GetRegistry().GetLoadBalancer("api"); !ok || got == nil {
		t.Fatal("selector did not register the service load balancer")
	}
	selector.SetServiceStrategy("api", StrategyIPHash)
	if err := selector.UpdateService(context.Background(), "api"); err != nil {
		t.Fatalf("UpdateService() error = %v", err)
	}
	tracker.Close()
	loadBalancer, ok := selector.GetRegistry().GetLoadBalancer("api")
	if !ok {
		t.Fatal("service load balancer disappeared")
	}
	if got := loadBalancer.GetConnections(instance.ID); got != 0 {
		t.Fatalf("connection count after tracker close = %d", got)
	}
	if _, _, err := selector.SelectInstance(context.Background(), "missing", ""); err == nil {
		t.Fatal("SelectInstance() succeeded for a missing service")
	}
}
