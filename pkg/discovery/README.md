# pkg/discovery

`pkg/discovery` 提供统一的服务发现抽象，用于在不同注册中心之间切换实现，同时保持业务侧注册、注销和查询服务实例的调用方式一致。

当前支持：

- `consul`
- `etcd`
- `zookeeper`
- `static`

它适合用于：

- 微服务注册与发现
- RPC / Gateway 实例发现
- 服务实例动态上下线
- 统一封装多种注册中心能力

## 目录结构

- `config.go`
  - discovery 配置结构与默认值
- `factory.go`
  - 根据配置创建具体 discovery 实现
- `interface.go`
  - 统一接口与 `ServiceInstance`
- `manager.go`
  - 更高层的 discovery 管理能力
- `loadbalancer.go`
  - 与服务发现配合的负载均衡能力
- `consul/`
  - Consul 注册与查询实现
- `etcd/`
  - Etcd 注册与查询实现
- `zookeeper/`
  - Zookeeper 注册与查询实现

## 核心抽象

### `ServiceInstance`

表示一个服务实例：

- `ID`
- `Name`
- `Address`
- `Port`
- `Payload`

示例：

```go
instance := &discovery.ServiceInstance{
    ID:      "user-service-1",
    Name:    "user-service",
    Address: "127.0.0.1",
    Port:    8080,
    Payload: map[string]string{
        "version": "1.0.0",
        "zone":    "cn-shanghai-a",
    },
}
```

### `Discovery`

统一接口：

```go
type Discovery interface {
    Register(ctx context.Context, instance *ServiceInstance) error
    Deregister(ctx context.Context, instance *ServiceInstance) error
    GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}
```

## 支持的注册中心

### Consul

特点：

- 适合传统服务注册发现场景
- 使用 agent service register
- 带 TCP 健康检查

配置类型：

- `RegistryTypeConsul`

### Etcd

特点：

- 使用 key/value 存储实例信息
- 使用 lease + keepalive 保持实例存活
- 注册路径格式为 `/services/{name}/{id}`

配置类型：

- `RegistryTypeEtcd`

### Zookeeper

特点：

- 使用 znode 保存实例信息
- 使用 `ephemeral` 节点跟随会话生命周期自动清理
- 默认根路径为 `/services`
- 支持通过 `Options["base_path"]` 自定义根路径

配置类型：

- `RegistryTypeZk`

### Static

特点：

- 不连接外部注册中心
- 适合本地开发或纯静态配置场景

配置类型：

- `RegistryTypeStatic`

## 配置结构

`pkg/discovery/config.go` 中的核心配置：

- `Type`
- `Address`
- `Namespace`
- `Timeout`
- `Username`
- `Password`
- `Token`
- `HealthCheckInterval`
- `DeregisterCriticalServiceAfter`
- `Options`
- `Enabled`

### 默认配置

```go
cfg := discovery.DefaultConfig()
```

### Consul 配置

```go
cfg := discovery.ConsulConfig("127.0.0.1:8500")
```

### Etcd 配置

```go
cfg := discovery.EtcdConfig("127.0.0.1:2379")
```

### Zookeeper 配置

```go
cfg := discovery.ZookeeperConfig("127.0.0.1:2181")
cfg.Options["base_path"] = "/services"
```

## 创建 discovery 实例

统一通过工厂创建：

```go
cfg := discovery.Config{
    Type:    discovery.RegistryTypeZk,
    Address: "127.0.0.1:2181",
    Timeout: 5 * time.Second,
    Options: map[string]interface{}{
        "base_path": "/services",
    },
}

d, err := discovery.NewDiscovery(cfg)
if err != nil {
    panic(err)
}
```

## 基础使用示例

### 注册服务

```go
ctx := context.Background()

instance := &discovery.ServiceInstance{
    ID:      "user-service-1",
    Name:    "user-service",
    Address: "127.0.0.1",
    Port:    8080,
    Payload: map[string]string{
        "version": "1.0.0",
    },
}

if err := d.Register(ctx, instance); err != nil {
    panic(err)
}
```

### 查询服务

```go
instances, err := d.GetService(ctx, "user-service")
if err != nil {
    panic(err)
}

for _, inst := range instances {
    _ = inst
}
```

### 注销服务

```go
if err := d.Deregister(ctx, instance); err != nil {
    panic(err)
}
```

## Zookeeper 使用说明

Zookeeper 实现当前采用：

- 服务目录：`{base_path}/{service_name}`
- 实例节点：`{base_path}/{service_name}/{instance_id}`
- 节点类型：`ephemeral`

这意味着：

- 进程会话断开后，实例节点会自动消失
- 不需要像 etcd 那样主动做 keepalive lease
- 更适合会话型注册发现模型

### Zookeeper 配置示例

```go
cfg := discovery.Config{
    Type:    discovery.RegistryTypeZk,
    Address: "127.0.0.1:2181",
    Timeout: 5 * time.Second,
    Options: map[string]interface{}{
        "base_path": "/services",
    },
}
```

## 与 `configs/discovery.yaml` 的关系

仓库当前已有：

- `configs/discovery.yaml`

但它的字段风格更偏应用层配置，例如：

- `registry_type`
- `registry_address`
- `auto_register`
- `service_name`

而 `pkg/discovery` 当前直接使用的代码结构体是：

- `Type`
- `Address`
- `Timeout`
- `Options`

也就是说：

- `configs/discovery.yaml` 更像上层应用配置示例
- `pkg/discovery.Config` 是底层 discovery 模块配置模型

如果后续需要，我可以继续把两者映射关系补齐。

## 注意事项

- **Consul**
  - 依赖本地或远端 Consul agent

- **Etcd**
  - 注册后依赖 lease 保活
  - 实例信息会写入 `/services/...`

- **Zookeeper**
  - 依赖有效会话
  - 使用 ephemeral 节点，连接断开会自动删除实例

- **Static**
  - 更适合开发和测试，不适合作为动态注册中心

## 测试

当前相关回归可以执行：

```bash
go test ./pkg/discovery/...
```

如果你本地具备对应环境，也可以补充集成测试环境变量后运行：

- `DISCOVERY_CONSUL_ADDR`
- `DISCOVERY_ETCD_ADDR`

当前仓库里还没有单独的 zookeeper 集成测试环境变量说明，如果需要可以继续补。

## 总结

`pkg/discovery` 现在提供了统一的服务发现抽象，并支持 `consul`、`etcd`、`zookeeper` 和 `static` 四种模式，可直接用于服务注册、注销和实例查询。
