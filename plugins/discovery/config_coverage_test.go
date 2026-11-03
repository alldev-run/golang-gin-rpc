package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscoveryConfigPresets(t *testing.T) {
	configs := []struct {
		name string
		cfg  Config
		typ  RegistryType
	}{
		{name: "default", cfg: DefaultConfig(), typ: RegistryTypeConsul},
		{name: "consul", cfg: ConsulConfig("consul:8500"), typ: RegistryTypeConsul},
		{name: "etcd", cfg: EtcdConfig("etcd:2379"), typ: RegistryTypeEtcd},
		{name: "zookeeper", cfg: ZookeeperConfig("zk:2181"), typ: RegistryTypeZk},
		{name: "nacos", cfg: NacosConfig("nacos:8848"), typ: RegistryTypeNacos},
		{name: "static", cfg: StaticConfig(), typ: RegistryTypeStatic},
		{name: "development", cfg: DevelopmentConfig(), typ: RegistryTypeConsul},
		{name: "production", cfg: ProductionConfig("consul.prod:8500"), typ: RegistryTypeConsul},
	}
	for _, test := range configs {
		t.Run(test.name, func(t *testing.T) {
			if test.cfg.Type != test.typ || test.cfg.Timeout <= 0 || !test.cfg.Enabled {
				t.Fatalf("configuration = %+v", test.cfg)
			}
			if err := test.cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	managerConfig := DefaultManagerConfig()
	if managerConfig.HealthCheckTimeout != 2*time.Second || managerConfig.Timeout <= 0 {
		t.Fatalf("DefaultManagerConfig() = %+v", managerConfig)
	}
}

func TestConsulDiscoveryAdapterAgainstHTTPServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health/service/api":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Node":{"Node":"node-1","Address":"127.0.0.1"},"Service":{"ID":"api-1","Service":"api","Address":"127.0.0.1","Port":8080,"Meta":{"weight":"3"}},"Checks":[]}]`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	registry, err := NewDiscovery(Config{Type: RegistryTypeConsul, Address: server.URL})
	if err != nil {
		t.Fatalf("NewDiscovery() error = %v", err)
	}
	ctx := context.Background()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	instances, err := registry.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(instances) != 1 || instances[0].ID != instance.ID || instances[0].Payload["weight"] != "3" {
		t.Fatalf("GetService() = %+v", instances)
	}
	if err := registry.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
}
