# plugins/websocket

`plugins/websocket` 提供基础可用的 WebSocket client / server 封装，覆盖：

- client 建连、收发、关闭
- server 启动、关闭、路由注册
- 连接管理器与广播
- client 自动重连
- client 心跳保活
- Gin / HTTP 路由集成
- tracing
- 多节点广播与定向投递
- 节点 / 集群配置文件
- 游戏协议层:envelope 信封 + action 路由 + 请求-响应关联 + 按连接有序 Seq
- 会话恢复:断线重连按 Seq 补发错过的下行消息
- 房间系统:成员索引广播、生命周期、空房回收
- 保护性控制:每连接限流、连接数上限、消息大小、origin 校验、permessage-deflate
- 发送队列背压:慢客户端不阻塞广播,队列满自动踢线

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

## 游戏协议层(envelope / 路由 / 房间 / 会话恢复)

面向实时游戏场景的完整消息层,推荐用 `Router` 替代裸 read-loop。

### 消息信封 Envelope

```json
{"seq": 42, "action": "chat.say", "req_id": "r1", "code": 0, "data": {...}}
```

- `action`:路由键,服务端 Router 按它分发
- `req_id`:请求-响应关联,服务端通过 `conn.Reply` 原样回填
- `seq`:服务端为该连接分配的单调递增序号(含广播),配合会话重放实现重连补发
- `code`/`err`:业务结果码

### action 路由

```go
router := websocket.NewRouter()
router.Handle("room.join", func(ctx context.Context, conn *websocket.Conn, env *websocket.Envelope) {
    var roomID string
    _ = json.Unmarshal(env.Data, &roomID)
    if _, err := rooms.Join(roomID, conn); err != nil {
        _ = conn.Reply(ctx, env, nil, err)
        return
    }
    _ = conn.Reply(ctx, env, map[string]string{"joined": roomID}, nil)
})
router.HandleDefault(func(ctx context.Context, conn *websocket.Conn, env *websocket.Envelope) {
    _ = conn.Reply(ctx, env, nil, fmt.Errorf("unknown action %q", env.Action))
})

server.Handle("/ws", router.Handler())
```

`Router.Handler()` 内部按连接逐条串行分发(保持消息顺序);耗时逻辑请自行开 goroutine。

### 房间系统

```go
rooms := websocket.NewRoomManager(websocket.RoomOptions{
    MaxMembers: 100,
    OnEmpty:    func(roomID string) { /* 空房回收通知 */ },
})
defer rooms.Close()

n, _ := rooms.Join("room-1", conn)           // 成员索引 O(成员数),并打上 "room:<id>" group 标签
rooms.Get("room-1").Broadcast(ctx, env)      // 每个成员独立 Seq + 进入各自会话重放缓冲
rooms.RemoveConnection(conn.Identity().ConnectionID) // 或用 rooms.Observer() 挂到 Observer 链
```

### 会话恢复(断线补发)

服务端开启后,下行消息按连接(含房间广播)记录进有界重放缓冲:

```yaml
session_resume:
  buffer_capacity: 1024   # 每会话重放缓冲条数
  ttl: 60s                # 断线后可恢复时长
  max_sessions: 0
```

客户端首次连接可带 `?session_id=<uuid>`;断线重连带 `?session_id=<同 id>&last_seq=<已收最大 seq>`,
服务端按序补发 seq > last_seq 的消息后再进入正常处理。同一 session 的旧连接会被关闭。

### 保护性控制

- `rate_limit: {rate: 50, burst: 100}`:每连接入站令牌桶,超限报错并由 read-loop 关闭连接
- `max_connections: 10000`:超限握手直接返回 503
- `origin_patterns: ["*.example.com"]`:开启跨域校验(默认沿用旧的任意 origin 行为,公网部署务必配置)
- `read_limit: 65536`:单条消息上限
- `compression: true`:permessage-deflate(无上下文接管,内存友好)

### 发送模型(背压)

每连接一个有界发送队列 + 单写 goroutine:慢客户端不会阻塞广播调用方;队列满时该连接被以
`send_queue_full` 关闭(踢慢消费者)。`Close()` 会先排空队列(有界等待)再关连接,
因此"先发最后一条状态再 Close"的语义仍然成立。写错误通过 `OnDisconnect` 与
`close_code=write_error` 暴露,不再阻塞在 `Send*` 返回值上。

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
- `MaxConnections`(连接数上限,0 不限)
- `SendQueueSize`(每连接发送队列容量,默认 256)
- `ReadLimit`(单条消息大小上限,0=32KiB,-1 不限)
- `OriginPatterns`(origin 校验模式,空=沿用任意 origin)
- `Compression`(permessage-deflate)
- `RateLimit`(每连接入站限流:`rate` 每秒消息数 + `burst` 突发容量)
- `SessionResume`(会话恢复:`buffer_capacity` 重放缓冲 / `ttl` 会话存活 / `max_sessions`)
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
    "github.com/alldev-run/golang-gin-rpc/plugins/websocket"
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

`plugins/websocket` 现在提供独立的配置结构：

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
  - 适合通过 `plugins/messaging` 接入真实多节点总线

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

`plugins/websocket` 已接入仓库现有 `plugins/tracing`，覆盖：

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

`plugins/websocket` 提供了两组集成辅助：

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

当前 `plugins/websocket` 适合：

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
go test ./plugins/websocket
```

后续修改 `plugins/websocket` 后，建议至少执行：

```bash
go test ./plugins/websocket
```

## 总结

`plugins/websocket` 现在已经具备基础的 client / server / manager / 广播 / 自动重连 / 心跳 / Gin/HTTP 集成能力，可以作为业务项目中的基础 WebSocket 能力层继续开发使用。
