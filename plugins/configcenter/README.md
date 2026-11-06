# 配置中心插件

以 `Provider` 抽象配置存储，提供带 TTL 本地缓存、版本信息和 namespace 变更订阅。
内置 Memory、Consul、etcd provider。

## 安装

`github.com/alldev-run/golang-gin-rpc/plugins/configcenter`

## 配置项

没有统一 YAML `Config`。由 provider 构造参数决定后端设置；配置中心本身支持：

| 选项 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `WithCacheTTL(ttl)` | `time.Duration` | `30s` | 本地缓存有效期 |
| `WithLogger(logger)` | `Logger` | 无操作 logger | 可选日志适配器 |
| `WithConsulPrefix(prefix)` | `string` | `configcenter` | Consul KV 前缀 |
| `WithConsulPollInterval(interval)` | `time.Duration` | `2s` | Consul 变更轮询间隔 |
| `WithEtcdPrefix(prefix)` | `string` | `/configcenter` | etcd key 前缀 |
| `NewEtcdProvider(cfg)` | `clientv3.Config` | 必填 | etcd client 配置 |
| `NewConsulProvider(cfg)` | `*api.Config` | Consul 默认配置 | Consul client 配置 |

## 快速开始

```go
provider := configcenter.NewMemoryProvider()
center := configcenter.New(provider, configcenter.WithCacheTTL(30*time.Second))
defer center.Close()
if _, err := center.Set(ctx, "orders", "limit", []byte("25"), nil); err != nil {
	return err
}
value, version, err := center.Get(ctx, "orders", "limit")
if err != nil {
	return err
}
_ = version
_ = value
return nil
```

## API 概览

- `Provider`：`Get`、`Set`、`Delete`、`Watch`、`Close`
- `New`、`Get`、`Set`、`Delete`、`Subscribe`、`Close`
- `NewMemoryProvider`、`NewConsulProvider`、`NewEtcdProvider`
- `WithCacheTTL`、`WithLogger`、`WithConsulPrefix`、`WithConsulPollInterval`、
  `WithEtcdPrefix`

## 错误与注意事项

缺失键返回 `ErrNotFound`；后端错误通过调用结果返回。`Subscribe` 返回的订阅需调用
`Close`，并为其 context 设置生命周期。缓存 TTL 可能使跨实例读取短暂滞后。

## 生产运维建议

为 Consul/etcd 配置认证、TLS、可靠超时与健康监控；限制订阅数量，避免 handler 阻塞变更
消费。敏感配置应由密钥管理系统管理，不要写入普通配置值或日志。
