# 审计日志插件

将审计事件写入日志、文件或 SQL；提供异步缓冲、多 worker、批量 SQL、字段脱敏和多 sink 组合。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/audit`

## 配置项

该包没有统一 YAML `Config`。构造参数如下：

| 结构体 / 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `AsyncSinkConfig.BufferSize` | `int` | `1024` | 异步队列容量 |
| `AsyncSinkConfig.Workers` | `int` | `1` | 写入 worker 数 |
| `AsyncSinkConfig.DropOnFull` | `bool` | `false` | 满队列时立即报错而非等待 |
| `BatchSQLSinkConfig.Table` | `string` | `audit_events` | SQL 表名（需通过标识符校验） |
| `BatchSQLSinkConfig.BatchSize` | `int` | `100` | 单批事件数 |
| `BatchSQLSinkConfig.FlushInterval` | `time.Duration` | `5s` | 定时刷新周期 |
| `BatchSQLSinkConfig.MaxRetries` | `int` | `3` | 失败重试次数 |
| `BatchSQLSinkConfig.RetryDelay` | `time.Duration` | `100ms` | 重试间隔 |
| `SQLSinkConfig.Table` | `string` | `audit_events` | SQL sink 表名 |

## 快速开始

```go
sink := audit.NewAsyncSink(audit.NewWriterFileSink(os.Stdout), audit.DefaultAsyncSinkConfig())
defer sink.Close()
err := sink.Write(ctx, audit.Event{
	Action: audit.ActionLogin, UserID: "u-123", Result: "success",
})
return err
```

## API 概览

- `Event`、`ActionRead`、`ActionCreate`、`ActionUpdate`、`ActionDelete`、`ActionLogin`
- `Sink`、`NewAsyncSink`、`NewMultiSink`、`LogSink`
- `NewFileSink`、`NewWriterFileSink`、`NewSQLSink`、`NewBatchSQLSink`
- `NewMasker`、`(*Masker).Mask`、各 sink 的 `Write` / `Close`

## 错误与注意事项

`Write` 错误应纳入业务错误处理；异步 sink 可因 context 取消、关闭或缓冲区满而失败。
调用 `Close` 等待已入队事件处理完成。文件 sink 需单独管理文件权限和轮转。

## 生产运维建议

审计记录应避免写入密码、令牌等秘密；按吞吐调节队列、批次和 worker 数，并监控丢弃、
写入错误及持久化延迟。对高价值审计事件评估持久队列或同步写入。
