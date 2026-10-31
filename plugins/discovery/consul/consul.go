package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/api"
)

// ServiceInstance represents a service node information
type ServiceInstance struct {
	ID      string            // Unique instance ID
	Name    string            // Service name (e.g., user-service)
	Address string            // IP address
	Port    int               // Port number
	Payload map[string]string // Additional metadata
}

// Discovery unified interface
type Discovery interface {
	// Register registers a service
	Register(ctx context.Context, instance *ServiceInstance) error
	// Deregister unregisters a service
	Deregister(ctx context.Context, instance *ServiceInstance) error
	// GetService retrieves instance list by service name
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}

type Registry struct {
	client *api.Client
}

func NewRegistry(addr string) (*Registry, error) {
	config := api.DefaultConfig()
	config.Address = addr
	client, err := api.NewClient(config)
	return &Registry{client: client}, err
}

func (r *Registry) Register(ctx context.Context, inst *ServiceInstance) error {
	return r.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:      inst.ID,
		Name:    inst.Name,
		Address: inst.Address,
		Port:    inst.Port,
		Check: &api.AgentServiceCheck{ // Consul 自动健康检查
			TCP:      fmt.Sprintf("%s:%d", inst.Address, inst.Port),
			Interval: "10s",
			Timeout:  "5s",
		},
	})
}

func (r *Registry) Deregister(ctx context.Context, inst *ServiceInstance) error {
	return r.client.Agent().ServiceDeregister(inst.ID)
}

func (r *Registry) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	services, _, err := r.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return nil, err
	}

	var instances []*ServiceInstance
	for _, service := range services {
		inst := &ServiceInstance{
			ID:      service.Service.ID,
			Name:    service.Service.Service,
			Address: service.Service.Address,
			Port:    service.Service.Port,
			Payload: service.Service.Meta,
		}
		instances = append(instances, inst)
	}
	return instances, nil
}
