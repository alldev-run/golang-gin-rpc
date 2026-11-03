# 健康检查插件

提供依赖检查器注册、定期检查、聚合报告和 HTTP 状态端点。支持数据库、缓存、HTTP、
TCP 与自定义检查器。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/health`

## 配置项

| `HealthConfig` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Enabled` | `bool` | `true` | 启用服务 |
| `Host` / `Port` | `string` / `int` | `0.0.0.0` / `8081` | 服务监听地址 |
| `Path` | `string` | `/health` | 健康端点路径 |
| `Timeout` / `CheckInterval` | `time.Duration` | `5s` / `30s` | 操作超时与周期检查间隔 |
| `Detailed` | `bool` | `true` | 返回检查详情 |
| `Version` / `Build` | `string` | `1.0.0` / 空 | 响应中的构建信息 |
| `Description` | `string` | `AllDev Gin RPC Service` | 服务说明 |

| `HealthCheckConfig` 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `Name` / `Type` | `string` | `default` / `http` | 检查器名称和类型 |
| `Target` | `string` | `http://localhost:8080/health` | URL、host:port 或检查目标 |
| `Timeout` / `Interval` | `time.Duration` | `5s` / `30s` | 单次超时与检查周期 |
| `FailureThreshold` / `SuccessThreshold` | `int` | `3` / `2` | 状态翻转阈值 |
| `Enabled` / `Critical` | `bool` | `true` / `true` | 检查开关及是否影响整体状态 |
| `Options` | `map[string]interface{}` | 空 map | 检查器专属参数 |

## 快速开始

```go
manager := health.NewHealthManager()
manager.RegisterChecker(
	health.NewHTTPHealthChecker("catalog", "http://catalog/health"),
	health.HTTPCheckConfig("catalog", "http://catalog/health"),
)
manager.Start()
defer manager.Stop()
http.HandleFunc("/health", manager.HTTPHandler())
```

## API 概览

- `DefaultHealthConfig`、`DefaultHealthCheckConfig`、`HTTPCheckConfig`、`TCPCheckConfig`
- `NewHealthManager`、`RegisterChecker`、`CheckHealth`、`CheckHealthByName`、
  `HTTPHandler`、`Start`、`Stop`
- `NewDatabaseHealthChecker`、`NewCacheHealthChecker`、`NewHTTPHealthChecker`、
  `NewCustomHealthChecker`

## 错误与注意事项

失败详情位于 `CheckResult` / `HealthReport`；关键检查失败会影响整体状态。请按依赖
实际地址和认证设置检查目标。端点可用于存活或就绪探测，但两者策略应分别设计。

## 生产运维建议

避免在详细报告中暴露凭据和内部拓扑；检查间隔应大于检查超时，并监控失败率与延迟。
