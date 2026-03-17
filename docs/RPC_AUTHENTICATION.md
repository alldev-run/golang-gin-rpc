# RPC 认证指南

本文档介绍了如何在 RPC 服务中配置和使用 API 密钥认证。

## 概述

RPC 认证系统提供了以下功能：

- **API 密钥认证**：基于 API 密钥的简单认证机制
- **方法级控制**：可以配置跳过认证的 RPC 方法
- **多种传输方式**：支持 HTTP 头和查询参数传递密钥
- **动态管理**：运行时添加、删除和验证 API 密钥

## 配置

### 1. 基本配置

```yaml
rpc_auth:
  enabled: true
  header_name: "X-API-Key"
  query_name: "api_key"
  
  # 跳过认证的方法
  skip_methods:
    - "system.ping"
    - "health.check"
    - "service.stats"
  
  # API 密钥列表 (key -> description/user)
  api_keys:
    "sk-1234567890abcdef": "admin-user"
    "sk-abcdef1234567890": "service-account"
    "sk-9876543210fedcba": "readonly-user"
```

### 2. 环境变量配置

```yaml
rpc_auth:
  enabled: true
  api_keys:
    "${API_KEY_ADMIN}": "admin-user"
    "${API_KEY_SERVICE}": "service-account"
    "${API_KEY_READONLY}": "readonly-user"
```

## 使用示例

### 服务端配置

```go
package main

import (
    "alldev-gin-rpc/pkg/rpc"
    "alldev-gin-rpc/pkg/rpc/examples"
)

func main() {
    // 创建 JSON-RPC 服务器
    config := rpc.Config{
        Type:    rpc.ServerTypeJSONRPC,
        Host:    "localhost",
        Port:    8080,
        Network: "tcp",
    }
    
    server := rpc.NewJSONRPCServer(config)
    
    // 配置认证
    authConfig := rpc.DefaultAuthConfig()
    authConfig.Enabled = true
    
    // 添加 API 密钥
    authConfig.APIKeys = map[string]string{
        "sk-1234567890abcdef": "admin-user",
        "sk-abcdef1234567890": "service-account",
        "sk-9876543210fedcba": "readonly-user",
    }
    
    server.SetAuthConfig(authConfig)
    
    // 注册服务
    userService := examples.NewUserService()
    server.RegisterService(userService)
    
    // 启动服务器
    server.Start()
}
```

### 客户端调用

#### 使用 HTTP 头传递 API 密钥

```go
config := rpc.DefaultClientConfig()
config.Headers = map[string]string{
    "X-API-Key": "sk-1234567890abcdef",
}

client := rpc.NewTracedClient(config)
var result interface{}
err := client.Call(context.Background(), "user.create", userData, &result)
```

#### 使用查询参数传递 API 密钥

```go
// 在 URL 中包含 API 密钥
// http://localhost:8080/rpc?api_key=sk-1234567890abcdef&method=user.list
```

## API 密钥管理

### 程序化管理

```go
// 添加 API 密钥
server.AddAPIKey("new-key-123", "new-user")

// 检查密钥是否存在
exists := server.GetAuthConfig().HasAPIKey("new-key-123")

// 删除 API 密钥
server.RemoveAPIKey("new-key-123")

// 启用/禁用认证
server.EnableAuth()
server.DisableAuth()
```

### 动态配置

```go
// 更新认证配置
newAuthConfig := rpc.AuthConfig{
    Enabled: true,
    HeaderName: "X-Custom-API-Key",
    QueryName: "custom_api_key",
    APIKeys: map[string]string{
        "custom-key": "custom-user",
    },
}
server.SetAuthConfig(newAuthConfig)
```

## 安全最佳实践

### 1. 密钥生成

```go
import "crypto/rand"
import "encoding/hex"

func generateAPIKey() string {
    bytes := make([]byte, 16)
    rand.Read(bytes)
    return "sk-" + hex.EncodeToString(bytes)
}

// 示例: sk-5d41402abc4b2a76b9719d911017c592
```

### 2. 密钥存储

- **环境变量**：在生产环境中使用环境变量存储密钥
- **密钥管理服务**：集成 AWS KMS、HashiCorp Vault 等
- **定期轮换**：定期更换 API 密钥

```go
// 从环境变量加载密钥
func loadAPIKeysFromEnv() map[string]string {
    return map[string]string{
        os.Getenv("API_KEY_ADMIN"):     "admin-user",
        os.Getenv("API_KEY_SERVICE"):   "service-account",
        os.Getenv("API_KEY_READONLY"):  "readonly-user",
    }
}
```

### 3. 访问控制

```go
// 基于用户的权限控制
authConfig.APIKeys = map[string]string{
    "sk-admin-key":    "admin",     // 完全访问权限
    "sk-service-key":  "service",   // 服务间调用
    "sk-readonly-key": "readonly",  // 只读权限
}

// 在 RPC 方法中检查用户权限
func (s *UserService) DeleteUser(ctx context.Context, req *DeleteUserRequest) (*DeleteUserResponse, error) {
    // 获取 API 用户信息
    apiUser, exists := rpc.GetAPIUserFromContext(ctx)
    if !exists {
        return nil, status.Error(codes.Unauthenticated, "authentication required")
    }
    
    // 检查权限
    if apiUser != "admin" && apiUser != "service" {
        return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
    }
    
    // 执行删除操作
    return s.deleteUser(req.ID)
}
```

## 错误处理

### 认证失败

```go
client := rpc.NewTracedClient(config)
err := client.Call(context.Background(), "user.create", userData, &result)

if err != nil {
    // 检查是否为认证错误
    if strings.Contains(err.Error(), "API key required") {
        // 处理缺少 API 密钥的情况
    } else if strings.Contains(err.Error(), "invalid API key") {
        // 处理无效 API 密钥的情况
    }
}
```

### 服务端日志

```go
// 认证中间件会自动记录以下信息：
// - 认证成功/失败
// - 使用的 API 密钥
// - 调用的方法
// - 客户端 IP 地址
```

## 配置选项详解

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用认证 |
| `header_name` | string | "X-API-Key" | HTTP 头名称 |
| `query_name` | string | "api_key" | 查询参数名称 |
| `skip_methods` | []string | [] | 跳过认证的方法列表 |
| `api_keys` | map[string]string | {} | API 密钥映射 |

## 故障排除

### 1. 认证不生效

检查：
- `rpc_auth.enabled` 是否为 `true`
- API 密钥是否正确添加
- 客户端是否正确传递密钥

### 2. 密钥验证失败

检查：
- 密钥是否在允许列表中
- 密钥格式是否正确
- 传输方式是否匹配配置

### 3. 方法被拒绝

检查：
- 方法是否在 `skip_methods` 列表中
- 认证是否正确配置

## 示例配置文件

### 开发环境

```yaml
rpc_auth:
  enabled: true
  api_keys:
    "dev-key-123": "developer"
    "test-key-456": "tester"
  skip_methods:
    - "system.ping"
    - "debug.*"
```

### 生产环境

```yaml
rpc_auth:
  enabled: true
  header_name: "X-API-Key"
  api_keys:
    "${API_KEY_ADMIN}": "admin"
    "${API_KEY_SERVICE}": "service"
    "${API_KEY_PARTNER}": "partner"
  skip_methods:
    - "health.check"
    - "service.stats"
```

这样就实现了完整的 RPC API 密钥认证系统，确保只有授权的客户端才能调用 RPC 服务。
