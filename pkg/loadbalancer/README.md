# Load Balancer

企业级负载均衡组件，支持多种负载均衡策略，适用于 HTTP 网关、RPC 调用、数据库连接池等场景。

## 特性

- ✅ **线程安全** - 所有操作都是并发安全的
- ✅ **多种策略** - 轮询、随机、加权、最少连接
- ✅ **健康检查** - 自动过滤不健康的目标
- ✅ **动态更新** - 运行时更新目标列表
- ✅ **指标开关预留** - 提供 `WithMetrics` 配置项（当前未内置 Prometheus 指标导出）
- ✅ **优雅关闭** - 支持资源清理
- ✅ **可扩展** - 支持自定义策略

## 快速开始

### 基本用法

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "alldev-gin-rpc/pkg/loadbalancer"
)

func main() {
    // 创建工厂
    factory := loadbalancer.NewLoadBalancerFactory()
    
    // 创建加权负载均衡器
    lb, err := factory.Create(loadbalancer.StrategyWeighted)
    if err != nil {
        log.Fatal(err)
    }
    defer lb.Close()
    
    // 设置目标
    targets := []*loadbalancer.Target{
        loadbalancer.NewTarget("http://server1:8080"),
        loadbalancer.NewTarget("http://server2:8080"),
        loadbalancer.NewTarget("http://server3:8080"),
    }
    
    // 设置权重
    targets[0].SetWeight(5) // 50%
    targets[1].SetWeight(3) // 30%
    targets[2].SetWeight(2) // 20%
    
    // 更新目标
    err = lb.UpdateTargets(targets)
    if err != nil {
        log.Fatal(err)
    }
    
    // 选择目标
    for i := 0; i < 10; i++ {
        target, err := lb.Select(context.Background(), nil)
        if err != nil {
            log.Printf("Select failed: %v", err)
            continue
        }
        fmt.Printf("Selected: %s (weight: %d)\n", target.Address, target.Weight)
    }
}
```

### 配置选项

```go
// 自定义配置
opts := []loadbalancer.Option{
    loadbalancer.WithStrategy(loadbalancer.StrategyWeighted),
    loadbalancer.WithMetrics(true),
    loadbalancer.WithHealthCheck(true),
    loadbalancer.WithUpdateInterval(30 * time.Second),
    loadbalancer.WithLogger(customLogger),
}

factory := loadbalancer.NewLoadBalancerFactory(opts...)
lb, _ := factory.Create(loadbalancer.StrategyWeighted)
```

## 负载均衡策略

### 1. 轮询 (Round Robin)

按顺序轮流选择目标，确保均匀分布。

```go
lb, _ := factory.Create(loadbalancer.StrategyRoundRobin)
```

### 2. 随机 (Random)

随机选择目标，适用于目标性能相近的场景。

```go
lb, _ := factory.Create(loadbalancer.StrategyRandom)
```

### 3. 加权随机 (Weighted)

根据权重概率分布选择目标，权重越高被选中概率越大。

```go
lb, _ := factory.Create(loadbalancer.StrategyWeighted)

// 设置权重
targets[0].SetWeight(5) // 50% 概率
targets[1].SetWeight(3) // 30% 概率
targets[2].SetWeight(2) // 20% 概率
```

### 4. 最少连接 (Least Connections)

选择当前连接数最少的目标，适用于长连接场景。

```go
lb, _ := factory.Create(loadbalancer.StrategyLeastConnections)

// 释放连接
lb.(*loadbalancer.LeastConnectionsLoadBalancer).ReleaseConnection(target.Address)
```

## 健康检查

组件支持健康状态过滤，只有健康的目标会被选中：

```go
// 启用健康检查
opts := loadbalancer.WithHealthCheck(true)

// 设置目标健康状态
target.SetHealthy(false) // 标记为不健康

// 负载均衡器会自动跳过不健康的目标
```

## 目标管理

### 创建目标

```go
// 基本目标
target := loadbalancer.NewTarget("http://server:8080")

// 设置权重
target.SetWeight(5)

// 设置健康状态
target.SetHealthy(true)

// 设置元数据
target.SetMetadata("region", "us-west")
target.SetMetadata("version", "v1.2.3")
```

### 动态更新

```go
// 运行时更新目标列表
newTargets := []*loadbalancer.Target{
    loadbalancer.NewTarget("http://new-server:8080"),
}
err := lb.UpdateTargets(newTargets)
```

## 日志记录

组件提供结构化日志记录：

```go
// 自定义日志器
type MyLogger struct{}

func (l *MyLogger) Debug(msg string, fields ...loadbalancer.Field) {
    fmt.Printf("DEBUG: %s %v\n", msg, fields)
}

func (l *MyLogger) Info(msg string, fields ...loadbalancer.Field) {
    fmt.Printf("INFO: %s %v\n", msg, fields)
}

func (l *MyLogger) Warn(msg string, fields ...loadbalancer.Field) {
    fmt.Printf("WARN: %s %v\n", msg, fields)
}

func (l *MyLogger) Error(msg string, fields ...loadbalancer.Field) {
    fmt.Printf("ERROR: %s %v\n", msg, fields)
}

// 使用自定义日志器
factory := loadbalancer.NewLoadBalancerFactory(
    loadbalancer.WithLogger(&MyLogger{}),
)
```

## 错误处理

```go
target, err := lb.Select(context.Background(), nil)
if err != nil {
    switch err {
    case loadbalancer.ErrNoTargetsAvailable:
        // 没有可用目标
        log.Println("No healthy targets available")
    case loadbalancer.ErrLoadBalancerFailed:
        // 负载均衡器失败
        log.Println("Load balancer failed")
    default:
        // 其他错误
        log.Printf("Unexpected error: %v", err)
    }
}
```

## 性能基准

```
BenchmarkRoundRobin-8   	50000000	        25.3 ns/op
BenchmarkRandom-8       	30000000	        42.1 ns/op
BenchmarkWeighted-8      	20000000	        58.7 ns/op
```

## 集成示例

### HTTP 网关集成

```go
// pkg/gateway 中已集成
factory := gateway.NewLoadBalancerFactory()
lb := factory.Create("weighted")
```

### RPC 客户端集成

```go
type RPCClient struct {
    lb loadbalancer.LoadBalancer
}

func (c *RPCClient) Call(ctx context.Context, req *Request) (*Response, error) {
    target, err := c.lb.Select(ctx, nil)
    if err != nil {
        return nil, err
    }
    
    // 使用选中的目标发起 RPC 调用
    return c.callTarget(target.Address, req)
}
```

### 数据库连接池集成

```go
type DBPool struct {
    lb loadbalancer.LoadBalancer
    dbs map[string]*sql.DB
}

func (p *DBPool) GetDB() (*sql.DB, error) {
    target, err := p.lb.Select(context.Background(), nil)
    if err != nil {
        return nil, err
    }
    
    return p.dbs[target.Address], nil
}
```

## 最佳实践

1. **选择合适的策略**
   - 轮询：目标性能相近
   - 加权：目标性能差异明显
   - 最少连接：长连接场景

2. **健康检查**
   - 启用健康检查避免故障目标
   - 定期更新目标健康状态

3. **监控指标**
   - 启用指标监控观察负载分布
   - 监控目标选择频率

4. **优雅关闭**
   - 应用关闭时调用 `lb.Close()`
   - 避免资源泄漏

## API 参考

详细 API 文档请参考 [GoDoc](https://pkg.go.dev/alldev-rpc/pkg/loadbalancer)。

## 许可证

本项目采用 MIT 许可证。
