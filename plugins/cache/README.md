# cache

缓存插件。统一 `Cache` 接口，后端含 Redis / Memcache / Failover（主从切换）。

## 配置段

```yaml
plugins:
  cache:
    enabled: true
cache:
  type: redis
  redis:
    mode: single
    host: 127.0.0.1
    port: 6379
```

## 依赖

- `cache/redis`、`cache/memcache`、`cache/failover` 子包

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/cache"

c, _ := cache.NewRedisCache(redisCfg)
_ = c.Set(ctx, "k", "v", time.Hour)
v, _ := c.Get(ctx, "k")
```
