# 缓存插件

定义通用 `Cache` 接口，提供 Redis、Memcached、failover、多级/击穿保护及分布式锁组件。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/cache`

## 配置项

主包构造器接受子包配置：`RedisConfig` 等同 `cache/redis.Config`，
`MemcacheConfig` 等同 `cache/memcache.Config`，`FailoverConfig` 等同
`cache/failover.Config`。

| Redis 字段 | 类型 | 说明 |
|---|---|---|
| `Mode` / `Nodes` | `Mode` / `[]NodeConfig` | 单实例、集群、哨兵、主从或多节点；节点含 `Host`、`Port`、`Password`、`Database`、`Weight`、`IsMaster` |
| `Host` / `Port` / `Password` / `Database` | 基本连接参数 | 兼容单 Redis 实例 |
| `KeyPrefix` / `Enabled` | `string` / `bool` | key 前缀和启用开关 |
| `Timeout` / `DialTimeout` / `ReadTimeout` / `WriteTimeout` | `time.Duration` | 操作和连接超时 |
| `MaxRetries` / `PoolSize` / `MinIdleConns` / `MaxIdleConns` | `int` | 重试及连接池 |
| `ConnMaxIdleTime` / `ConnMaxLifetime` / `PoolTimeout` / `IdleTimeout` / `IdleCheckFrequency` | `time.Duration` | 连接生命周期和池等待 |
| `Sentinel` | 内嵌结构 | `MasterName`、`SentinelAddrs`、`SentinelPassword` |
| `Cluster` | 内嵌结构 | `MaxRedirects`、`ReadOnly`、`RouteByLatency`、`RouteRandomly` |
| `Multi` | 内嵌结构 | `ShardingStrategy`、`KeyPrefixRoutes`、`DefaultNode` |

| Memcached 字段 | 类型 | 默认值 |
|---|---|---|
| `Hosts` | `[]string` | `["localhost:11211"]` |
| `MaxIdleConns` / `MaxRetries` | `int` | `2` / `3` |
| `Timeout` / `DialTimeout` / `ReadTimeout` / `WriteTimeout` / `IdleTimeout` | `time.Duration` | `5s` / `5s` / `3s` / `3s` / `5m` |
| `HashAlgorithm` / `KeyPrefix` | `string` | `md5` / 空 |
| `CompressionEnabled` / `FailoverEnabled` / `Enabled` | `bool` | `false` / `true` / `true` |
| `SerializationFormat` | `string` | `json` |
| `HealthCheckInterval` | `time.Duration` | `1m` |

Redis 的 `NodeConfig` 字段为 `Host`、`Port`、`Password`、`Database`、`Weight`、
`IsMaster`；全部字段由 `redis.DefaultConfig()` 等配置工厂初始化。`Mode` 支持
`single`、`cluster`、`sentinel`、`master_slave` 和 `multi`。

| Failover 配置结构 / 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Config.StorageType` | `StorageType` | `file` | 主存储类型 |
| `Config.StoragePath` / `FileSuffix` | `string` | `/tmp/failover_cache` / `.failover` | 文件存储位置与后缀 |
| `Config.MaxRetries` / `RetryDelay` | `int` / `time.Duration` | `3` / `1s` | 重试次数与间隔 |
| `Config.HealthCheckInterval` | `time.Duration` | `30s` | 后端健康检查周期 |
| `Config.FallbackOrder` | `[]string` | `["file","memory"]` | 后端顺序 |
| `Config.Redis` / `Memcache` | 对应指针配置 | `nil` | 选择这些后端时必须提供配置 |
| `FileStorageConfig.Directory` / `FileSuffix` | `string` | 默认路径/后缀 | 文件存储目录与文件后缀 |
| `FileStorageConfig.MaxSize` / `MaxFiles` | `int64` / `int` | `0` | 文件大小及数量限制 |
| `FileStorageConfig.CleanupInterval` | `time.Duration` | `0` | 清理周期 |
| `RedisStorageConfig.Host` / `Port` / `Password` / `Database` | `string` / `int` / `string` / `int` | 由部署指定 | Redis 连接参数 |
| `RedisStorageConfig.KeyPrefix` / `Timeout` | `string` / `time.Duration` | 空 / 后端默认 | Redis key 前缀与超时 |
| `MemcacheStorageConfig.Hosts` | `[]string` | 由部署指定 | Memcached 服务列表 |
| `MemcacheStorageConfig.KeyPrefix` / `Timeout` | `string` / `time.Duration` | 空 / 后端默认 | key 前缀与超时 |

`memcache.DefaultConfig()` 提供 Memcached 客户端全部字段默认值；`redis.DefaultConfig()`
提供 Redis 连接池和模式默认值。failover 的零值超时/文件限制按存储构造器默认策略处理。

## 快速开始

```go
import (
	"context"
	"time"

	cache "github.com/alldev-run/golang-gin-rpc/plugins/cache"
	cacheRedis "github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
)

func cacheExample(ctx context.Context) error {
client, err := cache.NewRedisCache(cache.RedisConfig{
	Mode: cacheRedis.ModeSingle, Host: "127.0.0.1", Port: 6379,
})
if err != nil {
	return err
}
if err := client.Set(ctx, "session:42", []byte("value"), time.Hour); err != nil {
	return err
}
value, err := client.Get(ctx, "session:42")
_ = value
return err
}
```

`RedisConfig` 是 `plugins/cache/redis.Config` 的别名。

## API 概览

- `Cache`、`NewRedisCache`、`NewMemcacheCache`、`NewFailoverCache`
- `NewBreakdownCache`、`GetWithLock` 防止热点 key 并发击穿
- `cache/redis.NewDistributedLock`：`TryAcquire`、`Acquire`、`Release`、`EnableWatchdog`
- `cache/failover.NewFailoverCacheFromConfig` 构造备用存储链

## 错误与注意事项

后端错误经接口返回；Redis 构造会连接并检查服务。TTL 为零值的语义需按所用后端确认。
分布式锁需使用稳定且唯一的 key，锁超时应覆盖业务临界区。

## 生产运维建议

设置连接池、读写及拨号超时；为 Redis 设置 ACL/TLS，不在日志中记录值或凭据。监控命中率、
延迟、驱逐和 failover 状态。分布式锁仅用于短临界区，并处理续租失败。

Redis 分布式锁采用 SET NX PX 与唯一 token，并通过 Lua 校验释放/续期；看门狗按 TTL
周期续租。Redis 集成测试需服务端：`go test -tags integration ./...`。
