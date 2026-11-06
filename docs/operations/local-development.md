# 本地开发与启动指南

本文档说明本地环境下的启动方式、配置加载与兜底逻辑、热重载以及 Docker 本地运行。

## 目录

1. [启动方式](#启动方式)
2. [配置加载与兜底](#配置加载与兜底)
3. [热重载](#热重载)
4. [Docker 本地运行](#docker-本地运行)
5. [常用 make 目标速查](#常用-make-目标速查)

## 启动方式

### 方式一：make run（推荐）

构建根模块与全部插件模块，然后启动：

```bash
make run
```

### 方式二：启动脚本

```bash
./start.sh              # 完整流程：检查 Go → 兜底配置 → 建目录 → 下载依赖 → 构建 → 运行
./start.sh deps         # 仅下载依赖
./start.sh build        # 仅构建
./start.sh run          # 仅运行（二进制缺失时自动构建）
./start.sh clean        # 清理 bin/ logs/ temp/ 与构建缓存
```

### 方式三：手动构建运行

```bash
go build -o bin/alldev-gin-rpc .
mkdir -p logs temp
./bin/alldev-gin-rpc
```

启动后验证：

```bash
curl http://localhost:8080/health   # 期望 200
```

## 配置加载与兜底

- 默认配置路径为 `./configs/config.yaml`（相对于工作目录）。
- 可用**第一个命令行参数**覆盖配置路径（Docker 的 `CMD` 即利用此机制）：

  ```bash
  ./bin/alldev-gin-rpc ./configs/config.yaml
  ```

- `make setup` 与 `./start.sh` 在 `configs/config.yaml` 缺失时，会从
  [`configs/config.example.yaml`](../../configs/config.example.yaml) 复制生成默认配置；
  模板同样缺失时明确报错退出，不会生成旧结构配置。
- 注意：配置从**空结构体**反序列化（非 `DefaultConfig()`），yaml 中缺失的段即零值。
  请以 `config.example.yaml` 为基线修改，不要从空白文件手写。

## 热重载

```bash
make dev        # 需要 air，未安装时执行 make install-tools 或回退 make run
```

行为由仓库根的 `.air.toml` 控制：

- 构建命令：`go build -o ./temp/alldev-gin-rpc .`（仅根模块；`pkg/`、`plugins/` 经
  go.work + replace 参与根构建，改动会触发重建）
- 监听扩展名：`.go`、`.yaml`、`.yml`；修改 `configs/` 下的配置也会重启
- 排除目录：`bin/`、`temp/`、`logs/`、`examples/`、`api/`、`docs/` 等噪声目录
  （`api/` 下脚手架应用是独立模块，根构建不依赖）
- 产物写入 `temp/`，`make clean` 与 `./start.sh clean` 均会清理

## Docker 本地运行

仓库根的 [Dockerfile](../../Dockerfile) 为多阶段构建，入口二进制不含配置文件，
需挂载配置；`CMD` 为配置文件路径参数。

```bash
make docker-build    # 等价于 docker build -t alldev-gin-rpc:latest .
make docker-run      # 挂载 ./configs 只读；存在 .env 时自动附加 --env-file
```

等价的手动命令：

```bash
docker run --rm -p 8080:8080 \
  -v "$(pwd)/configs:/app/configs:ro" \
  alldev-gin-rpc:latest

# 覆盖配置路径
docker run --rm -p 8080:8080 \
  -v "$(pwd)/my-configs:/app/my-configs:ro" \
  alldev-gin-rpc:latest /app/my-configs/config.yaml
```

注意：

- `.env` 已加入 `.gitignore`；`make docker-run` 会在其存在时附加 `--env-file .env`。
  当前配置框架尚未启用环境变量覆盖（`pkg/config` 无 `env` 标签字段），数据库密码等
  仍需写入挂载的配置文件，`.env` 仅作预留。
- 容器内以 `appuser`（uid 10001）运行，配置挂载为只读即可。

## 本地测试配置（连接本地 MySQL/Redis）

仓库不含本地服务配置（含密码，已 gitignore）。按需创建 `configs/config.local.yaml` 后用配置路径参数启动：

```bash
./bin/alldev-gin-rpc ./configs/config.local.yaml
```

基于 `config.example.yaml` 修改时，注意框架实际读取的段与示例文件中的历史写法不同：

| 段 | 框架实际读取 | 说明 |
|---|---|---|
| 数据库 | `database.primary`（扁平结构：`enabled/driver/host/port/database/username/password`） | 示例文件中的 `database.mysql_primary` 嵌套格式（`type/mysql/{...}`）是服务级格式，根应用不消费 |
| Redis | 顶层 `redis:` 段（`mode/host/port/password/database/pool_size`） | 示例文件中的 `cache:` 段当前无消费者 |

配置好 `plugins.db.enabled: true`、`plugins.cache.enabled: true` 与本地 MySQL/Redis 连接信息后启动，验证日志：

```text
"Database connected successfully", "name": "primary"
"Redis cache initialized"
```

**端口分配**（从 8080 顺延，`internal/bootstrap.resolveGatewayPort` 保证 Gateway 不与主应用抢占端口）：

| 端口 | 组件 | 配置来源 |
|---|---|---|
| 8080 | 主应用 HTTP（业务路由、`/health`） | `server.http.port`（兼容旧扁平 `server.port` 写法） |
| 8081 | JSON-RPC | `rpc.servers.jsonrpc.port` |
| 8082 | Gateway | `gateway.port`；未配置时自动取主应用端口 +2 |
| 50051 | gRPC | `rpc.servers.grpc.port` |
| 9090 | metrics 独立监听 | `observability.metrics.listen_addr` |

其他原硬编码项已配置化（见 `config.example.yaml` 注释）：

- `server.shutdown_timeout`：优雅停机最长等待（默认 30s，作用于 main/bootstrap/gateway 停机）
- `rpc.servers.*.timeout` / `max_msg_size` / `reflection`：RPC 服务器参数（默认 30s / 4MB / 按配置）

历史上 `gateway.port` 未被消费、Gateway 硬编码复用主应用端口导致绑定冲突（主应用 API 全部 503），
现已修复：`framework.services` 含 `api-gateway` 时 Gateway 监听 8082，主应用保留 8080。

## 常用 make 目标速查

| 目标 | 说明 |
|---|---|
| `make run` | 构建并运行（含全部插件模块） |
| `make dev` | air 热重载 |
| `make setup` | 初始化目录、兜底配置与依赖 |
| `make deps` | `go mod download` + `go mod tidy` |
| `make build` / `make build-debug` | 构建 / 带调试符号构建（配合 dlv） |
| `make test` / `make test-coverage` | 测试 / 覆盖率报告 |
| `make lint` / `make vet` | golangci-lint / go vet |
| `make clean` | 清理 `bin/ logs/ temp/` 与覆盖率产物 |
| `make docker-build` / `make docker-run` | 构建镜像 / 本地运行容器 |
| `make help` | 查看全部目标 |

---

相关文档：[生产部署](production-deployment.md) · [配置治理](../architecture/config-governance.md)
