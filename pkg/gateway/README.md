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
    "golang-gin-rpc/internal/bootstrap"
    "golang-gin-rpc/pkg/gateway"
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
    "golang-gin-rpc/pkg/gateway"
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

Gateway提供了以下中间件：

- **CORS中间件**: 处理跨域请求
- **限流中间件**: 基于IP的请求限流
- **请求ID中间件**: 为每个请求生成唯一ID
- **日志中间件**: 记录请求日志

## 健康检查

Gateway提供以下健康检查端点：

- `GET /health`: 基本健康检查
- `GET /ready`: 就绪检查
- `GET /info`: Gateway信息

## 示例

参考 `examples/gateway_example.go` 查看完整的使用示例。

## 注意事项

1. 确保在关闭时正确调用 `gateway.Stop()` 方法
2. 在生产环境中建议使用Consul或etcd进行服务发现
3. 根据业务需求调整超时和重试配置
4. 监控Gateway的性能指标和日志
