# Gateway RPC 专用认证完整指南

## 🎯 **项目概述**

Gateway 支持 API Key 认证功能，**仅针对 RPC 协议的服务**（gRPC 和 JSON-RPC）。普通的 HTTP 路由不会受到认证影响，确保了系统的灵活性和安全性。

## 📋 **功能特性**

### 🔐 RPC 专用认证
- **仅 RPC 认证** - 只对 gRPC 和 JSON-RPC 路由进行认证
- **HTTP 路由免认证** - 普通的路由不需要 API Key
- **智能路由识别** - 自动识别 RPC 协议类型
- **多种提取方式** - 支持 HTTP 头部和查询参数

### 🛡️ 安全特性
- **路径白名单** - 支持路径和方法级别的认证跳过
- **通配符支持** - 支持路径通配符匹配
- **上下文传递** - 认证信息通过 Gin Context 传递
- **动态管理** - 运行时添加/删除 API Key

## 🔍 **路由识别机制**

### ✅ **需要认证的 RPC 路由**

#### **gRPC 路由**
- **明确协议**: `protocol: "grpc"`
- **路径模式**: `/grpc/*`, `/api/grpc/*`, `/v1/*`, `/v2/*`
- **关键词匹配**: 包含 `grpc` 的路径

#### **JSON-RPC 路由**
- **明确协议**: `protocol: "jsonrpc"`
- **路径模式**: `/rpc/*`, `/api/rpc/*`, `/jsonrpc/*`, `/api/jsonrpc/*`
- **方法限制**: 必须使用 `POST` 方法

### ❌ **不需要认证的 HTTP 路由**

#### **普通 HTTP 路由**
- **明确协议**: `protocol: "http"`
- **常规路径**: `/api/*`, `/web/*`, `/static/*`
- **健康检查**: `/health`, `/ready`, `/info`
- **调试端点**: `/debug/*`

## 📊 **配置结构**

### ✅ **完整配置文件**

```yaml
# HTTP Gateway 配置
# 企业级多协议统一网关

# =============================================================================
# 服务器基本配置
# =============================================================================
host: "0.0.0.0"
port: 8080
service_name: "http-gateway"
read_timeout: "30s"
write_timeout: "30s"
idle_timeout: "60s"

# =============================================================================
# CORS 跨域配置
# =============================================================================
cors:
  allowed_origins:
    - "*"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
    - "OPTIONS"
  allowed_headers:
    - "*"
  exposed_headers: []
  allow_credentials: false
  max_age: 86400

# =============================================================================
# 限流保护配置
# =============================================================================
rate_limit:
  enabled: true
  requests: 1000
  window: "1m"

# =============================================================================
# 服务发现配置
# =============================================================================
discovery:
  type: consul
  endpoints: 
    - "localhost:8500"
  namespace: default
  timeout: "5s"
  enabled: false
  options: {}

# =============================================================================
# 负载均衡配置
# =============================================================================
load_balancer:
  strategy: "round_robin"  # round_robin, random, weighted, least_connections

# =============================================================================
# 分布式追踪配置
# =============================================================================
tracing:
  enabled: false
  type: "jaeger"
  service_name: "http-gateway"
  endpoint: "http://localhost:14268/api/traces"
  sample_rate: 0.1

# =============================================================================
# 路由配置
# =============================================================================
routes:
  # 用户服务路由 - 使用服务发现
  - path: /api/user/*
    method: "GET"
    protocol: "http"
    service: user-service
    targets: ["http://localhost:8081"]
    timeout: "30s"
    retries: 3

  # 订单服务路由 - 使用服务发现
  - path: /api/order/*
    method: "POST"
    protocol: "http"
    service: order-service
    targets: ["http://localhost:8082"]
    timeout: "30s"
    retries: 3

  # gRPC 用户服务路由 - 使用服务发现
  - path: /grpc/user/*
    method: "GET"
    protocol: "grpc"
    service: user-grpc-service
    targets: ["http://localhost:50051"]
    timeout: "30s"
    retries: 3

  # gRPC 订单服务路由 - 使用服务发现
  - path: /grpc/order/*
    method: "POST"
    protocol: "grpc"
    service: order-grpc-service
    targets: ["http://localhost:50052"]
    timeout: "30s"
    retries: 3

  # JSON-RPC 支付服务路由 - 使用服务发现
  - path: /rpc/payment
    method: "POST"
    protocol: "jsonrpc"
    service: payment-rpc-service
    targets: ["http://localhost:8087/rpc", "http://localhost:8088/rpc"]
    timeout: "30s"
    retries: 3

  # 调试端点 - 本地路由
  - path: /debug/*
    method: "*"
    protocol: "http"
    service: debug-service
    targets:
      - "http://localhost:8080"
    strip_prefix: false
    timeout: "5s"
    retries: 1

  # Hello 服务 - 本地路由
  - path: /
    method: "*"
    protocol: "http"
    service: hello-service
    targets:
      - "http://localhost:8080"
    strip_prefix: false
    timeout: "5s"
    retries: 1

# =============================================================================
# 协议支持配置
# =============================================================================
protocols:
  http: true
  http2: true
  grpc: true
  jsonrpc: true
  
  # gRPC 配置
  grpc_config:
    enable_tls: false
    cert_file: ""
    key_file: ""
    ca_file: ""
    server_name: ""
    insecure: false
    timeout: "30s"
  
  # JSON-RPC 配置
  jsonrpc_config:
    version: "2.0"
    enable_batch: false
    timeout: "30s"
    headers: {}
  
  # RPC 安全配置
  security:
    # RPC 服务认证配置
    auth:
      enabled: false  # 默认禁用，需要手动启用
      type: "apikey"  # apikey, jwt, oauth2
      header_name: "X-API-Key"
      query_name: "api_key"
      skip_paths:
        - "/health"
        - "/ready"
        - "/info"
        - "/debug/*"
      skip_methods:
        - "OPTIONS"
      api_keys: {}  # 空配置，需要手动添加 API Keys
    
    # TLS 配置
    tls:
      enabled: false
      cert_file: ""
      key_file: ""
      ca_file: ""
      server_name: ""
      insecure: false

# =============================================================================
# 日志配置
# =============================================================================
logging:
  level: "info"
  format: "json"
```

### ✅ **配置说明**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `enabled` | bool | 否 | 是否启用认证 |
| `type` | string | 否 | 认证类型 (apikey, jwt, oauth2) |
| `header_name` | string | 否 | HTTP 头部名称 |
| `query_name` | string | 否 | 查询参数名称 |
| `skip_paths` | []string | 否 | 跳过认证的路径 |
| `skip_methods` | []string | 否 | 跳过认证的 HTTP 方法 |
| `api_keys` | map[string]string | 否 | API 密钥映射 |

## 🚀 **使用方法**

### ✅ **RPC 路由认证**

#### **gRPC 服务认证**
```bash
# 使用 API Key 认证
curl -H "X-API-Key: frontend-api-key" \
     http://localhost:8080/grpc/users

# 使用查询参数认证
curl "http://localhost:8080/grpc/users?api_key=frontend-api-key"
```

#### **JSON-RPC 服务认证**
```bash
# 使用 API Key 认证
curl -X POST -H "X-API-Key: frontend-api-key" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc": "2.0", "method": "payment.process", "params": {}, "id": 1}' \
     http://localhost:8080/rpc/payment
```

### ✅ **HTTP 路由（无需认证）**

#### **普通 API 路由**
```bash
# 这些路由不需要认证
curl http://localhost:8080/api/users
curl http://localhost:8080/web/dashboard
curl http://localhost:8080/static/style.css
```

#### **健康检查**
```bash
# 这些路径不需要认证
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/info
```

## 🛠️ **配置管理工具**

### ✅ **动态配置管理**

为了避免写死配置，提供了完整的配置管理工具：

```bash
# 添加 API Key
go run examples/config-manager.go add <key> <description>

# 删除 API Key
go run examples/config-manager.go remove <key>

# 列出所有 API Keys
go run examples/config-manager.go list

# 启用认证
go run examples/config-manager.go enable

# 禁用认证
go run examples/config-manager.go disable

# 查看状态
go run examples/config-manager.go status
```

### ✅ **使用示例**

```bash
# 添加前端应用 API Key
go run examples/config-manager.go add "frontend-2024" "Frontend Web App"

# 添加移动应用 API Key
go run examples/config-manager.go add "mobile-2024" "Mobile iOS App"

# 添加管理后台 API Key
go run examples/config-manager.go add "admin-2024" "Admin Dashboard"

# 查看所有 API Keys
go run examples/config-manager.go list

# 启用认证
go run examples/config-manager.go enable
```

## 🌍 **环境变量配置**

### ✅ **环境变量支持**

```bash
# 启用认证并设置 API Keys
export GATEWAY_AUTH_ENABLED=true
export GATEWAY_API_KEYS='{"prod-key":"prod-app","admin-key":"admin-panel"}'

# 启动服务
./http-gateway
```

### ✅ **不同环境配置**

#### **开发环境**
```bash
# 开发环境可以禁用认证
export GATEWAY_AUTH_ENABLED=false
./http-gateway
```

#### **生产环境**
```bash
# 生产环境使用生产密钥
export GATEWAY_AUTH_ENABLED=true
export GATEWAY_API_KEYS='{"prod-key-2024":"prod-app","admin-key-2024":"admin-panel"}'
./http-gateway
```

#### **测试环境**
```bash
# 测试环境使用测试密钥
export GATEWAY_AUTH_ENABLED=true
export GATEWAY_API_KEYS='{"test-key":"test-app"}'
./http-gateway
```

## 🔧 **代码结构**

### ✅ **配置结构定义**

```go
// Config holds gateway configuration
type Config struct {
    // 服务器配置 - 扁平结构
    Host         string        `yaml:"host"`
    Port         int           `yaml:"port"`
    ServiceName  string        `yaml:"service_name"`
    ReadTimeout  time.Duration `yaml:"read_timeout"`
    WriteTimeout time.Duration `yaml:"write_timeout"`
    IdleTimeout  time.Duration `yaml:"idle_timeout"`
    
    // 其他配置...
    CORS         CORSConfig         `yaml:"cors"`
    RateLimit    RateLimitConfig     `yaml:"rate_limit"`
    Discovery    DiscoveryConfig     `yaml:"discovery"`
    LoadBalancer LoadBalancerConfig  `yaml:"load_balancer"`
    Tracing      *tracing.Config     `yaml:"tracing"`
    Protocols    ProtocolConfig      `yaml:"protocols"`
    Routes       []RouteConfig       `yaml:"routes"`
    Logging      LoggingConfig       `yaml:"logging"`
}

// ProtocolConfig holds protocol support configuration
type ProtocolConfig struct {
    HTTP    bool         `yaml:"http"`
    HTTP2   bool         `yaml:"http2"`
    GRPC    bool         `yaml:"grpc"`
    JSONRPC bool         `yaml:"jsonrpc"`
    
    // gRPC 配置
    GRPCConfig GRPCConfig `yaml:"grpc_config"`
    
    // JSON-RPC 配置
    JSONRPCConfig JSONRPCConfig `yaml:"jsonrpc_config"`
    
    // RPC 安全配置 (嵌套在这里)
    Security SecurityConfig `yaml:"security"`
}

// SecurityConfig holds RPC security configuration
type SecurityConfig struct {
    // RPC authentication configuration
    Auth AuthConfig `yaml:"auth"`
    
    // TLS configuration for transport layer security
    TLS TLSConfig `yaml:"tls"`
}

// AuthConfig holds RPC authentication configuration
type AuthConfig struct {
    // Enabled indicates if RPC authentication is enabled
    Enabled bool `yaml:"enabled"`
    
    // Type indicates the RPC authentication type (apikey, jwt, oauth2)
    Type string `yaml:"type"`
    
    // HeaderName is the header name for API key (default: X-API-Key)
    HeaderName string `yaml:"header_name"`
    
    // QueryName is the query parameter name for API key (default: api_key)
    QueryName string `yaml:"query_name"`
    
    // SkipPaths are RPC paths that skip authentication
    SkipPaths []string `yaml:"skip_paths"`
    
    // SkipMethods are HTTP methods that skip RPC authentication
    SkipMethods []string `yaml:"skip_methods"`
    
    // APIKeys is a map of valid RPC API keys (key -> description/user)
    APIKeys map[string]string `yaml:"api_keys"`
}
```

### ✅ **配置访问路径**

```go
// 代码中的配置访问路径
g.config.Protocols.Security.Auth.Enabled    // 认证启用状态
g.config.Protocols.Security.Auth.Type       // 认证类型
g.config.Protocols.Security.Auth.APIKeys    // API Keys 映射
g.config.Protocols.Security.Auth.HeaderName // 头部名称
g.config.Protocols.Security.Auth.QueryName  // 查询参数名称
```

## 📊 **验证和测试**

### ✅ **配置验证工具**

```bash
# 验证配置结构
go run examples/config-validation.go

# 验证 API Keys
go run examples/config-manager.go status

# 验证认证功能
curl -H "X-API-Key: test-key" http://localhost:8080/grpc/users
```

### ✅ **功能测试**

```bash
# 运行所有认证测试
go test ./pkg/gateway -v -run TestGatewayAuth

# 测试结果示例
=== RUN   TestGatewayAuth_Execute_Disabled
--- PASS: TestGatewayAuth_Execute_Disabled (0.00s)
=== RUN   TestGatewayAuth_Execute_Enabled_NoKey
--- PASS: TestGatewayAuth_Execute_Enabled_NoKey (0.02s)
=== RUN   TestGatewayAuth_Execute_Enabled_ValidKey
--- PASS: TestGatewayAuth_Execute_Enabled_ValidKey (0.00s)
=== RUN   TestGatewayAuth_Execute_Enabled_ValidKey_QueryParam
--- PASS: TestGatewayAuth_Execute_Enabled_ValidKey_QueryParam (0.00s)
=== RUN   TestGatewayAuth_Execute_Enabled_InvalidKey
--- PASS: TestGatewayAuth_Execute_Enabled_InvalidKey (0.00s)
=== RUN   TestGatewayAuth_SkipPath
--- PASS: TestGatewayAuth_SkipPath (0.00s)
=== RUN   TestGatewayAuth_SkipMethod
--- PASS: TestGatewayAuth_SkipMethod (0.00s)
=== RUN   TestGatewayAuth_APIKeyManagement
--- PASS: TestGatewayAuth_APIKeyManagement (0.00s)
=== RUN   TestGatewayAuth_Execute_NonRPCRoute
--- PASS: TestGatewayAuth_Execute_NonRPCRoute (0.00s)
=== RUN   TestGatewayAuth_Execute_RPCRoute_PatternMatching
--- PASS: TestGatewayAuth_Execute_RPCRoute_PatternMatching (0.00s)
=== RUN   TestGatewayAuth_ShouldSkipAuth
--- PASS: TestGatewayAuth_ShouldSkipAuth (0.00s)
=== RUN   TestGatewayAuth_IsRPCRoute
--- PASS: TestGatewayAuth_IsRPCRoute (0.00s)
PASS
```

## 🛡️ **安全最佳实践**

### ✅ **API Key 管理**

1. **定期轮换**
   ```bash
   # 每月轮换 API Keys
   go run examples/config-manager.go remove old-key
   go run examples/config-manager.go add new-key "App Name"
   ```

2. **最小权限原则**
   ```bash
   # 每个应用使用独立的 API Key
   go run examples/config-manager.go add "frontend-ro" "Frontend Read Only"
   go run examples/config-manager.go add "admin-rw" "Admin Read Write"
   ```

3. **监控和审计**
   ```bash
   # 定期检查 API Keys 使用情况
   go run examples/config-manager.go list
   # 检查日志中的认证记录
   grep "API key authentication" /var/log/gateway.log
   ```

### ✅ **部署最佳实践**

#### **Docker 部署**
```bash
# Docker 运行
docker run -d \
  -e GATEWAY_AUTH_ENABLED=true \
  -e GATEWAY_API_KEYS='{"docker-key":"docker-app"}' \
  -p 8080:8080 \
  gateway:latest
```

#### **Kubernetes 部署**
```yaml
# ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-config
data:
  GATEWAY_AUTH_ENABLED: "true"
  GATEWAY_API_KEYS: '{"k8s-key":"k8s-app"}'

---
# Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
spec:
  template:
    spec:
      containers:
      - name: gateway
        image: gateway:latest
        envFrom:
        - configMapRef:
            name: gateway-config
```

## 🚀 **部署和运维**

### ✅ **健康检查**

```bash
# 检查认证状态
curl http://localhost:8080/health

# 检查配置加载
curl http://localhost:8080/info
```

### ✅ **日志监控**

```bash
# 查看认证日志
grep "RPC API key authentication" /var/log/gateway.log

# 查看认证失败日志
grep "API key extraction failed" /var/log/gateway.log
```

### ✅ **配置备份**

```bash
# 备份配置文件
cp config/config.yaml config/config.backup.$(date +%Y%m%d)

# 恢复配置文件
cp config.config.backup.20240322 config/config.yaml
```

## 🎯 **架构优势**

### ✅ **分离认证策略**
- 🛡️ **RPC 服务** - 使用强 API Key 认证
- 🌐 **HTTP 服务** - 使用其他认证方式（如 Session、JWT）
- 🔧 **管理接口** - 使用更严格的认证

### ✅ **性能优化**
- ⚡ **只认证 RPC** - 只对必要的 RPC 路由进行认证
- 🚀 **HTTP 直通** - HTTP 路由直接处理，无额外开销
- 🧠 **智能识别** - 减少不必要的检查

### ✅ **安全保障**
- 🔒 **严格保护** - RPC 服务受到严格的 API Key 保护
- 🌐 **开放访问** - 公开的 HTTP 服务保持可访问性
- 📉 **降低复杂度** - 认证系统更加简洁高效

## 🎉 **总结**

### ✅ **完美实现**
- 🎯 **精准认证** - 只对 RPC 服务进行认证
- 🌐 **HTTP 免认证** - 普通服务保持开放
- 🧠 **智能识别** - 自动识别协议类型
- 🛡️ **安全保障** - RPC 服务受到严格保护
- ⚡ **性能优化** - 最小化性能开销
- 🔧 **易于管理** - 灵活的配置和管理

### ✅ **企业级特性**
- 🏢 **生产就绪** - 完整的企业级功能
- 📊 **可观测性** - 详细的日志和监控
- 🛡️ **安全可靠** - 多层安全保障
- 🚀 **高性能** - 优化的性能表现
- 🔧 **易于维护** - 清晰的代码结构

### ✅ **配置管理**
- 🔄 **动态配置** - 支持运行时修改
- 🌍 **环境变量** - 支持不同环境配置
- 🛠️ **管理工具** - 完整的配置管理 CLI
- 📊 **验证工具** - 配置验证和状态检查

**Gateway RPC 专用认证功能已完美实现，提供了企业级的认证解决方案！** 🎯🚀
