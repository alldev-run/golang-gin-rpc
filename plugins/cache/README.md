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

## Redis 分布式锁(DistributedLock)

`redis.DistributedLock` 提供正确的互斥语义:SET NX PX 获取 + 唯一持有者
token + Lua 校验释放/续期(不会误删他人的锁),可选看门狗按 ttl/3 自动续期。

```go
l := redis.NewDistributedLock(client, "order:42", 30*time.Second)
l.EnableWatchdog() // 可选:业务执行期间自动续期
if ok, _ := l.TryAcquire(ctx); ok {
    defer l.Release(ctx)
    // ... 临界区
}
// 阻塞式:l.Acquire(ctx, 100*time.Millisecond)
```

集成测试(需要真实 Redis):`go test -tags integration ./plugins/cache/redis/`
