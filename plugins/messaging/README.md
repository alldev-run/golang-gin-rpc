# messaging

消息队列插件。统一 `Client` 接口，后端含 RabbitMQ / Kafka / NATS / Redis / Memory。

## 配置段

```yaml
plugins:
  messaging:
    enabled: true
messaging:
  type: rabbitmq
  address: "amqp://guest:guest@localhost:5672/"
```

## 依赖

- 无硬依赖；按 `type` 选择后端

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/messaging"

c, _ := messaging.NewClient(messaging.Config{Type: messaging.MessageTypeKafka, Brokers: []string{"localhost:9092"}})
_ = c.Publish(ctx, "topic", []byte("hello"))
```

## 请求-应答(Request/Reply)

基于通用 `Client` 接口(发布/订阅)实现的 RPC 风格消息模式,信封携带
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
