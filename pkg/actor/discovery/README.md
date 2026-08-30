# Actor Discovery

`pkg/actor/discovery` 将 Actor 系统与服务发现后端（`pkg/discovery.Discovery`）桥接，
让 Actor 既能注册/注销服务实例，也能以消息形式接收发现结果。

## 特性

- **Registry**：直接调用式 API（`Register` / `Deregister` / `Lookup` / `Discover`）。
- **RegistryActor**：把发现操作封装成 Actor 消息，融入 Actor 模型。
- **消息化结果**：`Discover` 把查询结果以 `*ServiceInstances` 投递给目标 Actor。
- **超时控制**：所有后端操作默认 5 秒超时。

## 快速开始

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/alldev-run/golang-gin-rpc/pkg/actor"
    "github.com/alldev-run/golang-gin-rpc/pkg/actor/discovery"
    sd "github.com/alldev-run/golang-gin-rpc/pkg/discovery"
)

type receiver struct {
    done chan struct{}
}

func (r *receiver) Receive(ctx actor.Context, msg actor.Message) {
    if m, ok := msg.(*discovery.ServiceInstances); ok {
        log.Printf("discovered %d instances for %s", len(m.Instances), m.Service)
        close(r.done)
    }
}

func main() {
    sys := actor.New()
    defer sys.Shutdown()

    // discovery 是任意实现了 sd.Discovery 的后端（etcd/consul/...）
    var backend sd.Discovery = /* ... */

    // 1) 直接 API
    reg := discovery.NewRegistry(sys, backend)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    _ = reg.Register(ctx, &sd.ServiceInstance{ID: "1", Name: "svc", Address: "127.0.0.1", Port: 8080})

    // 2) Actor 化
    pid, err := discovery.Start(sys, backend, "registry")
    if err != nil {
        panic(err)
    }

    res := &receiver{done: make(chan struct{})}
    reply, _ := sys.SpawnNamed("receiver", res)

    _ = sys.Send(pid, &discovery.DiscoverMsg{Service: "svc", ReplyTo: reply})
    <-res.done
}
```

## API

### Registry

```go
reg := discovery.NewRegistry(sys, backend)

// 直接调用后端
err := reg.Register(ctx, instance)
err := reg.Deregister(ctx, instance)
instances, err := reg.Lookup(ctx, "svc")

// 查询并以消息形式投递给目标 Actor
err := reg.Discover(ctx, "svc", targetPID)
```

`Discover` 的返回值是后端查询本身发生的错误；消息投递失败也会返回。
即使查询失败，也会向 target 投递一条带 `Error` 字段的 `*ServiceInstances`。

### RegistryActor

| 消息类型 | 说明 | 回执 |
|---|---|---|
| `*RegisterMsg` | 注册服务实例 | `*AckMsg`（成功/失败），失败时额外投递 `*Error` |
| `*DeregisterMsg` | 注销服务实例 | `*AckMsg`（成功/失败），失败时额外投递 `*Error` |
| `*DiscoverMsg` | 查询服务，结果发往 `ReplyTo` | `*ServiceInstances`（含 `Error` 字段表示失败） |

### 便捷启动

```go
pid, err := discovery.Start(sys, backend, "registry")
```

等价于：

```go
reg := discovery.NewRegistry(sys, backend)
pid, err := sys.SpawnNamed("registry", discovery.NewRegistryActor(reg))
```

## 消息类型

| 类型 | 字段 | 说明 |
|---|---|---|
| `ServiceInstances` | `Service`, `Instances []*sd.ServiceInstance`, `Error string` | 投递给 Actor 的发现结果。 |
| `Error` | `Op`, `Err` | 结构化错误（兼容旧调用方）。 |
| `RegisterMsg` | `Instance *sd.ServiceInstance` | 注册请求。 |
| `DeregisterMsg` | `Instance *sd.ServiceInstance` | 注销请求。 |
| `DiscoverMsg` | `Service string`, `ReplyTo actor.PID` | 发现请求。 |
| `AckMsg` | `Op`, `Err` | 注册/注销操作的回执，`Err` 非空表示失败。 |

## 注意事项

- `RegistryActor` 内所有后端操作使用 5 秒超时。
- `Discover` 即使后端报错也会向 target 投递结果消息，Actor 侧应检查 `Error` 字段。
- `Register`/`Deregister` 完成后会向发送方投递 `*AckMsg`；失败时还会额外投递 `*Error` 以保持向后兼容。
