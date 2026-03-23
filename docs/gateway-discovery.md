# Gateway 服务发现配置指南

本文档描述 `pkg/gateway` 中服务发现相关能力，已按当前实现校准。

## 实现说明

- Gateway 服务发现开关：`discovery.enabled`
- 开启后通过 `pkg/discovery` 按 `discovery.type` 和 `discovery.endpoints` 拉取实例
- 关闭或拉取失败时，会回退使用每条路由里的静态 `targets`
- 路由端点会定时刷新（后台循环）

## 支持字段

`gateway.DiscoveryConfig` 当前有效字段：

```yaml
discovery:
  type: "static"          # static / consul / etcd / zookeeper（由 pkg/discovery 决定）
  endpoints: []
  namespace: "default"
  timeout: "5s"
  enabled: false
  options: {}
```

说明：文档中常见的 `discovery_cache_ttl`、`failover_threshold`、`refresh_interval` 等字段不在当前 `gateway.Config` 结构内。

## 配置示例

### 静态路由（默认推荐起步）

```yaml
discovery:
  type: "static"
  enabled: false
  endpoints: []

routes:
  - path: "/api/user/*"
    method: "*"
    protocol: "http"
    service: "user-service"
    targets:
      - "http://localhost:8081"
      - "http://localhost:8082"
```

### 启用 Consul 动态发现

```yaml
discovery:
  type: "consul"
  enabled: true
  endpoints: ["localhost:8500"]
  namespace: "default"
  timeout: "5s"
  options: {}

routes:
  - path: "/api/user/*"
    method: "*"
    protocol: "http"
    service: "user-service"
    targets: ["http://localhost:8081"] # discovery 失败时回退
```

## 运行时行为

1. 启动时初始化 `ServiceDiscovery`
2. 初始化路由时，若开启发现则优先尝试 `GetServiceEndpoints(service)`
3. 失败或返回空时，回退到 `route.targets`
4. 后台定时刷新服务端点并更新负载均衡目标

## 排查建议

### `no healthy upstream`

- 检查路由是否至少有一个可达 `targets`
- 如果 `discovery.enabled=true`，检查注册中心里该 `service` 是否有可用实例
- 查看 `GET /ready` 返回的 `healthy_routes`

### `service discovery failed`

- 检查 `discovery.endpoints` 可达性
- 检查 `discovery.type` 与注册中心是否匹配
- 查看网关日志中 `Service discovery refresh failed` / `Using static targets` 信息

## 相关文档

- `pkg/gateway/README.md`
- `docs/gateway.md`
- `pkg/discovery/README.md`
