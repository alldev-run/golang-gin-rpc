# github.com/alldev-run/golang-gin-rpc
 
 `github.com/alldev-run/golang-gin-rpc` 是一个基于 Go 的服务端项目骨架，围绕 `internal`（应用编排）和 `pkg`（可复用能力）组织，适合作为 HTTP / RPC 服务的起步框架。
 
 ## 功能概览（按 `pkg` 模块）

- **基础能力**
  - `pkg/config`：统一配置加载与管理
  - `pkg/logger`：结构化日志
  - `pkg/errors`、`pkg/panicx`：错误与 panic 处理
- **服务通信**
  - `pkg/rpc`：`gRPC` / `JSON-RPC` 服务管理
  - `pkg/gateway`：网关路由与转发
  - `pkg/router`、`pkg/middleware`、`pkg/cors`：HTTP 路由与中间件
  - `pkg/websocket`：WebSocket 服务
- **数据与状态**
  - `pkg/db`：数据库客户端、工厂与 `orm`
  - `pkg/cache`：缓存抽象（含 `redis` / `memcache` 适配）
  - `pkg/search`：搜索相关能力
- **治理与可观测**
  - `pkg/discovery`：服务发现（`consul`、`etcd`、`zookeeper`、`static`）
  - `pkg/health`、`pkg/metrics`、`pkg/tracing`：健康检查、指标与链路追踪
  - `pkg/loadbalancer`、`pkg/circuitbreaker`、`pkg/ratelimit`：流量治理能力
- **安全与业务支撑**
  - `pkg/auth`、`pkg/rbac`：认证与权限
  - `pkg/requestid`、`pkg/response`、`pkg/status_code`：请求与响应辅助
  - `pkg/messaging`、`pkg/audit`、`pkg/alert`：消息、审计与告警支持

说明：以上是根目录 `pkg` 的主要模块分组，完整子目录以仓库实际代码为准。
 
 ## 项目结构
 
 ```text
 github.com/alldev-run/golang-gin-rpc/
├── configs/                  # 配置文件
├── internal/
│   ├── app/                  # 应用启动与 HTTP Server 封装
│   ├── bootstrap/            # 配置加载与模块初始化
│   └── router/               # 路由注册
├── pkg/
│   ├── auth/                 # 认证
│   ├── cache/                # 缓存
│   ├── config/               # 配置
│   ├── db/                   # 数据库与 ORM
│   ├── discovery/            # 服务发现
│   ├── gateway/              # 网关
│   ├── logger/               # 日志
│   ├── messaging/            # 消息队列
│   ├── middleware/           # HTTP 中间件
│   ├── rpc/                  # RPC
│   ├── tracing/              # 链路追踪
│   ├── websocket/            # WebSocket
│   └── ...                   # 其他通用组件（见 pkg 目录）
├── main.go                   # 主入口
├── start.sh                  # 启动脚本
└── README.md                 # 项目说明
```
 
 ## 启动流程
 
 当前主程序入口是 `main.go`，整体启动顺序大致如下：

- **加载主配置**
  - 通过 `bootstrap.NewBootstrap("./configs/config.yaml")`
- **通过框架入口启动依赖与服务**
  - `bootstrap.StartFramework(ctx, options)`
  - 依赖初始化按需启用：数据库、缓存、服务发现、鉴权、链路追踪
  - 托管服务按需选择：`api-gateway` / `rpc` / `websocket`
- **启动 HTTP 应用**
  - 注册路由
  - 监听退出信号
- **执行优雅关闭**
  - `bootstrap.StopFramework(...)` 停止托管服务

### 服务组合（FrameworkOptions）

`bootstrap.FrameworkOptions` 用于描述框架启动清单。

- 依赖开关：`InitDatabases` / `InitCache` / `InitDiscovery` / `InitTracing` / `InitAuth`
- 托管服务：`Services: []string{bootstrap.ServiceRPC, bootstrap.ServiceAPIGateway, bootstrap.ServiceWebSocket}`
- WebSocket 可选注册参数：`WebSocket: &bootstrap.WebSocketServiceOptions{...}`

自定义 API Gateway（含业务路由）可通过：

- `bootstrap.RegisterAPIGatewayServiceFactory(bootstrap.APIGatewayServiceOptions{...})`
 
 ## 配置文件

项目运行时主要会读取：

- `configs/config.yaml`（主配置，由 `bootstrap.NewBootstrap` 加载）

推荐在 `configs/config.yaml` 中配置 `framework` 启动清单，例如：

```yaml
framework:
  init_databases: true
  init_cache: true
  init_discovery: true
  init_tracing: true
  init_auth: true
  init_metrics: true
  init_health: true
  init_errors: true
  validate_dependency_coverage: true
  services: ["rpc"]
```

- `init_*` 决定 bootstrap 是否初始化对应依赖
- `services` 决定托管服务启动清单（支持 `rpc` / `api-gateway` / `websocket`）
- `validate_dependency_coverage` 会校验关键依赖是否已注入 bootstrap 容器

### `http-gateway` 配置优先级

`http-gateway` 启动时会同时读取两份配置：

- 框架配置（全局基线）：`configs/config.yaml`
- 服务配置（最高优先级）：`api/http-gateway/config/config.yaml`

合并规则：

- 先用框架配置构建网关基础配置
- 再加载服务配置覆盖同名字段
- 最终以服务配置为准（未配置的字段继承框架配置）

启动参数示例：

```bash
./http-gateway ./api/http-gateway/config/config.yaml ./configs/config.yaml
```

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

#### 第三方项目一键生成 HTTP Gateway

在你自己的项目中（非本仓库）也可以直接使用脚手架：

```bash
# 1) 初始化你的项目（已有 go.mod 可跳过）
mkdir myapp && cd myapp
go mod init example.com/myapp

# 2) 引入框架（确保模板在模块缓存可见）
go get github.com/alldev-run/golang-gin-rpc@v0.0.2

# 3) 安装脚手架命令
go install github.com/alldev-run/golang-gin-rpc/cmd/scaffold@v0.0.2

# 4) 若 scaffold 未在 PATH，先临时加入
export PATH="$(go env GOPATH)/bin:$PATH"

# 5) 在你的项目根目录执行
scaffold create-api --name my-gateway --template http-gateway

# 6) 启动生成的网关
go run ./api/my-gateway
```

Windows PowerShell：

```powershell
# 1) 初始化你的项目（已有 go.mod 可跳过）
mkdir myapp
cd myapp
go mod init example.com/myapp

# 2) 引入框架（确保模板在模块缓存可见）
go get github.com/alldev-run/golang-gin-rpc@v0.0.2

# 3) 安装脚手架命令
go install github.com/alldev-run/golang-gin-rpc/cmd/scaffold@v0.0.2

# 4) 若 scaffold 未在 PATH，先临时加入
$env:Path = "$(go env GOPATH)\bin;$env:Path"

# 5) 生成网关并启动
scaffold create-api --name my-gateway --template http-gateway
go run .\api\my-gateway
```

如果你想强制使用本地模板目录：

```bash
export SCAFFOLD_TEMPLATE_DIR=/path/to/templates
scaffold create-api --name my-gateway --template http-gateway
```

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
docker run -p 8080:8080 github.com/alldev-run/golang-gin-rpc:latest
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
// 使用查询构建器（统一到 pkg/db/orm）
client := mysqlClient // 从 bootstrap 获取
rows, err := orm.NewSelectBuilder(client, "users").
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
import "github.com/alldev-run/golang-gin-rpc/pkg/logger"

logger.Info("Request processed", logger.String("path", "/api/users"))
logger.Error("Database failed", logger.Error(err))
```

## License

Apache License 2.0 - See `LICENSE` file for details.

## Author

- John James
- Email: `nbjohn999@gmail.com`
 
 ## 相关文档

### 📚 核心功能文档
- `docs/API_USAGE_GUIDE.md` - API 使用指南
- `docs/RPC_GUIDE.md` - RPC 使用指南  
- `docs/MESSAGING_GUIDE.md` - 消息队列指南
- `docs/DISCOVERY_GUIDE.md` - 服务发现指南
- `docs/TRACING_GUIDE.md` - 链路追踪指南

### 🗄️ 数据库相关
- `docs/orm-usage-guide.md` - ORM 使用指南
- `docs/ENTERPRISE_TRANSACTION_MANAGER.md` - 企业级事务管理器

### 🚀 缓存相关
- `docs/FAILOVER_CACHE_GUIDE.md` - 故障转移缓存指南
- `docs/FILE_CACHE_GUIDE.md` - 文件缓存指南

### 🌐 网关相关
- `docs/gateway.md` - 网关基础指南
- `docs/gateway-discovery.md` - 网关服务发现
- `docs/gateway-auth-complete-guide.md` - 网关认证完整指南

### 🔐 安全相关
- `docs/RPC_AUTHENTICATION.md` - RPC 认证
- `docs/JSON_RPC_TRACING.md` - JSON RPC 追踪

### 🛠️ 框架相关
- `docs/gin-router-system.md` - Gin 路由系统
- `docs/production-deployment.md` - 生产环境部署指南

### 📖 其他文档
- `docs/README.md` - 文档总览
- `pkg/discovery/README.md` - 服务发现包文档
 
 ## 总结
 
 `github.com/alldev-run/golang-gin-rpc` 提供了以 `pkg` 为核心的可复用模块集合，可用于快速搭建 Go 的 HTTP / RPC 服务。
