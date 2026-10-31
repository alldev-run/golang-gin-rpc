# audit

审计日志插件。以异步 sink 方式采集关键操作事件，支持缓冲、多 worker 与背压丢弃策略。

## 配置段

```yaml
plugins:
  audit:
    enabled: true
    buffer_size: 1024
    workers: 1
    drop_on_full: false
```

## 依赖

- 无硬依赖

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/audit"

sink := audit.NewAsyncSink(auditor, audit.DefaultAsyncSinkConfig())
sink.Record(ctx, audit.Event{Action: "user.login", Actor: "u-123"})
```
