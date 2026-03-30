# Config Governance Guide

## Overview

`pkg/configcenter` provides enterprise configuration governance:

- Unified `Provider` abstraction (`etcd`, `consul`, `memory`)
- Read-through cache with TTL
- Namespace-level change subscription
- Runtime hot-update support

## Core Components

- `ConfigCenter`: cache + subscriptions + provider orchestration
- `Provider`: backend adapter interface
- `EtcdProvider`: etcd-backed implementation
- `ConsulProvider`: Consul KV-backed implementation
- `MemoryProvider`: local testing implementation

## Quick Start

### MemoryProvider

```go
provider := configcenter.NewMemoryProvider()
cc := configcenter.New(provider)
defer cc.Close()

ctx := context.Background()
_, _ = cc.Set(ctx, "governance", "ratelimit/user", []byte(`{"qps":100}`), nil)
value, version, err := cc.Get(ctx, "governance", "ratelimit/user")
_ = value
_ = version
_ = err
```

### Subscribe Changes

```go
sub, err := cc.Subscribe(ctx, "governance", func(change configcenter.ConfigChange) {
    // apply policy hot-reload
})
if err != nil {
    panic(err)
}
defer sub.Close()
```

### EtcdProvider

```go
etcdProvider, err := configcenter.NewEtcdProvider(clientv3.Config{
    Endpoints:   []string{"127.0.0.1:2379"},
    DialTimeout: 5 * time.Second,
}, configcenter.WithEtcdPrefix("/configcenter"))
if err != nil {
    panic(err)
}
cc := configcenter.New(etcdProvider, configcenter.WithCacheTTL(10*time.Second))
```

### ConsulProvider

```go
consulCfg := api.DefaultConfig()
consulCfg.Address = "127.0.0.1:8500"

consulProvider, err := configcenter.NewConsulProvider(
    consulCfg,
    configcenter.WithConsulPrefix("configcenter"),
    configcenter.WithConsulPollInterval(2*time.Second),
)
if err != nil {
    panic(err)
}
cc := configcenter.New(consulProvider)
```

## Boundary with Service Discovery

`pkg/configcenter` and `pkg/discovery` may both use Consul/Etcd backends, but they are **different control planes**.

| Module | Responsibility | Typical Data |
|---|---|---|
| `pkg/configcenter` | Configuration governance and dynamic policy distribution | rate limit thresholds, circuit breaker policies, feature flags, routing rules |
| `pkg/discovery` | Service registration and endpoint discovery | service instances, host:port, health state |

### Do

- Put runtime policy and business governance config in `configcenter`
- Put service endpoint lifecycle (register/deregister/find) in `discovery`
- Use `configcenter.Subscribe(...)` to hot-reload local policy cache

### Don't

- Do not store service instance lists in `configcenter`
- Do not use `discovery` as a key-value config storage

## Recommended Enterprise Usage

1. Keep all governance rules under namespace `governance/*`
2. Build app-side policy appliers (ratelimit, circuit-breaker, router) subscribed to config changes
3. Keep strict ownership: platform team manages global namespace, service team manages service namespace
4. Use short cache TTL for critical policies, longer TTL for static flags

## Notes

- `ConfigCenter.Close()` will stop subscriptions and release provider resources
- `Get/Set/Delete/Subscribe` return errors after close to prevent unsafe lifecycle usage
