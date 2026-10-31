# tracing

链路追踪插件。支持 Zipkin / Jaeger / OTLP 三种后端，提供 Gin / gRPC / HTTP 中间件。

## 配置段

```yaml
plugins:
  tracing:
    enabled: true
observability:
  tracing:
    enabled: true
    type: otlp
    endpoint: "localhost:4317"
    sample_rate: 1.0
    service_name: my-service
```

## 依赖

- `go.opentelemetry.io/otel`（OTLP 后端）

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/tracing"

tracer, _ := tracing.NewTracer(tracing.DefaultConfig())
defer tracer.Shutdown(ctx)
```
