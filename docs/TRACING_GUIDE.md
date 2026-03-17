# 分布式链路追踪指南

本文档介绍如何在 alldev-gin-rpc 项目中使用分布式链路追踪功能。

## 概述

项目集成了 OpenTelemetry 和 Zipkin，提供完整的分布式链路追踪解决方案。支持：

- HTTP 请求追踪
- gRPC 调用追踪  
- 自定义业务追踪
- 跨服务链路传播
- 与 Zipkin 集成

## 快速开始

### 1. 启动 Zipkin

```bash
# 使用 Docker 启动 Zipkin
docker run -d -p 9411:9411 openzipkin/zipkin

# 或使用 Docker Compose
docker-compose -f deploy/docker-compose.zipkin.yml up -d
```

### 2. 配置追踪

编辑 `configs/tracing.yaml`：

```yaml
tracing:
  service_name: "alldev-gin-rpc"
  service_version: "1.0.0"
  environment: "development"
  enabled: true
  zipkin_url: "http://localhost:9411/api/v2/spans"
  sample_rate: 1.0  # 开发环境 100% 采样
  batch_timeout: 5s
  max_export_batch_size: 512
```

### 3. 启动应用

```bash
# 主应用
go run main.go

# HTTP Gateway
cd api/http-gateway/cmd
go run main.go
```

### 4. 查看追踪数据

访问 Zipkin UI：http://localhost:9411

## 配置说明

### 基本配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `service_name` | 服务名称 | `alldev-gin-rpc` |
| `service_version` | 服务版本 | `1.0.0` |
| `environment` | 环境 | `development` |
| `enabled` | 是否启用追踪 | `false` |
| `zipkin_url` | Zipkin 端点 | `http://localhost:9411/api/v2/spans` |
| `sample_rate` | 采样率 (0.0-1.0) | `1.0` |
| `batch_timeout` | 批处理超时 | `5s` |
| `max_export_batch_size` | 最大批处理大小 | `512` |

### 环境配置

#### 开发环境
```yaml
tracing:
  enabled: true
  sample_rate: 1.0  # 100% 采样
  batch_timeout: 5s
```

#### 生产环境
```yaml
tracing:
  enabled: true
  sample_rate: 0.1  # 10% 采样，减少性能影响
  batch_timeout: 10s
  max_export_batch_size: 1024
```

## 使用指南

### HTTP 追踪

HTTP 请求会自动被追踪，包括：

- 请求路径、方法、状态码
- 请求/响应时间
- 错误信息和堆栈
- 自定义标签

```go
// 在 Gin 处理器中访问追踪信息
func GetUser(c *gin.Context) {
    traceID := c.GetString("trace_id")
    spanID := c.GetString("span_id")
    
    // 添加自定义标签
    span := tracing.SpanFromContext(c.Request.Context())
    tracing.SetSpanAttributes(span, map[string]interface{}{
        "user_id": 123,
        "operation": "get_user",
    })
}
```

### gRPC 追踪

gRPC 调用会自动被追踪：

```go
// 服务端拦截器
server := grpc.NewServer(
    grpc.UnaryInterceptor(tracingInterceptor.UnaryServerInterceptor()),
    grpc.StreamInterceptor(tracingInterceptor.StreamServerInterceptor()),
)

// 客户端拦截器
conn, err := grpc.Dial(
    address,
    grpc.WithUnaryInterceptor(tracingInterceptor.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(tracingInterceptor.StreamClientInterceptor()),
)
```

### 自定义追踪

```go
// 创建自定义 span
func ProcessOrder(ctx context.Context, orderID string) error {
    ctx, span := tracing.GlobalTracer().StartSpan(ctx, "process_order")
    defer span.End()
    
    // 添加属性
    tracing.SetSpanAttributes(span, map[string]interface{}{
        "order_id": orderID,
        "user_id": getUserID(ctx),
    })
    
    // 业务逻辑
    if err := validateOrder(orderID); err != nil {
        tracing.SetSpanError(span, err)
        return err
    }
    
    tracing.SetSpanOK(span)
    return nil
}

// 创建子 span
func validateOrder(ctx context.Context, orderID string) error {
    ctx, span := tracing.GlobalTracer().StartSpan(ctx, "validate_order")
    defer span.End()
    
    // 验证逻辑
    return nil
}
```

### 跨服务追踪

追踪信息会自动在 HTTP 头和 gRPC 元数据中传播：

```go
// HTTP 客户端
client := &http.Client{}
req, _ := http.NewRequest("GET", "http://service-b/api/data", nil)

// 注入追踪信息
tracing.InjectHeaders(ctx, req.Header)
resp, err := client.Do(req)

// gRPC 客户端
client := pb.NewServiceBClient(conn)
resp, err := client.GetData(ctx, request)
```

## 最佳实践

### 1. 采样策略

- **开发环境**: 100% 采样 (`sample_rate: 1.0`)
- **测试环境**: 50% 采样 (`sample_rate: 0.5`)
- **生产环境**: 10% 采样 (`sample_rate: 0.1`)

### 2. Span 命名

使用清晰、一致的 span 名称：

```go
// 好的命名
"database.query.get_user"
"cache.get.user_profile"
"grpc.call.user_service"

// 避免的命名
"query"
"get_user"
"operation"
```

### 3. 属性标签

添加有意义的业务属性：

```go
tracing.SetSpanAttributes(span, map[string]interface{}{
    "user_id": userID,
    "order_id": orderID,
    "payment_method": "credit_card",
    "amount": 99.99,
    "currency": "USD",
})
```

### 4. 错误处理

正确记录错误信息：

```go
if err := process(); err != nil {
    tracing.SetSpanError(span, err)
    return err
}
tracing.SetSpanOK(span)
```

## 故障排除

### 常见问题

1. **追踪数据未出现在 Zipkin**
   - 检查 Zipkin 是否正常运行
   - 验证 `zipkin_url` 配置
   - 确认 `enabled: true`

2. **性能影响**
   - 降低采样率
   - 增加 `batch_timeout`
   - 调整 `max_export_batch_size`

3. **跨服务追踪断开**
   - 确保客户端正确注入追踪头
   - 检查服务端正确提取追踪信息

### 调试模式

启用详细日志：

```yaml
# 在 logger 配置中
logger:
  level: "debug"
```

检查追踪状态：

```go
tracer := tracing.GlobalTracer()
fmt.Printf("Tracing enabled: %v\n", tracer.IsEnabled())
```

## 集成示例

### 与 Gin 集成

```go
import "alldev-gin-rpc/pkg/tracing"

func main() {
    r := gin.New()
    
    // 添加追踪中间件
    r.Use(tracing.GinMiddleware("my-service"))
    
    // 路由
    r.GET("/api/users", handleUsers)
}
```

### 与 gRPC 集成

```go
import "alldev-gin-rpc/pkg/tracing"

func setupGRPCServer() *grpc.Server {
    tracer := tracing.GlobalTracer()
    interceptor := tracing.NewGRPCInterceptor(tracer)
    
    return grpc.NewServer(
        grpc.UnaryInterceptor(interceptor.UnaryServerInterceptor()),
        grpc.StreamInterceptor(interceptor.StreamServerInterceptor()),
    )
}
```

## 监控和告警

### 关键指标

- 追踪成功率
- Span 延迟分布
- 错误率
- 采样率

### Zipkin 告警

可以在 Zipkin 中设置告警规则，监控：

- 高延迟请求
- 高错误率服务
- 异常追踪模式

## 扩展

### 自定义导出器

可以扩展支持其他追踪后端：

```go
// 实现 Jaeger 导出器
func NewJaegerExporter(url string) (sdktrace.SpanExporter, error) {
    // Jaeger 导出器实现
}
```

### 追追踪策略

实现自定义采样策略：

```go
type CustomSampler struct {
    // 自定义采样逻辑
}
```

## 参考资料

- [OpenTelemetry 官方文档](https://opentelemetry.io/docs/)
- [Zipkin 官方文档](https://zipkin.io/pages/)
- [gRPC 追踪指南](https://grpc.io/docs/what-is-grpc/tracing/)
