# health

健康检查插件。提供可注册 checker 的健康端点（`/healthz`），聚合 db / cache / 自定义探针结果。

## 配置段

```yaml
plugins:
  health:
    enabled: true
    path: /healthz
    check_interval: 30s
```

## 依赖

- 软依赖：`db`、`cache`（存在时自动注册对应 checker，缺失时降级）

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/health"

hm := health.NewHealthManager()
hm.RegisterChecker("my-service", myChecker, health.DefaultHealthCheckConfig())
http.Handle("/healthz", hm.Handler())
```
