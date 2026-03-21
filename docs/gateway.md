# HTTP Gateway 使用指南

## 🌟 概述

HTTP Gateway 是一个企业级多协议统一网关，支持 HTTP/HTTPS、gRPC、JSON-RPC 协议代理，集成分布式追踪、负载均衡、服务发现等企业级特性。

## 🚀 快速开始

### 启动方式

#### 启动 Gateway 网关
```bash
go run ./api/http-gateway ./api/http-gateway/config/config.yaml
```

### 验证服务

#### 健康检查
```bash
curl http://localhost:8080/health
```

#### 就绪状态
```bash
curl http://localhost:8080/ready
```

#### 主页访问
```bash
curl http://localhost:8080/
```

#### 调试端点
```bash
curl http://localhost:8080/debug/ok
curl http://localhost:8080/debug/tracing
curl http://localhost:8080/debug/request-id
```

## 🔧 配置说明

### 配置特点
- 🌐 **零外部依赖** - 默认配置指向本地服务
- 🚫 **可选追踪** - 追踪功能默认关闭，需要时启用
- 🚫 **可选协议** - gRPC/JSON-RPC 默认关闭，需要时启用
- 🏥 **健康检查** - 使用 Gateway 内置端点
- 🔧 **灵活配置** - 可根据需要启用企业级功能

### 启用高级功能

#### 启用分布式追踪
```yaml
tracing:
  enabled: true
  sample_rate: 1.0  # 调试时 100% 采样
```

#### 启用 gRPC 协议
```yaml
protocols:
  grpc: true
```

#### 启用 JSON-RPC 协议
```yaml
protocols:
  jsonrpc: true
```

#### 添加外部服务路由
```yaml
routes:
  - path: /api/user/*
    targets:
      - "http://user-service-1:8080"
      - "http://user-service-2:8080"
```

## ❌ 故障排查

### "no healthy upstream" 错误

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

### 路由冲突

**错误**：`handlers are already registered for path '/health'`

**解决方案**：Gateway 已提供 `/health` 端点，无需在业务路由中重复注册

### 服务启动失败

**检查步骤**：
```bash
# 1. 检查端口占用
netstat -an | grep :8080

# 2. 检查编译状态
go build ./api/http-gateway

# 3. 检查配置文件
yamllint ./api/http-gateway/config/config-simple.yaml
```

## 📋 验证清单

- [ ] **服务启动** - Gateway 成功启动
- [ ] **端口监听** - 8080 端口可用
- [ ] **健康检查** - `/health` 返回 200
- [ ] **就绪状态** - `/ready` 显示 ready
- [ ] **主页访问** - `/` 返回正常内容
- [ ] **调试端点** - `/debug/ok` 正常
- [ ] **无错误日志** - 控制台无错误信息

## 🌐 可用端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/ready` | GET | 就绪状态 |
| `/info` | GET | 网关信息 |
| `/metrics` | GET | Prometheus 指标 |
| `/` | GET | 主页 |
| `/debug/ok` | GET | 调试端点 |
| `/debug/tracing` | GET | 追踪信息 |
| `/debug/request-id` | GET | 请求ID信息 |

## 🔄 启用高级功能

### 启用分布式追踪

1. **修改配置文件**：
```yaml
tracing:
  enabled: true
  sample_rate: 1.0  # 调试时 100% 采样
```

2. **启动追踪后端**：
   - Jaeger: `localhost:16686`
   - Zipkin: `localhost:9411`

3. **重启服务**：
```bash
go run ./api/http-gateway ./api/http-gateway/config/config.yaml
```

### 启用多协议支持

1. **启用 gRPC**：
```yaml
protocols:
  grpc: true
```

2. **启用 JSON-RPC**：
```yaml
protocols:
  jsonrpc: true
```

3. **添加外部服务路由**：
```yaml
routes:
  - path: /api/user/*
    targets:
      - "http://user-service-1:8080"
      - "http://user-service-2:8080"
  
  - path: /grpc/user/*
    protocol: "grpc"
    targets:
      - "grpc://user-grpc-1:50051"
      - "grpc://user-grpc-2:50051"
  
  - path: /rpc/payment
    protocol: "jsonrpc"
    targets:
      - "http://payment-service:8080/rpc"
```

4. **验证功能**：
```bash
# 测试 gRPC 路由
curl -X POST http://localhost:8080/grpc/user/123

# 测试 JSON-RPC 路由
curl -X POST http://localhost:8080/rpc/payment
```

## 📚 相关文档

- [Gateway 详细文档](../pkg/gateway/README.md)
- [API Gateway 示例](../api/http-gateway/README.md)
- [配置模板](../pkg/gateway/templates/http-gateway/)

## 🛠️ 开发指南

### 添加新路由

1. 在配置文件中添加路由：
```yaml
routes:
  - path: /new-service/*
    method: "*"
    protocol: "http"
    service: new-service
    targets:
      - "http://localhost:8080"
```

2. 在业务代码中处理请求：
```go
func (r *Router) newServiceHandler(w http.ResponseWriter, req *http.Request) {
    // 处理新服务逻辑
}
```

### 添加新中间件

1. 在 `internal/mw/` 目录下创建中间件文件
2. 实现 `Middleware` 类型
3. 在 `registry.go` 中注册中间件

## 📞 获取帮助

如果遇到问题：
1. 查看控制台日志输出
2. 检查配置文件格式
3. 验证服务依赖
4. 提交 GitHub Issue
5. 查看故障排查文档
