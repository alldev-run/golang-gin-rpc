# ratelimit

限流插件。提供内存固定窗口 / 滑动窗口 / 令牌桶等算法，可独立用于网关或业务接口。

## 配置段

```yaml
plugins:
  ratelimit:
    enabled: true
security:
  rate_limit:
    enabled: true
    requests: 100
    window: 1s
```

## 依赖

- 无硬依赖

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/ratelimit"

lim := ratelimit.NewMemoryFixedWindow(100, time.Second)
if lim.Allow("client-ip") { /* pass */ }
```

## Redis 集中式限流(RedisLimiter)

`ratelimit.RedisLimiter` 提供跨实例共享配额的固定窗口限流(Lua 原子
INCR+PEXPIRE)。通过最小接口 `RedisScriptRunner` 注入执行器,
`plugins/cache/redis` 的 `*redis.Client` 天然满足,无需额外适配。

```go
limiter, err := ratelimit.NewRedisLimiter(redisClient,
    ratelimit.RedisLimiterConfig{Requests: 100, Window: time.Minute})
ok, err := limiter.Allow(ctx, userID) // false = 超出配额
```

多实例部署时,同一 `key`(如用户 ID / 客户端 IP)的全局限额由 Redis 计数共享;
Redis 故障时 `Allow` 返回 error,由调用方决定 fail-open 或 fail-closed。
