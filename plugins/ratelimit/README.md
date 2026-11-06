# 限流插件

提供进程内固定窗口、滑动窗口、令牌桶限流以及基于 Redis 的跨实例固定窗口限额。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/ratelimit`

## 配置项

该插件没有统一 YAML `Config`。子包 `ratelimiter.Config` 字段：

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Strategy` | `Strategy` | `StrategyTokenBucket` | 限流算法 |
| `Rate` | `float64` | `100` | 每秒请求速率 |
| `Burst` | `int` | `10` | 令牌桶突发容量 |
| `Window` | `time.Duration` | `1m` | 固定/滑动窗口时长 |
| `CleanupPeriod` | `time.Duration` | `10m` | 清理间隔 |

| `RedisLimiterConfig` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Requests` | `int` | `60` | 单窗口允许请求数 |
| `Window` | `time.Duration` | `1m` | 窗口时长 |
| `Prefix` | `string` | `ratelimit:` | Redis 计数键前缀 |

## 快速开始

```go
limiter := ratelimit.NewMemoryFixedWindow(100, time.Second)
if !limiter.Allow("client-ip") {
	return errors.New("rate limit exceeded")
}
return nil
```

Redis 版以 `RedisScriptRunner` 注入 Redis 执行器；`plugins/cache/redis` 的客户端可直接使用。

## API 概览

- `NewMemoryFixedWindow`、`NewMemorySlidingWindow`、`NewMemoryTokenBucket`
- `NewRedisLimiter`、`(*RedisLimiter).Allow`
- 子包 `ratelimiter`：`RateLimiter`、`Config`、`NewTokenBucketRateLimiter`、
  `Allow`、`AllowN`、`Wait`、`WaitN`、`Reserve`

## 错误与注意事项

Redis 故障时 `Allow` 返回错误；应用需显式选择 fail-open 或 fail-closed。相同 key 表示
共享一个主体配额，应避免将可控的高基数字符串直接作为 key。

## 生产运维建议

多实例需要集中式后端才能共享限额；按用户/IP/租户定义键并监控拒绝率。为 Redis 操作
设置超时，结合业务重试策略避免放大流量。
