# JSON-RPC 分布式追踪

本文档介绍了如何在 JSON-RPC 服务中启用和使用分布式追踪功能。

## 概述

JSON-RPC 追踪功能提供了以下能力：

- **服务端追踪**：自动追踪 JSON-RPC 请求的处理过程
- **客户端追踪**：追踪 JSON-RPC 客户端的调用过程
- **链路传播**：支持跨服务的追踪上下文传播
- **性能监控**：记录请求处理时间和状态

## 配置

### 1. 启用追踪

在配置文件中启用追踪：

```yaml
tracing:
  type: "zipkin"  # 或 jaeger, otlp
  enabled: true
  service_name: "jsonrpc-service"
  host: "localhost"
  port: 9411
  endpoint: "/api/v2/spans"
  sample_rate: 1.0
```

### 2. 服务端配置

JSON-RPC 服务器会自动集成追踪功能，无需额外配置：

```go
// 创建 JSON-RPC 服务器时会自动启用追踪
server := rpc.NewJSONRPCServer(config)
```

### 3. 客户端配置

使用追踪客户端替代普通客户端：

```go
// 普通客户端
client := jsonrpc.NewClient(config)

// 追踪客户端（推荐）
client := jsonrpc.NewTracedClient(config)
```

## 使用示例

### 服务端

```go
package main

import (
    "alldev-gin-rpc/pkg/rpc"
    "alldev-gin-rpc/pkg/bootstrap"
)

func main() {
    // 加载配置（包含追踪配置）
    bs, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        panic(err)
    }

    // 初始化所有组件（包括追踪）
    if err := bs.InitializeAll(); err != nil {
        panic(err)
    }

    // 创建 JSON-RPC 服务器（自动启用追踪）
    config := rpc.Config{
        Type:    rpc.ServerTypeJSONRPC,
        Host:    "localhost",
        Port:    8080,
        Network: "tcp",
    }
    
    server := rpc.NewJSONRPCServer(config)
    
    // 注册服务
    server.RegisterService(myService)
    
    // 启动服务器
    if err := server.Start(); err != nil {
        panic(err)
    }
}
```

### 客户端

```go
package main

import (
    "context"
    "alldev-gin-rpc/pkg/rpc/jsonrpc"
)

func main() {
    // 创建追踪客户端
    config := jsonrpc.DefaultClientConfig()
    client := jsonrpc.NewTracedClient(config)

    // 调用远程方法（自动追踪）
    var result interface{}
    err := client.Call(context.Background(), "calculator.add", map[string]float64{
        "a": 10.5,
        "b": 20.3,
    }, &result)
    
    if err != nil {
        // 追踪会自动记录错误
        panic(err)
    }
    
    fmt.Printf("Result: %v\n", result)
}
```

## 追踪数据

### Span 属性

JSON-RPC 追踪会记录以下属性：

#### 服务端
- `jsonrpc.method`: RPC 方法名
- `jsonrpc.service`: 服务名
- `jsonrpc.path`: 请求路径
- `jsonrpc.remote_addr`: 客户端地址
- `jsonrpc.user_agent`: 客户端 User-Agent
- `jsonrpc.status_code`: HTTP 状态码
- `jsonrpc.duration_ms`: 处理时间（毫秒）

#### 客户端
- `jsonrpc.method`: RPC 方法名
- `jsonrpc.service`: 服务名
- `jsonrpc.url`: 服务端 URL
- `jsonrpc.duration_ms`: 调用时间（毫秒）
- `jsonrpc.status`: 调用状态（success/error）
- `jsonrpc.error`: 错误信息（如果有）

### Span 名称

- 服务端：`jsonrpc.{method}` 或 `jsonrpc.request`
- 客户端：`jsonrpc.{method}`
- 批量调用：`jsonrpc.batch`

## 性能影响

- **最小开销**：当追踪禁用时，几乎没有性能开销
- **采样支持**：通过 `sample_rate` 控制采样率，减少性能影响
- **异步处理**：追踪数据异步发送，不阻塞请求处理

## 最佳实践

### 1. 始终使用追踪客户端

```go
// ✅ 推荐：使用追踪客户端
client := jsonrpc.NewTracedClient(config)

// ❌ 不推荐：使用普通客户端（会丢失追踪信息）
client := jsonrpc.NewClient(config)
```

### 2. 传递正确的上下文

```go
// ✅ 推荐：传递 context.Context
ctx := context.Background()
err := client.Call(ctx, "method", params, &result)

// ❌ 不推荐：使用 nil context
err := client.Call(nil, "method", params, &result)
```

### 3. 设置合理的采样率

```yaml
# 开发环境：高采样率
tracing:
  sample_rate: 1.0  # 100%

# 生产环境：低采样率
tracing:
  sample_rate: 0.1  # 10%
```

### 4. 使用有意义的方法名

```go
// ✅ 推荐：使用 service.method 格式
client.Call(ctx, "user.create", userData, &result)
client.Call(ctx, "order.calculate", orderData, &result)

// ❌ 不推荐：使用模糊的方法名
client.Call(ctx, "execute", data, &result)
```

## 故障排除

### 1. 追踪数据未出现

检查：
- 追踪是否启用：`tracing.enabled: true`
- 采样率是否合适：`tracing.sample_rate`
- 追踪服务器是否可访问

### 2. 性能问题

检查：
- 采样率是否过高
- 追踪服务器响应时间
- 网络连接质量

### 3. 链路断裂

检查：
- 客户端是否使用追踪版本
- 上下文是否正确传递
- 服务传播配置是否正确

## 集成示例

### 与 Gin Web 框架集成

```go
func main() {
    r := gin.New()
    
    // 添加追踪中间件
    r.Use(tracingMiddleware())
    
    // JSON-RPC 端点
    r.POST("/rpc", handleJSONRPC)
    
    r.Run(":8080")
}
```

### 与 gRPC 服务共存

```go
func main() {
    // 创建 gRPC 服务器（带追踪）
    grpcServer := rpc.NewGRPCServer(grpcConfig)
    
    // 创建 JSON-RPC 服务器（带追踪）
    jsonrpcServer := rpc.NewJSONRPCServer(jsonrpcConfig)
    
    // 两个服务器会共享同一个追踪上下文
    go grpcServer.Start()
    jsonrpcServer.Start()
}
```

这样就实现了完整的 JSON-RPC 分布式追踪功能，帮助监控和调试微服务架构中的 JSON-RPC 调用。
