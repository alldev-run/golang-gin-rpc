package discovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/discovery/consul"
	"github.com/alldev-run/golang-gin-rpc/pkg/discovery/etcd"
	"github.com/alldev-run/golang-gin-rpc/pkg/discovery/zookeeper"
)

// NewDiscovery 根据配置返回具体的实现实例
func NewDiscovery(conf Config) (Discovery, error) {
	switch conf.Type {
	case RegistryTypeConsul:
		registry, err := consul.NewRegistry(conf.Address)
		if err != nil {
			return nil, err
		}
		return &consulAdapter{registry: registry}, nil
	case RegistryTypeEtcd:
		registry, err := etcd.NewRegistry(conf.Address, conf.Timeout)
		if err != nil {
			return nil, err
		}
		return &etcdAdapter{registry: registry}, nil
	case RegistryTypeZk:
		registry, err := zookeeper.NewRegistry(conf.Address, conf.Timeout, conf.Options)
		if err != nil {
			return nil, err
		}
		return &zookeeperAdapter{registry: registry}, nil
	case "":
		return nil, errors.New("discovery type is required")
	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", conf.Type)
	}
}

type consulAdapter struct {
	registry *consul.Registry
}

func (c *consulAdapter) Register(ctx context.Context, instance *ServiceInstance) error {
	consulInst := &consul.ServiceInstance{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Payload: instance.Payload,
	}
	return c.registry.Register(ctx, consulInst)
}

func (c *consulAdapter) Deregister(ctx context.Context, instance *ServiceInstance) error {
	consulInst := &consul.ServiceInstance{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Payload: instance.Payload,
	}
	return c.registry.Deregister(ctx, consulInst)
}

func (c *consulAdapter) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	consulInstances, err := c.registry.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	var instances []*ServiceInstance
	for _, inst := range consulInstances {
		instances = append(instances, &ServiceInstance{
			ID:      inst.ID,
			Name:    inst.Name,
			Address: inst.Address,
			Port:    inst.Port,
			Payload: inst.Payload,
		})
	}
	return instances, nil
}

type etcdAdapter struct {
	registry *etcd.Registry
}

func (e *etcdAdapter) Register(ctx context.Context, instance *ServiceInstance) error {
	etcdInst := &etcd.ServiceInstance{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Payload: instance.Payload,
	}
	return e.registry.Register(ctx, etcdInst)
}

func (e *etcdAdapter) Deregister(ctx context.Context, instance *ServiceInstance) error {
	etcdInst := &etcd.ServiceInstance{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Payload: instance.Payload,
	}
	return e.registry.Deregister(ctx, etcdInst)
}

func (e *etcdAdapter) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	etcdInstances, err := e.registry.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	var instances []*ServiceInstance
	for _, inst := range etcdInstances {
		instances = append(instances, &ServiceInstance{
			ID:      inst.ID,
			Name:    inst.Name,
			Address: inst.Address,
			Port:    inst.Port,
			Payload: inst.Payload,
		})
	}
	return instances, nil
}

type zookeeperAdapter struct {
	registry *zookeeper.Registry
}

func (z *zookeeperAdapter) Register(ctx context.Context, instance *ServiceInstance) error {
	zkInst := &zookeeper.ServiceInstance{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Payload: instance.Payload,
	}
	return z.registry.Register(ctx, zkInst)
}

func (z *zookeeperAdapter) Deregister(ctx context.Context, instance *ServiceInstance) error {
	zkInst := &zookeeper.ServiceInstance{
		ID:      instance.ID,
		Name:    instance.Name,
		Address: instance.Address,
		Port:    instance.Port,
		Payload: instance.Payload,
	}
	return z.registry.Deregister(ctx, zkInst)
}

func (z *zookeeperAdapter) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	zkInstances, err := z.registry.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	var instances []*ServiceInstance
	for _, inst := range zkInstances {
		instances = append(instances, &ServiceInstance{
			ID:      inst.ID,
			Name:    inst.Name,
			Address: inst.Address,
			Port:    inst.Port,
			Payload: inst.Payload,
		})
	}
	return instances, nil
}
