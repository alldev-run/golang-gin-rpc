package nacos

import (
	"context"
	"fmt"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// ServiceInstance represents a service node information
type ServiceInstance struct {
	ID      string            // Unique instance ID
	Name    string            // Service name (e.g., user-service)
	Address string            // IP address
	Port    int               // Port number
	Payload map[string]string // Additional metadata
}

type Registry struct {
	client naming_client.INamingClient
}

func NewRegistry(addr string, timeout time.Duration, options map[string]interface{}) (*Registry, error) {
	if addr == "" {
		return nil, fmt.Errorf("nacos address is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	// Parse Nacos server address
	port := 8848
	if options != nil {
		if p, ok := options["port"].(int); ok && p > 0 {
			port = p
		}
	}

	serverConfig := []constant.ServerConfig{
		*constant.NewServerConfig(addr, uint64(port)),
	}

	clientConfig := *constant.NewClientConfig(
		constant.WithTimeoutMs(uint64(timeout.Milliseconds())),
		constant.WithNotLoadCacheAtStart(true),
	)

	// Apply additional options
	if options != nil {
		if namespace, ok := options["namespace"].(string); ok {
			clientConfig.NamespaceId = namespace
		}
		if username, ok := options["username"].(string); ok {
			clientConfig.Username = username
		}
		if password, ok := options["password"].(string); ok {
			clientConfig.Password = password
		}
		if logDir, ok := options["log_dir"].(string); ok {
			clientConfig.LogDir = logDir
		}
		if cacheDir, ok := options["cache_dir"].(string); ok {
			clientConfig.CacheDir = cacheDir
		}
	}

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfig,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos client: %w", err)
	}

	return &Registry{client: client}, nil
}

func (r *Registry) Register(ctx context.Context, inst *ServiceInstance) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("nacos registry is not initialized")
	}
	if inst == nil {
		return fmt.Errorf("service instance is nil")
	}

	success, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		ServiceName: inst.Name,
		Ip:          inst.Address,
		Port:        uint64(inst.Port),
		GroupName:   "DEFAULT_GROUP",
		Weight:      1,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    inst.Payload,
	})
	if err != nil {
		return fmt.Errorf("failed to register instance: %w", err)
	}
	if !success {
		return fmt.Errorf("failed to register instance: nacos returned false")
	}
	return nil
}

func (r *Registry) Deregister(ctx context.Context, inst *ServiceInstance) error {
	if r == nil || r.client == nil || inst == nil {
		return nil
	}

	success, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		ServiceName: inst.Name,
		Ip:          inst.Address,
		Port:        uint64(inst.Port),
		GroupName:   "DEFAULT_GROUP",
		Ephemeral:   true,
	})
	if err != nil {
		return fmt.Errorf("failed to deregister instance: %w", err)
	}
	if !success {
		return fmt.Errorf("failed to deregister instance: nacos returned false")
	}
	return nil
}

func (r *Registry) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("nacos registry is not initialized")
	}

	instances, err := r.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   "DEFAULT_GROUP",
		HealthyOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get service instances: %w", err)
	}

	result := make([]*ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		result = append(result, &ServiceInstance{
			ID:      inst.InstanceId,
			Name:    serviceName,
			Address: inst.Ip,
			Port:    int(inst.Port),
			Payload: inst.Metadata,
		})
	}
	return result, nil
}

func (r *Registry) GetAllServices(ctx context.Context) ([]string, error) {
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("nacos registry is not initialized")
	}

	services, err := r.client.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		GroupName: "DEFAULT_GROUP",
		NameSpace: "",
		PageNo:    1,
		PageSize:  100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get all services: %w", err)
	}

	names := make([]string, 0, len(services.Doms))
	for _, dom := range services.Doms {
		names = append(names, dom)
	}
	return names, nil
}

func (r *Registry) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	// Nacos SDK doesn't have a Close method, but we can clean up resources
	return nil
}
