# Actor NATS Bridge

`pkg/actor/nats` 将 Actor 系统与消息系统（NATS 或任意实现了 `messaging.Client` 的后端）
桥接，让 Actor 可以发布消息、订阅主题，并把订阅到的消息以 Actor 消息形式投递。

## 特性

- **Publish / PublishAsync**：从 Actor 系统外部向消息主题发布消息。
- **Subscribe**：把某个主题的消息转发给指定 Actor，Actor 收到的是 `*nats.Message`。
- **Unsubscribe / Close**：管理订阅生命周期。
- **配置驱动**：可通过 `messaging.Config` 直接构建底层 client。
- **后端无关**：`Bridge` 持有 `messaging.Client` 接口，NATS / Kafka / RabbitMQ 均可接入。

## 与 `pkg/messaging` 的关系

`pkg/actor/nats` 与 `pkg/messaging` 是**两层不同的抽象**，组合使用而非互相替代：

```
┌──────────────────────────────────────────────┐
│  pkg/actor/nats.Bridge                       │  集成层
│  - subject -> actor.PID 路由                  │  把消息投递到 Actor mailbox
│  - messaging.Message -> nats.Message 转换     │
└──────────────────┬───────────────────────────┘
                   │ 持有
┌──────────────────▼───────────────────────────┐
│  pkg/messaging.Client (接口)                  │  传输层
│  - NATSClient / KafkaClient / RabbitMQClient  │  处理连接、订阅、发布字节流
└──────────────────────────────────────────────┘
```

| 关注点 | `pkg/messaging` | `pkg/actor/nats.Bridge` |
|---|---|---|
| 知道 Actor 是什么？ | 否 | 是 |
| 处理线协议/连接 | 是 | 否 |
| 消息路由目标 | `MessageHandler` 回调 | 具体 `actor.PID` 的 mailbox |
| 维护订阅映射 | 各 Client 内部 | `subject -> PID` |

### 何时用哪个

- 只想发/收消息，不涉及 Actor → 直接用 `messaging.NATSClient`（或 `KafkaClient` 等）。
- 想让 **Actor** 收发消息 → 用 `actor/nats.Bridge`，它内部会持有任意 `messaging.Client`。

### 关于包名

包名沿用 `nats` 是历史原因——对外暴露的 `Message` 结构使用了 NATS 术语（`Subject` / `Reply`）。
但 `Bridge` 本身与 NATS 无强绑定，构造函数接受任意 `messaging.Client`：

```go
func NewBridge(system *actor.System, client messaging.Client) *Bridge
```

因此可以用 Kafka、RabbitMQ 等任意后端驱动同一个 Bridge。

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/alldev-run/golang-gin-rpc/pkg/actor"
    actorNats "github.com/alldev-run/golang-gin-rpc/pkg/actor/nats"
    "github.com/alldev-run/golang-gin-rpc/pkg/messaging"
)

type orderActor struct{}

func (a *orderActor) Receive(ctx actor.Context, msg actor.Message) {
    if m, ok := msg.(*actorNats.Message); ok {
        log.Printf("received on %s: %s (reply=%s)", m.Subject, string(m.Payload), m.Reply)
    }
}

func main() {
    sys := actor.New()
    defer sys.Shutdown()

    // 用任意 messaging.Client 实现；这里以 NATS 配置为例
    cfg := messaging.NATSConfig("localhost", 4222)
    bridge, err := actorNats.NewBridgeFromConfig(sys, cfg)
    if err != nil {
        panic(err)
    }
    defer bridge.Close()

    pid, err := sys.SpawnNamed("orders", &orderActor{})
    if err != nil {
        panic(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := bridge.Subscribe(ctx, "orders.created", pid); err != nil {
        panic(err)
    }

    if err := bridge.Publish(ctx, "orders.created", []byte("order-1"), nil); err != nil {
        panic(err)
    }

    time.Sleep(time.Second)
    fmt.Println("done")
}
```

## API

### 创建 Bridge

```go
// 已有 client
bridge := actorNats.NewBridge(sys, client)

// 从配置构建 client 并创建 bridge
bridge, err := actorNats.NewBridgeFromConfig(sys, messaging.NATSConfig("localhost", 4222))
```

### 发布

```go
err := bridge.Publish(ctx, "orders.created", []byte("payload"), map[string]string{"id": "1"})
err := bridge.PublishAsync(ctx, "orders.created", []byte("payload"), nil)
```

`headers` 会作为消息头传递；NATS reply subject 约定通过 header key `nats_reply` 传递。

### 订阅

```go
err := bridge.Subscribe(ctx, "orders.created", pid)
```

订阅后，目标 Actor 会收到 `*nats.Message`：

| 字段 | 说明 |
|---|---|
| `Subject` | 消息主题。 |
| `Reply` | NATS reply subject（来自 header `nats_reply`，无则为空）。 |
| `Payload` | 消息负载。 |
| `Headers` | 消息头（`map[string]string`）。 |

同一 subject 重复订阅会返回错误。`ctx` 取消后会自动清理内部订阅记录。

### 取消订阅 / 关闭

```go
err := bridge.Unsubscribe("orders.created")
err := bridge.Close()  // 关闭底层 client
```

### 访问底层对象

```go
sys := bridge.System()  // *actor.System
client := bridge.Client()  // messaging.Client
```

## 注意事项

- `Subscribe` 是有状态的：内部维护 subject -> PID 映射，重复订阅同一 subject 会失败。
- 订阅的 `ctx` 取消后，内部映射会被清理；底层 client 的取消订阅需显式调用 `Unsubscribe`。
- `Message.Reply` 通过消息头 `nats_reply` 传递；若发布方未设置该 header，则为空字符串。
