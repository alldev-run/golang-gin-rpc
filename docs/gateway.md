# HTTP Gateway 使用指南

本文档面向 `api/http-gateway` 示例与 `pkg/gateway` 组件的实际使用方式，内容已按当前代码实现校准。

## 快速启动

```bash
go run ./api/http-gateway ./api/http-gateway/config/config.yaml
```

## 核心端点

示例服务启动后，可直接验证：

```bash
curl http://localhost:8080/
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/info
curl http://localhost:8080/metrics
```

说明：`/debug/*` 为示例业务路由层提供的端点，不属于 `pkg/gateway` 固定内置端点。

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

## 常见问题

### `no healthy upstream`

常见原因是路由目标不可达。

排查建议：

1. 检查 `routes[*].targets` 对应服务是否真的在监听
2. 若启用了服务发现，检查注册中心内是否有健康实例
3. 用 `curl http://localhost:8080/ready` 查看当前健康路由数量

### 路由冲突（例如 `/health` 重复注册）

`pkg/gateway` 已注册 `/health`、`/ready`、`/info`、`/metrics`，业务层避免重复注册同路径。

### 配置看起来生效但行为异常

确认使用的是 `api/http-gateway/config/config.yaml`，并检查字段是否与 `pkg/gateway/config.go` 中结构一致。

## 验证建议

```bash
go test ./pkg/gateway -v
go test ./api/http-gateway -v
```

## 相关文档

- `pkg/gateway/README.md`
- `docs/gateway-discovery.md`
- `docs/gateway-auth-complete-guide.md`
