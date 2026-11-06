# 版本与发布政策（中文版）

本文规定 `github.com/alldev-run/golang-gin-rpc` 的版本号规则、发布分支模型与发布流程。
变更记录见 [CHANGELOG](../../CHANGELOG.md)（[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) 格式），
版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。现行工程规范见[代码政策](code-policy.zh-CN.md)。

> 本文为中文对照版；英文版为权威版本：[release-policy.md](release-policy.md)。

## 版本模型一览

版本号形如 `vMAJOR.MINOR.PATCH`，预发布后缀为 `-rc.N`（发布候选）与 `-beta.N`。
历史标签 `v0.1.17-bate1`/`-bate2` 为拼写遗留，后续不再使用 `bate` 拼写。

| 阶段 | 示例 | 含义 | 兼容性承诺 |
|---|---|---|---|
| `0.x` — **非稳定**（当前） | `v0.1.19`、`v0.2.0-rc.1` | API、插件契约与配置 schema 仍可能变化 | 小版本之间无承诺（Go modules 对 v0 的规则） |
| `1.0.0` — **稳定** | — | 稳定期开始：内核、bootstrap API 与配置 schema 冻结 | 1.x 兼容承诺（取 Go 1 兼容性保证的精神） |
| 小版本 `1.N.0` | `v1.1.0` | 功能版本——稳定期内的"大更新" | 只允许向后兼容的新增 |
| 补丁 `1.N.M` | `v1.1.1` | 缺陷修复、安全修复、依赖升级 | 不改 API 与配置语义 |
| 大版本 `X.0.0` | `v2.0.0` | 仅用于破坏性变更 | 需要迁移 |

稳定期内，用户感知的"大更新"是 MINOR 位递增（`1.0.x` → `1.1.0`）；
MAJOR 只保留给破坏性变更。长期维护的 Go 库正是这么做的——例如 Gin 多年
停留在 `v1.x`，以 `v1.10.0` 发功能、以 `v1.9.1` 发修复。

## 成熟项目的做法

| 项目 | 版本与发布分支实践 |
|---|---|
| [Go](https://go.dev/doc/devel/release) | Go 1（2012）以 Go 1 兼容性保证冻结语言与标准库；每年两个小版本（2 月/8 月），按需发补丁，为最近两个小版本提供安全修复；每个小版本保留长期分支 `release-branch.go1.N`。 |
| [Kubernetes](https://kubernetes.io/releases/version-skew-policy/) | 每年约三个 semver 小版本；每个小版本一个 `release-1.N` 分支；最近三个分支按月发补丁。 |
| [Gin](https://github.com/gin-gonic/gin) | v1.0 之前非稳定；多年稳定在 `v1.x`。功能走小版本、修复走补丁，从主分支直接发版，不为每个小版本留分支。 |
| [Terraform](https://developer.hashicorp.com/terraform/language/v1-compatibility) | 0.x 小版本之间携带破坏性变更；v1.0（2021）冻结 API，破坏性变更推给未来的大版本。 |
| [Node.js](https://nodejs.org/en/about/previous-releases) | 偶数大版本进入 LTS，保留按线维护的分支（`v20.x`、`v22.x`），支持约 30 个月。 |

本项目按 Go 模型按体量裁剪：0.x 直到 API 验证充分，之后进入 1.x 稳定期。
自 `v0.2.0` 起，每个正式版本都在逐小版本的发布分支上打标。

## 当前阶段：0.x（非稳定）

项目当前处于 0.x 阶段（最新：`v0.2.0-rc.1`）。早期 v0.0.x / v0.1.x 标签未严格
区分功能与修复，自 `v0.2.0` 起按本政策执行：

- **小版本 `0.N.0`** 是功能版本：允许包含破坏性变更（Go 模块对 v0 不承诺兼容），
  但必须满足：
  - CHANGELOG 中以 `**Breaking:**` 逐条标注，并给出迁移说明；
  - 对应提交使用 conventional commits 的 `!` 标记，如本仓库历史中的
    `refactor(config)!:`、`build!:`。
- **补丁 `0.N.M`** 只包含修复：不改导出 API 签名、不改配置键语义、不提升最低 Go 版本。
- **发布分支。** 每个正式版本都打在自己的 `release-0.N` 分支上（见
  [发布分支与维护](#发布分支与维护)）；rc 打在 `main` 上——`v0.2.0-rc.1` 即此形态。
  `v0.2.0` 定稿时，从发布合并提交切出 `release-0.2` 并在该分支上打标。

进入 `v1.0.0` 的条件：`pkg` 内核与 bootstrap API 稳定、配置 schema 稳定、
GA 插件覆盖主要能力且有可核实的生产案例——即 Terraform 2021 年走过的
0.x → 1.0 路径。

## 稳定阶段：1.x

- **小版本 `1.N.0` —— 功能版本。** 新插件、新能力、增量配置段、插件成熟度升级
  （Beta→GA）、弃用预告、依赖升级。**不允许**破坏性变更；已弃用 API 与旧配置键
  继续可用。
- **补丁 `1.N.M` —— 修复版本。** 缺陷修复、安全修复、依赖漏洞升级；除修复本身
  外不改行为与 API。安全修复遵循 [SECURITY.md](../../SECURITY.md) 的披露流程。
- **大版本 `X.0.0` —— 破坏性变更。** 删除/修改已发布导出 API、移除已弃用配置键
  或改变其语义、提升最低 Go 工具链版本、模块布局调整。大版本发布要求：立项
  issue 说明破坏面与迁移路径、至少提前一个小版本发弃用预告（CHANGELOG
  `### Deprecated`，弃用期内旧 API 保持可用）、发布 `vX.0.0-rc.1` 候选并冻结功能、
  CHANGELOG 含迁移说明（`**Breaking:**` 条目）。

## 分支与标签

长期分支：

- `develop` —— 集成分支，PR 一律面向它（见[贡献指南](../../CONTRIBUTING.md)）；
- `main` —— 就绪发布线；发布合并落在这里，rc 在此打标；
- `release-N.M` —— 每个受支持小版本一条（如 `release-0.2`、`release-1.1`）；
  功能版本/大版本的正式 tag 与补丁 tag 都在这里打（见下）。

## 代码中的框架版本

`pkg/version` 是代码内框架版本的唯一来源：

- `version.Version` —— 完整语义化版本字符串，不带 `v` 前缀、含预发布后缀
  （如 `0.2.0-rc.1`）；
- `version.Major` —— 大版本位；
- `version.Minor` —— 小版本位。

**每次发布前**，先落一个 release-prep 提交，把这些常量改为与即将打的 tag 完全
一致——功能版本/大版本改在 `develop` 上，补丁版本改在 `release-N.M` 分支上。
`pkg/version` 还停留在旧版本时禁止打 tag。`make build` 另经 LDFLAGS 注入
`git describe --tags --always`，发布产物从带 tag 的提交构建。

## 发布分支与维护

**发布版本是否保留对应 git 分支？** 保留——每个正式版本一条分支，
对应的发布 tag 都打在该分支上：

| 发布类型 | 分支 | 打标位置 |
|---|---|---|
| rc `vX.Y.Z-rc.N` | —（尚未切分支） | `main` |
| 功能版本 `v0.N.0` / `v1.N.0`（稳定期内的"大更新"）与大版本 `vX.0.0` | 从 `main` 的发布合并提交切出 `release-N.M` | `release-N.M` 分支 |
| 补丁 `vX.N.M` | 已有 `release-N.M` | `release-N.M` 分支 |

- **创建**：功能版本就绪后，`develop` → `main` 合并（`Merge develop for vX.Y.Z`），
  从该提交切出 `release-N.M`，并在发布分支上打 `vX.Y.Z` 标签——即 Go 的
  `release-branch.go1.N` / Kubernetes 的 `release-1.N` 惯例。（`main` 上的
  `v0.2.0-rc.1` 按 rc 规则执行；`v0.2.0` 定稿时切出 `release-0.2` 并在分支上打标。）
- **先改版本号**：更新 `pkg/version`（`Version` / `Major` / `Minor`）的
  release-prep 提交必须先于 tag 落地，不能事后补。
- **打补丁**：修复先落 `develop`，再 cherry-pick 到受影响的 `release-N.M`；
  在该分支上把 `pkg/version` 改为 `X.N.(M+1)` 并打 `vX.N.(M+1)` 标签。
  **禁止把 `develop` 合入发布分支**——那会带入下个小版本的功能。
- **保留时长**：分支在其版本线受支持期间保留。基线：最新小版本线（即
  [SECURITY.md](../../SECURITY.md) 当前承诺的范围）；随项目成熟，目标为最近两个
  小版本线（对齐 Go 的安全支持窗口）。支持窗口结束后分支可删除——tag 独立存在，
  任何已发布版本永远可检出。
- **多模块 tag**：带前缀的 tag（`pkg/vX.Y.Z`、`plugins/<name>/vX.Y.Z`）遵循同样的
  分支规则，从相同提交打标。

rc 在 `Merge develop for vX.Y.Z-rc.N` 合并后打在 `main` 上
（`v0.2.0-rc.1` 即此形态）。Docker 镜像标签与 git 标签对应
（当前 CI 仅构建镜像，不推送）。

## 多模块打标

仓库是多模块 monorepo（`go.work`）：根模块、core 模块 `pkg/`、27 个 `plugins/<name>/`
与 `examples/` 各自独立成模块。模块路径与标签规则：

| 模块 | 模块路径 | 标签格式 |
|---|---|---|
| 根（框架主模块） | `github.com/alldev-run/golang-gin-rpc` | `vX.Y.Z[-rc.N]` |
| core 内核 | `github.com/alldev-run/golang-gin-rpc/pkg` | `pkg/vX.Y.Z[-rc.N]` |
| 插件 | `github.com/alldev-run/golang-gin-rpc/plugins/<name>` | `plugins/<name>/vX.Y.Z[-rc.N]` |

- 仓库内开发不依赖远端版本：各模块以 `require ... v0.0.0` + `replace => 本地相对路径`
  引用（如 `plugins/db/go.mod` 的 `replace github.com/alldev-run/golang-gin-rpc/pkg => ../../pkg`）；
- 历史标签（`v0.0.1`…`v0.2.0-rc.1`）只覆盖根模块；**对外发布某个 core/插件模块时，
  必须按上表打带前缀的标签**，使用者才能
  `go get github.com/alldev-run/golang-gin-rpc/pkg@vX.Y.Z`；
- `examples/` 仅供仓库内部参考，不对外发布、不打标；
- 拆分为独立仓库的插件（见[插件架构](../architecture/plugin-architecture.md)）改用该仓库自身的标签体系。

## 发布检查清单

1. `develop` 上 CI 全绿：build / vet / `go test -count=1 -race -cover` / 模块 tidy 无 diff /
   golangci-lint / 插件覆盖率门禁（≥50%）/ core 依赖边界检查 / docker build；
2. `govulncheck ./...` 无可达漏洞；
3. CHANGELOG 的 `## [Unreleased]` 整理为 `## [vX.Y.Z] - YYYY-MM-DD`，含
   Added / Changed / Fixed / Security / Deprecated / Removed 分节，破坏性条目带 `**Breaking:**`；
4. 同步 README、`docs/` 与相关插件 README 中随版本变化的内容；
5. 更新代码内框架版本：落一个 release-prep 提交，把 `pkg/version`
   （`Version`、`Major`、`Minor`）改为与即将打的版本号完全一致（含预发布后缀）；
6. 功能版本 `vX.Y.0` / 大版本 `vX.0.0`：`develop` → `main` 合并，从合并提交
   切出 `release-N.M`，在发布分支上打 `vX.Y.Z` 标签，推送分支与标签
   （core/插件模块对外发布时同步打前缀标签）；
7. rc：`develop` → `main` 合并，在 `main` 上打 `vX.Y.Z-rc.N` 标签；
8. 补丁版本 `vX.Y.Z`：将修复 cherry-pick 到 `release-N.M`，在该分支上更新
   `pkg/version`，打 `vX.Y.Z` 标签，并按需发布对应前缀的子模块标签。

## 支持的版本

与 [SECURITY.md](../../SECURITY.md) 一致：仅为最新 `main` 发布线与 `develop`
提供修复（含安全修复）；更早的标签不再维护。"最新发布线"指受支持的
`release-N.M` 分支——基线为最新一条，目标为最新两条。
