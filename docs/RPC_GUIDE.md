# RPC 框架使用指南

## 🚀 RPC 框架概述

本项目提供了统一的 RPC 框架，支持 gRPC 和 JSON-RPC 两种协议，具有以下特性：

- **多协议支持**: 同时支持 gRPC 和 JSON-RPC
- **统一接口**: 通过 Manager 统一管理所有 RPC 服务
- **服务注册**: 简单的服务注册和管理机制
- **客户端支持**: 提供 gRPC 和 JSON-RPC 客户端
- **健康检查**: 内置健康检查和监控功能
- **中间件支持**: 支持请求中间件链

## 📁 项目结构

```
pkg/rpc/
├── README.md              # RPC 包说明文档
├── auth.go                # 认证相关功能
├── client.go              # RPC 客户端核心实现
├── degradation.go         # 服务降级功能
├── manager.go             # RPC 管理器
├── observer.go            # 观察者模式实现
├── server.go              # RPC 服务器核心
├── service.go             # 基础服务实现
├── grpc/                  # gRPC 相关
│   └── (gRPC 实现文件)
├── jsonrpc/               # JSON-RPC 相关
│   └── (JSON-RPC 实现文件)
├── clients/               # 客户端实现 (目录预留)
├── servers/               # 服务器实现 (目录预留)
├── logs/                  # 日志相关 (目录预留)
└── examples/              # 示例代码
    └── (示例文件)
```

### RPC 配置 (configs/config.yaml)

```yaml
rpc:
  servers:
    grpc:
      type: "grpc"
      host: "localhost"
      port: 50051
      network: "tcp"
      timeout: 30
      max_msg_size: 4194304  # 4MB
      enable_tls: false
      reflection: true
    
    jsonrpc:
      type: "jsonrpc"
      host: "localhost"
      port: 8081
      network: "tcp"
      timeout: 30
      max_msg_size: 4194304  # 4MB
      enable_tls: false
  
  timeout: 30s
  graceful_shutdown_timeout: 10s
```

## 🚀 快速开始

### 1. 创建 RPC 服务

```go
package main

import (
    "context"
    "github.com/alldev-run/golang-gin-rpc/pkg/rpc"
    "github.com/alldev-run/golang-gin-rpc/pkg/rpc/examples"
)

// 创建自定义服务
type MyService struct {
    *rpc.BaseService
}

func NewMyService() *MyService {
    return &MyService{
        BaseService: rpc.NewBaseService("my_service"),
    }
}

func (s *MyService) Register(server interface{}) error {
    s.SetMetadata("version", "1.0.0")
    return nil
}

func (s *MyService) MyMethod(ctx context.Context, req interface{}) (interface{}, error) {
    return map[string]interface{}{
        "message": "Hello from MyService",
        "service": s.Name(),
    }, nil
}
```

### 2. 启动 RPC 服务

```go
package main

import (
    "log"
    "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/rpc/examples"
)

func main() {
    // 初始化 bootstrap
    boot, err := bootstrap.NewBootstrap("./configs/config.yaml")
    if err != nil {
        log.Fatalf("Failed to initialize bootstrap: %v", err)
    }
    defer boot.Close()

    // 初始化 RPC 服务
    if err := boot.InitializeRPC(); err != nil {
        log.Fatalf("Failed to initialize RPC: %v", err)
    }

    // 注册服务
    rpcManager := boot.GetRPCManager()
    
    // 注册用户服务
    userService := examples.NewUserService()
    if err := rpcManager.RegisterService(userService); err != nil {
        log.Fatalf("Failed to register user service: %v", err)
    }

    // 注册计算器服务
    calculatorService := examples.NewCalculatorService()
    if err := rpcManager.RegisterService(calculatorService); err != nil {
        log.Fatalf("Failed to register calculator service: %v", err)
    }

    // 启动服务
    log.Println("RPC servers started:")
    log.Println("  gRPC: localhost:50051")
    log.Println("  JSON-RPC: localhost:8081")

    // 等待关闭信号
    select {}
}
```

## 📡 客户端使用

### gRPC 客户端

```go
package main

import (
    "context"
    "log"
    "github.com/alldev-run/golang-gin-rpc/pkg/rpc/grpc"
)

func main() {
    // 创建 gRPC 客户端
    config := grpc.DefaultClientConfig()
    client, err := grpc.NewClient(config)
    if err != nil {
        log.Fatalf("Failed to create gRPC client: %v", err)
    }
    defer client.Close()

    log.Printf("gRPC client connected to %s", client.Address())
    
    // 使用客户端调用服务...
}
```

### JSON-RPC 客户端

```go
package main

import (
    "context"
    "log"
    "github.com/alldev-run/golang-gin-rpc/pkg/rpc/jsonrpc"
    "github.com/alldev-run/golang-gin-rpc/pkg/rpc/examples"
)

func main() {
    // 创建 JSON-RPC 客户端
    config := jsonrpc.DefaultClientConfig()
    client := jsonrpc.NewClient(config)

    // 调用计算器服务
    addReq := &examples.AddRequest{
        Operand1: 10.5,
        Operand2: 20.3,
    }
    var addResp examples.AddResponse
    
    err := client.Call(context.Background(), "calculator.add", addReq, &addResp)
    if err != nil {
        log.Printf("JSON-RPC call failed: %v", err)
    } else {
        log.Printf("Result: %.2f", addResp.Result)
    }

    // 调用系统服务
    var pingResult interface{}
    err = client.Call(context.Background(), "system.ping", nil, &pingResult)
    if err != nil {
        log.Printf("Ping failed: %v", err)
    } else {
        log.Printf("Ping result: %v", pingResult)
    }
}
```

## 🔍 内置服务

### 系统服务 (SystemService)

提供系统级别的 RPC 方法：

- `system.health` - 健康检查
- `system.ping` - 简单 ping 测试
- `system.info` - 服务信息
- `system.listMethods` - 列出所有方法

```bash
# JSON-RPC 示例
curl -X POST http://localhost:8081/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "system.ping",
    "params": {},
    "id": 1
  }'
```

### 计算器服务 (CalculatorService)

提供基本的计算功能：

- `calculator.add` - 加法
- `calculator.subtract` - 减法
- `calculator.multiply` - 乘法
- `calculator.divide` - 除法
- `calculator.power` - 幂运算
- `calculator.sqrt` - 平方根
- `calculator.random` - 随机数
- `calculator.getHistory` - 计算历史

```bash
# JSON-RPC 示例
curl -X POST http://localhost:8081/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "calculator.add",
    "params": {"operand1": 10, "operand2": 20},
    "id": 1
  }'
```

### 用户服务 (UserService)

提供用户管理功能：

- `user.createUser` - 创建用户
- `user.getUser` - 获取用户
- `user.listUsers` - 列出用户
- `user.updateUser` - 更新用户
- `user.deleteUser` - 删除用户
- `user.searchUsers` - 搜索用户

## 🛠️ 高级功能

### 中间件支持

```go
// 创建认证中间件
authMiddleware := rpc.NewMiddleware("auth", func(ctx context.Context, req interface{}) (interface{}, error) {
    // 验证请求
    if !isValidRequest(req) {
        return nil, fmt.Errorf("invalid request")
    }
    return req, nil
})

// 创建日志中间件
loggingMiddleware := rpc.NewMiddleware("logging", func(ctx context.Context, req interface{}) (interface{}, error) {
    log.Printf("Processing request: %v", req)
    return req, nil
})

// 添加到管理器
rpcManager.AddMiddleware(authMiddleware)
rpcManager.AddMiddleware(loggingMiddleware)
```

### 健康检查

```go
// 创建健康检查器
healthChecker := rpc.NewHealthChecker(rpcManager)

// 检查健康状态
err := healthChecker.CheckHealth(context.Background())
if err != nil {
    log.Printf("RPC manager unhealthy: %v", err)
}

// 获取详细健康信息
healthInfo, err := healthChecker.GetDetailedHealth(context.Background())
if err == nil {
    log.Printf("Health info: %+v", healthInfo)
}
```

### 服务信息

```go
// 获取所有服务信息
services := rpcManager.ListServices()
log.Printf("Registered services: %v", services)

// 获取服务状态
status := rpcManager.GetStatus()
log.Printf("RPC Manager Status: %+v", status)
```

## 🧪 测试

### 运行示例

```bash
# 运行 RPC 示例
go run ./examples/rpc

# 运行完整应用
./start.sh
```

### 测试 JSON-RPC

```bash
# 测试 ping
curl -X POST http://localhost:8081/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "system.ping",
    "params": {},
    "id": 1
  }'

# 测试计算器
curl -X POST http://localhost:8081/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "calculator.add",
    "params": {"operand1": 15, "operand2": 25},
    "id": 2
  }'
```

### 测试 gRPC

使用 grpcurl 或其他 gRPC 客户端工具：

```bash
# 列出服务（需要启用 reflection）
grpcurl -plaintext localhost:50051 list

# 获取服务信息
grpcurl -plaintext localhost:50051 describe
```

## 📊 监控和日志

RPC 框架集成了完整的日志和监控功能：

- 所有 RPC 调用都会记录结构化日志
- 支持请求/响应追踪
- 内置健康检查端点
- 服务状态和指标收集

## 🔧 扩展开发

### 添加新的 RPC 服务

1. 实现 `Service` 接口：
```go
type MyService struct {
    *rpc.BaseService
}

func (s *MyService) Name() string {
    return "my_service"
}

func (s *MyService) Register(server interface{}) error {
    // 注册服务到服务器
    return nil
}
```

2. 注册到管理器：
```go
myService := NewMyService()
rpcManager.RegisterService(myService)
```

### 添加新的服务器类型

1. 在 `server.go` 中添加新的 `ServerType`
2. 实现对应的 `Server` 接口
3. 更新 `NewServer` 函数

## 🚨 注意事项

1. **端口冲突**: 确保 gRPC 和 JSON-RPC 使用不同端口
2. **TLS 配置**: 生产环境建议启用 TLS
3. **超时设置**: 根据业务需求调整超时时间
4. **中间件顺序**: 中间件按添加顺序执行
5. **服务注册**: 必须在管理器启动前注册服务

## 📚 更多示例

查看 `examples/` 目录下的完整示例：

- `examples/rpc/main.go` - RPC 框架使用示例
- `pkg/rpc/examples/user_service.go` - 用户管理服务示例
- `pkg/rpc/examples/calculator_service.go` - 计算器服务示例

---

**Happy RPC Coding! 🎉**
