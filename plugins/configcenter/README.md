# configcenter

配置中心插件。抽象 `Provider` 接口，提供带 TTL 缓存 + 订阅变更推送的配置读取层。

## 配置段

```yaml
plugins:
  configcenter:
    enabled: true
```

## 依赖

- 无硬依赖；具体 Provider（nacos / apollo / etcd …）按需接入

## 示例

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/configcenter"

cc := configcenter.New(provider, configcenter.WithCacheTTL(30*time.Second))
val, _ := cc.Get(ctx, "namespace", "key")
cc.Watch("namespace", "key", func(newVal []byte) { /* ... */ })
```
