# 认证插件

提供 JWT 访问/刷新令牌签发、校验与 Gin 中间件。JWT 配置和令牌 API 位于
`jwtx` 子包；密钥应从密钥管理系统注入，不能提交到配置库。

## 安装

```text
github.com/alldev-run/golang-gin-rpc/plugins/auth
github.com/alldev-run/golang-gin-rpc/plugins/auth/jwtx
```

## 配置项

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `AuthConfig.Enabled` | `bool` | `false` | 是否初始化 JWT 管理器 |
| `AuthConfig.JWT` | `jwtx.Config` | TTL 使用 jwtx 默认值 | JWT 签名与存储配置 |
| `jwtx.Config.Secret` | `string` | 空 | HMAC 签名密钥；启用时必填 |
| `jwtx.Config.AccessTokenTTL` | `time.Duration` | 15 分钟 | 访问令牌有效期 |
| `jwtx.Config.RefreshTokenTTL` | `time.Duration` | 7 天 | 刷新令牌有效期 |
| `jwtx.Config.Store` | `jwtx.Store` | `nil` | 可选的令牌版本/持久化存储 |

## 快速开始

```go
package main

import (
	"log"

	"github.com/alldev-run/golang-gin-rpc/plugins/auth"
	"github.com/alldev-run/golang-gin-rpc/plugins/auth/jwtx"
	"github.com/gin-gonic/gin"
)

func main() {
	manager := auth.NewAuthManager(auth.AuthConfig{
		Enabled: true,
		JWT:     jwtx.Config{Secret: "load-this-from-a-secret-store"},
	})
	if !manager.IsReady() {
		log.Fatal(manager.ValidationError())
	}
	tokens, err := manager.JWT().GenerateTokenPair("user-1", "alice", "web")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := manager.JWT().ValidateAccessToken(tokens.AccessToken); err != nil {
		log.Fatal(err)
	}

	router := gin.New()
	router.Use(manager.JWT().Middleware())
}
```

## API 概览

- `auth.NewAuthManager`、`(*AuthManager).IsReady`、`ValidationError`、`JWT`
- `jwtx.NewManager`、`(*Manager).GenerateTokenPair`、`SignAccessClaims`、
  `ValidateAccessToken`、`Middleware`
- `jwtx.Config`、`jwtx.Claims`、`jwtx.TokenPair`

## 错误与注意事项

- `Enabled: true` 时 `JWT.Secret` 不能为空；检查 `ValidationError()`。
- 令牌 API 返回错误，调用方必须检查；不要记录 token 或密钥。
- `Middleware()` 会验证访问令牌；不要用刷新令牌访问受保护路由。

## 生产运维建议

- 使用密钥管理系统轮换高熵密钥；HTTPS 是令牌传输的前提。
- 根据风险设置短访问令牌 TTL，并妥善保护刷新令牌。
- 需要跨实例撤销/版本控制时实现并配置 `jwtx.Store`。
