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
