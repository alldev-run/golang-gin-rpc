# HTTP Gateway

`pkg/gateway` 提供一个可嵌入的多协议网关组件，支持 HTTP / gRPC / JSON-RPC 路由、服务发现、负载均衡、基础安全中间件、追踪和健康检查。

## 当前实现能力

- 协议路由：按 `route.protocol` 分发到 HTTP、gRPC、JSON-RPC 处理器
- 基础中间件：CORS、限流、请求 ID、访问日志
- RPC 认证：`protocols.security.auth` 对 RPC 路由生效（HTTP 路由默认不校验）
- 追踪：通过 `pkg/tracing` 注入/透传追踪上下文
- 负载均衡：`round_robin` / `random` / `weighted` / `least_connections`
- 服务发现：启用时从 `pkg/discovery` 拉取实例，失败回退到路由静态 `targets`
- 可观测性端点：`/health`、`/ready`、`/info`、`/metrics`

## 配置结构（与代码一致）

`gateway.Config` 是扁平结构，配置文件顶层直接写字段，不使用 `gateway:` 包裹。

```yaml
host: "0.0.0.0"
port: 8080
service_name: "gateway"
read_timeout: "30s"
write_timeout: "30s"
idle_timeout: "60s"

cors:
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["*"]
  exposed_headers: []
  allow_credentials: false
  max_age: 86400

rate_limit:
  enabled: false
  requests: 100
  window: "1m"

discovery:
  type: "static"
  endpoints: []
  namespace: "default"
  timeout: "5s"
  enabled: false
  options: {}

load_balancer:
  strategy: "round_robin"

tracing:
  enabled: false
  type: "jaeger"
  service_name: "gateway"
  host: "localhost"
  port: 6831
  endpoint: "/api/traces"
  sample_rate: 1.0

protocols:
  http: true
  http2: true
  grpc: false
  jsonrpc: false

  grpc_config:
    enable_tls: false
    timeout: "30s"

  jsonrpc_config:
    version: "2.0"
    enable_batch: false
    timeout: "30s"
    headers: {}

  security:
    auth:
      enabled: false
      type: "apikey"
      header_name: "X-API-Key"
      query_name: "api_key"
      skip_paths: ["/health", "/ready", "/info", "/debug/*"]
      skip_methods: ["OPTIONS"]
      api_keys: {}

routes:
  - path: "/api/*"
    method: "*"
    protocol: "http"
    service: "user-service"
    targets: ["http://localhost:8081"]
    strip_prefix: false
    timeout: "30s"
    retries: 3

logging:
  level: "info"
  format: "json"
```

## 代码接入

```go
cfg := gateway.DefaultConfig()
cfg.Protocols.GRPC = true
cfg.Protocols.JSONRPC = true

gw := gateway.NewGateway(cfg)
if err := gw.Initialize(); err != nil {
    panic(err)
}
if err := gw.Start(); err != nil {
    panic(err)
}
defer gw.Stop()

r := gin.New()
gw.SetupRoutes(r)
_ = r.Run(":8080")
```

## `HTTPService` 封装

`NewHTTPServiceWithOptions` 可把网关处理器与业务处理器组合，并额外挂载 `net/http` 中间件。

```go
svc, err := gateway.NewHTTPServiceWithOptions(cfg, gateway.HTTPServiceOptions{
    BizHandler: myBizHandler,
    IsBusinessPath: func(path string) bool {
        return strings.HasPrefix(path, "/api")
    },
    Middlewares: []gateway.Middleware{myMW},
})
if err != nil {
    panic(err)
}
defer svc.Close()
```

## 端点说明

- `GET /health`：网关存活状态
- `GET /ready`：路由与 upstream 就绪情况
- `GET /info`：网关运行信息
- `GET /metrics`：Prometheus 指标

## 注意事项

- `routes[*].path` 会自动从 `/*` 归一化为 Gin 可识别的 `/*path`
- 启用服务发现后，若发现失败会回退使用路由静态 `targets`
- RPC 认证只针对识别为 gRPC / JSON-RPC 的路由
- gRPC 代理当前实现主要用于网关链路和路由联调，业务层需结合实际 gRPC 协议定义继续完善

## 本地验证

```bash
go test ./pkg/gateway -v
go test ./pkg/loadbalancer -v
```
