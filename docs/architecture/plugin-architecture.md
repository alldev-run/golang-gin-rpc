# 插件化架构（Plugin Architecture）

本框架采用**编译期注册 + 配置驱动**的插件机制（`pkg/kernel`），将功能组件与框架内核解耦。

## 核心理念："不用的就不加载"

加载裁剪发生在两个层面：

| 层面 | 机制 | 效果 |
|---|---|---|
| **链接期** | 应用只 `import` 需要的插件包 | 未 import 的插件及其重型 SDK 根本不进二进制 |
| **运行期** | 对内置适配器配置 `plugins.<name>.enabled: false` | 已注册但被禁用的内置组件跳过 Configure/Start/Stop |

**内核插件边界：**只有 `internal/bootstrap/plugins_builtin.go` 中列出的 10 个内置组件
被包装为 `kernel.Plugin`，并参与内核注册、排序及启停配置。其他 `plugins/<name>` 模块是
可独立导入的 Go 库；除非应用或组合根显式注册、装配，否则不会因为存在
`plugins.<name>.enabled` 配置项而自动加载或启停。

## 目录布局

```text
pkg/                  核心内核（始终链接）
├── kernel/           插件契约 + 注册表 + 生命周期加载器
├── config/ logger/ errors/ status_code/ response/
└── panicx/ requestid/ cors/ utils/ version/ bootstrap/

plugins/<name>/       功能插件，每个子目录一个独立 Go 模块
                      （自带 go.mod；搬去独立仓库时只需改模块名）
examples/             独立 Go 模块（examples/go.mod）：示例代码不进入
                      框架主模块的 require 集，go.work 下照常 go run
```

当前 27 个插件模块：`actor` `alert` `audit` `auth` `bloomfilter` `cache`
`circuitbreaker` `configcenter` `db` `discovery` `gameserver` `gateway`
`geoip` `health` `loadbalancer` `messaging` `metrics` `middleware` `push`
`ratelimit` `rbac` `rpc` `scheduler` `search` `tracing` `upload` `websocket`

> 特殊布局：`plugins/gateway` 内含 `middleware/gin`、`middleware/nethttp`、
> `router`、`httplog`（网关家族）；`plugins/ratelimit` 内含 `ratelimiter`
> 子包；`plugins/gameserver` 内含 `aoi`（九宫格空间索引）；
> `plugins/db` 的重型驱动（clickhouse/mongodb）与
> `plugins/search/elasticsearch` 通过**驱动注册表**自注册；redis/memcache
> 驱动同样自注册，但位于 `plugins/cache` 的对应子包（`plugins/db` 不依赖
> `plugins/cache`，orm 查询缓存的 redis/memcache/failover 后端在
> `plugins/cache/ormcache`）。

## 模块依赖边界

框架层与插件模块的依赖方向经过专项解耦（2026-11 审计，CI 的依赖边界以代码为准）：

- **core 模块独立**：`pkg/` 是独立 Go 模块 `github.com/alldev-run/golang-gin-rpc/pkg`
  （kernel/config/logger/errors/...），不 import 任何 `plugins/*` 或 `internal/*`，
  CI 的 `core module (pkg)` job 用 `go list -deps` 强制检查。
- **插件只依赖 core 模块**：插件 go.mod require `.../pkg`（本地以 `replace => ../../pkg`
  指向），不再 require 根模块；根模块与插件模块之间不存在双向依赖，插件可连同
  `pkg` 模块单独拆出。
- **bootstrap 是全量组合根**：根模块的 `bootstrap` 包（原 `pkg/bootstrap`）经
  `internal/bootstrap` 链接全部内置插件，并非零依赖 facade。插件对象的类型化访问用
  `bootstrap.Dependency[T](boot, key)`，依赖键使用导出常量（如 `bootstrap.DepDBFactory`、
  `bootstrap.DepCache`、`bootstrap.DepActorSystem`），值与历史字符串键一致。
- **内置插件配置不在 core**：redis、rpc、discovery、messaging、security 配置段由
  `bootstrap/pluginconfig.Config` 定义（`boot.GetPluginConfig()` 读取），YAML key 不变；
  `pkg/config.GlobalConfig` 只保留 app/framework/server/database/logger/observability。
- **每个 Bootstrap 独占注册表**：内置插件注册到 `kernel.NewRegistry()` 创建的实例注册表，
  经 `kernel.WithRegistry` 交给 Loader；空导入自注册的插件仍在默认注册表中，Loader 合并两者
  （同名时实例注册表优先）。
- **plugins/db 不依赖 plugins/cache**：redis/memcache 驱动由 `plugins/cache`
  的 `redis/register.go`、`memcache/register.go` 自注册（import 即注册，
  `db.Config.Redis`/`Memcache` 以 any 携带配置）；依赖方向为 cache -> db。
  通过 cache 间接依赖 db 的插件（如 gateway、health、rpc）需要在 go.mod 中保留
  `plugins/db` 的 replace，`GOWORK=off go mod tidy` 才能解析。
- **examples/ 独立模块**：主模块 require 集只保留框架实际引用的插件；
  被插件子树传递需要的模块以 replace（无 require）形式存在。
- **插件间无循环依赖**；插件 -> 框架仅使用 `pkg/{logger,config,cors,panicx,
  requestid,response,errors}` 等 core 包。

两条组装路径（链接期裁剪的具体落点）：

1. **全量便捷入口**：`bootstrap`（根模块）——经组合根装配 10 个内置插件与托管服务，
   二进制包含全部内置依赖；
2. **最小组装**：直接用 `pkg/kernel`（依赖闭包不含任何插件），应用按需 import
   并 `kernel.Register` 插件模块——未引入的插件及其重型 SDK 完全不进二进制。

## 插件成熟度分级

每个插件按 API 稳定性、测试覆盖、生产验证程度划分为三个等级，便于使用者评估接入风险。
本次审计只保留原已列为 GA 的插件；GA 门槛为至少一个可核实的生产案例、模块 statement
coverage ≥50%、README 完整且无占位实现。生产案例未核实前不将 Beta 插件升级为 GA。

覆盖率门槛由 `scripts/coverage_gate.sh` 强制执行（CI 的 plugins matrix 中每个
GA/GA-ready 插件必须 ≥50%，否则构建失败；本地可用 `make coverage-gate PLUGIN=<name>`
或 `bash scripts/coverage_gate.sh report --all` 复核）。脚本中的 `GATED_PLUGINS`
名单需与下方分级表保持同步。

| 等级 | 含义 | 承诺 |
|---|---|---|
| **GA** | 满足审计门槛的稳定组件 | 至少一个可核实的生产案例、statement coverage ≥50%、README 完整、无占位实现 |
| **Beta** | 功能完整但尚未广泛验证 | 接口基本稳定，允许小幅调整；测试覆盖中等 |
| **Experimental** | 实验阶段或接口尚未稳定 | 允许破坏性变更，部分能力可能尚未验证 |

### 当前分级

GA 仅保留审计前已列为 GA、且达到覆盖率门槛、README 完整并且无占位实现的插件。
没有可核实生产案例的插件不升级为 GA；`search` 保持 Experimental。

| 插件 | 等级 | statement coverage |
|---|---|---:|
| actor | Beta | 83.0% |
| alert | Beta | 88.2% |
| audit | Beta | 78.8% |
| auth | GA | 71.0% |
| bloomfilter | Beta | 97.4% |
| cache | GA | 51.3% |
| circuitbreaker | Beta | 47.2% |
| configcenter | Beta | 37.4% |
| db | Beta | 65.5% |
| discovery | Beta | 86.5% |
| gameserver | Beta | 91.0% |
| gateway | GA | 57.2% |
| geoip | Beta | 29.6% |
| health | GA | 65.7% |
| loadbalancer | Beta | 73.9% |
| messaging | Beta | 34.3% |
| metrics | GA | 89.4% |
| middleware | Beta | 37.0% |
| push | Beta | 33.1% |
| ratelimit | Beta | 36.6% |
| rbac | Beta | 89.5% |
| rpc | GA | 56.0% |
| scheduler | Beta | 90.8% |
| search | Experimental | 25.6% |
| tracing | GA | 50.9% |
| upload | Beta | 93.1% |
| websocket | GA | 66.9% |

### GA 就绪插件（待生产案例核实）

以下 Beta 插件已满足 GA 的技术门槛（statement coverage ≥50%、README 完整、无占位实现），
**仅需核实至少一个可核实的生产案例**即可正式升级为 GA：

| 插件 | 覆盖率 | 备注 |
|---|---:|---|
| actor | 83.0% | 核心内置组件，本轮补齐 nats.PublishAsync/NewBridgeFromConfig、Props.WithSupervision、discovery.Start |
| alert | 88.2% | 通知插件 |
| audit | 78.8% | 审计日志 |
| bloomfilter | 97.4% | 布隆过滤器 |
| db | 65.5% | 核心内置组件，本轮从 33.5% 提升；补齐 orm 事务管理器、postgres/mongodb 客户端与根包适配器测试，修复 poolcb SQLClient 代理缺陷 |
| discovery | 86.5% | 核心内置组件，本轮从 40.5% 提升；zookeeper 后端引入 zkConn 注入缝并全覆盖（zk 包 92.4%） |
| gameserver | 91.0% | 游戏服务器 |
| loadbalancer | 73.9% | 负载均衡 |
| rbac | 89.5% | 角色权限控制 |
| scheduler | 90.8% | 任务调度 |
| upload | 93.1% | 核心内置组件，本轮从 31.7% 提升 |

## Plugin 契约（pkg/kernel）

```go
type Plugin interface {
    Name() string                      // 唯一名，同时是 plugins.<name> 配置键
    Depends() []string                 // 硬依赖：缺失或被禁用 → 构建失败
    Configure(ctx *Context) error      // 解析自身配置段；返回 ErrDisabled 主动退出
    Start(ctx *Context) error          // 按依赖序启动（建连接、起后台任务、注册服务）
    Stop(ctx *Context) error           // 依赖逆序停止
}

// 可选接口：软依赖
type SoftDepender interface {
    SoftDepends() []string             // 目标存在则保证顺序；被禁用/未注册则降级
}
```

`*kernel.Context` 内嵌 `context.Context`（生命周期取消信号），同时是插件间的
**服务定位器**：上游插件 `SetService("cache", ...)`，下游 `GetService("cache")`。

## 插件如何读取自己的配置

`pkg/config.Loader` 保留原始 yaml 文档并实现 `kernel.ConfigSource`：

```go
func (p *myPlugin) Configure(ctx *kernel.Context) error {
    var cfg struct {
        Enabled bool   `yaml:"enabled"`
        // ...插件自有配置结构体,不污染全局 GlobalConfig
    }
    found, err := ctx.Config().UnmarshalSection("my_plugin", &cfg)
    if err != nil || !found || !cfg.Enabled {
        return kernel.ErrDisabled
    }
    p.cfg = cfg
    return nil
}
```

## 生命周期

```
kernel.Register（init 或组合根） → Loader.Build() → App.Start(ctx) → App.Stop(ctx)
                                     │
                                     ├─ 1. 内置适配器运行期过滤:plugins.<name>.enabled
                                     ├─ 2. Configure（可主动 ErrDisabled）
                                     ├─ 3. 依赖校验（硬依赖必须可用）
                                     └─ 4. 拓扑排序（环检测,注册序决胜）
```

- **Start 失败回滚**：已启动插件按逆序 Stop，返回聚合错误
- **Stop**：按依赖逆序，错误聚合返回

## 内置组件插件

框架内置组件由组合根 `internal/bootstrap/plugins_builtin.go` 包装为
kernel.Plugin 注册（实例通过适配器绑定 Bootstrap）：

| 插件名 | 依赖 | 说明 |
|---|---|---|
| `db` | — | 数据库工厂（驱动注册表） |
| `cache` | — | Redis 缓存 |
| `discovery` | — | 服务发现管理器 |
| `tracing` | — | OTel 链路追踪 |
| `auth` | — | JWT 认证 |
| `metrics` | — | Prometheus 指标服务 |
| `health` | *(soft)* db, cache | 健康检查（db/cache 禁用时少注册 checker） |
| `errors` | — | 错误处理初始化 |
| `upload` | — | 文件上传（由 upload 配置段决定） |
| `actor` | — | Actor 系统（由 actor 配置段决定：`enabled`/`mailbox_size`/`restart_always`/`max_restarts`） |

托管服务（`api-gateway` / `rpc` / `websocket`）不走插件机制，仍由
`ManagedService`/`ServiceFactory` 按 `framework.services` 清单启停。
websocket 服务可通过 `WebSocketServiceOptions` 挂载 `Router`（envelope 路由）
与 `Rooms`（房间生命周期），工厂自动做连接注册、Manager 装配与停止排空。

## 配置示例

```yaml
plugins:
  db:      { enabled: true }
  cache:   { enabled: true }
  metrics: { enabled: true }
  health:  { enabled: true }
  # 未出现在此段的插件默认启用;显式 false 才禁用

framework:
  services: ["rpc", "api-gateway"]
```

启动日志（内置适配器运行期裁剪可观测）：

```
kernel: plugin disabled by config   plugin=db
kernel: plugin app built            plugins=3 order="metrics -> health -> errors"
kernel: starting plugin             plugin=metrics
```

## 如何裁剪一个应用

1. **运行期**：在 `config.yaml` 把不需要的内置组件 `enabled: false`；此机制只覆盖上述 10 个内置适配器
2. **链接期**：不 import 不需要的插件模块（二进制体积与攻击面同步减小）
3. 验证：`go tool nm <binary> | grep <重型SDK>` 应无符号

## 如何把插件搬去独立仓库

插件模块已按独立仓库形态准备（各自 go.mod、依赖显式声明）：

1. `cp -r plugins/<name> /path/to/new-repo`
2. 修改其 `go.mod` 模块名（如 `github.com/alldev-run/gf-<name>`）
3. 删除其中的本地 `replace` 段，为框架内核与其他依赖插件补上线上版本
4. 依赖方全量替换 import 前缀
5. 本仓库 `go.work` 移除对应 `use` 行、根 go.mod 移除对应 require/replace
6. `examples/go.mod` 若引用该模块，同步替换其 replace 为线上版本

> 依赖提示：`plugins/cache` 依赖 `plugins/db`（驱动自注册），`plugins/health`
> 依赖 `plugins/cache`；`gateway`/`rpc`/`middleware` 自身聚合多个插件，
> 拆仓时子树一并处理。其余插件（alert/auth/bloomfilter/configcenter/gameserver/
> push/scheduler/loadbalancer/rbac/messaging/metrics/tracing/discovery 等）
> 只依赖框架内核公共包，可独立拆出。

## 常见问题

**Q: 插件 A 依赖插件 B,但 B 被禁用了？**
硬依赖 → 构建失败并明确报错；软依赖（`SoftDepends`）→ A 降级运行。

**Q: 为什么 mongo-driver 还在我的二进制里？**
gin v1.12 的 `binding` 包自身引入了 bson（MsgPack 绑定支持），与本框架的
db 插件无关；clickhouse-go / go-elasticsearch 已确认不会被链接。

**Q: 如何新增一个插件？**
实现 `kernel.Plugin` → 在组合根或应用侧 `kernel.Register` → 需要时添加
配置段。若组件有自己的重型 SDK,按 db 驱动注册表模式做子包自注册。
