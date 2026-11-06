# HTTP Gateway 使用指南

本文档面向 `api/http-gateway` 示例与 `plugins/gateway` 组件，说明启动、配置、服务发现和常见排查方式；内容按当前代码实现校准。

## 快速启动

```bash
go run ./api/http-gateway ./api/http-gateway/config/config.yaml
```

**框架配置与组件开关**：网关启动时按 `./configs/config.yaml` → `./config/config.yaml` →
网关配置路径 的顺序解析框架配置（第二个命令行参数可显式指定）。内置组件（db/cache/auth
等）的启停由框架配置的顶层 `plugins.<name>.enabled` 段驱动（v0.2 起唯一机制，
`framework.init_*` 已移除）；需要框架加载数据库时，除提供 `database.yml` 外，
还需在框架配置中设置 `plugins.db.enabled: true`。

## 核心端点

示例服务启动后，可直接验证：

```bash
curl http://localhost:8080/
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/info
curl http://localhost:8080/metrics
```

说明：`/debug/*` 为示例业务路由层提供的端点，不属于 `plugins/gateway` 固定内置端点。

## 配置要点

- 配置结构为扁平结构（顶层 `host` / `port` / `protocols` 等），不是 `gateway:` 嵌套
- 动态服务发现由 `discovery.enabled` 控制
- 负载均衡策略由 `load_balancer.strategy` 控制
- RPC 认证配置在 `protocols.security.auth`

最小配置示例：

```yaml
host: "0.0.0.0"
port: 8080
service_name: "http-gateway"

protocols:
  http: true
  http2: true
  grpc: true
  jsonrpc: true
  security:
    auth:
      enabled: false
      type: "apikey"
      header_name: "X-API-Key"
      query_name: "api_key"
      api_keys: {}

discovery:
  type: "static"
  enabled: false
  endpoints: []

load_balancer:
  strategy: "round_robin"
```

## 服务发现（静态 & Consul）

- Gateway 服务发现开关：`discovery.enabled`
- 开启后通过 `plugins/discovery` 按 `discovery.type` 和 `discovery.endpoints` 拉取实例
- 关闭或拉取失败时，会回退使用每条路由里的静态 `targets`
- 路由端点会在后台定时刷新

`gateway.DiscoveryConfig` 当前有效字段：

```yaml
discovery:
  type: "static"          # static / consul / etcd / zookeeper（由 plugins/discovery 决定）
  endpoints: []
  namespace: "default"
  timeout: "5s"
  enabled: false
  options: {}
```

说明：`discovery_cache_ttl`、`failover_threshold`、`refresh_interval` 等字段不在当前 `gateway.Config` 结构内。

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

## 常见问题

### `no healthy upstream`

常见原因是路由目标不可达或注册中心没有可用实例：

1. 检查 `routes[*].targets` 对应服务是否在监听
2. 若启用了服务发现，检查注册中心内是否有健康实例
3. 用 `curl http://localhost:8080/ready` 查看当前健康路由数量

### `service discovery failed`

- 检查 `discovery.endpoints` 可达性
- 检查 `discovery.type` 与注册中心是否匹配
- 查看网关日志中的 `Service discovery refresh failed` / `Using static targets`

### 路由冲突（例如 `/health` 重复注册）

`plugins/gateway` 已注册 `/health`、`/ready`、`/info`、`/metrics`，业务层应避免重复注册同路径。

### 配置看起来生效但行为异常

确认使用的是 `api/http-gateway/config/config.yaml`，并检查字段是否与 `plugins/gateway/config.go` 中结构一致。

## 验证建议

```bash
go test ./plugins/gateway -v
go test ./api/http-gateway -v
```

## 相关文档

- `plugins/gateway/README.md`
- [Gateway 认证](authentication.md)
- [服务发现插件](../discovery/README.md)
