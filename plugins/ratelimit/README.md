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
