# pkg/websocket

`pkg/websocket` 提供基础可用的 WebSocket client / server 封装，覆盖：

- client 建连、收发、关闭
- server 启动、关闭、路由注册
- 连接管理器与广播
- client 自动重连
- client 心跳保活
- Gin / HTTP 路由集成
- tracing
- 多节点广播与定向投递
- 节点 / 集群配置文件

它适合作为业务项目中的基础 WebSocket 能力层，用于快速接入实时消息、通知推送、轻量会话通道等场景。

## 目录结构

- `client.go`
  - WebSocket client
- `server.go`
  - WebSocket server 与服务端连接封装
- `manager.go`
  - 连接管理器与广播能力
- `integration.go`
  - Gin / HTTP 集成辅助
- `cluster.go`
  - 多节点传输抽象与集群事件
- `config.go`
  - 节点 / 集群配置结构
- `client_test.go`
- `server_test.go`
- `enhanced_test.go`

## Client

客户端支持：

- `Connect(ctx)`
- `IsConnected()`
- `Close()`
- `SendText(ctx, message)`
- `SendBinary(ctx, payload)`
- `SendJSON(ctx, payload)`
- `SendProto(ctx, message)`
- `SendMessage(ctx, payload)`
- `Receive(ctx)`
- `ReceiveJSON(ctx, dest)`
- `ReceiveProto(ctx, dest)`
- `ReceiveMessage(ctx, dest)`
- `Ping(ctx)`

### Client 配置

`Config` 支持：

- `URL`
- `Origin`
- `MessageFormat`
- `Headers`
- `HandshakeTimeout`
- `ReadTimeout`
- `WriteTimeout`
- `AutoReconnect`
- `ReconnectInterval`
- `MaxReconnectAttempts`
- `HeartbeatInterval`
- `HeartbeatMessage`
- `Tracer`

`MessageFormat` 可选值：

- `json`（默认）
- `protobuf`

当配置为 `protobuf` 时：

- `SendMessage` 要求 `payload` 为 `proto.Message`
- `ReceiveMessage` 要求 `dest` 为 `proto.Message`
- 传输使用 WebSocket binary frame

### 自动重连

当 `AutoReconnect` 打开后，client 会在连接断开后按 `ReconnectInterval` 周期尝试恢复连接。

相关字段：

- `AutoReconnect`
- `ReconnectInterval`
- `MaxReconnectAttempts`

### 心跳保活

当 `HeartbeatInterval` 大于 0 时，client 会按周期发送心跳消息。

相关字段：

- `HeartbeatInterval`
- `HeartbeatMessage`

## Server

服务端支持：

- `NewServer(config)`
- `Handle(path, handler)`
- `HandleManaged(path, manager, handler)`
- `Start()`
- `StartListener(listener)`
- `Stop(ctx)`
- `Addr()`

### ServerConfig

服务端配置支持：

- `Addr`
- `Path`
- `MessageFormat`
- `ReadTimeout`
- `WriteTimeout`
- `RequireAuth`
- `HeartbeatTimeout`
- `IdleTimeout`
- `Tracer`

### 服务端连接 `Conn`

服务端连接支持：

- `Receive(ctx)`
- `ReceiveJSON(ctx, dest)`
- `ReceiveProto(ctx, dest)`
- `ReceiveMessage(ctx, dest)`
- `SendText(ctx, message)`
- `SendBinary(ctx, payload)`
- `SendJSON(ctx, payload)`
- `SendProto(ctx, payload)`
- `SendMessage(ctx, payload)`
- `Close()`

### 消息协议选择（json / protobuf）

`Client` 与 `Server` 均支持配置：

- `message_format: "json"`
- `message_format: "protobuf"`

推荐实践：

- 业务已使用 protobuf 的场景，设置为 `protobuf` 并统一走 `SendMessage/ReceiveMessage`
- 前端浏览器轻量交互场景，保留 `json`

示例（protobuf）：

```go
client := websocket.NewClient(websocket.Config{
    URL:           "ws://localhost:8080/ws",
    Origin:        "http://localhost/",
    MessageFormat: websocket.MessageFormatProtobuf,
})

_ = client.SendMessage(ctx, &userpb.UserRequest{Id: 1})

var req userpb.UserRequest
_ = client.ReceiveMessage(ctx, &req)
```

### 错误分支判断建议

业务代码里建议优先使用 `websocket` 提供的 helper 做错误分支判断，而不是直接匹配错误字符串。

常用方法：

- `websocket.IsProtobufPayloadTypeMismatchError(err)`
- `websocket.IsProtobufDestinationTypeMismatchError(err)`
- `websocket.IsProtobufFrameTypeMismatchError(err)`

示例：

```go
if err := client.ReceiveMessage(ctx, &req); err != nil {
    switch {
    case websocket.IsProtobufFrameTypeMismatchError(err):
        // 收到非 binary frame，按协议错误处理
    case websocket.IsProtobufDestinationTypeMismatchError(err):
        // 目标类型不是 proto.Message
    default:
        // 其他错误
    }
}
```

### 服务端 handler 统一映射示例

下面示例展示在服务端 `handler` 里如何把 websocket 错误统一映射到业务错误码，并在握手阶段映射 HTTP 状态。

```go
import (
    "context"
    "net/http"

    apperrors "github.com/alldev-run/golang-gin-rpc/pkg/errors"
    "github.com/alldev-run/golang-gin-rpc/pkg/websocket"
    userpb "github.com/alldev-run/golang-gin-rpc/proto"
)

func wsHandler(ctx context.Context, conn *websocket.Conn) {
    defer conn.Close()

    var req userpb.UserRequest
    if err := conn.ReceiveMessage(ctx, &req); err != nil {
        var appErr *apperrors.AppError

        switch {
        case websocket.IsProtobufFrameTypeMismatchError(err):
            appErr = apperrors.New(apperrors.ErrCodeWebSocketProtoFrameType, "invalid websocket frame type")
        case websocket.IsProtobufPayloadTypeMismatchError(err):
            appErr = apperrors.New(apperrors.ErrCodeWebSocketProtoPayloadType, "invalid protobuf payload type")
        default:
            appErr = apperrors.Wrap(err, apperrors.ErrCodeInternalServer, "failed to receive websocket message")
        }

        // 可按业务协议把 appErr.Code 回写给客户端
        _ = conn.SendJSON(ctx, map[string]interface{}{
            "code":    appErr.Code,
            "message": appErr.Message,
        })
        return
    }
}

// 握手阶段（HTTP）错误可直接映射 HTTP 状态
func authErrorToHTTP(err error) int {
    if apperrors.IsCode(err, apperrors.ErrCodeUnauthorized) {
        return http.StatusUnauthorized
    }
    if apperrors.IsCode(err, apperrors.ErrCodeForbidden) {
        return http.StatusForbidden
    }
    return http.StatusInternalServerError
}
```

## 连接管理器与广播

`Manager` 用于维护当前在线连接，并提供广播能力。

支持：

- `NewManager()`
- `Register(conn)`
- `Unregister(conn)`
- `Count()`
- `EnableCluster(ctx, config)`
- `DisableCluster()`
- `BroadcastText(ctx, message)`
- `BroadcastBinary(ctx, payload)`
- `BroadcastJSON(ctx, payload)`
- `BroadcastToGroup(ctx, group, message)`
- `BroadcastToUser(ctx, userID, message)`
- `BroadcastToClient(ctx, clientID, message)`
- `SendToConnection(ctx, connectionID, message)`
- `CloseAll()`

### 广播示例

```go
manager := websocket.NewManager()
server := websocket.NewServer(websocket.DefaultServerConfig())

server.HandleManaged("/ws", manager, func(ctx context.Context, conn *websocket.Conn) {
    <-ctx.Done()
})

_ = manager.BroadcastText(context.Background(), "hello all")
```

## 节点与集群配置

`pkg/websocket` 现在提供独立的配置结构：

- `NodeConfig`
- `ClusterTransportConfig`
- `ClusterRuntimeConfig`
- `ConfigFile`

### 配置文件示例

仓库已新增：

- `configs/websocket.yaml`

核心字段包括：

- **节点信息**
  - `node.node_id`
  - `node.name`
  - `node.host`
  - `node.port`
  - `node.path`

- **服务端参数**
  - `server.addr`
  - `server.message_format`
  - `server.read_timeout`
  - `server.write_timeout`
  - `server.heartbeat_timeout`
  - `server.idle_timeout`
  - `server.require_auth`

- **客户端参数**
  - `client.url`
  - `client.message_format`
  - `client.auto_reconnect`
  - `client.heartbeat_interval`
  - `client.max_reconnect_interval`

- **集群参数**
  - `cluster.enabled`
  - `cluster.node_id`
  - `cluster.topic`
  - `cluster.transport.type`
  - `cluster.transport.messaging`

### 集群 transport 类型

当前配置层支持这些类型值：

- `memory`
- `messaging`
- `rabbitmq`
- `kafka`

其中：

- `memory`
  - 适合本地测试

- `rabbitmq` / `kafka`
  - 适合通过 `pkg/messaging` 接入真实多节点总线

### 代码侧启用多节点

```go
manager := websocket.NewManager()

clusterConfig := websocket.DefaultClusterConfig()
clusterConfig.NodeID = "ws-node-1"
clusterConfig.Topic = "websocket.cluster.events"
clusterConfig.Transport = websocket.NewInMemoryClusterBus()

if err := manager.EnableCluster(context.Background(), clusterConfig); err != nil {
    panic(err)
}
defer manager.DisableCluster()
```

如果你要接入真实消息系统，可以用：

```go
msgClient, err := messaging.NewClient(msgConfig)
if err != nil {
    panic(err)
}

clusterConfig.Transport = websocket.NewMessagingClusterTransport(msgClient)
```

## 链路追踪

`pkg/websocket` 已接入仓库现有 `pkg/tracing`，覆盖：

- client 握手
- client 收发消息
- client 心跳 / 重连
- server 握手
- server 收发消息
- manager 广播

可以通过以下字段注入 tracer：

- `Config.Tracer`
- `ServerConfig.Tracer`

## Gin / HTTP 路由集成

`pkg/websocket` 提供了两组集成辅助：

- `HTTPHandler`
- `ManagedHTTPHandler`
- `GinHandler`
- `ManagedGinHandler`

### Gin 集成示例

```go
engine := gin.New()
manager := websocket.NewManager()
config := websocket.DefaultServerConfig()

engine.GET("/ws", websocket.ManagedGinHandler(config, manager, func(ctx context.Context, conn *websocket.Conn) {
    defer conn.Close()
    for {
        _, payload, err := conn.Receive(ctx)
        if err != nil {
            return
        }
        _ = conn.SendText(ctx, string(payload))
    }
}))
```

### HTTP 集成示例

```go
mux := http.NewServeMux()
config := websocket.DefaultServerConfig()

mux.Handle("/ws", websocket.HTTPHandler(config, func(ctx context.Context, conn *websocket.Conn) {
    defer conn.Close()
    _, payload, err := conn.Receive(ctx)
    if err != nil {
        return
    }
    _ = conn.SendText(ctx, string(payload))
}))
```

## 基础使用示例

### 启动服务端

```go
server := websocket.NewServer(websocket.ServerConfig{
    Addr: ":8080",
    Path: "/ws",
})

server.Handle("/ws", func(ctx context.Context, conn *websocket.Conn) {
    defer conn.Close()
    for {
        _, payload, err := conn.Receive(ctx)
        if err != nil {
            return
        }
        _ = conn.SendText(ctx, string(payload))
    }
})

if err := server.Start(); err != nil {
    panic(err)
}
```

### 启动客户端

```go
client := websocket.NewClient(websocket.Config{
    URL:               "ws://localhost:8080/ws",
    Origin:            "http://localhost/",
    AutoReconnect:     true,
    ReconnectInterval: 3 * time.Second,
    HeartbeatInterval: 30 * time.Second,
    HeartbeatMessage:  "ping",
})

ctx := context.Background()
if err := client.Connect(ctx); err != nil {
    panic(err)
}

defer client.Close()

_ = client.SendText(ctx, "hello")
_, payload, _ := client.Receive(ctx)
_ = payload
```

## 适用范围

当前 `pkg/websocket` 适合：

- 基础实时通信开发
- WebSocket echo / chat / notification 场景
- 业务服务中的轻量推送通道
- Gin / HTTP 服务中的快速接入
- 多节点广播 / 定向消息分发
- 带 tracing 的实时链路观测

## 当前保留项

当前实现仍可以继续增强：

- 基于 RabbitMQ / Kafka 的生产级 cluster transport 示例
- 连接路由索引与节点感知路由
- 更强的 dead-letter / 重试处理
- 更细粒度的 metrics 暴露

## 回归

本次相关能力已通过：

```bash
go test ./pkg/websocket
```

后续修改 `pkg/websocket` 后，建议至少执行：

```bash
go test ./pkg/websocket
```

## 总结

`pkg/websocket` 现在已经具备基础的 client / server / manager / 广播 / 自动重连 / 心跳 / Gin/HTTP 集成能力，可以作为业务项目中的基础 WebSocket 能力层继续开发使用。
