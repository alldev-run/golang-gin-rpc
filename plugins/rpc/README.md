# pkg/rpc

`pkg/rpc` 提供统一的 RPC 基础设施封装，覆盖：

- gRPC 服务端与客户端
- JSON-RPC 服务端与客户端
- 多服务统一注册与生命周期管理
- 中间件链
- 限流、降级、基础治理
- 观测扩展点（request / retry / governance observer）
- 服务发现集成
- 客户端重试、目标缓存与故障摘除

它适合作为业务服务开发中的统一 RPC 层，支持从单体服务内嵌，到网关/微服务间通信的常见场景。

## 目录结构

- `manager.go`
  - RPC 管理器，统一管理多个 server、service 与 middleware
- `server.go`
  - gRPC / JSON-RPC 服务端封装
- `client.go`
  - 统一 RPC client 封装与治理能力
- `service.go`
  - 基础 service、注册表、中间件链等公共能力
- `degradation.go`
  - 降级管理器与降级中间件
- `grpc/`
  - 独立 gRPC 客户端工具封装
- `jsonrpc/`
  - 独立 JSON-RPC 客户端工具封装
- `examples/`
  - `calculator` / `echo` / `user` 示例 service

## 核心概念

### Manager

`Manager` 用于统一管理：

- 多个 RPC server
- 已注册 service
- middleware chain
- 限流器
- 降级管理器
- 服务发现注册

常用入口：

- `DefaultManagerConfig()`
- `NewManager(config)`
- `RegisterService(service)`
- `AddMiddleware(middleware)`
- `Start()`
- `Stop()`

### Server

`pkg/rpc` 支持两类 server：

- `ServerTypeGRPC`
- `ServerTypeJSONRPC`

常用入口：

- `DefaultConfig()`
- `NewServer(config)`
- `NewGRPCServer(config)`
- `NewJSONRPCServer(config)`

### Client

统一 client 入口位于 `client.go`，支持：

- gRPC
- JSON-RPC
- 服务发现解析
- 重试
- 幂等方法控制
- 目标缓存
- 故障摘除与恢复

常用入口：

- `DefaultClientConfig()`
- `NewGRPCClient(config)`
- `NewJSONRPCClient(config)`

此外还提供独立客户端工具：

- `pkg/rpc/grpc`
- `pkg/rpc/jsonrpc`

## 快速开始

### 1. 创建 Manager

```go
config := rpc.DefaultManagerConfig()
manager := rpc.NewManager(config)
```

### 2. 注册 service

你的 service 需要实现：

```go
type Service interface {
    Name() string
    Register(server interface{}) error
}
```

示例中推荐嵌入 `BaseService`：

```go
svc := examples.NewUserService()
if err := manager.RegisterService(svc); err != nil {
    return err
}
```

### 3. 添加治理能力

```go
dm, err := rpc.NewDegradationManager(rpc.DefaultDegradationConfig())
if err != nil {
    return err
}
manager.SetDegradationManager(dm)

manager.AddMiddleware(rpc.NewMiddleware("logging", func(ctx context.Context, req interface{}) (interface{}, error) {
    return req, nil
}))
```

### 4. 启动与停止

```go
if err := manager.Start(); err != nil {
    return err
}
defer manager.Stop()
```

## 配置说明

### ManagerConfig

关键字段：

- `Servers`
  - 要启动的 server 列表
- `Timeout`
  - 整体超时
- `GracefulShutdownTimeout`
  - 优雅关闭超时

默认会创建：

- gRPC `localhost:50051`
- JSON-RPC `localhost:8080`

### Server Config

关键字段：

- `Type`
  - `grpc` 或 `jsonrpc`
- `Host`
- `Port`
- `Network`
- `Timeout`
- `MaxMsgSize`
- `EnableTLS`
- `CertFile`
- `KeyFile`
- `Reflection`

### ClientConfig

关键字段：

- `Type`
- `Host`
- `Port`
- `Timeout`
- `EnableTLS`
- `ServiceName`
- `RetryCount`
- `RetryBackoff`
- `RetryJitter`
- `IdempotentMethods`
- `MaxRetryElapsedTime`
- `DiscoveryCacheTTL`
- `FailoverThreshold`
- `FailoverCooldown`

## 示例

### 包内示例 service

位于：

- `pkg/rpc/examples/calculator_service.go`
- `pkg/rpc/examples/echo_service.go`
- `pkg/rpc/examples/user_service.go`

### 可执行示例

位于：

- `examples/rpc/main.go`

该示例会：

- 创建 `Manager`
- 注册 `user` / `calculator` / `echo` / `system` service
- 启动 gRPC 与 JSON-RPC server
- 启动示例客户端进行调用演示

## 示例检查结果

本次已检查：

```bash
go test ./pkg/rpc/... ./examples/rpc
```

结果：通过。

说明当前：

- `pkg/rpc` 主包可编译
- `pkg/rpc/examples` 可编译
- `examples/rpc` 可编译

注意：本次检查主要确认了示例的**构建可用性**。如果要做端到端示例运行验证，可以继续执行真实启动检查，这会实际监听端口并启动示例服务。

## 当前适用范围

当前 `pkg/rpc` 已适合：

- 基础业务 RPC 开发
- gRPC / JSON-RPC 双栈服务封装
- 带限流 / 降级 / observer 的日常服务治理
- 与服务发现结合的客户端调用

## 当前保留项

当前仍建议继续关注：

- 真实运行示例时的端口占用与环境依赖
- 业务级 gRPC proto/stub 的进一步集成
- 更完整的 example 端到端运行脚本
- 更细粒度的文档化配置样例

## 回归建议

修改 `pkg/rpc` 后，建议至少执行：

```bash
go test ./pkg/rpc/...
go test ./examples/rpc
```

如果有服务发现、TLS 或治理逻辑改动，建议再补：

```bash
go test ./pkg/...
```
