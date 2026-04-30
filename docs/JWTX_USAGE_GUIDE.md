# JWTX 认证使用指南

本文档介绍 `github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx` 包的使用方法，该包提供了 JWT token 管理功能，包括访问令牌/刷新令牌对生成、令牌验证、令牌刷新、令牌撤销以及 Gin 中间件集成。

## 概述

JWTX 提供了以下核心功能：

- **双令牌机制**：访问令牌（短期）和刷新令牌（长期）
- **令牌加密**：使用 AES-GCM 加密保护令牌内容
- **令牌撤销**：支持单个令牌撤销和用户级撤销
- **版本控制**：通过用户版本机制实现批量令牌失效
- **存储集成**：支持自定义存储后端（如 Redis）
- **中间件集成**：提供 Gin 中间件用于 HTTP 请求认证

## 配置

### 1. 配置文件配置

在 `configs/config.yaml` 中配置 JWT 认证：

```yaml
security:
  jwt:
    enabled: true
    secret: "your-secret-key-here"
    expiration: 15m  # 访问令牌有效期
```

### 2. 使用 Bootstrap 初始化

```go
package main

import (
    "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
)

func main() {
    // 加载配置（包含 JWT 配置）
    bs, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        panic(err)
    }
    defer bs.Close()

    // 初始化所有组件（包括认证）
    if err := bs.InitializeAll(); err != nil {
        panic(err)
    }

    // 获取认证管理器
    authManager := bs.GetAuthManager()
    if authManager != nil && authManager.IsEnabled() {
        jwtManager := authManager.JWT()
        // 使用 jwtManager 生成和验证令牌
    }
}
```

### 3. 配置参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用 JWT 认证 |
| `secret` | string | 必填 | JWT 签名密钥，用于令牌加密 |
| `expiration` | duration | 15m | 访问令牌有效期 |

刷新令牌有效期默认为访问令牌的 7 倍。

## 存储后端

### Store 接口

```go
type Store interface {
    Set(key string, value string, ttl time.Duration) error
    Get(key string) (string, error)
    Del(key string) error
}
```

### 使用项目 Redis 存储

项目已集成 Redis，可以直接使用项目的 Redis 客户端作为存储后端：

```go
package main

import (
    "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
    "github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
)

func main() {
    // 加载配置
    bs, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        panic(err)
    }
    defer bs.Close()

    // 初始化缓存
    if err := bs.InitializeCache(); err != nil {
        panic(err)
    }

    // 获取 Redis 客户端
    redisCache := bs.GetCache()
    redisClient := redisCache.(*redis.RedisCache).Client()

    // 创建 Redis Store 适配器
    redisStore := &RedisStoreAdapter{client: redisClient}

    // 初始化 JWTX
    jwtx.Init(jwtx.Config{
        Secret:         "your-secret-key",
        AccessTokenTTL: 15 * time.Minute,
        RefreshTokenTTL: 7 * 24 * time.Hour,
        Store:          redisStore,
    })
}

// RedisStoreAdapter 将项目 Redis 客户端适配为 jwtx.Store 接口
type RedisStoreAdapter struct {
    client *redis.Client
}

func (r *RedisStoreAdapter) Set(key string, value string, ttl time.Duration) error {
    ctx := context.Background()
    return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisStoreAdapter) Get(key string) (string, error) {
    ctx := context.Background()
    return r.client.Get(ctx, key).Result()
}

func (r *RedisStoreAdapter) Del(key string) error {
    ctx := context.Background()
    return r.client.Del(ctx, key).Err()
}
```

### 内存存储实现示例（用于测试）

```go
import (
    "sync"
    "time"
)

type MemoryStore struct {
    data map[string]expiryItem
    mu   sync.RWMutex
}

type expiryItem struct {
    value  string
    expiry time.Time
}

func NewMemoryStore() *MemoryStore {
    ms := &MemoryStore{
        data: make(map[string]expiryItem),
    }
    go ms.cleanup()
    return ms
}

func (m *MemoryStore) Set(key string, value string, ttl time.Duration) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.data[key] = expiryItem{
        value:  value,
        expiry: time.Now().Add(ttl),
    }
    return nil
}

func (m *MemoryStore) Get(key string) (string, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    item, ok := m.data[key]
    if !ok || time.Now().After(item.expiry) {
        return "", errors.New("key not found")
    }
    return item.value, nil
}

func (m *MemoryStore) Del(key string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.data, key)
    return nil
}

func (m *MemoryStore) cleanup() {
    ticker := time.NewTicker(time.Minute)
    for range ticker.C {
        m.mu.Lock()
        for key, item := range m.data {
            if time.Now().After(item.expiry) {
                delete(m.data, key)
            }
        }
        m.mu.Unlock()
    }
}
```

## 使用示例

### 1. 生成令牌对

```go
package main

import (
    "log"
    "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
)

func main() {
    // 初始化 JWTX
    jwtx.Init(jwtx.Config{
        Secret: "your-secret-key",
    })

    // 生成访问令牌和刷新令牌
    userID := "user123"
    username := "john_doe"
    deviceID := "device-abc-123"

    pair, err := jwtx.GenerateTokenPair(userID, username, deviceID)
    if err != nil {
        log.Fatalf("Failed to generate token pair: %v", err)
    }

    log.Println("Access Token:", pair.AccessToken)
    log.Println("Refresh Token:", pair.RefreshToken)
}
```

### 2. 验证访问令牌

```go
// 验证访问令牌
claims, err := jwtx.ValidateAccessToken(pair.AccessToken)
if err != nil {
    log.Fatalf("Invalid access token: %v", err)
}

log.Println("User ID:", claims.UserID)
log.Println("Username:", claims.Username)
log.Println("Device ID:", claims.DeviceID)
log.Println("Token ID:", claims.TokenID)
log.Println("Version:", claims.Version)
log.Println("Type:", claims.Type)
```

### 3. 刷新令牌

```go
// 使用刷新令牌获取新的令牌对
newPair, err := jwtx.Refresh(pair.RefreshToken)
if err != nil {
    log.Fatalf("Failed to refresh token: %v", err)
}

log.Println("New Access Token:", newPair.AccessToken)
log.Println("New Refresh Token:", newPair.RefreshToken)
```

### 4. 令牌撤销（登出）

```go
// 撤销单个令牌（将其加入黑名单）
err := jwtx.Logout(pair.AccessToken)
if err != nil {
    log.Fatalf("Failed to logout: %v", err)
}

// 撤销后，该令牌将无法通过验证
_, err = jwtx.ValidateAccessToken(pair.AccessToken)
if err != nil {
    log.Println("Token revoked:", err) // 输出: token revoked
}
```

### 5. 用户级撤销

```go
// 撤销用户的所有令牌
userID := "user123"
jwtx.RevokeUser(userID)

// 撤销后，该用户的所有令牌都将失效
_, err = jwtx.ValidateAccessToken(pair.AccessToken)
if err != nil {
    log.Println("Token invalid due to user revocation:", err)
}
```

## Gin 中间件集成

项目提供了 `pkg/middleware/auth.go` 中的 JWT 中间件，推荐使用项目提供的中间件而非 jwtx 包自带的中间件。

### 1. 使用项目提供的 JWT 中间件

```go
package main

import (
    "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/middleware"
    "github.com/gin-gonic/gin"
)

func main() {
    // 初始化 bootstrap
    bs, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        panic(err)
    }
    defer bs.Close()

    // 初始化认证
    if err := bs.InitializeAuth(); err != nil {
        panic(err)
    }

    // 创建 Gin 路由
    router := gin.Default()

    // 配置 JWT 中间件
    authConfig := middleware.AuthConfig{
        TokenLookup: "header:Authorization:Bearer ",
        SkipPaths:   []string{"/login", "/register", "/health"},
    }

    // 应用 JWT 中间件
    router.Use(middleware.JWT(authConfig))

    // 受保护的路由
    router.GET("/protected", func(c *gin.Context) {
        userID, _ := middleware.GetUserID(c)
        username, _ := middleware.GetUsername(c)
        
        c.JSON(200, gin.H{
            "message":  "Access granted",
            "user_id":  userID,
            "username": username,
        })
    })

    router.Run(":8080")
}
```

### 2. 分组路由保护

```go
// 公开路由
public := router.Group("/public")
{
    public.GET("/login", loginHandler)
    public.GET("/register", registerHandler)
}

// 受保护路由
protected := router.Group("/api")
protected.Use(middleware.JWT(authConfig))
{
    protected.GET("/profile", profileHandler)
    protected.POST("/logout", logoutHandler)
    protected.GET("/data", dataHandler)
}
```

### 3. 可选认证中间件

使用 `JWTOptional` 中间件，令牌无效时不中止请求：

```go
// 可选认证中间件
router.Use(middleware.JWTOptional(authConfig))

router.GET("/data", func(c *gin.Context) {
    userID, exists := middleware.GetUserID(c)
    if exists {
        // 已认证用户
        c.JSON(200, gin.H{"user_id": userID})
    } else {
        // 未认证用户
        c.JSON(200, gin.H{"message": "guest"})
    }
})
```

### 4. 权限控制中间件

结合 RBAC 进行权限控制：

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/rbac"

// 创建 RBAC 策略
policy := rbac.NewPolicy()

// 受权限保护的路由
adminGroup := router.Group("/admin")
adminGroup.Use(
    middleware.JWT(authConfig),
    middleware.RequirePermission(policy, "admin:access"),
)
{
    adminGroup.GET("/users", listUsersHandler)
}
```

## 完整示例：登录/登出 API

```go
package main

import (
    "errors"
    "net/http"
    "time"
    
    "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
    "github.com/alldev-run/golang-gin-rpc/pkg/middleware"
    "github.com/gin-gonic/gin"
)

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
    DeviceID string `json:"device_id"`
}

type LoginResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int64  `json:"expires_in"`
}

type RefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}

func main() {
    // 初始化 bootstrap
    bs, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        panic(err)
    }
    defer bs.Close()

    // 初始化认证
    if err := bs.InitializeAuth(); err != nil {
        panic(err)
    }

    router := gin.Default()

    // 配置 JWT 中间件
    authConfig := middleware.AuthConfig{
        TokenLookup: "header:Authorization:Bearer ",
        SkipPaths:   []string{"/login", "/refresh", "/health"},
    }

    // 登录接口
    router.POST("/login", loginHandler)

    // 刷新令牌接口
    router.POST("/refresh", refreshHandler)

    // 登出接口（需要认证）
    router.POST("/logout", middleware.JWT(authConfig), logoutHandler)

    // 受保护的接口
    router.GET("/profile", middleware.JWT(authConfig), profileHandler)

    router.Run(":8080")
}

func loginHandler(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 验证用户名密码（这里应该查询数据库）
    userID, err := authenticateUser(req.Username, req.Password)
    if err != nil {
        c.JSON(401, gin.H{"error": "invalid credentials"})
        return
    }

    // 生成令牌对
    pair, err := jwtx.GenerateTokenPair(userID, req.Username, req.DeviceID)
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to generate tokens"})
        return
    }

    c.JSON(200, LoginResponse{
        AccessToken:  pair.AccessToken,
        RefreshToken: pair.RefreshToken,
        ExpiresIn:    int64(15 * time.Minute / time.Second),
    })
}

func refreshHandler(c *gin.Context) {
    var req RefreshRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 刷新令牌
    newPair, err := jwtx.Refresh(req.RefreshToken)
    if err != nil {
        c.JSON(401, gin.H{"error": "invalid refresh token"})
        return
    }

    c.JSON(200, LoginResponse{
        AccessToken:  newPair.AccessToken,
        RefreshToken: newPair.RefreshToken,
        ExpiresIn:    int64(15 * time.Minute / time.Second),
    })
}

func logoutHandler(c *gin.Context) {
    token := c.GetHeader("Authorization")
    // 移除 Bearer 前缀
    token = token[7:]
    
    // 撤销令牌
    err := jwtx.Logout(token)
    if err != nil {
        c.JSON(500, gin.H{"error": "failed to logout"})
        return
    }

    c.JSON(200, gin.H{"message": "logged out successfully"})
}

func profileHandler(c *gin.Context) {
    userID, _ := middleware.GetUserID(c)
    username, _ := middleware.GetUsername(c)

    c.JSON(200, gin.H{
        "user_id":  userID,
        "username": username,
        // 其他用户信息...
    })
}

func authenticateUser(username, password string) (string, error) {
    // 实际应用中应该查询数据库验证用户
    if username == "admin" && password == "password" {
        return "user123", nil
    }
    return "", errors.New("invalid credentials")
}
```

## 高级用法

### 1. 使用多个 Manager 实例

```go
// 为不同的服务创建不同的 Manager
manager1 := jwtx.NewManager(jwtx.Config{
    Secret: "service1-secret",
})

manager2 := jwtx.NewManager(jwtx.Config{
    Secret: "service2-secret",
})

// 使用特定 Manager 生成令牌
pair1, err := manager1.GenerateTokenPair("user1", "alice", "device1")
pair2, err := manager2.GenerateTokenPair("user2", "bob", "device2")
```

### 2. 自定义 Claims Payload

Claims 结构已经支持 Payload 字段，可以在生成令牌时添加自定义信息：

```go
// 生成令牌后，修改 claims 的 payload
pair, err := jwtx.GenerateTokenPair(userID, username, deviceID)
if err != nil {
    return err
}

// 解码令牌以修改 payload
claims, err := jwtx.DefaultManager().decodeClaims(pair.AccessToken)
if err != nil {
    return err
}

// 添加自定义信息
if claims.Payload == nil {
    claims.Payload = make(map[string]string)
}
claims.Payload["role"] = "admin"
claims.Payload["permissions"] = "read,write,delete"

// 重新编码令牌
newToken, err := jwtx.DefaultManager().encodeClaims(*claims)
if err != nil {
    return err
}

pair.AccessToken = newToken
```

### 3. 令牌版本管理

```go
import "strconv"

// 当用户修改密码或敏感信息时，增加用户版本
func onPasswordChanged(userID string, store jwtx.Store) {
    // 获取当前版本
    version, _ := store.Get("user:version:" + userID)
    newVersion := 1
    if version != "" {
        newVersion, _ = strconv.Atoi(version)
        newVersion++
    }
    
    // 更新版本
    store.Set("user:version:"+userID, strconv.Itoa(newVersion), 0)
    
    // 或者使用 RevokeUser 快速撤销所有令牌
    jwtx.RevokeUser(userID)
}
```

## 安全建议

1. **密钥管理**：使用环境变量或密钥管理服务存储 Secret，不要硬编码在代码中
2. **HTTPS**：在生产环境中始终使用 HTTPS 传输令牌
3. **令牌存储**：刷新令牌应存储在 HTTP-only cookie 中，避免 XSS 攻击
4. **密钥轮换**：定期轮换签名密钥
5. **令牌过期**：合理设置访问令牌和刷新令牌的有效期
6. **存储安全**：使用 Redis 等持久化存储时，确保存储后端的安全配置

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

## 测试

```go
package jwtx_test

import (
    "testing"
    "time"
    "github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
)

func TestTokenFlow(t *testing.T) {
    // 初始化
    jwtx.Init(jwtx.Config{
        Secret: "test-secret",
        Store:  NewMemoryStore(),
    })

    // 生成令牌
    pair, err := jwtx.GenerateTokenPair("user1", "alice", "device1")
    if err != nil {
        t.Fatalf("GenerateTokenPair failed: %v", err)
    }

    // 验证访问令牌
    claims, err := jwtx.ValidateAccessToken(pair.AccessToken)
    if err != nil {
        t.Fatalf("ValidateAccessToken failed: %v", err)
    }

    if claims.UserID != "user1" {
        t.Errorf("Expected userID 'user1', got '%s'", claims.UserID)
    }

    // 刷新令牌
    newPair, err := jwtx.Refresh(pair.RefreshToken)
    if err != nil {
        t.Fatalf("Refresh failed: %v", err)
    }

    if newPair.AccessToken == pair.AccessToken {
        t.Error("Access token should change after refresh")
    }

    // 撤销令牌
    err = jwtx.Logout(newPair.AccessToken)
    if err != nil {
        t.Fatalf("Logout failed: %v", err)
    }

    // 验证撤销后的令牌
    _, err = jwtx.ValidateAccessToken(newPair.AccessToken)
    if err == nil {
        t.Error("Expected error after logout")
    }
}
```

运行测试：
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

## 常见问题

### Q: 为什么要使用双令牌机制？
A: 双令牌机制提供了更好的安全性和用户体验。访问令牌短期有效，即使泄露风险也有限；刷新令牌长期有效，用于获取新的访问令牌，避免频繁要求用户重新登录。

### Q: 如何处理令牌过期？
A: 客户端应在访问令牌过期前使用刷新令牌获取新的令牌对。可以在响应头中返回令牌过期时间，客户端据此判断何时刷新。

### Q: 不使用存储后端可以吗？
A: 可以，但会失去令牌撤销和版本控制功能。如果不需要这些功能，可以不配置 Store。

### Q: 如何实现多设备登录管理？
A: 通过 deviceID 区分不同设备的令牌。可以限制每个用户的最大设备数，或在用户登录时撤销旧设备的令牌。

### Q: 项目中间件和 jwtx 中间件有什么区别？
A: 项目提供的 `pkg/middleware/auth.go` 中间件功能更丰富，支持跳过路径、多种令牌查找方式（header/query/cookie）、RBAC 集成等。推荐使用项目提供的中间件。

## 相关文档

- [RPC 认证指南](./RPC_AUTHENTICATION.md)
- [网关认证完整指南](./gateway-auth-complete-guide.md)
