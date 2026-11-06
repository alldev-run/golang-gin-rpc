# 代码政策（中文版）

本文汇总仓库**现行生效**的工程规范与强制点。版本与发布规则见
[版本与发布政策](release-policy.zh-CN.md)；漏洞报告见
[SECURITY.md](../../SECURITY.md)。

> 本文为中文对照版；英文版为权威版本：[code-policy.md](code-policy.md)。

## 分支与变更流

- 长期分支：`main`（发布线）、`develop`（集成分支）。
- Pull request 一律面向 `develop`；发布经由 `develop` → `main` 合并
  （见[发布政策](release-policy.zh-CN.md)）。
- 保持变更聚焦：一个 PR 解决一件事；行为变更必须带回归测试。
- 不在缺乏具体需要与评审的情况下新增第三方依赖。

## 提交规范

Conventional Commits：`type(scope): subject`。在用类型：
`feat`、`fix`、`docs`、`style`、`refactor`、`test`、`build`、`chore`；
`scope` 为模块/包名（如 `fix(cache): close singleflight race`）。
破坏性变更在类型后加 `!`，如
`refactor(config)!: move built-in plugin sections out of GlobalConfig`。

## 代码风格与静态检查

- 格式化以 `gofmt` 为准（`.golangci.yml` 的 `formatters` 段）；
- `.golangci.yml` 启用 `standard` linter 集；`_test.go` 豁免 `errcheck`，
  `examples/` 不参与 lint，comments / common-false-positives /
  std-error-handling 走 preset 豁免；
- lint 版本固定 `golangci-lint v2.15.0`（CI 与本地一致）；
- 本地检查命令与 CI 相同（见[贡献指南](../../CONTRIBUTING.md)）。

## 质量门禁（CI 强制点）

`.github/workflows/ci.yml` 按 job 划分检查，任何一项失败即阻塞合并：

| 门禁 | 范围 | 要求 |
|---|---|---|
| Build / vet | 全部模块 | 编译与 `go vet` 通过 |
| Test | 根、`pkg`、27 个插件 | `go test -count=1 -race -cover` |
| Coverage gate | GA + GA-ready 插件 | statement coverage ≥50%（`scripts/coverage_gate.sh`；`GATED_PLUGINS` 名单与插件架构文档保持同步） |
| Dependency boundary | core 模块（`pkg`） | `go list -deps` 不得出现 `plugins/*`、`internal/*` |
| Tidy check | 全部模块 | `go mod tidy` 后无 diff（`git diff --exit-code -- go.mod go.sum`） |
| Lint | 根、`pkg`、27 个插件 | golangci-lint 通过 |
| Docker build | 仓库根 | Dockerfile 可构建 |

`examples/` 只跑 build、vet 与 tidy 检查。

## 模块依赖边界

与 CI 联动的架构约定（详见[插件架构](../architecture/plugin-architecture.md)）：

- `pkg/` 是独立 core 模块，不 import 任何 `plugins/*` 或 `internal/*`（CI 强制）；
- 插件模块只 require core 模块 `.../pkg`，不 require 根模块；仓库内以
  `require v0.0.0` + 本地 `replace` 引用；
- 插件之间无循环依赖；已知方向：`cache → db`（redis/memcache 驱动自注册）、
  `health → cache → db`；
- 聚合插件面向小接口编程（`rpc.MetricsRecorder`、`middleware.PermissionChecker`、
  `gateway.PermissionChecker`），不直接依赖具体插件类型；
- `examples/` 为独立模块，不进入主模块的 require 集。

## 插件政策

- 新插件 = `plugins/<name>` 独立模块 + 完整 README + 满足 `kernel.Plugin` 契约；
  重型 SDK 按驱动注册表子包自注册模式接入；
- 成熟度分三级：**GA / Beta / Experimental**（定义与当前分级表见插件架构文档）；
- GA 门槛：至少一个可核实的生产案例、statement coverage ≥50%、README 完整、
  无占位实现；生产案例未核实前 Beta 不升级 GA；
- GA 与 GA-ready 插件由覆盖率门禁强制（≥50%）；升降级时必须同步
  `scripts/coverage_gate.sh` 的 `GATED_PLUGINS` 名单与插件架构文档的分级表。

## 依赖与工具链

- Go 工具链固定 `toolchain go1.27.3`（根 `go.mod`），CI `GO_VERSION=1.27.3`，
  Docker 构建镜像一致；
- 依赖升级以安全优先：`govulncheck ./...` 必须保持零可达漏洞；
- 每个模块 `go mod tidy` 必须无 diff（按 CI 方式以 `GOWORK=off` 执行）。

## 配置政策

- 组件开关唯一机制是 `plugins.<name>.enabled`；`framework.init_*` 兼容键已在
  v0.2 移除，不得再新增框架级兼容翻译层；
- 插件解析自己的配置段（`UnmarshalSection`），不得把私有配置塞进
  `pkg/config.GlobalConfig`；内置插件的独立配置段在 `bootstrap/pluginconfig.Config`；
- 破坏性配置变更必须在 CHANGELOG 标注 `**Breaking:**` 并给出迁移说明
  （见[发布政策](release-policy.zh-CN.md)）；
- `configs/config.example.yaml` 是配置基线，新增配置项须同步该文件与相关文档。

## 文档政策

- 文档按主题放在 `docs/` 分类目录，入口为 [docs/README.md](../README.md)；
- 新增插件、新增服务发现后端、变更用户可见行为时，必须同步对应文档与插件 README；
- CHANGELOG 按 Keep a Changelog 1.1.0 维护，变更落地时随时记入 `## [Unreleased]`。

## 仓库卫生

不提交：构建产物与生成的二进制（`bin/`、`scaffold` 等）、覆盖率报告、日志、
凭据与本地配置（`.env`）、编辑器/IDE 目录。

## 安全

漏洞通过 GitHub Security Advisories 按 [SECURITY.md](../../SECURITY.md) 私密披露，
不得在公开 issue/PR 中描述未修复的漏洞。审计事件与排障见
[安全审计](../security/audit.md)与[审计排障手册](audit-runbook.md)。
