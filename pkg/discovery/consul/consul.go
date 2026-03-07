package consul

import (
	"context"
	"fmt"
	"golang-gin-rpc/pkg/discovery"

	"github.com/hashicorp/consul/api"
)

type Registry struct {
	client *api.Client
}

func NewRegistry(addr string) (*Registry, error) {
	config := api.DefaultConfig()
	config.Address = addr
	client, err := api.NewClient(config)
	return &Registry{client: client}, err
}

func (r *Registry) Register(ctx context.Context, inst *discovery.ServiceInstance) error {
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
