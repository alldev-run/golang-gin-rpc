# 告警插件

提供分级告警、同步发送、异步队列、级别回调，以及日志和 JSON HTTP Webhook 通道。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/alert`

## 配置项

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Config.Enabled` | `bool` | `true` | 启用后台队列；同步发送可独立使用 |
| `Config.BufferSize` | `int` | `1000` | 异步队列容量 |
| `Config.FlushInterval` | `time.Duration` | `5s` | 队列刷新间隔 |
| `Config.Channels` | `[]ChannelConfig` | 空 | 通道配置列表 |
| `ChannelConfig.Type` | `string` | — | 通道类型 |
| `ChannelConfig.Name` | `string` | — | 通道名称 |
| `ChannelConfig.Enabled` | `bool` | — | 通道开关 |
| `ChannelConfig.Config` | `map[string]interface{}` | — | 通道特定配置 |

## 快速开始

```go
manager := alert.NewManager(alert.DefaultConfig())
manager.RegisterChannel(alert.NewWebhookChannel(
	"ops", "https://alerts.example.com/webhook", map[string]string{"X-Token": secret},
))
err := manager.Send(ctx, alert.Alert{
	Level: alert.LevelCritical, Title: "database unavailable",
	Source: "orders", Message: "primary database check failed",
})
if err != nil {
	return err
}
return manager.Close()
```

Webhook 默认超时为 10 秒；如需自定义 transport 或复用连接池，使用
`NewWebhookChannelWithClient`。请求支持 context、JSON 内容类型和自定义 headers。

## API 概览

- `Alert`、`LevelDebug`、`LevelInfo`、`LevelWarning`、`LevelError`、`LevelCritical`
- `NewManager`、`RegisterChannel`、`Send`、`Queue`、`Close`、`OnAlert`
- `NewWebhookChannel`、`NewWebhookChannelWithClient`、`NewLogChannel`
- `Debug`、`Info`、`Warning`、`Error`、`Critical`

## 错误与注意事项

Webhook 网络错误、context 取消及非 2xx 状态会由 `Send` 返回错误。`Queue` 不会向
调用方返回发送错误；队列满或关闭时应配置降级路径。应用退出时调用 `Close`。

## 生产运维建议

为 Webhook 配置 TLS、认证和 secret 管理，避免在 headers 或日志中泄密；按峰值调整
队列容量并监控积压与投递失败。
