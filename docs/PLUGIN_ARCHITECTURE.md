# 插件化架构（Plugin Architecture）

本框架采用**编译期注册 + 配置驱动**的插件机制（`pkg/kernel`），将功能组件与框架内核解耦。

## 核心理念："不用的就不加载"

加载裁剪发生在两个层面：

| 层面 | 机制 | 效果 |
|---|---|---|
| **链接期** | 应用只 `import` 需要的插件包 | 未 import 的插件及其重型 SDK 根本不进二进制 |
| **运行期** | 配置 `plugins.<name>.enabled: false` | 已注册但被禁用的插件跳过 Configure/Start/Stop |

## 目录布局

```text
pkg/                  核心内核（始终链接）
├── kernel/           插件契约 + 注册表 + 生命周期加载器
├── config/ logger/ errors/ status_code/ response/
└── panicx/ requestid/ cors/ utils/ version/ bootstrap/

plugins/<name>/       功能插件，每个子目录一个独立 Go 模块
                      （自带 go.mod；搬去独立仓库时只需改模块名）
```

当前 26 个插件模块：`actor` `alert` `audit` `auth` `bloomfilter` `cache`
`circuitbreaker` `configcenter` `db` `discovery` `gateway` `geoip` `health`
`loadbalancer` `messaging` `metrics` `middleware` `push` `ratelimit` `rbac`
`rpc` `scheduler` `search` `tracing` `upload` `websocket`

> 特殊布局：`plugins/gateway` 内含 `middleware/gin`、`middleware/nethttp`、
> `router`、`httplog`（网关家族）；`plugins/ratelimit` 内含 `ratelimiter`
> 子包；`plugins/db` 的重型驱动（clickhouse/mongodb）与
> `plugins/search/elasticsearch` 通过**驱动注册表**自注册。

## 插件成熟度分级

每个插件按 API 稳定性、测试覆盖、生产验证程度划分为三个等级，便于使用者评估接入风险。

| 等级 | 含义 | 承诺 |
|---|---|---|
| **GA** | 生产就绪 | 接口稳定（遵循 semver）、测试覆盖完整、已在生产环境验证 |
| **Beta** | 功能完整但尚未广泛验证 | 接口基本稳定，允许小幅调整；测试覆盖中等 |
| **Experimental** | 占位 / 原型 / 接口不稳定 | 允许破坏性变更，可能缺少实现或测试 |

### 当前分级

- **GA**：`db`、`cache`、`gateway`、`rpc`、`discovery`、`auth`、`health`、`metrics`、`tracing`、`middleware`、`websocket`、`messaging`
- **Beta**：`actor`、`audit`、`bloomfilter`、`circuitbreaker`、`loadbalancer`、`push`、`ratelimit`、`rbac`、`upload`、`configcenter`、`scheduler`、`alert`、`geoip`
- **Experimental**：`search`（当前为空壳占位，接口未定义）

> 分级随版本演进调整；升级至 GA 的硬性门槛：至少 1 个生产案例 + 完整 README + 关键路径测试覆盖。

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
                                     ├─ 1. 运行期过滤:plugins.<name>.enabled
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

托管服务（`api-gateway` / `rpc` / `websocket`）不走插件机制，仍由
`ManagedService`/`ServiceFactory` 按 `framework.services` 清单启停。

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

启动日志（运行期裁剪可观测）：

```
kernel: plugin disabled by config   plugin=db
kernel: plugin app built            plugins=3 order="metrics -> health -> errors"
kernel: starting plugin             plugin=metrics
```

## 如何裁剪一个应用

1. **运行期**：在 `config.yaml` 把不需要的内置插件 `enabled: false`
2. **链接期**：不 import 不需要的插件模块（二进制体积与攻击面同步减小）
3. 验证：`go tool nm <binary> | grep <重型SDK>` 应无符号

## 如何把插件搬去独立仓库

插件模块已按独立仓库形态准备（各自 go.mod、依赖显式声明）：

1. `cp -r plugins/<name> /path/to/new-repo`
2. 修改其 `go.mod` 模块名（如 `github.com/alldev-run/gf-<name>`）
3. 删除其中的本地 `replace` 段，为框架内核与其他依赖插件补上线上版本
4. 依赖方全量替换 import 前缀
5. 本仓库 `go.work` 移除对应 `use` 行、根 go.mod 移除对应 require/replace

## 常见问题

**Q: 插件 A 依赖插件 B,但 B 被禁用了？**
硬依赖 → 构建失败并明确报错；软依赖（`SoftDepends`）→ A 降级运行。

**Q: 为什么 mongo-driver 还在我的二进制里？**
gin v1.12 的 `binding` 包自身引入了 bson（MsgPack 绑定支持），与本框架的
db 插件无关；clickhouse-go / go-elasticsearch 已确认不会被链接。

**Q: 如何新增一个插件？**
实现 `kernel.Plugin` → 在组合根或应用侧 `kernel.Register` → 需要时添加
配置段。若组件有自己的重型 SDK,按 db 驱动注册表模式做子包自注册。
