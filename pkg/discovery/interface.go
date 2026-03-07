package discovery

import "context"

// ServiceInstance 代表一个服务节点的信息
type ServiceInstance struct {
	ID      string            // 实例唯一ID
	Name    string            // 服务名 (如: user-service)
	Address string            // IP
	Port    int               // 端口
	Payload map[string]string // 额外元数据
}

// Discovery 统一接口
type Discovery interface {
	// Register 注册服务
	Register(ctx context.Context, instance *ServiceInstance) error
	// Deregister 注销服务
	Deregister(ctx context.Context, instance *ServiceInstance) error
	// GetService 根据服务名获取实例列表
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}
