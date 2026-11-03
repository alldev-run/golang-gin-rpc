# 链路追踪插件

基于 OpenTelemetry 提供 tracer provider、span 工具、Gin / HTTP / gRPC 中间件和上下文传播。
导出器由应用配置的全局 `TracerProvider` 决定。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/tracing`

## 配置项

| `Config` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Type` | `string` | `zipkin` | 后端类型 |
| `ServiceName` | `string` | 模块路径 | 服务名称 |
| `ServiceVersion` | `string` | `1.0.0` | 服务版本 |
| `Environment` | `string` | `development` | 部署环境 |
| `Enabled` | `bool` | `false` | 是否启用追踪 |
| `Host` / `Port` | `string` / `int` | `localhost` / `9411` | 后端地址 |
| `Endpoint` | `string` | `/api/v2/spans` | 导出 endpoint |
| `Username` / `Password` | `string` | 空 | 可选认证 |
| `SampleRate` | `float64` | `1.0` | 采样率 `[0,1]` |
| `BatchTimeout` | `time.Duration` | `5s` | 批量导出间隔 |
| `MaxExportBatchSize` | `int` | `512` | 最大批次 span 数 |
| `Options` | `map[string]interface{}` | 空 map | 后端扩展选项 |

## 快速开始

```go
provider, err := tracing.NewTracerProvider(tracing.DevelopmentConfig("orders"))
if err != nil {
	return err
}
ctx, span := provider.StartSpan(ctx, "order.lookup")
defer span.End()
_ = ctx
return provider.Shutdown(ctx)
```

生产环境可从 `ProductionConfig(serviceName)` 开始，并配置 endpoint、采样率与批次。

## API 概览

- `DefaultConfig`、`DevelopmentConfig`、`ProductionConfig`、`NewTracerProvider`
- `InitGlobalTracer`、`GlobalTracer`、`ShutdownGlobalTracer`
- `NewHTTPMiddleware`、`NewGRPCInterceptor`、`GinMiddleware`
- `InjectHeaders`、`ExtractHeaders`、`GetTraceID`、`GetSpanID`
- `StartSpan`、`SetSpanAttributes`、`SetSpanError`、`SetSpanOK`

## 错误与注意事项

provider 初始化和 shutdown 会返回错误。采样率为 `0` 表示不采样、`1` 表示全采样；
不要将凭据或个人敏感信息写入 span 属性。
Jaeger 后端使用 OTLP gRPC 接收端（默认端口 `4317`），不使用旧式 Thrift collector；
Zipkin 后端通过 Zipkin v2 HTTP endpoint 导出。

## 生产运维建议

按流量设置采样率和 exporter 超时，跨服务传播 trace context，并在退出时调用
`Shutdown` 冲刷缓冲数据。
