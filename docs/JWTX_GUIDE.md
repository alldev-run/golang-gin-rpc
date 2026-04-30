# JWT Token 管理指南（`pkg/auth/jwtx`）

本文档介绍 `pkg/auth/jwtx` 包的 JWT Token 生成、验证、刷新与注销机制，以及 Gin 中间件集成方式。

## 概述

`jwtx` 提供了一套受 AES-GCM 加密保护的 Token 管理能力，区别于标准 JWT 的 Base64 编码 + 签名模式，本实现将 Claims 序列化为 JSON 后使用 AES-GCM 加密，具备以下特性：

- **双 Token 模型**：Access Token（短期） + Refresh Token（长期）
- **加密安全**：AES-256-GCM 加密，非透明编码
- **Token 撤销**：支持单 Token 注销与全用户 Token 失效
- **Refresh Token 轮转**：刷新后旧 Refresh Token 立即失效
- **存储扩展**：可选 Redis 等后端实现持久化与黑名单
- **Gin 中间件**：开箱即用的 HTTP 认证中间件

## 核心结构

### Claims

```go
type Claims struct {
    UserID   string            `json:"user_id"`
    Username string            `json:"username"`
    DeviceID string            `json:"device_id"`
    TokenID  string            `json:"token_id"`
    Version  int               `json:"version"`
    Type     TokenType         `json:"type"`       // "access" 或 "refresh"
    IssuedAt time.Time         `json:"issued_at"`
    ExpireAt time.Time         `json:"expire_at"`
    Payload  map[string]string `json:"payload,omitempty"`
}
```

### 配置

```go
type Config struct {
    Secret          string        // AES 加密密钥（必填）
    AccessTokenTTL  time.Duration // 访问令牌有效期，默认 15 分钟
    RefreshTokenTTL time.Duration // 刷新令牌有效期，默认 7 天
    Store           Store         // 可选存储后端
}

type Store interface {
    Set(key string, value string, ttl time.Duration) error
    Get(key string) (string, error)
    Del(key string) error
}
```

## 初始化

### 默认 Manager（全局）

```go
package main

import "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"

func main() {
    jwtx.Init(jwtx.Config{
        Secret:          "your-secret-key-here",
        AccessTokenTTL:  time.Minute * 15,
        RefreshTokenTTL: time.Hour * 24 * 7,
    })
}
```

### 独立 Manager（多租户/多密钥）

```go
managerA := jwtx.NewManager(jwtx.Config{
    Secret: "tenant-a-secret",
    Store:  redisStore,
})

managerB := jwtx.NewManager(jwtx.Config{
    Secret: "tenant-b-secret",
    Store:  redisStore,
})
```

## Token 生命周期

### 1. 生成 Token 对

```go
pair, err := jwtx.GenerateTokenPair("user123", "alice", "device-456")
if err != nil {
    log.Fatal(err)
}

fmt.Println(pair.AccessToken)   // 短期访问令牌
fmt.Println(pair.RefreshToken)  // 长期刷新令牌
```

### 2. 验证 Access Token

```go
claims, err := jwtx.ValidateAccessToken(pair.AccessToken)
if err != nil {
    // 可能原因：token 过期、token 被撤销、token 类型错误、密钥不匹配
    log.Fatal(err)
}

fmt.Println(claims.UserID)    // "user123"
fmt.Println(claims.Username)  // "alice"
```

### 3. 刷新 Token

```go
newPair, err := jwtx.Refresh(pair.RefreshToken)
if err != nil {
    // 可能原因：refresh token 过期、已被使用、不在 store 中
    log.Fatal(err)
}

// 旧 Refresh Token 已被删除，必须使用新的 TokenPair
```

### 4. 注销

#### 单 Token 注销（加入黑名单）

```go
err := jwtx.Logout(pair.AccessToken)
```

#### 全用户 Token 失效（版本递增）

```go
jwtx.RevokeUser("user123")
// 该用户所有已签发 Token（含各设备）立即失效
```

## Gin 中间件集成

### 基础认证中间件

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
)

func main() {
    jwtx.Init(jwtx.Config{
        Secret: "your-secret-key",
    })

    r := gin.Default()
    r.Use(jwtx.Middleware())
}
```

中间件行为：
- 从 `Authorization` Header 读取 Token
- 验证通过后写入 `user_id`、`username` 到 Gin Context
- 验证失败返回 `401 Unauthorized`

### 在 Handler 中获取用户信息

```go
func profileHandler(c *gin.Context) {
    userID, _ := c.Get("user_id")
    username, _ := c.Get("username")

    c.JSON(200, gin.H{
        "user_id":  userID,
        "username": username,
    })
}
```

### 与 pkg/middleware 的高级认证中间件配合使用

项目 `pkg/middleware` 提供了更灵活的认证中间件，支持自定义 Token 查找位置、跳过路径、RBAC 权限校验：

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/middleware"

r.Use(middleware.JWT(middleware.AuthConfig{
    TokenLookup: "header:Authorization:Bearer ",
    SkipPaths:   []string{"/health", /public/*"},
}))

// 需要权限
r.GET("/admin", middleware.RequirePermission(policy, "admin:read"), adminHandler)
```

## 结合 Redis Store 的完整示例

```go
package main

import (
    "time"

    "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
    "github.com/gin-gonic/gin"
)

// 实现 jwtx.Store 接口
type RedisStore struct{ /* ... */ }

func (s *RedisStore) Set(key, value string, ttl time.Duration) error { /* ... */ }
func (s *RedisStore) Get(key string) (string, error)                 { /* ... */ }
func (s *RedisStore) Del(key string) error                          { /* ... */ }

func main() {
    store := &RedisStore{}

    jwtx.Init(jwtx.Config{
        Secret:          "change-me-in-production",
        AccessTokenTTL:  time.Minute * 15,
        RefreshTokenTTL: time.Hour * 24 * 7,
        Store:           store,
    })

    r := gin.Default()

    // 公开接口
    r.POST("/login", loginHandler)
    r.POST("/refresh", refreshHandler)

    // 需认证接口
    auth := r.Group("/")
    auth.Use(jwtx.Middleware())
    {
        auth.GET("/profile", profileHandler)
        auth.POST("/logout", logoutHandler)
    }

    r.Run(":8080")
}

func loginHandler(c *gin.Context) {
    // 验证用户名密码后 ...
    pair, _ := jwtx.GenerateTokenPair("user123", "alice", c.GetHeader("X-Device-ID"))
    c.JSON(200, pair)
}

func refreshHandler(c *gin.Context) {
    refreshToken := c.PostForm("refresh_token")
    pair, err := jwtx.Refresh(refreshToken)
    if err != nil {
        c.JSON(401, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, pair)
}

func logoutHandler(c *gin.Context) {
    token := c.GetHeader("Authorization")
    _ = jwtx.Logout(token)
    c.JSON(200, gin.H{"message": "logged out"})
}
```

## Bootstrap 集成

项目启动器在 `internal/bootstrap` 中自动初始化 `AuthManager`，配置来自 YAML：

```yaml
security:
  jwt:
    enabled: true
    secret: "your-secret-key"
    expiration: 15m  # Access Token TTL
```

初始化代码（`internal/bootstrap/bootstrap.go`）：

```go
authManager := auth.NewAuthManager(auth.AuthConfig{
    Enabled: b.config.Security.JWT.Enabled,
    JWT: jwtx.Config{
        Secret:         b.config.Security.JWT.Secret,
        AccessTokenTTL: b.config.Security.JWT.Expiration,
        RefreshTokenTTL: b.config.Security.JWT.Expiration * 7,
    },
})
```

## 验证规则汇总

| 校验项 | Access Token | Refresh Token |
|---|---|---|
| 加密解密 | 通过 Secret 生成 AES-256-GCM 密钥 | 同上 |
| Token 类型 | 必须为 `"access"` | 必须为 `"refresh"` |
| 有效期 | `ExpireAt > now` | `ExpireAt > now` |
| Store 黑名单 | `blacklist:<token_id>` 不存在 | — |
| Store Refresh 记录 | — | `refresh:<token_id>` 必须存在，使用后删除 |
| 用户版本 | `user:version:<user_id>` 与 Claims.Version 一致 | — |

## 错误速查

| 错误信息 | 含义 |
|---|---|
| `missing token` | 请求未携带 Authorization Header |
| `token expired` | Access Token 已超过 ExpireAt |
| `token revoked` | Token 已被加入黑名单（Logout） |
| `token invalid` | 用户版本已变更（RevokeUser） |
| `invalid token type` | 用 Refresh Token 当 Access Token 使用 |
| `invalid refresh token` | 用 Access Token 当 Refresh Token 使用 |
| `refresh token expired` | Refresh Token 已超过 ExpireAt |
| `refresh token invalid` | Refresh Token 不在 Store 中（已被使用或不存在） |

## 回归测试

```bash
go test ./pkg/auth/jwtx/... -v
```

测试覆盖：
- Token 生成与验证
- 过期判定
- 黑名单注销
- 用户版本失效
- Refresh Token 轮转
- 并发安全
- 多 Manager 隔离
- 加解密正确性
