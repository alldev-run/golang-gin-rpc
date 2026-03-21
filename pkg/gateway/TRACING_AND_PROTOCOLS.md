# 链路追踪和协议支持

Gateway 现已集成企业级链路追踪功能，并支持 HTTP、gRPC 和 JSON-RPC 多种协议代理。

## 🔍 链路追踪 (Distributed Tracing)

### 支持的追踪后端
- **Jaeger** - 高性能分布式追踪系统
- **Zipkin** - Twitter 开源的追踪系统  
- **OTLP** - OpenTelemetry Protocol
- **Prometheus** - 指标和追踪（未来支持）

### 配置示例

```yaml
gateway:
  service_name: "my-gateway"
  
  tracing:
    type: "jaeger"           # 追踪后端类型
    service_name: "gateway"  # 服务名
    enabled: true            # 启用追踪
    host: "localhost"        # 追踪服务器地址
    port: 6831              # 追踪服务器端口
    sample_rate: 1.0        # 采样率 (0.0-1.0)
    batch_timeout: "5s"      # 批量导出超时
```

### 自动追踪功能

1. **HTTP 请求追踪** - 自动追踪所有 HTTP 请求
2. **代理请求追踪** - 追踪到上游服务的请求
3. **协议特定追踪** - gRPC 和 JSON-RPC 专用追踪
4. **链路传播** - 自动在服务间传播追踪上下文
5. **性能指标** - 延迟、错误率等指标

### 追踪信息

每个请求都会自动添加以下追踪信息：
- `trace-id` - 链路追踪ID
- `span-id` - 当前跨度ID  
- `X-Trace-ID` - 响应头中的追踪ID
- `X-Span-ID` - 响应头中的跨度ID

## 🌐 多协议支持

### HTTP/HTTPS 代理
```yaml
routes:
  - path: "/api/*"
    method: "*"
    protocol: "http"
    targets: 
      - "http://service1:8080"
      - "http://service2:8080"
```

### gRPC 代理
```yaml
routes:
  - path: "/grpc/*"
    method: "POST"
    protocol: "grpc"
    targets:
      - "grpc://service1:50051"
      - "grpc://service2:50051"

protocols:
  grpc: true
  grpc_config:
    enable_tls: false
    timeout: "30s"
    server_name: "grpc.example.com"
```

### JSON-RPC 代理
```yaml
routes:
  - path: "/rpc"
    method: "POST"
    protocol: "jsonrpc"
    targets:
      - "http://rpc-service:8080/rpc"

protocols:
  jsonrpc: true
  jsonrpc_config:
    version: "2.0"
    enable_batch: false
    timeout: "30s"
    headers:
      "Content-Type": "application/json"
```

## 📊 追踪与监控集成

### Prometheus 指标
Gateway 自动导出以下指标：
- `http_requests_total` - HTTP 请求总数
- `http_request_duration_seconds` - 请求延迟
- `grpc_requests_total` - gRPC 请求总数
- `grpc_request_duration_seconds` - gRPC 请求延迟
- `jsonrpc_requests_total` - JSON-RPC 请求总数

### 健康检查端点
- `/health` - 基本健康检查
- `/ready` - 就绪状态检查
- `/info` - 网关信息（包含追踪配置）

## 🚀 使用示例

### 基本配置
```go
config := gateway.DefaultConfig()
config.ServiceName = "api-gateway"
config.Tracing = &tracing.Config{
    Type:        "jaeger",
    ServiceName: "api-gateway",
    Enabled:     true,
    Host:        "localhost",
    Port:        6831,
    SampleRate:  1.0,
}

gw := gateway.NewGateway(config)
```

### 多协议路由
```go
config.Routes = []gateway.RouteConfig{
    {
        Path:     "/api/*",
        Method:   "*",
        Protocol: "http",
        Targets:  []string{"http://backend:8080"},
    },
    {
        Path:     "/grpc/*",
        Method:   "POST", 
        Protocol: "grpc",
        Targets:  []string{"grpc://backend:50051"},
    },
    {
        Path:     "/rpc",
        Method:   "POST",
        Protocol: "jsonrpc",
        Targets:  []string{"http://rpc-backend:8080/rpc"},
    },
}
```

## 🔧 高级配置

### 追踪采样
```yaml
tracing:
  type: "jaeger"
  sample_rate: 0.1  # 10% 采样率，生产环境推荐
```

### TLS 配置
```yaml
protocols:
  grpc_config:
    enable_tls: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
    ca_file: "/path/to/ca.pem"
    server_name: "grpc.example.com"
```

### 批量 JSON-RPC
```yaml
protocols:
  jsonrpc_config:
    version: "2.0"
    enable_batch: true  # 启用批量请求
    timeout: "60s"
```

## 📈 性能优化

1. **连接池** - gRPC 客户端连接复用
2. **负载均衡** - 多目标自动负载均衡
3. **健康检查** - 自动剔除不健康节点
4. **超时控制** - 各协议独立超时配置
5. **重试机制** - 自动重试失败请求

## 🛡️ 安全特性

1. **TLS 支持** - gRPC 和 HTTPS 加密传输
2. **证书验证** - 可配置 CA 证书验证
3. **头部过滤** - 自动过滤敏感头部
4. **访问控制** - 基于协议的访问控制

## 📝 最佳实践

1. **生产环境** - 设置合理的采样率（0.1-0.01）
2. **服务命名** - 使用有意义的服务名
3. **协议分离** - 不同协议使用不同路径
4. **监控告警** - 配置基于追踪指标的告警
5. **日志关联** - 在日志中包含 trace-id

## 🔍 故障排查

### 查看追踪信息
```bash
# 查看请求的 trace-id
curl -I http://localhost:8080/api/test
# 返回头包含: X-Trace-ID: abc123...

# 在 Jaeger UI 中搜索 trace-id
http://localhost:16686/trace/abc123
```

### 调试模式
```yaml
tracing:
  enabled: true
  sample_rate: 1.0  # 调试时 100% 采样
```

### 健康检查
```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready  
curl http://localhost:8080/info
```

通过这些功能，Gateway 现在具备了企业级的可观测性和多协议支持能力。
