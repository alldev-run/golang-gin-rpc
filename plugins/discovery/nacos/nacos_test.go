package nacos

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// fakeNamingClient implements naming_client.INamingClient with canned results
// and call recording, so Registry can be exercised without a Nacos server.
type fakeNamingClient struct {
	registerResult   bool
	registerErr      error
	lastRegister     vo.RegisterInstanceParam
	registerCalls    int
	deregisterResult bool
	deregisterErr    error
	lastDeregister   vo.DeregisterInstanceParam
	deregisterCalls  int
	selectInstances  []model.Instance
	selectErr        error
	serviceList      model.ServiceList
	serviceListErr   error
}

func (f *fakeNamingClient) RegisterInstance(param vo.RegisterInstanceParam) (bool, error) {
	f.registerCalls++
	f.lastRegister = param
	return f.registerResult, f.registerErr
}

func (f *fakeNamingClient) BatchRegisterInstance(vo.BatchRegisterInstanceParam) (bool, error) {
	return true, nil
}

func (f *fakeNamingClient) DeregisterInstance(param vo.DeregisterInstanceParam) (bool, error) {
	f.deregisterCalls++
	f.lastDeregister = param
	return f.deregisterResult, f.deregisterErr
}

func (f *fakeNamingClient) UpdateInstance(vo.UpdateInstanceParam) (bool, error) {
	return true, nil
}

func (f *fakeNamingClient) GetService(vo.GetServiceParam) (model.Service, error) {
	return model.Service{}, nil
}

func (f *fakeNamingClient) SelectAllInstances(vo.SelectAllInstancesParam) ([]model.Instance, error) {
	return nil, nil
}

func (f *fakeNamingClient) SelectInstances(vo.SelectInstancesParam) ([]model.Instance, error) {
	return f.selectInstances, f.selectErr
}

func (f *fakeNamingClient) SelectOneHealthyInstance(vo.SelectOneHealthInstanceParam) (*model.Instance, error) {
	return nil, nil
}

func (f *fakeNamingClient) Subscribe(*vo.SubscribeParam) error { return nil }

func (f *fakeNamingClient) Unsubscribe(*vo.SubscribeParam) error { return nil }

func (f *fakeNamingClient) GetAllServicesInfo(vo.GetAllServiceInfoParam) (model.ServiceList, error) {
	return f.serviceList, f.serviceListErr
}

func (f *fakeNamingClient) ServerHealthy() bool { return true }

func (f *fakeNamingClient) CloseClient() {}

var _ naming_client.INamingClient = (*fakeNamingClient)(nil)

func TestNewRegistryRequiresAddress(t *testing.T) {
	_, err := NewRegistry("", time.Second, nil)
	if err == nil {
		t.Fatal("NewRegistry(\"\") succeeded, want error")
	}
	if err.Error() != "nacos address is required" {
		t.Fatalf("NewRegistry(\"\") error = %v, want address-required message", err)
	}
}

func newFakeRegistry(client naming_client.INamingClient) *Registry {
	return &Registry{client: client}
}

func TestRegisterInstance(t *testing.T) {
	ctx := context.Background()
	instance := &ServiceInstance{
		ID:      "api-1",
		Name:    "api",
		Address: "127.0.0.1",
		Port:    8080,
		Payload: map[string]string{"version": "1.0.0"},
	}

	t.Run("nil registry and client", func(t *testing.T) {
		var nilRegistry *Registry
		if err := nilRegistry.Register(ctx, instance); err == nil {
			t.Fatal("nil registry Register() succeeded, want error")
		}
		if err := newFakeRegistry(nil).Register(ctx, instance); err == nil {
			t.Fatal("nil client Register() succeeded, want error")
		}
	})

	t.Run("nil instance", func(t *testing.T) {
		if err := newFakeRegistry(&fakeNamingClient{}).Register(ctx, nil); err == nil {
			t.Fatal("Register(nil instance) succeeded, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		client := &fakeNamingClient{registerResult: true}
		registry := newFakeRegistry(client)
		if err := registry.Register(ctx, instance); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if client.registerCalls != 1 {
			t.Fatalf("RegisterInstance calls = %d, want 1", client.registerCalls)
		}
		if client.lastRegister.ServiceName != instance.Name ||
			client.lastRegister.Ip != instance.Address ||
			client.lastRegister.Port != uint64(instance.Port) ||
			client.lastRegister.Metadata["version"] != "1.0.0" {
			t.Fatalf("RegisterInstance param = %+v, want mirrored instance fields", client.lastRegister)
		}
	})

	t.Run("sdk error", func(t *testing.T) {
		client := &fakeNamingClient{registerErr: errors.New("boom")}
		if err := newFakeRegistry(client).Register(ctx, instance); err == nil {
			t.Fatal("Register() succeeded despite SDK error")
		}
	})

	t.Run("sdk returned false", func(t *testing.T) {
		client := &fakeNamingClient{registerResult: false}
		if err := newFakeRegistry(client).Register(ctx, instance); err == nil {
			t.Fatal("Register() succeeded despite SDK returning false")
		}
	})
}

func TestDeregisterInstance(t *testing.T) {
	ctx := context.Background()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}

	t.Run("nil registry, client or instance is a no-op", func(t *testing.T) {
		var nilRegistry *Registry
		if err := nilRegistry.Deregister(ctx, instance); err != nil {
			t.Fatalf("nil registry Deregister() error = %v", err)
		}
		if err := newFakeRegistry(nil).Deregister(ctx, instance); err != nil {
			t.Fatalf("nil client Deregister() error = %v", err)
		}
		if err := newFakeRegistry(&fakeNamingClient{}).Deregister(ctx, nil); err != nil {
			t.Fatalf("nil instance Deregister() error = %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		client := &fakeNamingClient{deregisterResult: true}
		if err := newFakeRegistry(client).Deregister(ctx, instance); err != nil {
			t.Fatalf("Deregister() error = %v", err)
		}
		if client.deregisterCalls != 1 ||
			client.lastDeregister.ServiceName != instance.Name ||
			client.lastDeregister.Ip != instance.Address ||
			client.lastDeregister.Port != uint64(instance.Port) {
			t.Fatalf("DeregisterInstance calls/param = %d %+v, want 1 with mirrored fields",
				client.deregisterCalls, client.lastDeregister)
		}
	})

	t.Run("sdk error and false result", func(t *testing.T) {
		client := &fakeNamingClient{deregisterErr: errors.New("boom")}
		if err := newFakeRegistry(client).Deregister(ctx, instance); err == nil {
			t.Fatal("Deregister() succeeded despite SDK error")
		}
		client = &fakeNamingClient{deregisterResult: false}
		if err := newFakeRegistry(client).Deregister(ctx, instance); err == nil {
			t.Fatal("Deregister() succeeded despite SDK returning false")
		}
	})
}

func TestGetService(t *testing.T) {
	ctx := context.Background()

	t.Run("nil registry and client", func(t *testing.T) {
		var nilRegistry *Registry
		if _, err := nilRegistry.GetService(ctx, "api"); err == nil {
			t.Fatal("nil registry GetService() succeeded, want error")
		}
		if _, err := newFakeRegistry(nil).GetService(ctx, "api"); err == nil {
			t.Fatal("nil client GetService() succeeded, want error")
		}
	})

	t.Run("success maps nacos fields", func(t *testing.T) {
		client := &fakeNamingClient{selectInstances: []model.Instance{
			{InstanceId: "n-1", Ip: "10.0.0.1", Port: 9090, Metadata: map[string]string{"weight": "2"}},
			{InstanceId: "n-2", Ip: "10.0.0.2", Port: 9091},
		}}
		instances, err := newFakeRegistry(client).GetService(ctx, "api")
		if err != nil {
			t.Fatalf("GetService() error = %v", err)
		}
		if len(instances) != 2 {
			t.Fatalf("GetService() returned %d instances, want 2", len(instances))
		}
		if instances[0].ID != "n-1" || instances[0].Address != "10.0.0.1" || instances[0].Port != 9090 ||
			instances[0].Name != "api" || instances[0].Payload["weight"] != "2" {
			t.Fatalf("GetService()[0] = %+v", instances[0])
		}
		if instances[1].ID != "n-2" || instances[1].Payload != nil {
			t.Fatalf("GetService()[1] = %+v", instances[1])
		}
	})

	t.Run("empty result stays non-nil", func(t *testing.T) {
		instances, err := newFakeRegistry(&fakeNamingClient{}).GetService(ctx, "api")
		if err != nil || instances == nil || len(instances) != 0 {
			t.Fatalf("GetService() = %v, %v; want empty slice", instances, err)
		}
	})

	t.Run("sdk error", func(t *testing.T) {
		client := &fakeNamingClient{selectErr: errors.New("boom")}
		if _, err := newFakeRegistry(client).GetService(ctx, "api"); err == nil {
			t.Fatal("GetService() succeeded despite SDK error")
		}
	})
}

func TestGetAllServices(t *testing.T) {
	ctx := context.Background()

	t.Run("nil registry and client", func(t *testing.T) {
		var nilRegistry *Registry
		if _, err := nilRegistry.GetAllServices(ctx); err == nil {
			t.Fatal("nil registry GetAllServices() succeeded, want error")
		}
		if _, err := newFakeRegistry(nil).GetAllServices(ctx); err == nil {
			t.Fatal("nil client GetAllServices() succeeded, want error")
		}
	})

	t.Run("success", func(t *testing.T) {
		client := &fakeNamingClient{serviceList: model.ServiceList{Count: 2, Doms: []string{"api", "web"}}}
		names, err := newFakeRegistry(client).GetAllServices(ctx)
		if err != nil {
			t.Fatalf("GetAllServices() error = %v", err)
		}
		if len(names) != 2 || names[0] != "api" || names[1] != "web" {
			t.Fatalf("GetAllServices() = %v, want [api web]", names)
		}
	})

	t.Run("sdk error", func(t *testing.T) {
		client := &fakeNamingClient{serviceListErr: errors.New("boom")}
		if _, err := newFakeRegistry(client).GetAllServices(ctx); err == nil {
			t.Fatal("GetAllServices() succeeded despite SDK error")
		}
	})
}

func TestClose(t *testing.T) {
	var nilRegistry *Registry
	if err := nilRegistry.Close(); err != nil {
		t.Fatalf("nil registry Close() error = %v", err)
	}
	if err := newFakeRegistry(nil).Close(); err != nil {
		t.Fatalf("nil client Close() error = %v", err)
	}
	if err := newFakeRegistry(&fakeNamingClient{}).Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
