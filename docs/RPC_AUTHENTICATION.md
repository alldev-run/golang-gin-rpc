# RPC 认证指南（`pkg/rpc`）

本文档描述的是 `pkg/rpc` 当前实现的 API Key 认证能力（以 `pkg/rpc/auth.go` 与 `pkg/rpc/server.go` 为准）。

说明：网关侧认证请参考 `docs/gateway-auth-complete-guide.md`。

## 能力范围

- 认证模型：API Key
- 作用范围：`pkg/rpc` 的 gRPC / JSON-RPC 服务端
- 控制粒度：按 RPC 方法名跳过认证（`SkipMethods`）
- 运行时管理：支持设置配置、增删查 API Key

## 配置结构

`pkg/rpc` 认证配置是代码结构体 `rpc.AuthConfig`，不是固定的 `rpc_auth:` YAML 节点。

```go
type AuthConfig struct {
    APIKeys     map[string]string
    HeaderName  string
    QueryName   string
    SkipMethods []string
    Enabled     bool
}
```

默认值来自 `rpc.DefaultAuthConfig()`：

- `HeaderName`: `X-API-Key`
- `QueryName`: `api_key`
- `SkipMethods`: `system.ping`, `health.check`, `service.stats`
- `Enabled`: `false`

## 启用方式

### gRPC Server

```go
cfg := rpc.DefaultAuthConfig()
cfg.Enabled = true
cfg.APIKeys = map[string]string{
    "svc-key": "service-user",
}

grpcServer, _ := rpc.NewGRPCServer(rpc.DefaultConfig())
grpcServer.SetAuthConfig(cfg)
```

### JSON-RPC Server

```go
cfg := rpc.DefaultAuthConfig()
cfg.Enabled = true
cfg.APIKeys = map[string]string{
    "frontend-key": "frontend",
}

jsonServer, _ := rpc.NewJSONRPCServer(rpc.DefaultConfig())
jsonServer.SetAuthConfig(cfg)
```

## 请求中的 API Key 提取

### gRPC

- 从 metadata 读取 header（默认 `x-api-key`）
- 实际 key 名受 `HeaderName` 影响（会转为小写匹配）

### JSON-RPC（HTTP）

- 优先读取 HTTP Header：`HeaderName`（默认 `X-API-Key`）
- Header 不存在时读取 Query：`QueryName`（默认 `api_key`）

## 认证判定流程

1. `Enabled=false` 直接放行
2. 若方法名命中 `SkipMethods`，放行
3. 从上下文提取 API Key（由 gRPC metadata / JSON-RPC HTTP 层写入）
4. 校验 key 是否存在于 `APIKeys`
5. 成功后在上下文写入 `api_key` 与 `api_user`

## 运行时管理接口

服务端可用：

- `SetAuthConfig(config AuthConfig)`（gRPC / JSON-RPC 都有）
- `GetAuthConfig() *RPCAuth`（gRPC / JSON-RPC 都有）

`JSONRPCServer` 额外提供：

- `AddAPIKey(key, description)`
- `RemoveAPIKey(key)`
- `EnableAuth()`
- `DisableAuth()`

## 上下文工具函数

`pkg/rpc/auth.go` 暴露了以下工具函数：

- `GetAPIKeyFromContext(ctx)`
- `GetAPIUserFromContext(ctx)`
- `SetRPCMethodInContext(ctx, method)`
- `SetAPIKeyInContext(ctx, apiKey)`
- `SetAPIUserInContext(ctx, apiUser)`

## 已知限制

- 当前仅实现 API Key 认证
- `SkipMethods` 是精确匹配，不支持通配符模式
- 文档中若出现 `rpc_auth:` YAML 节点，可视为旧版本写法

## 回归建议

```bash
go test ./pkg/rpc/... -v
```
