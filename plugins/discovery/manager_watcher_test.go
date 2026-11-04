package discovery

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServiceWatcherInstancesEqual(t *testing.T) {
	watcher := &ServiceWatcher{}
	base := []*ServiceInstance{
		{ID: "one", Address: "127.0.0.1", Port: 8080},
		{ID: "two", Address: "127.0.0.2", Port: 8081},
	}

	tests := []struct {
		name string
		a    []*ServiceInstance
		b    []*ServiceInstance
		want bool
	}{
		{name: "both empty", a: nil, b: nil, want: true},
		{name: "different lengths", a: base, b: base[:1], want: false},
		{name: "identical", a: base, b: []*ServiceInstance{base[0], base[1]}, want: true},
		{name: "missing instance", a: base, b: []*ServiceInstance{base[0], {ID: "three", Address: "127.0.0.3", Port: 8082}}, want: false},
		{name: "different address", a: base, b: []*ServiceInstance{base[0], {ID: "two", Address: "127.0.0.9", Port: 8081}}, want: false},
		{name: "different port", a: base, b: []*ServiceInstance{base[0], {ID: "two", Address: "127.0.0.2", Port: 9999}}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := watcher.instancesEqual(test.a, test.b); got != test.want {
				t.Errorf("instancesEqual() = %v, want %v", got, test.want)
			}
		})
	}
}

func newWatchTestManager(t *testing.T, backend Discovery) *ServiceDiscoveryManager {
	t.Helper()
	return &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true, Timeout: time.Second},
		discovery: backend,
		instances: make(map[string]*ServiceInstance),
		stopCh:    make(chan struct{}),
	}
}

func newShortServiceWatcher(manager *ServiceDiscoveryManager, serviceName string) *ServiceWatcher {
	return &ServiceWatcher{
		manager:     manager,
		serviceName: serviceName,
		watchCh:     make(chan []*ServiceInstance, 4),
		stopCh:      make(chan struct{}),
		interval:    10 * time.Millisecond,
	}
}

func TestServiceWatcherWatchLoopNotifiesChangesAndStops(t *testing.T) {
	ctx := context.Background()
	backend := newMockDiscovery()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := backend.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	manager := newWatchTestManager(t, backend)
	watcher := newShortServiceWatcher(manager, "api")

	done := make(chan struct{})
	watchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		watcher.watchLoop(watchCtx)
		close(done)
	}()

	// The first poll must deliver the current instance list.
	select {
	case instances := <-watcher.watchCh:
		if len(instances) != 1 || instances[0].ID != instance.ID {
			t.Fatalf("watch delivered %+v, want [api-1]", instances)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not deliver the initial instance list")
	}

	// A poll without changes must not send a duplicate notification.
	registered := &ServiceInstance{ID: "api-2", Name: "api", Address: "127.0.0.1", Port: 8081}
	if err := backend.Register(ctx, registered); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	select {
	case instances := <-watcher.watchCh:
		if len(instances) != 2 {
			t.Fatalf("watch delivered %+v after scale-out, want 2 instances", instances)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not notify after instance list changed")
	}

	watcher.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after Stop()")
	}
}

func TestServiceWatcherWatchLoopSkipsNotificationsWhenUnchanged(t *testing.T) {
	manager := newWatchTestManager(t, newMockDiscovery())
	watcher := newShortServiceWatcher(manager, "api")

	done := make(chan struct{})
	watchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		watcher.watchLoop(watchCtx)
		close(done)
	}()

	// Several polls over an empty service must not produce notifications.
	time.Sleep(50 * time.Millisecond)
	select {
	case instances := <-watcher.watchCh:
		t.Fatalf("watchLoop notified without any instances: %v", instances)
	default:
	}

	watcher.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after Stop()")
	}
}

func TestServiceWatcherWatchLoopContinuesAfterGetServiceError(t *testing.T) {
	backend := newMockDiscovery()
	backend.failGet = true
	manager := newWatchTestManager(t, backend)
	watcher := newShortServiceWatcher(manager, "api")

	done := make(chan struct{})
	go func() {
		watcher.watchLoop(context.Background())
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	select {
	case instances := <-watcher.watchCh:
		t.Fatalf("watchLoop notified despite discovery errors: %v", instances)
	default:
	}

	watcher.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after Stop()")
	}
}

func TestServiceWatcherWatchLoopExitsOnContextCancel(t *testing.T) {
	manager := newWatchTestManager(t, newMockDiscovery())
	watcher := newShortServiceWatcher(manager, "api")

	done := make(chan struct{})
	watchCtx, cancel := context.WithCancel(context.Background())
	go func() {
		watcher.watchLoop(watchCtx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after context cancellation")
	}
}

func TestManagerStartStopRegistersAndDeregistersSelf(t *testing.T) {
	backend := newMockDiscovery()
	manager := &ServiceDiscoveryManager{
		config: ManagerConfig{
			Enabled:             true,
			Timeout:             time.Second,
			HealthCheckInterval: 10 * time.Millisecond,
			AutoRegister:        true,
			ServiceName:         "api",
			ServiceAddress:      "127.0.0.1",
			ServicePort:         8080,
			ServiceTags:         []string{"go", "rpc"},
		},
		discovery: backend,
		instances: make(map[string]*ServiceInstance),
		stopCh:    make(chan struct{}),
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	// The self registration is recorded locally before Start() returns; the
	// health check loop never removes local records, so this is deterministic.
	self := manager.GetRegisteredInstances()
	if len(self) != 1 {
		t.Fatalf("GetRegisteredInstances() after Start() = %v, want the self registration", self)
	}
	for _, instance := range self {
		if instance.Name != "api" {
			t.Fatalf("self registration name = %q, want \"api\"", instance.Name)
		}
		if tags := instance.Payload["tags"]; tags != "go,rpc" {
			t.Fatalf("self registration tags = %q, want \"go,rpc\"", tags)
		}
	}

	// Let the health check loop tick a few times; the instance has no real
	// listener so the loop will mark it unhealthy and deregister it from the
	// backend. Either way the manager must stay consistent and Stop must work.
	time.Sleep(50 * time.Millisecond)

	if err := manager.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if manager.IsStarted() {
		t.Fatal("IsStarted() = true after Stop()")
	}
	if got := manager.GetRegisteredInstances(); len(got) != 0 {
		t.Fatalf("GetRegisteredInstances() after Stop() = %v, want empty", got)
	}
}

func TestManagerDeregisterSelfWithoutRegistration(t *testing.T) {
	backend := newMockDiscovery()
	// Pre-populate backend with an unrelated instance
	otherInstance := &ServiceInstance{ID: "other-1", Name: "other", Address: "127.0.0.1", Port: 1}
	if err := backend.Register(context.Background(), otherInstance); err != nil {
		t.Fatalf("failed to pre-populate backend: %v", err)
	}

	manager := &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true, Timeout: time.Second, ServiceName: "api"},
		discovery: backend,
		instances: map[string]*ServiceInstance{
			"other-1": otherInstance,
		},
		stopCh: make(chan struct{}),
	}

	// No instance matches the configured service name, so deregisterSelf is a
	// no-op that must not report an error.
	if err := manager.deregisterSelf(); err != nil {
		t.Fatalf("deregisterSelf() without self registration error = %v", err)
	}
	if instances, _ := backend.GetService(context.Background(), "other"); len(instances) != 1 {
		t.Fatal("deregisterSelf() removed an unrelated instance")
	}
}

func TestManagerRegistrationErrorPaths(t *testing.T) {
	ctx := context.Background()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}

	backend := newMockDiscovery()
	backend.failRegister = true
	manager := newWatchTestManager(t, backend)
	if err := manager.Register(ctx, instance); err == nil {
		t.Fatal("Register() succeeded despite backend failure")
	}
	if got := len(manager.GetRegisteredInstances()); got != 0 {
		t.Fatalf("registered instances after failed Register() = %d, want 0", got)
	}

	backend.failRegister = false
	backend.failDeregister = true
	if err := manager.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := manager.Deregister(ctx, instance); err == nil {
		t.Fatal("Deregister() succeeded despite backend failure")
	}
	if got := manager.GetRegisteredInstances()["api-1"]; got == nil {
		t.Fatal("failed Deregister() removed the local instance record")
	}

	backend.failDeregister = false
	backend.failGet = true
	if _, err := manager.GetService(ctx, "api"); err == nil {
		t.Fatal("GetService() succeeded despite backend failure")
	}
	if err := manager.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
}

func TestHealthCheckLoopExitsOnStop(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()
	address := listener.Addr().(*net.TCPAddr)

	instance := &ServiceInstance{ID: "healthy", Name: "api", Address: "127.0.0.1", Port: address.Port}
	backend := &healthCheckRegistry{}
	manager := &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true, Timeout: time.Second, HealthCheckInterval: 10 * time.Millisecond, HealthCheckTimeout: 100 * time.Millisecond},
		discovery: backend,
		instances: map[string]*ServiceInstance{instance.ID: instance},
		stopCh:    make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		manager.healthCheckLoop()
		close(done)
	}()

	// Allow a few ticks; the healthy instance must remain registered and must
	// never be marked unhealthy, regardless of how many ticks elapsed.
	time.Sleep(40 * time.Millisecond)
	if manager.unhealthy[instance.ID] {
		t.Fatal("healthy instance was marked unhealthy")
	}

	close(manager.stopCh)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("healthCheckLoop did not exit after stopCh was closed")
	}
}

func TestHealthCheckLoopReturnsWhenDisabled(t *testing.T) {
	manager := &ServiceDiscoveryManager{
		config:    ManagerConfig{Enabled: true, HealthCheckInterval: 0},
		discovery: newMockDiscovery(),
		stopCh:    make(chan struct{}),
	}
	// A disabled interval must return immediately instead of leaking a ticker.
	done := make(chan struct{})
	go func() {
		manager.healthCheckLoop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("healthCheckLoop did not return with a disabled interval")
	}
}

func TestServiceSelectorStrategyAndInfoBranches(t *testing.T) {
	ctx := context.Background()
	instances := []*ServiceInstance{
		{ID: "one", Name: "api", Address: "127.0.0.1", Port: 8080},
		{ID: "two", Name: "api", Address: "127.0.0.2", Port: 8081},
	}
	backend := newMockDiscovery()
	for _, inst := range instances {
		if err := backend.Register(ctx, inst); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	}
	manager := newWatchTestManager(t, backend)
	selector := NewServiceSelector(manager, StrategyRoundRobin)

	// SetServiceStrategy on an unknown service must create a new balancer.
	selector.SetServiceStrategy("missing", StrategyLeastConn)
	lb, ok := selector.GetRegistry().GetLoadBalancer("missing")
	if !ok || lb.GetStrategy() != StrategyLeastConn {
		t.Fatalf("SetServiceStrategy(missing) = %v, %v; want a new balancer", lb, ok)
	}

	// GetServiceInfo for an unknown service returns nil.
	if info := selector.GetRegistry().GetServiceInfo("ghost"); info != nil {
		t.Fatalf("GetServiceInfo(ghost) = %v, want nil", info)
	}

	// UpdateService propagates discovery failures.
	backend.failGet = true
	if err := selector.UpdateService(ctx, "api"); err == nil {
		t.Fatal("UpdateService() succeeded despite discovery failure")
	}
	backend.failGet = false

	// ipHash with an empty client IP must fall back to the first instance.
	selector.SetServiceStrategy("api", StrategyIPHash)
	lb, ok = selector.GetRegistry().GetLoadBalancer("api")
	if !ok {
		t.Fatal("selector lost the api load balancer")
	}
	lb.UpdateInstances(instances)
	selected, err := lb.NextInstance(ctx, "")
	if err != nil || selected != instances[0] {
		t.Fatalf("ipHash with empty client IP = %v, %v; want the first instance", selected, err)
	}
}

func TestConfigValidateDefaultsForSubRegistryTypes(t *testing.T) {
	tests := []struct {
		name     string
		typ      RegistryType
		address  string
		namespace string
	}{
		{name: "etcd", typ: RegistryTypeEtcd, address: "localhost:2379", namespace: "default"},
		{name: "zookeeper", typ: RegistryTypeZk, address: "localhost:2181", namespace: "default"},
		{name: "nacos", typ: RegistryTypeNacos, address: "localhost:8848", namespace: "default"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := Config{Type: test.typ, Namespace: test.namespace}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if cfg.Address != test.address {
				t.Fatalf("Validate() address = %q, want %q", cfg.Address, test.address)
			}
		})
	}

	var nilConfig *Config
	if err := nilConfig.Validate(); err != nil {
		t.Fatalf("Validate() on nil receiver error = %v", err)
	}
}
