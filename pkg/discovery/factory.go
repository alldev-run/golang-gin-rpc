package discovery

import (
	"context"
	"errors"
	"fmt"

	"alldev-gin-rpc/pkg/discovery/consul"
	"alldev-gin-rpc/pkg/discovery/etcd"
)

// NewDiscovery 根据配置返回具体的实现实例
func NewDiscovery(conf Config) (Discovery, error) {
	switch conf.Type {
	case RegistryTypeConsul:
		// 调用 consul 目录下的初始化函数
		registry, err := consul.NewRegistry(conf.Address)
		if err != nil {
			return nil, err
		}
		return &consulAdapter{registry: registry}, nil
	case RegistryTypeEtcd:
		// 调用 etcd 目录下的初始化函数
		registry, err := etcd.NewRegistry(conf.Address, conf.Timeout)
		if err != nil {
			return nil, err
		}
		return &etcdAdapter{registry: registry}, nil
	case "":
		return nil, errors.New("discovery type is required")
	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", conf.Type)
	}
}

// consulAdapter 适配器，将 consul.Registry 适配为 discovery.Discovery
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

// etcdAdapter 适配器，将 etcd.Registry 适配为 discovery.Discovery
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
