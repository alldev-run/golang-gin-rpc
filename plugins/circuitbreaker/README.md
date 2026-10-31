# circuitbreaker

熔断器插件。实现 closed / open / half-open 三态熔断，可包装任意 `func(ctx) error` 调用。

## 配置段

```yaml
plugins:
  circuitbreaker:
    enabled: true
```

## 依赖

- 无硬依赖

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/circuitbreaker"

cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
if err := cb.Execute(ctx, func() error { return callDownstream() }); err != nil {
    // open 或调用失败
}
```
