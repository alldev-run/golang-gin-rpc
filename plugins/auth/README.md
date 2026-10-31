# auth

认证插件。提供 `AuthManager` 与 JWT（`jwtx` 子包）接入，负责访问令牌签发与校验。

## 配置段

```yaml
plugins:
  auth:
    enabled: true
security:
  jwt:
    enabled: true
    secret: "change-me"
    expiration: 3600
```

## 依赖

- `auth/jwtx`（同插件内子包）

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/auth"

mgr := auth.NewAuthManager(auth.AuthConfig{Enabled: true, JWT: jwtx.Config{Secret: "..."}})
token, err := mgr.Issue(ctx, claims)
```
