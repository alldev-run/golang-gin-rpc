# Gin 路由系统

## 概述

本项目已成功将 http-gateway 的路由系统从标准库 `net/http` 迁移到 Gin 框架，提供了更强大的路由功能和更好的性能。

## 架构

### 核心组件

1. **`pkg/router/interfaces.go`** - 路由接口定义
   - `IRouterBuilder`: 路由构建器接口
   - `RouterFactory`: 路由工厂接口
   - 支持路由实现的灵活替换

2. **`pkg/router/registry.go`** - 路由注册器和配置
   - `RouteRegistry`: 路由注册器实现
   - `RouteGroup`: 路由组管理
   - `RouteConfig`: 路由配置定义
   - 中间件链式组合

3. **`pkg/router/builder.go`** - 路由构建器实现
   - 实现 `IRouterBuilder` 接口
   - 集成 Gin 引擎和中间件配置
   - 调试路由和业务路由管理

4. **`pkg/middleware/gin/gin_middleware.go`** - Gin 中间件集合
   - 完全基于 Gin 的中间件实现
   - 支持请求ID、CORS、限流、日志等
   - 兼容 Gateway 配置系统

5. **`api/http-gateway/internal/httpapi/router.go`** - 主路由器
   - 使用路由工厂创建路由构建器
   - 完全依赖 pkg 下的路由系统
   - 便于后期统一更换

6. **`api/http-gateway/internal/routes/`** - 业务路由
   - 用户路由模块
   - 可扩展的路由注册系统

## 主要特性

### ✅ 解决的问题
- **循环导入**: 通过 `pkg/route` 包打破循环依赖
- **路由管理**: 提供更强大的路由组和中间件管理
- **性能提升**: Gin 框架的高性能路由匹配
- **中间件统一**: 完全基于 Gin 的中间件系统

### ✅ 保持的兼容性
- **配置系统**: 完全兼容现有的 Gateway 配置
- **API 接口**: 路由注册 API 保持一致
- **功能完整**: 所有原有功能都得到保留
- **接口抽象**: 通过接口实现路由系统的可替换性

## 路由系统更换

### 设计原则
通过接口抽象和工厂模式，实现路由系统的完全可替换性：

```go
// 路由构建器接口
type IRouterBuilder interface {
    RegisterDebugRoutes()
    RegisterBusinessRoutes(registrar interface{})
    Build() http.Handler
    GetEngine() interface{}
    GetRegistry() interface{}
}

// 路由工厂接口
type RouterFactory interface {
    CreateRouterBuilder(cfg *gateway.Config) IRouterBuilder
}
```

### 更换路由实现

1. **实现接口**: 创建新的路由构建器实现 `IRouterBuilder`
2. **创建工厂**: 实现对应的 `RouterFactory`
3. **全局替换**: 调用 `router.SetRouterFactory()` 更换实现

```go
// 示例：更换为自定义路由实现
router.SetRouterFactory(&CustomRouterFactory{})
newRouter := httpapi.NewRouter(cfg) // 现在使用自定义实现
```

### 更换示例
参考 `examples/router-switching-demo.go` 查看完整的更换示例。

## 中间件系统

### 内置中间件

1. **Recovery** - 恢复中间件
   - 自动恢复 panic
   - 基于 Gin 内置实现

2. **RequestID** - 请求ID中间件
   - 自动生成或使用现有请求ID
   - 设置到响应头和上下文

3. **CORS** - 跨域中间件
   - 基于 Gateway 配置
   - 支持预检请求处理

4. **RateLimit** - 限流中间件
   - 添加限流响应头
   - 可扩展集成 Redis 等存储

5. **Logging** - 日志中间件
   - 完全基于 pkg/logger 的结构化日志输出
   - 根据HTTP状态码自动选择日志级别（INFO/WARN/ERROR）
   - 慢请求检测和告警（>1秒）
   - 请求ID跟踪和上下文日志
   - 统一的日志格式和字段

6. **Tracing** - 追踪中间件
   - 集成现有的追踪系统
   - 可配置启用/禁用

### 增强的日志功能

httplog 包现在完全基于 pkg/logger 实现，提供了更强大的日志功能：

#### 1. 智能日志级别
- **2xx**: INFO 级别
- **3xx**: INFO 级别  
- **4xx**: WARN 级别
- **5xx**: ERROR 级别

#### 2. 慢请求检测
- 自动检测超过 1 秒的请求
- 记录 WARN 级别的慢请求日志
- 包含阈值和实际延迟信息

#### 3. 请求ID跟踪
- 自动为每个请求生成或使用现有请求ID
- 在日志中统一包含请求ID字段
- 便于日志聚合和问题追踪

#### 4. 统一日志格式
```json
{"level":"INFO","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Request","method":"GET","path":"/debug/ok","client_ip":"::1","status":200,"latency":0,"request_id":"agordfsr3azu3cttts2pydb46i","user_agent":"Mozilla/5.0..."}
```

#### 5. 丰富的日志函数
- `httplog.Log()` - 基础日志记录
- `httplog.LogWithLevel()` - 根据状态码自动选择级别
- `httplog.LogError()` - 错误日志记录
- `httplog.LogSlowRequest()` - 慢请求日志记录

### 日志格式统一

Gin 中间件现在使用与 http 中间件完全相同的日志格式：

```json
{"level":"INFO","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Request","method":"GET","path":"/debug/ok","client_ip":"::1","status":200,"latency":0,"request_id":"agordfsr3azu3cttts2pydb46i","user_agent":"Mozilla/5.0..."}
```

这确保了：
- **统一的日志格式**: 无论使用 Gin 还是 http 中间件，日志格式完全一致
- **相同的日志字段**: method, path, client_ip, status, latency, request_id, user_agent
- **统一的日志系统**: 都使用 `pkg/httplog` 包进行日志输出
- **一致的日志分析**: 日志聚合和分析工具可以统一处理

### 中间件配置

```go
// 在 NewRouter 中自动配置所有中间件
engine.Use(
    middleware.Recovery(),
    middleware.RequestID(),
    middleware.CORSFromGatewayConfig(cfg),
    middleware.RateLimitFromGatewayConfig(cfg),
    middleware.Logging(),
)

// 条件性添加追踪中间件
if cfg.Tracing != nil && cfg.Tracing.Enabled {
    engine.Use(middleware.TracingFromGatewayConfig(cfg))
}
```

## 使用示例

### 注册新路由

```go
// 在 routes/user_routes.go 中
func RegisterUserRoutes(registry *route.RouteRegistry) {
    // 创建路由组
    userGroup := registry.Group("user", "/api/user")
    
    // 添加中间件
    userGroup.Use(authMiddleware(), loggingMiddleware("user"))
    
    // 注册路由
    userGroup.POST("", handleUserCreate, "创建用户")
    userGroup.GET("/:id", handleUserGet, "获取用户详情")
}
```

### 自定义中间件

```go
func authMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        apiKey := c.GetHeader("X-API-Key")
        if apiKey != "test-api-key" {
            c.JSON(401, gin.H{"error": "Unauthorized"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

## API 端点

### 调试端点
- `GET /` - Hello 端点
- `GET /debug/ok` - 状态检查
- `GET /debug/request-id` - 请求 ID 调试
- `GET /debug/tracing` - 追踪信息

### 业务端点
- `GET /api/users` - 用户列表
- `POST /api/user` - 创建用户（需要 API Key）
- `GET /api/user/:id` - 获取用户（需要 API Key）
- `PUT /api/user/:id` - 更新用户（需要 API Key）
- `DELETE /api/user/:id` - 删除用户（需要 API Key）

## 测试

运行测试服务器：
```bash
go run examples/gin-router-test.go
```

测试 API：
```bash
# 获取用户列表（自动添加请求ID）
curl -i http://localhost:8080/api/users

# 创建用户（需要 API Key）
curl -X POST http://localhost:8080/api/user \
  -H "X-API-Key: test-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"张三","email":"zhangsan@example.com","age":25}'

# 测试 CORS
curl -H "Origin: http://example.com" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: X-API-Key" \
     -X OPTIONS http://localhost:8080/api/user
```

## 迁移说明

### 从旧系统迁移
1. 路由处理器从 `http.HandlerFunc` 改为 `gin.HandlerFunc`
2. 中间件从 `nethttp.Middleware` 改为 `gin.HandlerFunc`
3. 响应写入使用 `c.JSON()` 而不是 `writeJSON()`
4. 中间件配置更加简洁和统一

### 性能优势
- **更快的路由匹配**: Gin 的 Radix Tree 路由算法
- **更好的内存使用**: Gin 的上下文对象池
- **统一中间件**: 避免了包装开销
- **原生支持**: 完全基于 Gin 生态系统

## 扩展

### 添加新的路由模块
1. 在 `api/http-gateway/internal/routes/` 创建新文件
2. 实现 `RegisterXxxRoutes` 函数
3. 在 `routes/registry.go` 中注册

### 自定义中间件
1. 创建 `gin.HandlerFunc` 函数
2. 使用 `c.Next()` 继续处理链
3. 使用 `c.Abort()` 终止请求
4. 可以访问 Gin 上下文的所有功能

### 集成第三方中间件
```go
// 可以直接使用 Gin 生态系统的中间件
engine.Use(gzip.Gzip(gzip.DefaultCompression))
engine.Use(session.Sessions("mysession", sessions.NewCookieStore([]byte("secret"))))
```

## 配置

### Gin 模式
通过环境变量控制：
- 开发模式: `GIN_MODE=debug`
- 生产模式: `GIN_MODE=release`

默认设置为 Release 模式以获得最佳性能。

### 中间件配置
所有中间件都通过 Gateway 配置文件控制：
```yaml
cors:
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
  allowed_headers: ["*"]

rate_limit:
  enabled: true
  requests: 100
  window: "1m"

tracing:
  enabled: true
  type: "jaeger"
  service_name: "http-gateway"
```
