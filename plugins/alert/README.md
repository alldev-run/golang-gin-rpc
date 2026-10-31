# alert

告警与通知插件。提供分级告警（debug/info/warning/error/critical）与多渠道下发能力，供业务在异常路径上触发通知。

## 配置段

```yaml
plugins:
  alert:
    enabled: true
```

## 依赖

- 无硬依赖
- 可选依赖：`messaging`（用于邮件/IM 通道）

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/alert"

alert.Fire(ctx, alert.Alert{
    Level:   alert.LevelError,
    Title:   "db conn lost",
    Message: "primary mysql unreachable",
})
```
