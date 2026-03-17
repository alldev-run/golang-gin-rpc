# Golang Gin RPC Framework

一个基于 Gin 框架的高性能 RPC 微服务框架，集成了数据库、缓存、日志、监控等企业级功能。

## 🚀 特性

- **高性能**: 基于 Gin 框架，支持高并发
- **数据库支持**: MySQL、PostgreSQL、Redis、MongoDB、ClickHouse、Elasticsearch
- **缓存系统**: Redis、Memcached 支持
- **ORM 框架**: 内置查询构建器 ORM
- **日志系统**: 结构化日志，支持 Zap
- **配置管理**: YAML 配置文件支持
- **优雅关闭**: 支持优雅关闭和重启
- **中间件**: 认证、限流、监控等中间件
- **连接池**: 数据库连接池管理
- **事务管理**: 分布式事务支持

## 📁 项目结构

```
alldev-gin-rpc/
├── cmd/                    # 应用程序入口
│   └── server/
│       └── main.go
├── internal/               # 内部包（不对外暴露）
│   ├── app/               # 应用程序核心
│   ├── bootstrap/         # 初始化引导
│   └── router/            # 路由管理
├── pkg/                   # 公共包（可对外暴露）
│   ├── auth/              # 认证模块
│   ├── cache/             # 缓存模块
│   ├── db/                # 数据库模块
│   │   ├── orm/           # ORM 框架
│   │   └── pool/          # 连接池
│   ├── logger/            # 日志模块
│   ├── middleware/        # 中间件
│   └── response/          # 响应格式化
├── configs/               # 配置文件
├── examples/              # 使用示例
├── docs/                  # 文档
├── scripts/               # 脚本文件
├── docker-compose.yml     # Docker Compose 配置
├── Dockerfile            # Docker 镜像配置
├── Makefile              # 构建脚本
├── go.mod                # Go 模块文件
└── README.md             # 项目说明
```

## 🛠️ 快速开始

### 1. 环境要求

- Go 1.21+
- MySQL 5.7+ / PostgreSQL 10+ / Redis 5.0+
- Docker (可选)

### 2. 安装依赖

```bash
# 下载依赖
make deps

# 或者使用 go mod
go mod download
go mod tidy
```

### 3. 配置文件

复制并修改配置文件：

```bash
cp configs/config.example.yaml configs/config.yaml
```

编辑 `configs/config.yaml`：

```yaml
server:
  host: "localhost"
  port: "8080"
  mode: "debug"

database:
  mysql_primary:
    type: "mysql"
    mysql:
      host: "localhost"
      port: 3306
      database: "myapp"
      username: "root"
      password: "your_password"

cache:
  type: "redis"
  redis:
    host: "localhost"
    port: 6379
    database: 0

logger:
  level: "info"
  env: "dev"
  log_path: "./logs/app.log"
```

### 4. 启动应用

```bash
# 使用启动脚本
./start.sh

# 或者使用 Makefile
make run

# 或者直接运行
go run main.go
```

### 5. 验证运行

```bash
# 健康检查
curl http://localhost:8080/health

# API 示例
curl http://localhost:8080/api/v1/users
```

## 📋 可用命令

### Makefile 命令

```bash
# 开发相关
make run          # 构建并运行
make dev          # 热重载开发（需要 air）
make build        # 构建应用
make test         # 运行测试
make test-coverage # 测试覆盖率

# 代码质量
make fmt          # 格式化代码
make vet          # 代码检查
make lint         # 代码 lint
make quality      # 完整质量检查

# 清理
make clean        # 清理构建产物
make clean-all    # 清理所有缓存

# Docker
make docker-build # 构建 Docker 镜像
make docker-run   # 运行 Docker 容器

# 工具安装
make install-tools # 安装开发工具
make setup        # 项目初始化
```

### 启动脚本

```bash
./start.sh          # 完整启动流程
./start.sh build    # 仅构建
./start.sh run      # 仅运行
./start.sh clean    # 清理
./start.sh deps     # 下载依赖
./start.sh help     # 帮助信息
```

## 🔧 开发指南

### 添加新的 API

1. 在 `internal/router/router.go` 中添加路由：

```go
// 在 RegisterRoutes 方法中添加
v1 := router.Group("/api/v1")
{
    v1.GET("/new-endpoint", r.newEndpoint)
    v1.POST("/new-endpoint", r.createNewEndpoint)
}

// 实现处理函数
func (r *Router) newEndpoint(c *gin.Context) {
    response.Success(c, gin.H{
        "message": "Hello from new endpoint",
    })
}
```

### 数据库操作

使用内置 ORM：

```go
// 创建 ORM 实例
ormInstance := orm.NewORMWithDB(db.DB(), nil)

// 插入数据
userID, err := ormInstance.Insert("users").
    Set("name", "John").
    Set("email", "john@example.com").
    InsertGetID(ctx)

// 查询数据
err = ormInstance.Select("users").
    Columns("id", "name", "email").
    Where("id = ?", userID).
    QueryRow(ctx).
    Scan(&user.ID, &user.Name, &user.Email)

// 更新数据
_, err = ormInstance.Update("users").
    Set("name", "John Doe").
    Where("id = ?", userID).
    Exec(ctx)

// 删除数据
_, err = ormInstance.Delete("users").
    Where("id = ?", userID).
    Exec(ctx)
```

### 缓存使用

```go
// 获取缓存
value, err := cache.Get(ctx, "key")

// 设置缓存
err = cache.Set(ctx, "key", "value", time.Hour)

// 删除缓存
err = cache.Delete(ctx, "key")
```

### 日志使用

```go
// 使用应用日志
app.Logger().Info("Processing request")
app.Logger().Error("Error occurred: %v", err)

// 使用全局日志
logger.Info("Application started")
logger.Error("Database connection failed: %v", err)
```

## 🐳 Docker 部署

### 使用 Docker Compose

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

### 单独构建镜像

```bash
# 构建镜像
make docker-build

# 运行容器
docker run -p 8080:8080 alldev-gin-rpc:latest
```

## 📊 监控和日志

### 健康检查

- `GET /health` - 应用健康状态
- `GET /api/v1/db/status` - 数据库状态
- `GET /api/v1/cache/:key` - 缓存状态

### 日志文件

- 应用日志：`./logs/app.log`
- 错误日志：`./logs/error.log`
- 访问日志：通过 Gin 中间件记录

## 🧪 测试

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包测试
go test ./pkg/db/...

# 运行测试并生成覆盖率报告
make test-coverage
```

### 测试示例

```go
func TestUserAPI(t *testing.T) {
    // 测试用户 API
    router := setupTestRouter()
    
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/users", nil)
    router.ServeHTTP(w, req)
    
    assert.Equal(t, 200, w.Code)
}
```

## 🤝 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🆘 支持

如果你有任何问题或建议，请：

1. 查看 [文档](docs/)
2. 查看 [示例](examples/)
3. 提交 [Issue](https://github.com/your-username/alldev-gin-rpc/issues)
4. 联系维护者

## 🔄 更新日志

### v1.0.0
- 初始版本发布
- 基础 RPC 框架
- 数据库和缓存支持
- 配置管理系统
- 日志和监控功能

---

**Happy Coding! 🎉**
