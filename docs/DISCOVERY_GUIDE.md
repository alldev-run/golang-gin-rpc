# 服务发现使用指南

## 🚀 服务发现概述

本项目提供了完整的服务发现解决方案，支持多种注册中心（Consul、etcd），具有以下特性：

- **多注册中心支持**: 支持 Consul、etcd 等主流服务注册中心
- **负载均衡**: 内置多种负载均衡策略（轮询、随机、最少连接等）
- **健康检查**: 自动健康检查和服务状态监控
- **服务监听**: 实时监听服务变化
- **自动注册**: 支持服务自动注册和注销
- **连接追踪**: 支持连接数追踪和负载均衡

## 📁 项目结构

```
pkg/discovery/
├── interface.go              # 服务发现接口定义
├── factory.go                # 服务发现工厂
├── manager.go                 # 服务发现管理器
├── loadbalancer.go            # 负载均衡器
├── consul/                    # Consul 实现
│   ├── consul.go
│   ├── balancer.go
│   └── watcher.go
├── etcd/                      # etcd 实现
│   ├── etcd.go
│   ├── balancer.go
│   └── watcher.go
└── examples/                  # 示例服务
    └── discovery_example.go
```

## 🔧 配置

### 服务发现配置 (configs/config.yaml)

```yaml
discovery:
  enabled: true
  registry_type: "consul"  # consul, etcd
  registry_address: "localhost:8500"
  timeout: 30s
  health_check_interval: 30s
  auto_register: true
  service_name: "alldev-gin-rpc"
  service_address: "localhost"
  service_port: 8080
  service_tags:
    - "go"
    - "rpc"
    - "api"
    - "microservice"
```

## 🚀 快速开始

### 1. 启用服务发现

在配置文件中设置 `discovery.enabled: true`，然后启动应用：

```go
package main

import (
    "log"
    "alldev-gin-rpc/internal/bootstrap"
)

func main() {
    // 初始化 bootstrap
    boot, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        log.Fatalf("Failed to initialize bootstrap: %v", err)
    }
    defer boot.Close()

    // 初始化服务发现
    if err := boot.InitializeDiscovery(); err != nil {
        log.Fatalf("Failed to initialize service discovery: %v", err)
    }

    // 获取发现管理器
    discoveryManager := boot.GetDiscoveryManager()
    
    // 应用将自动注册到服务注册中心
    log.Println("Service discovery initialized")
    select {}
}
```

### 2. 手动注册服务

```go
package main

import (
    "context"
    "log"
    "alldev-gin-rpc/pkg/discovery"
)

func main() {
    // 创建发现管理器
    config := discovery.DefaultManagerConfig()
    config.Enabled = true
    config.RegistryType = "consul"
    config.RegistryAddress = "localhost:8500"

    manager, err := discovery.NewServiceDiscoveryManager(config)
    if err != nil {
        log.Fatalf("Failed to create discovery manager: %v", err)
    }
    defer manager.Stop()

    // 启动发现管理器
    if err := manager.Start(); err != nil {
        log.Fatalf("Failed to start discovery manager: %v", err)
    }

    // 注册服务
    instance := &discovery.ServiceInstance{
        ID:      "my-service-1",
        Name:    "my-service",
        Address: "localhost",
        Port:    8081,
        Payload: map[string]string{
            "version": "1.0.0",
            "region":  "us-west-1",
        },
    }

    ctx := context.Background()
    if err := manager.Register(ctx, instance); err != nil {
        log.Fatalf("Failed to register service: %v", err)
    }

    log.Println("Service registered successfully")
    select {}
}
```

### 3. 服务发现和负载均衡

```go
package main

import (
    "context"
    "log"
    "alldev-gin-rpc/pkg/discovery"
)

func main() {
    // 创建发现管理器
    config := discovery.DefaultManagerConfig()
    config.Enabled = true
    config.RegistryType = "consul"

    manager, err := discovery.NewServiceDiscoveryManager(config)
    if err != nil {
        log.Fatalf("Failed to create discovery manager: %v", err)
    }
    defer manager.Stop()

    if err := manager.Start(); err != nil {
        log.Fatalf("Failed to start discovery manager: %v", err)
    }

    // 创建服务选择器
    selector := discovery.NewServiceSelector(manager, discovery.StrategyRoundRobin)

    // 选择服务实例
    ctx := context.Background()
    instance, tracker, err := selector.SelectInstance(ctx, "user-service", "192.168.1.100")
    if err != nil {
        log.Fatalf("Failed to select instance: %v", err)
    }
    defer tracker.Close()

    log.Printf("Selected instance: %s:%d", instance.Address, instance.Port)
    
    // 在这里进行实际的 RPC/HTTP 调用
}
```

## 🔄 负载均衡策略

### 支持的负载均衡策略

1. **轮询 (Round Robin)**
   ```go
   selector.SetServiceStrategy("user-service", discovery.StrategyRoundRobin)
   ```

2. **随机 (Random)**
   ```go
   selector.SetServiceStrategy("user-service", discovery.StrategyRandom)
   ```

3. **最少连接 (Least Connections)**
   ```go
   selector.SetServiceStrategy("user-service", discovery.StrategyLeastConn)
   ```

4. **IP 哈希 (IP Hash)**
   ```go
   selector.SetServiceStrategy("user-service", discovery.StrategyIPHash)
   ```

5. **加权随机 (Weighted Random)**
   ```go
   selector.SetServiceStrategy("user-service", discovery.StrategyWeightedRandom)
   ```

### 负载均衡示例

```go
func demoLoadBalancing(selector *discovery.ServiceSelector) {
    ctx := context.Background()
    
    // 设置负载均衡策略
    selector.SetServiceStrategy("user-service", discovery.StrategyRoundRobin)
    
    // 进行多次选择，观察负载均衡效果
    for i := 0; i < 10; i++ {
        instance, tracker, err := selector.SelectInstance(ctx, "user-service", "192.168.1.100")
        if err != nil {
            log.Printf("Failed to select instance: %v", err)
            continue
        }
        
        log.Printf("Request %d: Selected instance %s:%d", 
            i+1, instance.Address, instance.Port)
        
        // 模拟请求处理
        time.Sleep(100 * time.Millisecond)
        tracker.Close()
    }
}
```

## 👀 服务监听

### 监听服务变化

```go
func watchServiceChanges(selector *discovery.ServiceSelector) {
    // 创建服务监听器
    watcher := discovery.NewServiceWatcher(
        selector.GetRegistry().GetManager(), 
        "user-service")
    defer watcher.Stop()

    ctx := context.Background()
    watchCh := watcher.Watch(ctx)

    for {
        select {
        case instances := <-watchCh:
            log.Printf("Service instances changed: %d instances", len(instances))
            for _, instance := range instances {
                log.Printf("  - %s:%d", instance.Address, instance.Port)
            }
        }
    }
}
```

## 🔍 健康检查

### 健康检查示例

```go
func checkServiceHealth(manager *discovery.ServiceDiscoveryManager) {
    // 创建健康检查器
    healthChecker := discovery.NewHealthChecker(manager)
    
    ctx := context.Background()
    
    // 简单健康检查
    err := healthChecker.CheckHealth(ctx)
    if err != nil {
        log.Printf("Service unhealthy: %v", err)
    } else {
        log.Println("Service is healthy")
    }
    
    // 详细健康信息
    health, err := healthChecker.GetDetailedHealth(ctx)
    if err == nil {
        log.Printf("Health info: %+v", health)
    }
}
```

## 📊 服务注册中心

### Consul 配置

```yaml
discovery:
  enabled: true
  registry_type: "consul"
  registry_address: "localhost:8500"
  # ... 其他配置
```

启动 Consul：
```bash
# 使用 Docker
docker run -d --name consul -p 8500:8500 consul:latest

# 或使用本地安装
consul agent -dev
```

### etcd 配置

```yaml
discovery:
  enabled: true
  registry_type: "etcd"
  registry_address: "localhost:2379"
  # ... 其他配置
```

启动 etcd：
```bash
# 使用 Docker
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  --env ALLOW_NONE_AUTHENTICATION=yes \
  --env ETCD_ADVERTISE_CLIENT_URLS=http://0.0.0.0:2379 \
  quay.io/coreos/etcd:v3.5.0

# 或使用本地安装
etcd --data-dir=/tmp/etcd-data --listen-client-urls=http://localhost:2379
```

## 🛠️ 高级功能

### 服务注册中心管理

```go
func manageRegistry(selector *discovery.ServiceSelector) {
    registry := selector.GetRegistry()
    
    // 列出所有服务
    services := registry.ListServices()
    log.Printf("Registered services: %v", services)
    
    // 获取服务信息
    for _, serviceName := range services {
        info := registry.GetServiceInfo(serviceName)
        log.Printf("Service %s info: %+v", serviceName, info)
    }
    
    // 获取所有服务信息
    allInfo := registry.GetAllServiceInfo()
    log.Printf("All services info: %+v", allInfo)
}
```

### 连接追踪

```go
func trackConnections(selector *discovery.ServiceSelector) {
    ctx := context.Background()
    
    // 选择实例并自动追踪连接
    instance, tracker, err := selector.SelectInstance(ctx, "user-service", "192.168.1.100")
    if err != nil {
        log.Printf("Failed to select instance: %v", err)
        return
    }
    
    // 查看连接数
    registry := selector.GetRegistry()
    lb, _ := registry.GetLoadBalancer("user-service")
    connections := lb.GetConnections(instance.ID)
    log.Printf("Current connections for %s: %d", instance.ID, connections)
    
    // 关闭连接（减少连接计数）
    tracker.Close()
    
    // 再次查看连接数
    connections = lb.GetConnections(instance.ID)
    log.Printf("Connections after close: %d", connections)
}
```

### 自定义服务实现

```go
type MyService struct {
    name     string
    manager  *discovery.ServiceDiscoveryManager
    selector *discovery.ServiceSelector
    instance *discovery.ServiceInstance
}

func NewMyService(name string, port int, manager *discovery.ServiceDiscoveryManager) *MyService {
    return &MyService{
        name:    name,
        manager: manager,
    }
}

func (s *MyService) Start() error {
    // 注册服务
    s.instance = &discovery.ServiceInstance{
        ID:      fmt.Sprintf("%s-%d", s.name, time.Now().Unix()),
        Name:    s.name,
        Address: "localhost",
        Port:    port,
        Payload: map[string]string{
            "version": "1.0.0",
            "type":    "custom",
        },
    }

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := s.manager.Register(ctx, s.instance); err != nil {
        return fmt.Errorf("failed to register: %w", err)
    }

    // 创建服务选择器
    s.selector = discovery.NewServiceSelector(s.manager, discovery.StrategyRoundRobin)

    log.Printf("Service %s started and registered", s.name)
    return nil
}

func (s *MyService) CallOtherService(serviceName string) error {
    ctx := context.Background()
    
    instance, tracker, err := s.selector.SelectInstance(ctx, serviceName, "127.0.0.1")
    if err != nil {
        return fmt.Errorf("failed to select instance: %w", err)
    }
    defer tracker.Close()

    log.Printf("Calling %s at %s:%d", serviceName, instance.Address, instance.Port)
    
    // 在这里进行实际的 RPC/HTTP 调用
    return nil
}

func (s *MyService) Stop() error {
    if s.instance != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := s.manager.Deregister(ctx, s.instance); err != nil {
            return fmt.Errorf("failed to deregister: %w", err)
        }
    }

    log.Printf("Service %s stopped", s.name)
    return nil
}
```

## 🧪 测试

### 运行示例

```bash
# 运行服务发现示例
go run examples/discovery_example.go

# 运行完整应用（包含服务发现）
./start.sh
```

### 测试 Consul

```bash
# 查看 Consul UI
open http://localhost:8500

# 查看注册的服务
curl http://localhost:8500/v1/catalog/services

# 查看特定服务
curl http://localhost:8500/v1/catalog/service/user-service
```

### 测试 etcd

```bash
# 查看注册的服务
etcdctl get --prefix /services/

# 查看特定服务
etcdctl get /services/user-service/instance-1
```

## 📈 监控和日志

服务发现框架集成了完整的监控和日志功能：

- 所有服务注册/注销操作都会记录日志
- 负载均衡选择过程可追踪
- 健康检查状态实时监控
- 连接数统计和监控
- 服务变化事件通知

## 🔧 最佳实践

### 1. 服务命名规范

```go
// 好的命名
"user-service"
"payment-service"
"order-api"

// 避免的命名
"userservice"  // 缺少分隔符
"service"     // 过于通用
"svc"         // 过于简短
```

### 2. 元数据使用

```go
payload := map[string]string{
    "version":     "1.2.3",
    "region":      "us-west-1",
    "zone":        "us-west-1a",
    "environment": "production",
    "team":        "platform",
}
```

### 3. 错误处理

```go
instance, tracker, err := selector.SelectInstance(ctx, "user-service", clientIP)
if err != nil {
    // 降级处理
    return fallbackResponse()
}
defer tracker.Close()

// 设置超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

### 4. 连接管理

```go
// 总是关闭连接追踪器
defer tracker.Close()

// 在 panic 时也要确保关闭
instance, tracker, err := selector.SelectInstance(ctx, service, clientIP)
if err != nil {
    return err
}
defer func() {
    if r := recover(); r != nil {
        tracker.Close()
        panic(r)
    }
}()

// 使用服务
err = callService(instance)
tracker.Close()
return err
```

## 🚨 注意事项

1. **注册中心可用性**: 确保注册中心服务高可用
2. **网络分区**: 处理网络分区导致的服务不可用
3. **服务版本**: 使用版本号管理服务升级
4. **优雅关闭**: 确保服务注销后再关闭
5. **超时设置**: 根据网络环境调整超时时间
6. **负载均衡策略**: 选择适合业务场景的策略

## 📚 更多示例

查看 `examples/` 目录下的完整示例：

- `discovery_example.go` - 完整的服务发现使用示例
- `rpc_example.go` - RPC 与服务发现集成示例

---

**Happy Service Discovery! 🎉**
