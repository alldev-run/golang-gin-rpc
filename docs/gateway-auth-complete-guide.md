# Gateway RPC 认证指南

本文档说明 `pkg/gateway` 当前 API Key 认证能力（以代码实现为准）。

## 认证边界

- 认证开关：`protocols.security.auth.enabled`
- 仅针对 RPC 路由生效（gRPC / JSON-RPC）
- 普通 HTTP 路由默认不走 API Key 认证

## 路由识别规则

认证中间件判断 RPC 路由方式：

1. 优先读取上下文 `protocol`（由 `SetupRoutes` 注入）
2. 无 `protocol` 时按路径模式推断
   - gRPC：`/grpc/`、`/api/grpc/`、`/v1/`、`/v2/` 或路径包含 `grpc`
   - JSON-RPC：`/rpc/`、`/api/rpc/`、`/jsonrpc/`、`/api/jsonrpc/` 且方法必须是 `POST`

## 配置位置

```yaml
protocols:
  security:
    auth:
      enabled: true
      type: "apikey"
      header_name: "X-API-Key"
      query_name: "api_key"
      skip_paths:
        - "/health"
        - "/ready"
        - "/info"
        - "/debug/*"
      skip_methods:
        - "OPTIONS"
      api_keys:
        "frontend-key": "frontend"
        "admin-key": "admin"
```

## 请求示例

### gRPC 路由（通过 Header）

```bash
curl -H "X-API-Key: frontend-key" \
  http://localhost:8080/grpc/users
```

### JSON-RPC 路由（通过 Query）

```bash
curl -X POST "http://localhost:8080/rpc/payment?api_key=frontend-key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"payment.process","params":{},"id":1}'
```

### HTTP 路由（默认不认证）

```bash
curl http://localhost:8080/api/users
```

## 上下文字段

认证成功后会在 `gin.Context` 写入：

- `api_key`
- `api_user`
- `authenticated=true`

可通过工具函数读取：

- `GetAPIKeyFromContext(c)`
- `GetAPIUserFromContext(c)`

## 动态管理（运行时）

`Gateway` 暴露了运行时 API：

- `AddAPIKey(key, description)`
- `RemoveAPIKey(key)`
- `HasAPIKey(key)`

也可使用仓库示例脚本修改配置文件：

```bash
go run examples/config-manager.go add <key> <description>
go run examples/config-manager.go list
go run examples/config-manager.go enable
```

## 已知限制

- 当前仅实现 API Key 校验；`type=jwt/oauth2` 仍属于预留配置
- 未提供环境变量自动覆盖 `auth` 配置的实现
- 路由识别含路径模式推断，建议显式配置 `route.protocol` 以避免歧义

## 测试

```bash
go test ./pkg/gateway -v -run TestGatewayAuth
```
