# 推送通知插件

提供 Apple APNs 与 Firebase Cloud Messaging 客户端，支持单播、多播、主题推送和健康检查。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/push`

## 配置项

| 结构体 / 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Config.Provider` | `ProviderType` | — | APNs 或 FCM |
| `Config.Enabled` | `bool` | `true` | 启用客户端 |
| `Config.Timeout` / `RetryAttempts` | `time.Duration` / `int` | `30s` / `3` | 请求超时和重试次数 |
| `Config.FCM` / `Config.APNs` | `FCMConfig` / `APNsConfig` | 零值 | 服务商配置 |
| `FCMConfig.ServiceAccountKeyPath` | `string` | 空 | Firebase JSON 密钥文件 |
| `FCMConfig.ServiceAccountKey` | `string` | 空 | Firebase JSON 密钥原文 |
| `FCMConfig.ProjectID` | `string` | 空 | Firebase 项目 ID |
| `FCMConfig.Credentials` | `ServiceAccountCredentials` | 运行时解析 | 不从配置文件读取 |
| `FCMConfig.UseLegacyAPI` / `LegacyServerKey` | `bool` / `string` | `false` / 空 | 旧版 FCM API 与密钥 |
| `APNsConfig.CertPath` / `KeyPath` | `string` | 空 | 证书与 PEM 私钥路径 |
| `APNsConfig.CertPassword` | `string` | 空 | 证书密码 |
| `APNsConfig.TeamID` / `KeyID` / `BundleID` | `string` | 空 | APNs token 身份信息 |
| `APNsConfig.AuthKey` / `AuthKeyPath` | `string` | 空 | p8 密钥内容或文件路径 |
| `APNsConfig.UseTokenAuth` / `UseSandbox` | `bool` | `true` / `false` | token 认证与沙箱环境 |

## 快速开始

```go
config := push.DefaultConfig()
config.Provider = push.ProviderType("fcm")
config.FCM.ProjectID = "project-id"
config.FCM.ServiceAccountKeyPath = "/run/secrets/firebase.json"
client, err := push.NewClient(config)
if err != nil {
	return err
}
defer client.Close()

notification := push.NewNotification("Order ready", "Your order is ready", push.PlatformAndroid).
	WithTokens(deviceToken).
	WithHighPriority()
_, err = client.Send(ctx, notification)
return err
```

## API 概览

- `DefaultConfig`、`ValidateConfig`、`NewClient`、`CreateClient`
- `Client.Send`、`SendMulticast`、`SendToTopic`、`IsHealthy`、`Close`
- `NewNotification`、`WithTokens`、`WithTopic`、`WithData`、`WithTTL`

## 错误与注意事项

配置校验及密钥文件读取失败会在构造时返回。通知必须提供 token 或 topic，且需满足
平台 payload 限制。`Close` 应在应用退出时调用。

## 生产运维建议

使用只读 secret mount 并限制文件权限；按平台限制调整超时和重试，监控拒绝、限流及
无效 token，避免对不可重放请求进行无界重试。
