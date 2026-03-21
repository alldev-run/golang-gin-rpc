# HTTP Gateway

这是一个基于Gin框架的HTTP网关实现，提供了路由、代理、负载均衡、服务发现等功能。

## 功能特性

- **HTTP路由和代理**: 支持灵活的路由配置和HTTP请求代理
- **负载均衡**: 支持多种负载均衡策略（轮询、随机、加权、最少连接）
- **服务发现**: 集成服务发现机制（Consul、etcd、静态配置）
- **CORS支持**: 完整的跨域资源共享配置
- **限流**: 基于客户端IP的请求限流
- **健康检查**: 自动健康检查和服务状态监控
- **优雅关闭**: 支持优雅关闭和资源清理

## 快速开始

### 1. 配置Gateway

在配置文件中添加gateway配置：

```yaml
gateway:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "60s"

  cors:
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]
    allow_credentials: false
    max_age: 86400

  rate_limit:
    enabled: true
    requests: 100
    window: "1m"

  discovery:
    type: "static"
    endpoints: []
    namespace: "default"
    timeout: "5s"

  load_balancer:
    strategy: "round_robin"

  routes:
    - path: "/api/user/*"
      method: "*"
      service: "user-service"
      strip_prefix: true
      timeout: "30s"
      retries: 3
```

### 2. 初始化Gateway

```go
package main

import (
    "github.com/gin-gonic/gin"
    "alldev-gin-rpc/internal/bootstrap"
    "alldev-gin-rpc/pkg/gateway"
)

func main() {
    // 创建bootstrap实例
    bs, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        panic(err)
    }

    // 初始化所有组件（包括gateway）
    if err := bs.InitializeAll(); err != nil {
        panic(err)
    }

    // 获取gateway实例
    gw := bs.GetGateway()

    // 创建Gin引擎
    r := gin.New()

    // 设置gateway路由
    gw.SetupRoutes(r)

    // 启动服务器
    r.Run(":8080")
}
```

### 3. 直接使用Gateway

```go
package main

import (
    "alldev-gin-rpc/pkg/gateway"
)

func main() {
    // 创建gateway配置
    config := gateway.DefaultConfig()
    config.Port = 8080
    
    // 添加路由
    config.Routes = []gateway.RouteConfig{
        {
            Path:    "/api/user/*",
            Method:  "*",
            Service: "user-service",
            StripPrefix: true,
            Timeout: 30 * time.Second,
            Retries: 3,
        },
    }

    // 创建gateway实例
    gw := gateway.NewGateway(config)

    // 初始化和启动
    if err := gw.Initialize(); err != nil {
        panic(err)
    }
    
    if err := gw.Start(); err != nil {
        panic(err)
    }

    // 设置路由
    r := gin.New()
    gw.SetupRoutes(r)
    r.Run(":8080")
}

```

## HTTPService（开箱即用 HTTP 网关服务封装）

为了让业务层不需要理解 Gin 细节，同时保持未来可替换底层 HTTP 框架，本项目在 `pkg/gateway` 内提供了一个标准库 `net/http` 形态的服务封装：

- `NewHTTPServiceWithOptions(cfg, opt)`
- 返回一个实现 `http.Handler` 的入口：`svc.Handler()`

典型用法：将 Gateway 路由（代理/健康检查等）与业务路由（`net/http`）组合在一个端口上。

```go
bizHandler := httpapi.NewRouter(gwCfg).Handler()

svc, err := gateway.NewHTTPServiceWithOptions(gwCfg, gateway.HTTPServiceOptions{
    BizHandler:     bizHandler,
    IsBusinessPath: httpapi.IsBusinessPath,
    Middlewares:    nil,
})
if err != nil {
    panic(err)
}
defer func() { _ = svc.Close() }()

srv := &http.Server{Addr: ":8080", Handler: svc.Handler()}
_ = srv.ListenAndServe()
```

### 自定义中间件注入（net/http 形态）

`HTTPServiceOptions.Middlewares` 支持注入自定义中间件，类型为：

- `type Middleware func(http.Handler) http.Handler`

它会包裹在服务入口最外层，对业务路由与网关路由同时生效。

## 模板与脚手架

为了快速创建新的 API 项目，模板位于：

- `pkg/gateway/templates/<template>/`

### 模板目录结构

```
pkg/gateway/templates/http-gateway/
├── config/
│   └── config.yaml          # 配置文件模板
├── internal/
│   ├── httpapi/
│   │   └── router.go.gotmpl # 业务路由模板
│   └── mw/
│       ├── demo.go.gotmpl   # 自定义中间件示例
│       └── registry.go.gotmpl # 中间件注册中心
└── main.go.gotmpl          # 入口文件模板
```

### 模板 Token

| Token | 说明 |
|-------|------|
| `__MODULE__` | go.mod 中的模块名称 |
| `__API_NAME__` | API 项目名称 |
| `__API_PATH__` | API 目录路径（如 `api/my-gateway`） |

### 使用脚手架

推荐使用 `cmd/scaffold`：

```bash
# 从模板生成新项目
go run ./cmd/scaffold create-api --name my-api --template http-gateway

# 将修改后的项目同步回模板
go run ./cmd/scaffold export-template --name my-api --template http-gateway
```

更多说明请参考 `cmd/scaffold/README.md`。

## 配置说明

### Gateway配置

| 字段 | 类型 | 说明 |
|------|------|------|
| host | string | 监听地址 |
| port | int | 监听端口 |
| read_timeout | duration | 读取超时 |
| write_timeout | duration | 写入超时 |
| idle_timeout | duration | 空闲超时 |

### CORS配置

| 字段 | 类型 | 说明 |
|------|------|------|
| allowed_origins | []string | 允许的源 |
| allowed_methods | []string | 允许的方法 |
| allowed_headers | []string | 允许的头部 |
| allow_credentials | bool | 是否允许凭证 |
| max_age | int | 预检请求缓存时间 |

### 限流配置

| 字段 | 类型 | 说明 |
|------|------|------|
| enabled | bool | 是否启用限流 |
| requests | int | 请求数量限制 |
| window | string | 时间窗口 |

限流器已内置 **TTL 清理** 和 **最大 key 数限制**（默认 10 万 key），防止被随机 IP/key 攻击导致内存无限增长。

### 服务发现配置

Gateway现在集成了项目现有的discovery包，支持以下服务发现类型：

| 字段 | 类型 | 说明 |
|------|------|------|
| type | string | 发现类型（consul/etcd/static） |
| endpoints | []string | 服务端点地址 |
| namespace | string | 命名空间 |
| timeout | duration | 超时时间 |

#### 支持的服务发现类型

**静态配置（Static）**
在配置文件中直接定义服务端点，适用于开发和测试环境。

**Consul**
集成Consul服务发现，自动获取健康的服务实例。
```yaml
discovery:
  type: "consul"
  endpoints: ["localhost:8500"]
  timeout: "5s"
```

**etcd**
集成etcd服务发现，支持动态服务注册和发现。
```yaml
discovery:
  type: "etcd"
  endpoints: ["localhost:2379"]
  timeout: "5s"
```

### 负载均衡配置

| 字段 | 类型 | 说明 |
|------|------|------|
| strategy | string | 负载均衡策略 |

支持的策略：
- `round_robin`: 轮询
- `random`: 随机
- `weighted`: 加权
- `least_connections`: 最少连接

### 路由配置

| 字段 | 类型 | 说明 |
|------|------|------|
| path | string | 路由路径 |
| method | string | HTTP方法 |
| service | string | 目标服务 |
| strip_prefix | bool | 是否去除前缀 |
| timeout | duration | 超时时间 |
| retries | int | 重试次数 |
| headers | map[string]string | 附加头部 |
| query | map[string]string | 附加查询参数 |

## 负载均衡策略

### 轮询（Round Robin）
按顺序轮流选择目标服务。

### 随机（Random）
随机选择一个目标服务。

### 加权（Weighted）
根据权重选择目标服务，权重越高被选中的概率越大。

**使用示例：**

```go
import (
    "alldev-gin-rpc/pkg/gateway"
)

// 创建带权重的负载均衡器
lb := gateway.NewWeightedLoadBalancer()

// 设置带权重的目标（权重越高，被选中的概率越大）
weightedTargets := []gateway.WeightedTarget{
    {Address: "http://server-a:8080", Weight: 5},  // 50% 概率
    {Address: "http://server-b:8080", Weight: 3},  // 30% 概率
    {Address: "http://server-c:8080", Weight: 2},  // 20% 概率
}
lb.SetWeights(weightedTargets)

// 选择目标（按权重概率分布）
target, err := lb.Select(nil)
if err != nil {
    log.Printf("Failed to select target: %v", err)
    return
}

// 使用选中的目标转发请求
fmt.Printf("Selected target: %s\n", target)
```

**配置示例：**

```yaml
gateway:
  load_balancer:
    strategy: "weighted"
  routes:
    - path: "/api/*"
      service: "api-service"
      # 权重通过服务发现的 metadata 或静态配置指定
```

### 最少连接（Least Connections）
选择当前连接数最少的目标服务。

## 服务发现

Gateway集成了项目现有的discovery包，提供统一的服务发现接口。该包支持Consul、etcd和静态配置三种服务发现方式。

### 工作原理

1. **初始化**: Gateway启动时根据配置创建相应的discovery实例
2. **服务发现**: 当请求到达时，动态获取目标服务的健康实例
3. **负载均衡**: 结合负载均衡器选择最优的服务实例
4. **健康检查**: 定期刷新服务实例列表，确保只路由到健康实例

### 集成优势

- **统一接口**: 使用项目现有的discovery包，保持架构一致性
- **多后端支持**: 同时支持Consul、etcd等主流服务注册中心
- **自动故障转移**: 自动剔除不健康的服务实例
- **动态更新**: 服务实例变化时自动更新路由表

### 配置示例

```yaml
gateway:
  discovery:
    type: "consul"  # 或 "etcd", "static"
    endpoints: ["localhost:8500"]
    timeout: "5s"
    
  routes:
    - path: "/api/user/*"
      service: "user-service"  # 服务名，discovery会自动解析
      strip_prefix: true
```

## 中间件

Gateway 提供了以下中间件（均复用 `pkg` 基础包实现）：

- **CORS 中间件**: 处理跨域请求（复用 `pkg/cors`）
- **限流中间件**: 基于 IP 的请求限流（复用 `pkg/ratelimit`，内置 TTL/LRU 自我保护）
- **请求ID 中间件**: 为每个请求生成唯一 ID（复用 `pkg/requestid`）
- **日志中间件**: 记录请求日志（复用 `pkg/httplog`）
- **Recovery 中间件**: 捕获 panic（复用 `pkg/panicx`）

## 健康检查

Gateway 提供以下健康检查端点：

- `GET /health` - 基本健康检查
- `GET /ready` - 就绪检查（反映 upstream 健康状态）
- `GET /info` - Gateway 信息

### 实现细节

健康检查复用 `pkg/health` 基础包：

- 使用 `HealthManager` 管理健康检查
- 自定义 `upstreamHealthChecker` 探活 upstream 目标
- 探活结果写入 `route.healthyTargets`，proxy 优先选择健康实例
- `/ready` 在无健康 upstream 时返回 503

## 指标监控

Gateway 暴露 Prometheus 格式的指标端点：

- `GET /metrics` - Prometheus 指标

### 实现细节

指标系统复用 `pkg/metrics` 基础包：

- `gateway_http_requests_total` - HTTP 请求数（按 method, path, status）
- `gateway_http_request_duration_seconds` - HTTP 请求耗时
- `gateway_upstream_errors_total` - Upstream 错误数（按 service, type）

可通过 `pkg/metrics.Handler()` 获取指标 handler。

## 注意事项

1. 确保在关闭时正确调用 `gateway.Stop()` 方法
2. 在生产环境中建议使用Consul或etcd进行服务发现
3. 根据业务需求调整超时和重试配置
4. 监控Gateway的性能指标和日志
