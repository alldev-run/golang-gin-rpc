# Gateway 服务发现配置指南

## 🔍 **服务发现概述**

Gateway 已完整集成 `pkg/discovery` 包，支持多种服务发现机制：

- **Consul** - 分布式服务发现和配置工具
- **etcd** - 分布式键值存储系统  
- **Zookeeper** - 分布式协调服务
- **Static** - 静态配置（开发测试用）

## 🚀 **快速配置**

### 1. Consul 服务发现

```yaml
discovery:
  type: consul
  endpoints: 
    - "localhost:8500"
  namespace: default
  timeout: "5s"
  enabled: true  # 启用服务发现
  options: {}

routes:
  # 使用服务发现中的服务名
  - path: /api/user/*
    method: "*"
    protocol: "http"
    service: user-service  # Consul 中注册的服务名
    strip_prefix: true
    timeout: "30s"
    retries: 3
```

### 2. 禁用服务发现（默认）

```yaml
discovery:
  type: static
  endpoints: []
  namespace: default
  timeout: "5s"
  enabled: false  # 禁用服务发现
  options: {}

routes:
  # 静态配置需要指定 targets
  - path: /api/user/*
    method: "*"
    protocol: "http"
    service: user-service
    targets: ["http://localhost:8081", "http://localhost:8082"]
    strip_prefix: true
    timeout: "30s"
    retries: 3
```

## 🔧 **RPC 服务发现集成**

### gRPC 服务发现

```yaml
protocols:
  grpc: true
  grpc_config:
    enable_tls: false
    timeout: "30s"
    discovery_cache_ttl: "10s"  # 服务发现缓存时间
    failover_threshold: 3       # 故障转移阈值
    failover_cooldown: "30s"    # 故障转移冷却时间

routes:
  - path: /grpc/user/*
    method: "POST"
    protocol: "grpc"
    service: user-grpc-service  # gRPC 服务名
    timeout: "30s"
    retries: 3
```

### JSON-RPC 服务发现

```yaml
protocols:
  jsonrpc: true
  jsonrpc_config:
    version: "2.0"
    timeout: "30s"
    discovery_cache_ttl: "10s"  # 服务发现缓存时间
    failover_threshold: 3       # 故障转移阈值
    failover_cooldown: "30s"    # 故障转移冷却时间

routes:
  - path: /rpc/payment
    method: "POST"
    protocol: "jsonrpc"
    service: payment-rpc-service  # JSON-RPC 服务名
    timeout: "30s"
    retries: 3
```

## 📋 **服务注册**

### RPC 服务注册配置

RPC 服务会自动注册到服务发现中心：

```go
// 在 RPC 服务启动时
manager.SetDiscoveryIntegration(discoveryManager, DiscoveryRegistrationConfig{
    Enabled:        true,
    ServiceName:    "user-grpc-service",
    ServiceAddress: "localhost:50051",
    Port:          50051,
    Metadata: map[string]string{
        "protocol": "grpc",
        "version":  "v1.0.0",
    },
})
```

### 手动服务注册

```go
// 手动注册服务实例
instance := &discovery.ServiceInstance{
    ID:      "user-service-1",
    Name:    "user-service",
    Address: "192.168.1.100",
    Port:    8081,
    Payload: map[string]string{
        "version": "v1.0.0",
        "zone":    "zone-1",
    },
}

err := discovery.Register(ctx, instance)
```

## 🔄 **服务发现流程**

### 1. 服务启动
```
RPC 服务 → 注册到 Consul/etcd → 服务可用
```

### 2. Gateway 路由
```
请求 → Gateway → 查询服务发现 → 获取健康实例 → 负载均衡 → 转发请求
```

### 3. 健康检查
```
服务发现中心 → 定期健康检查 → 标记不健康实例 → Gateway 自动剔除
```

## 🛠️ **故障处理**

### 服务发现失败
- **降级到静态配置** - 使用 fallback_endpoints
- **缓存机制** - 使用上一次的健康实例
- **重试机制** - 定期重试服务发现

### 服务实例故障
- **自动剔除** - 不健康实例自动从路由中移除
- **故障转移** - 自动切换到健康实例
- **恢复检测** - 定期检查故障实例恢复状态

## 📊 **监控和调试**

### 查看服务发现状态
```bash
# 健康检查
curl http://localhost:8080/health

# 就绪状态（包含服务发现信息）
curl http://localhost:8080/ready

# 网关信息
curl http://localhost:8080/info
```

### 日志信息
```json
{
  "level": "INFO",
  "msg": "Service discovery initialized",
  "type": "dynamic",
  "service": "user-service",
  "instances": 3
}
```

## 🎯 **最佳实践**

### 1. 服务命名规范
- 使用有意义的服务名：`user-service`, `order-service`
- 包含环境信息：`user-service-prod`, `user-service-dev`
- 版本化管理：`user-service-v1`, `user-service-v2`

### 2. 健康检查配置
```yaml
discovery:
  type: consul
  endpoints: ["localhost:8500"]
  timeout: "5s"
  options:
    health_check_interval: "30s"
    deregister_critical_service_after: "90s"
```

### 3. 负载均衡策略
```yaml
load_balancer:
  strategy: round_robin  # 轮询
  # strategy: random     # 随机
  # strategy: weighted   # 加权
  # strategy: least_conn  # 最少连接
```

### 4. 超时和重试配置
```yaml
routes:
  - path: /api/user/*
    service: user-service
    timeout: "30s"      # 请求超时
    retries: 3          # 重试次数
    strip_prefix: true  # 路径前缀处理
```

## 🚨 **故障排查**

### 常见问题

#### "service discovery failed"
**原因**: 服务发现中心不可用
**解决**: 
1. 检查 Consul/etcd 服务状态
2. 验证网络连接
3. 使用静态配置降级

#### "no healthy upstream"
**原因**: 没有健康的服务实例
**解决**:
1. 检查服务实例是否启动
2. 验证服务注册状态
3. 检查健康检查配置

#### "service not found"
**原因**: 服务名不匹配
**解决**:
1. 确认服务注册名称
2. 检查命名空间配置
3. 验证服务发现中心中的服务列表

### 调试命令
```bash
# 查看 Consul 服务列表
consul catalog services

# 查看特定服务实例
consul catalog service user-service

# 查看 etcd 服务
etcdctl get --prefix /services/

# 查看 Gateway 日志
tail -f logs/gateway.log | grep discovery
```

## 📚 **相关文档**

- [Consul 文档](https://www.consul.io/docs)
- [etcd 文档](https://etcd.io/docs/)
- [Zookeeper 文档](https://zookeeper.apache.org/doc/current/)
- [Gateway 详细文档](../pkg/gateway/README.md)
- [RPC 服务文档](../pkg/rpc/README.md)
