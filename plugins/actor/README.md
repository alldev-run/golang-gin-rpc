# Actor

轻量级 Actor 模型实现，位于 `pkg/actor`。

## 特性

- **串行消息处理**：每个 Actor 拥有独立邮箱，消息按 FIFO 顺序串行处理，避免锁竞争。
- **生命周期管理**：支持 `PreStart` / `PostStop` 钩子。
- **行为切换**：运行时使用 `Become` 切换消息处理函数。
- **优雅关闭**：`Shutdown` 会等待所有 Actor 处理完当前消息并退出。
- **Panic 恢复**：单条消息 panic 不会导致整个 Actor 退出。
- **超时发送**：`SendWithContext` 支持 context 超时控制。
- **死信回调**：通过 `Options.OnDeadLetter` 监听无法送达的消息。
- **工厂创建**：通过 `FromProducer` 支持每次重启创建新实例。

## 目录结构

```
pkg/actor/
├── actor.go          # 核心接口：Message / PID / Context / Actor / 生命周期接口
├── system.go         # Actor 系统：创建、查找、发送、停止
├── cell.go           # 单个 Actor 的运行时封装与主循环
├── mailbox.go        # 基于 channel 的邮箱实现
├── context.go        # Context 接口的实现
├── props.go          # Props：描述如何创建一个 Actor
├── options.go        # 系统级配置 Options
├── errors.go         # 哨兵错误
├── discovery/        # 服务注册与发现桥接（见 ./discovery/README.md）
└── nats/             # Actor 与 NATS/消息系统的桥接（见 ./nats/README.md）
```

## 快速开始

```go
package main

import (
    "fmt"
    "time"

    "github.com/alldev-run/golang-gin-rpc/pkg/actor"
)

type HelloActor struct{}

func (a *HelloActor) Receive(ctx actor.Context, msg actor.Message) {
    if s, ok := msg.(string); ok {
        fmt.Printf("[%s] received: %s\n", ctx.Self(), s)
    }
}

func main() {
    sys := actor.New()
    defer sys.Shutdown()

    pid, err := sys.SpawnNamed("hello", &HelloActor{})
    if err != nil {
        panic(err)
    }

    _ = sys.Send(pid, "world")
    time.Sleep(time.Second)
}
```

## API

### 创建系统

```go
sys := actor.New()
// 或
cfg := actor.DefaultOptions()
sys := actor.NewWithOptions(cfg)
```

`Options` 字段：

| 字段 | 说明 |
|---|---|
| `DefaultMailboxSize` | 默认邮箱缓冲区大小，<=0 时使用内置默认值 64。 |
| `OnDeadLetter` | 消息无法送达（如目标 Actor 不存在）时回调，参数为 `(pid, msg, err)`。 |

### 创建 Actor

```go
// 方式一：直接传入实例
pid, err := sys.Spawn(actor.FromActor(&MyActor{}).WithName("my-actor"))

// 方式二：便捷方法
pid, err := sys.SpawnNamed("my-actor", &MyActor{})

// 方式三：工厂函数，支持每次重启创建新实例
pid, err := sys.Spawn(actor.FromProducer(func() actor.Actor {
    return &MyActor{}
}).WithName("from-producer"))

// 自定义邮箱大小
pid, err := sys.Spawn(actor.FromActor(&MyActor{}).
    WithName("big-mailbox").
    WithMailboxSize(1024))
```

`Props` 链式方法：

| 方法 | 说明 |
|---|---|
| `WithName(name)` | 设置 Actor 名称（PID）。 |
| `WithMailboxSize(size)` | 设置邮箱缓冲区大小，<=0 时使用系统默认值。 |

### 发送消息

```go
err := sys.Send(pid, "hello")

// 带超时
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()
err := sys.SendWithContext(ctx, pid, "hello")
```

### 在 Actor 内部发送

```go
func (a *MyActor) Receive(ctx actor.Context, msg actor.Message) {
    _ = ctx.Send(ctx.Self(), "next")        // 给自己发
    _ = ctx.Send(someOtherPID, "forward")   // 给其他 Actor 发

    // 带超时
    c, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()
    _ = ctx.SendWithContext(c, someOtherPID, "forward")
}
```

### Context 方法

| 方法 | 说明 |
|---|---|
| `Self() PID` | 当前 Actor 的 PID。 |
| `Sender() PID` | 当前消息的发送方 PID，未知时为空字符串。 |
| `System() *System` | 所属的 Actor 系统。 |
| `Send(to, msg)` | 向目标 Actor 发送消息，发送方自动设为当前 Actor。 |
| `SendWithContext(ctx, to, msg)` | 带超时的发送。 |
| `StopSelf()` | 停止当前 Actor。 |
| `Become(behavior)` | 切换当前 Actor 的行为函数；传 `nil` 重置为默认 `Receive`。 |

### 停止

```go
// 停止单个 Actor（阻塞等待其退出）
_ = sys.Stop(pid)

// 关闭整个系统（阻塞等待所有 Actor 退出）
_ = sys.Shutdown()
```

### 系统状态

```go
sys.Running()  // bool，系统是否仍在运行
sys.Count()    // int，当前存活的 Actor 数量
```

### 行为切换

```go
func (a *MyActor) Receive(ctx actor.Context, msg actor.Message) {
    if msg == "init" {
        ctx.Become(a.running)
    }
}

func (a *MyActor) running(ctx actor.Context, msg actor.Message) {
    // 新的处理逻辑
}
```

### 生命周期钩子

Actor 可选实现 `PreStarter` / `PostStopper` 接口：

```go
func (a *MyActor) PreStart(ctx actor.Context) {
    // 启动后、处理第一条消息前调用
}

func (a *MyActor) PostStop(ctx actor.Context) {
    // 停止后调用
}
```

## 错误

| 错误 | 说明 |
|---|---|
| `ErrSystemStopped` | Actor 系统已经停止。 |
| `ErrActorNotFound` | 目标 Actor 不存在。 |
| `ErrActorAlreadyExists` | 同名 Actor 已存在。 |
| `ErrMailboxClosed` | Actor 邮箱已关闭。 |

## 子包

- **[discovery](./discovery/README.md)**：将 Actor 系统与服务发现后端桥接，提供 `Registry`、`RegistryActor` 及注册/发现消息类型。
- **[nats](./nats/README.md)**：将 Actor 系统与消息系统（NATS 或任意 `messaging.Client`，如 Kafka/RabbitMQ）桥接，把主题消息路由到指定 Actor 的 mailbox。与 `pkg/messaging` 是两层抽象：`messaging` 负责传输，`nats.Bridge` 负责 Actor 路由。

## 注意事项

- 一个 `System` 中不能存在同名的 Actor。
- `PreStart` / `PostStop` / `Receive` 内部 panic 会被自动捕获并记录日志。
- 停止系统时会先停止接收新消息，然后排空邮箱中已有消息再退出。
- `Spawn` 会先将 Actor 注册到系统再启动 goroutine，因此 `PreStart` 中调用 `ctx.Send(ctx.Self(), ...)` 是安全的。
