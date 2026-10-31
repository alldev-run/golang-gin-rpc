# metrics

Prometheus 指标插件。采集 HTTP / gRPC / 运行时指标，独立端口暴露 `/metrics`。

## 配置段

```yaml
plugins:
  metrics:
    enabled: true
observability:
  metrics:
    enabled: true
    endpoint: ":9090"
```

## 依赖

- `prometheus/client_golang`

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/metrics"

mc := metrics.NewMetricsCollector()
go mc.StartMetricsServer(":9090")
```
