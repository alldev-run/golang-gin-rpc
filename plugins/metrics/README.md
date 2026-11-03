# 指标插件

使用 Prometheus 暴露 HTTP、RPC、数据库、缓存、发现、认证、限流、审计、运行时和业务指标。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/metrics`

## 配置项

本包没有 YAML 配置结构。通过构造函数和启动参数配置：

| 参数 / 选项 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `NewMetricsCollector()` | — | Prometheus 默认 buckets | 创建 collector 与指标向量 |
| `StartMetricsServer(addr)` | `string` | 无 | 监听地址，暴露 `/metrics` |
| `StartMetricsServerWithContext(ctx, addr)` | `context.Context`, `string` | 无 | context 控制服务生命周期 |
| `NewSystemMetricsCollector(collector, interval)` | collector, `time.Duration` | 无 | 配置系统指标采样周期 |
| `NewDefaultMetricsExporter()` | — | 系统指标每 `30s` | 创建默认 exporter |

## 快速开始

```go
collector := metrics.NewMetricsCollector()
collector.RecordHTTPRequest("GET", "/orders", "200", 12*time.Millisecond)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
go func() {
	_ = collector.StartMetricsServerWithContext(ctx, "127.0.0.1:9090")
}()
```

## API 概览

- `NewMetricsCollector`、`NewSystemMetricsCollector`、`NewDefaultMetricsExporter`
- `RecordHTTPRequest`、`RecordRPCRequest`、`RecordDBQuery`、`RecordCacheOperation`
- `RecordAuthAttempt`、`RecordAuthFailure`、`RecordRateLimitHit`、`RecordAuditWrite`
- `UpdateMemoryUsage`、`UpdateGoroutineCount`、`UpdateActiveConnections`
- `StartMetricsServer`、`StartMetricsServerWithContext`、`MetricsMiddleware`

## 错误与注意事项

监听地址冲突或权限不足时，server start 返回错误。指标 label 应使用受控、低基数值；
不要直接将用户 ID、完整 URL 或任意错误文本作为 label。

## 生产运维建议

将指标端口限制在监控网络，设置 Prometheus scrape interval 与超时；监控指标基数和
进程内存，避免无界 label 导致时序爆炸。
