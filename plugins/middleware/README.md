# 中间件插件

提供 Gin HTTP 中间件，包括 JWT/API key 认证、CORS、限流、审计、请求日志、IP 过滤、安全头和 panic 恢复。
包内多数 handler 是 `gin.HandlerFunc`，并非通用 `net/http` middleware。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/middleware`

## 配置项

`Config` 的根字段为 `Auth AuthConfig`、`CORS CORSConfig`、`RateLimiter RateLimiterConfig`、
`Recovery RecoveryConfig`、`Logging LoggingConfig`、`Security SecurityConfig` 和
`IPFilter IPFilterConfig`。`DefaultConfig()` 提供组合默认值。

| 配置结构 | 字段（类型） | 默认说明 |
|---|---|---|
| `AuthConfig` | `SkipPaths []string`、`Skipper func(string) bool`、`Required bool`、`TokenHeader string`、`TokenQuery string`、`CookieName string`、`TokenLookup string`、`KeyFunc func(*interface{}) string`、`Enabled bool` | `DefaultAuthConfig()`；函数字段不参与 YAML/JSON |
| `CORSConfig` | `AllowedOrigins []string`、`AllowedMethods []string`、`AllowedHeaders []string`、`ExposedHeaders []string`、`AllowCredentials bool`、`MaxAge int`、`OptionsPassthrough bool`、`Enabled bool` | `DefaultCORSConfig()` |
| `RateLimiterConfig` | `RequestsPerMinute int`、`BurstSize int`、`SkipPaths []string`、`KeyGenerator func(string) string`、`SkipSuccessful bool`、`Message string`、`Enabled bool` | `DefaultRateLimiterConfig()`；函数字段不序列化 |
| `RecoveryConfig` | `StackSize int`、`Logger func(*gin.Context, interface{})`、`LogAllRequests bool`、`RequestBodyLimit int64`、`DisableStackAll bool`、`DisablePrintStack bool`、`Enabled bool` | `DefaultRecoveryConfig()`；logger 函数不序列化 |
| `LoggingConfig` | `SkipPaths []string`、`SkipLogger func(string) bool`、`Output string`、`Format string`、`UTC bool`、`SkipBody bool`、`MaxBodySize int`、`Enabled bool` | `DefaultLoggingConfig()`；函数字段不序列化 |
| `SecurityConfig` | `XSSProtection bool`、`ContentTypeNosniff bool`、`XFrameOptions string`、`HSTSMaxAge int`、`HSTSIncludeSubdomains bool`、`ContentSecurityPolicy string`、`ReferrerPolicy string`、`Enabled bool` | `DefaultSecurityConfig()` |
| `IPFilterConfig` | `Enabled bool`、`Mode IPFilterMode`、`IPList []string`、`CountryBlacklist []string`、`CountryWhitelist []string`、`EnableGeoIP bool`、`GeoIPDBPath string`、`BlockMessage string`、`BlockStatusCode int`、`SkipPaths []string`、`TrustProxy bool` | `DefaultIPFilterConfig()`；CIDR 支持见 `IPList` |
| `APIKeyConfig` | `APIKeys map[string]string`、`HeaderName string`、`QueryName string`、`SkipPaths []string`、`Skipper func(*gin.Context) bool`、`Enabled bool` | `DefaultAPIKeyConfig()`；默认关闭 |
| `AuditConfig` | `Enabled bool`、`SkipPaths []string`、`SensitiveKeys []string`、`Dynamic *DynamicAuditConfig`、`Sink audit.Sink`、`ActionResolver func(*gin.Context) audit.Action`、`ResourceResolver func(*gin.Context) string` | `DefaultAuditConfig()` |
| `RequestLoggerConfig` | `SkipPaths []string`、`MaxBodySize int64`、`LogRequestBody bool`、`LogResponseBody bool`、`LogHeaders bool`、`SensitiveHeaders []string` | `DefaultRequestLoggerConfig()` |

## 快速开始

```go
router := gin.New()
apiKeyConfig := middleware.DefaultAPIKeyConfig()
apiKeyConfig.Enabled = true
apiKeyConfig.APIKeys["secret-from-vault"] = "service-a"
router.Use(middleware.Recovery(), middleware.APIKey(apiKeyConfig))
router.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
```

## API 概览

- `JWT`、`JWTOptional`、`RequireAuth`、`RequireRole`、`RequirePermission`
- `APIKey`、`APIKeyOptional`、`CORS`、`RateLimiter`、`IPFilter`
- `Audit`、`RequestLogger`、`Recovery`、`RequestID`
- `DefaultConfig`、`DefaultAPIKeyConfig`、`DefaultAuditConfig`、
  `DefaultRequestLoggerConfig`

## 错误与注意事项

认证失败会中止请求；确保 skip paths 只覆盖公开端点。`TrustProxy` 仅应在可信反向代理
网络中启用。GeoIP 数据库无法加载时 `InitGeoIP` 会返回错误。

## 生产运维建议

仅使用 TLS 传输 API key，secret 通过 secret store 注入；限制 body/header 日志并脱敏，
设置稳定的路径级限流和低基数标签。CORS 不应对带凭据请求开放任意 origin。
