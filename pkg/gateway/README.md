# HTTP Gateway

企业级多协议统一网关，支持 HTTP/HTTPS、gRPC、JSON-RPC 协议代理，集成分布式追踪、负载均衡、服务发现等企业级特性。

## 🌟 核心特性

### 🌐 多协议支持
- **HTTP/HTTPS** - 传统 Web 协议，支持 HTTP/2
- **gRPC** - 高性能 RPC 协议，支持 TLS 加密
- **JSON-RPC** - 轻量级 RPC 协议，支持批量请求
- **协议路由** - 基于协议的智能路由分发

### 🔍 分布式追踪
- **多后端支持** - Jaeger、Zipkin、OTLP
- **自动追踪** - 全协议请求自动追踪
- **链路传播** - 跨服务追踪上下文传播
- **性能指标** - 延迟、错误率等完整指标

### ⚖️ 负载均衡
- **轮询 (Round Robin)** - 请求轮询分发
- **随机 (Random)** - 随机选择目标
- **加权 (Weighted)** - 基于权重的负载均衡
- **最少连接 (Least Connections)** - 选择连接数最少的服务

### 🔍 服务发现
- **Consul** - 基于 Consul 的服务发现
- **etcd** - 基于 etcd 的服务发现  
- **Zookeeper** - 基于 Zookeeper 的服务发现
- **Static** - 静态配置（开发测试）
- **动态更新** - 服务列表自动刷新
- **健康检查** - 自动探活服务实例
- **故障转移** - 自动剔除不健康实例

### 🛡️ 安全与可靠性
- **CORS 支持** - 完整的跨域资源共享配置
- **限流保护** - IP 限流 + TTL/LRU 自我保护
- **TLS 支持** - gRPC 和 HTTPS 加密传输
- **健康检查** - 自动探活 upstream 服务
- **优雅关闭** - 支持优雅停机和资源清理

### 📊 可观测性
- **结构化日志** - 基于 zap 的企业级日志
- **Prometheus 指标** - HTTP/gRPC/JSON-RPC 完整指标
- **健康检查端点** - `/health`、`/ready`、`/info`
- **分布式追踪** - 完整的请求链路追踪

## 🚀 快速开始

### 1. 基本配置

```yaml
gateway:
  # 服务器配置
  host: "0.0.0.0"
  port: 8080
  service_name: "api-gateway"
  
  # 超时配置
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"

  # CORS 配置
  cors:
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]
    allow_credentials: false
    max_age: 86400

  # 限流配置
  rate_limit:
    enabled: true
    requests: 1000
    window: "1m"

  # 服务发现配置
  discovery:
    type: "consul"  # static, consul, etcd, zookeeper
    endpoints: ["localhost:8500"]  # Consul 地址
    namespace: "default"
    timeout: "5s"
    options: {}

  # 负载均衡配置
  load_balancer:
    strategy: "round_robin"  # round_robin, random, weighted, least_connections

  # 分布式追踪配置
  tracing:
    type: "jaeger"  # jaeger, zipkin, otlp
    service_name: "gateway"
    enabled: true
    host: "localhost"
    port: 6831
    sample_rate: 0.1  # 生产环境建议 0.1

  # 协议支持配置
  protocols:
    http: true
    http2: true
    grpc: true
    jsonrpc: true
    
    grpc_config:
      enable_tls: false
      server_name: "grpc.example.com"
      timeout: "30s"
    
    jsonrpc_config:
      version: "2.0"
      enable_batch: false
      timeout: "30s"
      headers:
        "Content-Type": "application/json"

  # 路由配置
  routes:
    # HTTP 服务路由
    - path: "/api/*"
      method: "*"
      protocol: "http"
      service: "user-service"
      targets: 
        - "http://user-service-1:8080"
        - "http://user-service-2:8080"
      strip_prefix: false
      timeout: "30s"
      retries: 3

    # gRPC 服务路由
    - path: "/grpc/*"
      method: "POST"
      protocol: "grpc"
      service: "order-service"
      targets:
        - "grpc://order-service-1:50051"
        - "grpc://order-service-2:50051"
      timeout: "30s"
      retries: 3

    # JSON-RPC 服务路由
    - path: "/rpc"
      method: "POST"
      protocol: "jsonrpc"
      service: "payment-service"
      targets:
        - "http://payment-service:8080/rpc"
      timeout: "30s"
      retries: 3
```

### 2. 代码集成

```go
package main

import (
    "alldev-gin-rpc/pkg/gateway"
    "alldev-gin-rpc/pkg/logger"
    "github.com/gin-gonic/gin"
)

func main() {
    // 初始化日志
    logger.Init(logger.DefaultConfig())

    // 创建网关配置
    config := gateway.DefaultConfig()
    config.Tracing.Enabled = true
    config.Protocols.GRPC = true
    config.Protocols.JSONRPC = true

    // 创建网关
    gw := gateway.NewGateway(config)
    
    // 初始化网关
    if err := gw.Initialize(); err != nil {
        logger.Fatalf("Failed to initialize gateway: %v", err)
    }
    
    // 启动网关
    if err := gw.Start(); err != nil {
        logger.Fatalf("Failed to start gateway: %v", err)
    }
    
    // 创建 Gin 引擎
    r := gin.New()
    gw.SetupRoutes(r)
    
    // 启动服务器
    r.Run(":8080")
}
```

### 3. HTTPService 封装使用

```go
package main

import (
    "alldev-gin-rpc/pkg/gateway"
    "alldev-gin-rpc/pkg/logger"
    "net/http"
    "strings"
)

func main() {
    // 初始化日志
    logger.Init(logger.DefaultConfig())

    // 网关配置
    gwCfg := gateway.DefaultConfig()
    gwCfg.Tracing.Enabled = true
    gwCfg.Protocols.GRPC = true
    gwCfg.Protocols.JSONRPC = true

    // 业务路由
    bizHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello from business service"))
    })

    // 创建 HTTPService
    svc, err := gateway.NewHTTPServiceWithOptions(gwCfg, gateway.HTTPServiceOptions{
        BizHandler:     bizHandler,
        IsBusinessPath: func(path string) bool {
            return strings.HasPrefix(path, "/api")
        },
        Middlewares: []gateway.Middleware{
            // 自定义中间件
        },
    })
    if err != nil {
        logger.Fatalf("Failed to create HTTP service: %v", err)
    }
    defer svc.Close()

    // 启动服务器
    srv := &http.Server{
        Addr:    ":8080",
        Handler: svc.Handler(),
    }
    srv.ListenAndServe()
}
```

## 📋 配置详解

### 服务器配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `host` | string | "0.0.0.0" | 监听地址 |
| `port` | int | 8080 | 监听端口 |
| `service_name` | string | "gateway" | 服务名称 |
| `read_timeout` | duration | "30s" | 读取超时 |
| `write_timeout` | duration | "30s" | 写入超时 |
| `idle_timeout` | duration | "60s" | 空闲超时 |

### 负载均衡策略

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `round_robin` | 轮询分发 | 通用场景 |
| `random` | 随机选择 | 高并发场景 |
| `weighted` | 加权分发 | 服务性能不均 |
| `least_connections` | 最少连接 | 长连接场景 |

### 追踪配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `type` | string | "jaeger" | 追踪后端类型 |
| `enabled` | bool | false | 是否启用追踪 |
| `sample_rate` | float | 1.0 | 采样率 (0.0-1.0) |
| `host` | string | "localhost" | 追踪服务器地址 |
| `port` | int | 6831 | 追踪服务器端口 |

### 协议配置

#### gRPC 配置
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable_tls` | bool | false | 启用 TLS |
| `server_name` | string | "" | TLS 服务器名 |
| `timeout` | duration | "30s" | 连接超时 |

#### JSON-RPC 配置
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `version` | string | "2.0" | JSON-RPC 版本 |
| `enable_batch` | bool | false | 启用批量请求 |
| `timeout` | duration | "30s" | 请求超时 |

## 🔍 监控端点

### 健康检查
- `GET /health` - 基本健康状态
- `GET /ready` - 就绪状态检查
- `GET /info` - 网关信息

### 指标监控
- `GET /metrics` - Prometheus 指标

### 追踪信息
每个请求响应头包含：
- `X-Trace-ID` - 链路追踪ID
- `X-Span-ID` - 当前跨度ID
- `X-Request-ID` - 请求ID

## 📊 Prometheus 指标

### HTTP 指标
- `http_requests_total` - HTTP 请求总数
- `http_request_duration_seconds` - HTTP 请求延迟
- `http_response_status_codes` - HTTP 状态码分布

### gRPC 指标
- `grpc_requests_total` - gRPC 请求总数
- `grpc_request_duration_seconds` - gRPC 请求延迟
- `grpc_response_status_codes` - gRPC 状态码分布

### JSON-RPC 指标
- `jsonrpc_requests_total` - JSON-RPC 请求总数
- `jsonrpc_request_duration_seconds` - JSON-RPC 请求延迟
- `jsonrpc_response_status_codes` - JSON-RPC 状态码分布

## 🛠️ 开发和部署

### 本地开发
```bash
# 克隆项目
git clone <repository-url>
cd golang-gin-rpc

# 运行测试
go test ./pkg/gateway

# 构建项目
go build ./cmd/gateway

# 运行服务
./gateway --config config.yaml
```

### Docker 部署
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o gateway ./cmd/gateway

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/gateway .
COPY config.yaml .
CMD ["./gateway"]
```

### Kubernetes 部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
    spec:
      containers:
      - name: gateway
        image: gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_PATH
          value: "/etc/gateway/config.yaml"
        volumeMounts:
        - name: config
          mountPath: /etc/gateway
      volumes:
      - name: config
        configMap:
          name: gateway-config
```

## 🔧 故障排查

### 常见问题

1. **服务启动失败**
   ```bash
   # 检查配置文件语法
   go run ./cmd/gateway --config config.yaml --validate
   ```

2. **追踪数据未显示**
   ```bash
   # 检查追踪配置
   curl http://localhost:8080/info | jq .tracing
   ```

3. **负载均衡不生效**
   ```bash
   # 检查服务发现状态
   curl http://localhost:8080/health | jq .discovery
   ```

### 调试模式
```yaml
gateway:
  tracing:
    enabled: true
    sample_rate: 1.0  # 调试时 100% 采样
  
  # 启用详细日志
  logger:
    level: "debug"
```

## 📚 最佳实践

### 生产环境配置
1. **采样率** - 设置为 0.1-0.01
2. **超时配置** - 根据服务性能调整
3. **负载均衡** - 根据服务特性选择策略
4. **监控告警** - 配置基于指标的告警

### 性能优化
1. **连接池** - 合理配置连接池大小
2. **缓存策略** - 启用适当的缓存
3. **压缩** - 启用响应压缩
4. **限流** - 配置合理的限流策略

### 安全建议
1. **HTTPS** - 生产环境必须启用
2. **认证** - 集成认证授权系统
3. **网络隔离** - 使用网络策略
4. **审计日志** - 启用完整的审计日志

## 📄 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request。请确保：
1. 代码通过所有测试
2. 遵循项目代码规范
3. 添加适当的文档
4. 更新相关测试

## 📞 支持

如有问题或建议，请通过以下方式联系：
- 提交 GitHub Issue
- 发送邮件至 support@example.com
- 加入技术交流群

---

## 🔧 故障排查

### ❌ **常见问题**

#### **"no healthy upstream" 错误**
**原因**：配置文件中的路由指向了不存在的服务实例

**解决方案**：
1. 确保目标服务正常运行
2. 检查服务发现配置
3. 使用本地服务进行测试

**本地服务配置示例**：
```yaml
discovery:
  type: static
  endpoints: 
    - "http://localhost:8080"

routes:
  - path: /api/*
    targets: ["http://localhost:8080"]
```

#### **服务启动失败**
**检查步骤**：
```bash
# 1. 检查配置文件语法
go run ./cmd/gateway --config config.yaml --validate

# 2. 检查端口占用
netstat -an | grep :8080

# 3. 检查编译状态
go build ./pkg/gateway
```

#### **追踪数据未显示**
**检查步骤**：
```bash
# 检查追踪配置
curl http://localhost:8080/info | jq .tracing

# 确保追踪后端服务运行
# Jaeger: localhost:16686
# Zipkin: localhost:9411
```

### 🔍 **验证端点**

#### **健康检查**
```bash
# 基本健康状态
curl http://localhost:8080/health

# 预期响应
{
  "status": "healthy",
  "timestamp": 1774106799,
  "service": "gateway",
  "version": "1.0.0"
}
```

#### **就绪状态**
```bash
# 就绪状态检查
curl http://localhost:8080/ready

# 预期响应
{
  "status": "ready",
  "routes": 3,
  "services": {
    "local-service": {"total": 1, "healthy": 1}
  }
}
```

#### **调试端点**
```bash
# 调试信息
curl http://localhost:8080/debug/ok
curl http://localhost:8080/debug/tracing
curl http://localhost:8080/debug/request-id
```

### 🚀 **快速验证**

#### **启动验证**
```bash
# 1. 启动服务
go run ./api/http-gateway ./api/http-gateway/config/config.yaml

# 2. 验证健康状态
curl http://localhost:8080/health

# 3. 验证就绪状态
curl http://localhost:8080/ready

# 4. 验证主页
curl http://localhost:8080/
```

#### **功能验证**
```bash
# HTTP 请求
curl http://localhost:8080/api/user/123

# 调试端点
curl http://localhost:8080/debug/ok

# 追踪信息
curl http://localhost:8080/debug/tracing
```

### 📋 **验证清单**

- [ ] **服务启动** - Gateway 成功启动
- [ ] **端口监听** - 8080 端口可用
- [ ] **健康检查** - `/health` 返回 200
- [ ] **就绪状态** - `/ready` 显示 ready
- [ ] **主页访问** - `/` 返回正常内容
- [ ] **调试端点** - `/debug/ok` 正常
- [ ] **无错误日志** - 控制台无错误信息

### 🛠️ **调试模式**

```yaml
gateway:
  tracing:
    enabled: true
    sample_rate: 1.0  # 调试时 100% 采样
  
  logging:
    level: "debug"
    format: "console"
```

### 📞 **获取帮助**

如果遇到问题：
1. 查看控制台日志输出
2. 检查配置文件格式
3. 验证服务依赖
4. 提交 GitHub Issue
