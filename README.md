# alldev-gin-rpc
 
 `alldev-gin-rpc` 是一个基于 Go 的服务端项目骨架，集成了 HTTP、gRPC / JSON-RPC、数据库、缓存、服务发现、日志、链路追踪、消息队列和网关能力，适合作为微服务或 RPC 服务的基础框架。
 
 ## 功能概览
 
 - **HTTP 服务**
   - 基于 Gin 的 Web 应用与路由注册
 - **RPC 服务**
   - 支持 `gRPC` 和 `JSON-RPC`
 - **数据库支持**
   - MySQL、PostgreSQL、MongoDB、ClickHouse、Elasticsearch
 - **缓存支持**
   - Redis
 - **服务发现**
   - 支持 `consul`、`etcd`、`zookeeper`、`static`
 - **日志系统**
   - 基于 Zap 的结构化日志
   - JSON 输出不会写入 ANSI 颜色码
 - **链路追踪**
   - 支持 Zipkin、Jaeger、OTLP
 - **WebSocket 多节点**
   - 集群事件同步，支持 RabbitMQ / Kafka 消息总线
 - **消息队列**
   - RabbitMQ、Kafka
 - **网关能力**
   - 企业级多协议统一网关（HTTP/HTTPS、gRPC、JSON-RPC）
   - 分布式追踪、负载均衡、服务发现
   - 路由转发、健康检查、优雅关闭
 - **优雅关闭**
   - 应用退出时统一执行资源回收与 tracer shutdown
 
 ## 项目结构
 
 ```text
 alldev-gin-rpc/
 ├── configs/                  # 配置文件
 ├── internal/
 │   ├── app/                  # 应用启动与 HTTP Server 封装
 │   ├── bootstrap/            # 配置加载与模块初始化
 │   └── router/               # 路由注册
 ├── pkg/
 │   ├── auth/                 # 认证
 │   ├── cache/                # 缓存
 │   ├── config/               # 全局配置加载
 │   ├── db/                   # 数据库与客户端工厂
 │   ├── discovery/            # 服务发现
 │   ├── gateway/              # 网关
 │   ├── health/               # 健康检查
 │   ├── logger/               # 结构化日志
 │   ├── messaging/            # 消息队列
 │   ├── metrics/              # 指标
 │   ├── rpc/                  # RPC 服务管理
 │   └── tracing/              # 链路追踪
 ├── main.go                   # 主入口
 ├── start.sh                  # 启动脚本
 └── README.md                 # 项目说明
 ```
 
 ## 启动流程
 
 当前主程序入口是 `main.go`，整体启动顺序大致如下：
 
 - **初始化 tracing**
   - 读取 `configs/tracing.yaml`
 - **加载主配置**
   - 通过 `bootstrap.NewBootstrap("./configs/config.yaml")`
 - **初始化核心依赖**
   - 数据库
   - Redis 缓存
   - RPC 服务
   - 服务发现
 - **启动 HTTP 应用**
   - 注册路由
   - 监听退出信号
 - **执行优雅关闭**
 
 ## 配置文件
 
 项目运行时主要会读取：

- `configs/config.yaml`（主配置，由 `bootstrap.NewBootstrap` 加载）
- `configs/tracing.yaml`（启动前由 `tracing.InitFromFile` 加载）

仓库中也提供了其他示例配置文件（例如 `configs/discovery.yaml`），用于模块化参考。
 
 ## `configs/config.yaml` 主要模块
 
 当前主配置覆盖以下能力：
 
 - **server**
   - HTTP / gRPC 服务监听地址与超时
 - **database**
   - 主从数据库与连接池
 - **redis**
   - Redis 连接配置
 - **rpc**
   - gRPC / JSON-RPC 服务配置
 - **discovery**
   - 服务发现注册信息
 - **messaging**
   - RabbitMQ / Kafka
 - **observability**
   - logging / tracing / metrics
 - **security**
   - JWT 等安全配置
 
 ## 快速开始
 
 ### 环境要求
 
 - **Go**
   - 建议使用当前 `go.mod` 声明的版本或兼容版本
 - **Redis**
   - 若启用缓存
 - **MySQL / PostgreSQL / MongoDB / ClickHouse / Elasticsearch**
   - 按需启用
 - **Consul / Etcd / Zookeeper**
   - 若启用服务发现
 - **Zipkin / Jaeger / OTLP Collector**
   - 若启用 tracing
 
 ### 安装依赖
 
 ```bash
 go mod download
 go mod tidy
 ```
 
 ### 启动应用
 
 方式一：直接运行
 
 ```bash
 go run .
 ```
 
 方式二：使用启动脚本
 
 ```bash
 ./start.sh
 ```
 
 方式三：分步骤执行
 
 ```bash
 ./start.sh deps
 ./start.sh build
 ./start.sh run
 ```
 
 ## 启动脚本命令
 
 `start.sh` 当前支持：
 
 ```bash
 ./start.sh
 ./start.sh build
 ./start.sh run
 ./start.sh clean
 ./start.sh deps
 ./start.sh help
 ```
 
 ## Makefile 命令

项目同时提供 Makefile 支持：

```bash
make run           # 构建并运行
make build         # 仅构建应用
make test          # 运行测试
make test-coverage # 测试覆盖率
make fmt           # 格式化代码
make vet           # 代码检查
make lint          # 代码 lint
make docker-build  # 构建 Docker 镜像
```

### API 项目脚手架（基于 Gateway 模板）

模板存放在：

- `pkg/gateway/templates/`

通过脚手架命令可以：

- 从模板生成一个新的 API 项目目录：`api/<name>`
- 将你修改后的 `api/<name>` 反向导出回模板（用于维护模板演进）

Makefile：

```bash
make create-api NAME=<new-api> [TEMPLATE=http-gateway]
make export-template NAME=<api-name> [TEMPLATE=http-gateway]
```

Windows PowerShell（不依赖 make，推荐）：

```powershell
# 生成 api/demo-api
go run .\cmd\scaffold create-api --name demo-api --template http-gateway

# 启动生成的项目
go run .\api\demo-api

# 将 api/demo-api 导出回模板（写入 pkg/gateway/templates/http-gateway，Go 文件会变成 .gotmpl）
go run .\cmd\scaffold export-template --name demo-api --template http-gateway
```

使用 Docker Compose 启动依赖服务：

```bash
docker-compose up -d    # 启动 MySQL、Redis、RabbitMQ 等
docker-compose logs -f  # 查看日志
docker-compose down     # 停止服务
```

## 服务发现
 
 项目内的服务发现模块位于：
 
 - `pkg/discovery`
 
 当前支持的后端：
 
 - `consul`
 - `etcd`
 - `zookeeper`
 - `static`
 
 ### Zookeeper 支持
 
 当前 `zookeeper` discovery 已实现：
 
 - **服务注册**
   - 使用 `ephemeral` 节点保存实例
 - **服务注销**
   - 删除实例节点
 - **服务查询**
   - 读取服务路径下的实例列表
 - **自定义根路径**
   - 使用 `Options["base_path"]`
 
 示例：
 
 ```go
 cfg := discovery.Config{
     Type:    discovery.RegistryTypeZk,
     Address: "127.0.0.1:2181",
     Timeout: 5 * time.Second,
     Options: map[string]interface{}{
         "base_path": "/services",
     },
 }
 
 d, err := discovery.NewDiscovery(cfg)
 if err != nil {
     panic(err)
 }
 _ = d
 ```
 
 更详细的说明见：
 
 - `pkg/discovery/README.md`
 
 ## RPC 能力
 
 `bootstrap.InitializeRPC()` 当前会根据配置选择：
 
 - **gRPC**
 - **JSON-RPC**
 
 并支持：
 
 - 服务管理器统一启动与关闭
 - 降级管理
 - 与服务发现集成注册
 
 ## 日志
 
 日志模块位于：
 
 - `pkg/logger`
 
 当前特性：
 
 - **结构化日志**
 - **支持 stdout / stderr / file 输出**
 - **支持 JSON / console 两种格式**
 - **JSON 日志中的 `level` 字段不包含 ANSI 颜色码**
 
 常见配置项包括：
 
 - `level`
 - `output`
 - `format`
 - `log_path`
 - `enable_console_colors`
 - `enable_caller`
 - `enable_stacktrace`
 
 ## Tracing
 
 tracing 模块位于：
 
 - `pkg/tracing`
 
 当前支持：
 
 - `zipkin`
 - `jaeger`
 - `otlp`
 
 启动时会优先读取：
 
 - `configs/tracing.yaml`
 
 若 tracing 未启用，应用会继续运行。
 
 ## 健康检查与网关
 
 项目中已包含：
 
 - `pkg/health`
 - `pkg/gateway`
 
 可用于：
 
 - 服务健康状态管理
 - API 路由代理
 - 基于服务发现的转发
 - 负载均衡策略集成
 
 ## 常用测试命令
 
 针对不同模块可以执行：
 
 ```bash
go test ./pkg/...
go test ./pkg/discovery/...
go test ./pkg/logger/...
go test ./pkg/tracing/...
go test ./pkg/rpc/...
```

说明：`go test ./...` 会包含 `examples` 目录；若本地示例依赖未满足或存在示例编译约束，建议优先执行上面的 `pkg` 级回归命令。
 
 ## 开发建议

- **优先修改 `configs/config.yaml`**
  - 统一通过 bootstrap 加载配置
- **新增服务发现后端时**
  - 同步更新 `pkg/discovery` 与根 README
- **新增模块时**
  - 尽量接入 bootstrap 统一初始化流程
- **输出结构化日志时**
  - 推荐使用 `pkg/logger`

## Docker 部署

### 构建并运行

```bash
# 构建 Docker 镜像
make docker-build

# 运行容器
docker run -p 8080:8080 alldev-gin-rpc:latest
```

### 使用 Docker Compose

项目提供 `docker-compose.yml`，可一键启动所有依赖服务：

```bash
# 启动所有服务（MySQL、Redis、RabbitMQ 等）
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

## 开发指南

### 添加新的 API

在 `internal/router/router.go` 中添加路由：

```go
v1 := router.Group("/api/v1")
{
    v1.GET("/users", r.getUsers)
    v1.POST("/users", r.createUser)
}
```

### 数据库操作

```go
// 使用查询构建器
client := mysqlClient // 从 bootstrap 获取
rows, err := client.NewSelectBuilder("users").
    Where("status = ?", "active").
    Limit(10).
    Query(ctx)
```

### 缓存使用

```go
cache := boot.GetCache()
err = cache.Set(ctx, "key", "value", time.Hour)
```

### 日志使用

```go
import "alldev-gin-rpc/pkg/logger"

logger.Info("Request processed", logger.String("path", "/api/users"))
logger.Error("Database failed", logger.Error(err))
```
 
 ## 相关文档
 
 - `pkg/discovery/README.md`
 
 ## 总结
 
 `alldev-gin-rpc` 现在已经不只是一个数据库示例项目，而是一个包含 HTTP、RPC、缓存、日志、tracing、服务发现和网关能力的 Go 服务端基础框架。
