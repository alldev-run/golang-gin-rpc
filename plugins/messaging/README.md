# 消息队列插件

通过统一 `Client` / `Publisher` / `Subscriber` 接口使用 RabbitMQ、Kafka 或 NATS。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/messaging`

## 配置项

`Config` 字段均可序列化为 YAML/JSON。`DefaultConfig()` 适用于 RabbitMQ；默认凭据仅供本地
开发，生产中应覆盖。表中按默认配置列出：

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Type` | `MessageType` | `rabbitmq` | 后端类型 |
| `Host` / `Port` | `string` / `int` | `localhost` / `5672` | 服务地址 |
| `Username` / `Password` / `Database` | `string` | `guest` / `guest` / `/` | 认证与 vhost/database |
| `Timeout` / `ConnectionTimeout` | `time.Duration` | `30s` / `10s` | 操作与连接超时 |
| `ReadTimeout` / `WriteTimeout` | `time.Duration` | `30s` / `30s` | I/O 超时 |
| `MaxRetries` / `RetryDelay` | `int` / `time.Duration` | `3` / `5s` | 重试策略 |
| `PoolSize` / `IdleTimeout` | `int` / `time.Duration` | `10` / `60s` | 连接池设置 |
| `Heartbeat` / `PrefetchCount` | `time.Duration` / `int` | `30s` / `10` | 心跳和预取 |
| `DefaultExchange` / `DefaultQueue` / `DefaultRoutingKey` | `string` | 空 / `default` / `default` | RabbitMQ 发布/消费缺省项 |
| `DeliveryMode` / `Priority` | `DeliveryMode` / `int` | `persistent` / `0` | 投递持久性与优先级 |
| `TTL` / `Expiration` | `time.Duration` | `0` / `0` | 消息生命周期 |
| `Compression` / `Persistent` / `AutoReconnect` | `bool` | `false` / `true` / `true` | 压缩、持久连接及自动重连 |
| `ReconnectDelay` / `MaxReconnectAttempts` | `time.Duration` / `int` | `5s` / `5` | 重连策略 |
| `Options` | `map[string]interface{}` | 空 map | 后端扩展参数 |
| `Enabled` | `bool` | `true` | 是否启用 |

## 快速开始

```go
config := messaging.DefaultConfig()
client, err := messaging.NewClient(config)
if err != nil {
	return err
}
defer client.Close()
err = client.Publish(ctx, "orders.created", &messaging.Message{
	Topic: "orders.created", Payload: []byte(`{"id":"o-42"}`),
})
return err
```

## API 概览

- `DefaultConfig`、`RabbitMQConfig`、`KafkaConfig`、`NewClient`
- `Client.Publish`、`PublishAsync`、`Subscribe`、`Unsubscribe`、`Close`
- `Message`、`MessageHandler`、`DeliveryModeTransient`、`DeliveryModePersistent`
- `HandleRequests`、`RequestJSON` 提供 request/reply 帮助方法。

## 错误与注意事项

当前 `NewClient` 支持 RabbitMQ、Kafka 和 NATS；其他 `MessageType` 不代表已有驱动实现。
发布/订阅返回的错误需由调用方处理。Kafka reply topic 需允许自动创建或预先创建。

## 生产运维建议

生产环境配置 TLS、强认证和有限超时；合理设置 Kafka/RabbitMQ 预取和连接池。限制重试并
确保消费者幂等，监控积压、重连、发布失败和消费延迟。

## 请求-应答(Request/Reply)

基于通用 `Client` 接口(发布/订阅)实现 RPC 风格消息模式，信封携带
`correlation_id` 与 `reply_to` 临时主题;NATS/RabbitMQ 开箱即用,
Kafka 的 reply 主题需允许自动创建或预创建。

```go
// 应答方:把业务函数包装成 MessageHandler 挂到请求主题
_ = client.Subscribe(ctx, "calc.double", messaging.HandleRequests(client,
    func(ctx context.Context, payload []byte) ([]byte, error) {
        var req CalcRequest
        if err := json.Unmarshal(payload, &req); err != nil {
            return nil, err
        }
        return json.Marshal(CalcReply{N: req.N * 2})
    }))

// 请求方:发布请求并等待响应
out, err := messaging.RequestJSON(ctx, client, "calc.double",
    CalcRequest{N: 21}, 3*time.Second)
// 远端业务错误以 *messaging.ReplyError 返回;超时为 messaging.ErrRequestTimeout
```
