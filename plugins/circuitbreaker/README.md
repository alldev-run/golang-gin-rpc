# 熔断器插件

提供并发安全的熔断器、分组管理、健康检查与 Gin 中间件。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/circuitbreaker`

## 配置项

| `Config` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `MaxRequests` | `uint32` | `1` | 半开状态允许的最大探测请求 |
| `Interval` | `time.Duration` | `1m` | 闭合状态统计窗口 |
| `Timeout` | `time.Duration` | `1m` | 打开状态后转为半开的等待时间 |
| `ReadyToTrip` | `func(Counts) bool` | 连续失败超过 5 次 | 打开状态的触发条件 |
| `OnStateChange` | `func(string, State, State)` | 结构化日志 | 状态变化回调 |
| `IsSuccessful` | `func(error) bool` | `err == nil` | 调用成功判定 |
| `Fallback` | `func(error) error` | `nil` | 可选降级回调 |

使用 `DefaultConfig()` 获取上述默认值，再覆盖业务相关字段。

## 快速开始

```go
config := circuitbreaker.DefaultConfig()
breaker := circuitbreaker.NewCircuitBreaker("inventory", config)
value, err := breaker.Execute(func() (interface{}, error) {
	return inventoryClient.Lookup(ctx, sku)
})
if err != nil {
	return err
}
_ = value
```

## API 概览

- `NewCircuitBreaker`、`Execute`、`ExecuteContext`、`State`、`Counts`
- `NewCircuitBreakerGroup` 管理多个命名熔断器。
- `NewHealthChecker`、`NewMiddleware` 提供健康检查和请求中间件。

## 错误与注意事项

打开状态会拒绝调用或执行已配置的 fallback；调用方仍须处理业务错误。不要在
`Fallback` 中递归调用同一熔断器。

## 生产运维建议

为下游调用设置 context deadline；按依赖分别命名熔断器，并监控状态变化、连续失败和
降级次数。统计窗口和半开探测数应与服务容量相符。
