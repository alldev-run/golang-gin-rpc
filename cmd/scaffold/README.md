# Scaffold

本目录提供一个用于快速生成/维护 API 项目的脚手架命令：`cmd/scaffold`。

## 生成 API 项目

模板目录位于：

- `pkg/gateway/templates/<template>/`

在仓库根目录执行：

```bash
go run ./cmd/scaffold create-api --name <new-api> --template <template>
```

示例（Windows PowerShell）：

```powershell
# 从 pkg/gateway/templates/http-gateway 生成 api/demo-api
go run .\cmd\scaffold create-api --name demo-api --template http-gateway

# 启动生成的项目
go run .\api\demo-api
```

## 导出/同步模板（维护模板）

当你在 `api/<name>` 内对目录结构、路由、中间件等做了改动后，可以将其反向导出到模板目录，便于后续新项目自动继承你的改动。

在仓库根目录执行：

```bash
go run ./cmd/scaffold export-template --name <api-name> --template <template>
```

示例（Windows PowerShell）：

```powershell
# 将 api/demo-api 导出回 pkg/gateway/templates/http-gateway
# 其中 *.go 会导出为 *.go.gotmpl，并自动注入 token
go run .\cmd\scaffold export-template --name demo-api --template http-gateway
```

## 模板文件约定

- 模板内的 Go 文件使用 `.gotmpl` 后缀，例如：
  - `main.go.gotmpl`
  - `internal/httpapi/router.go.gotmpl`
- `create-api` 时会将 `.gotmpl` 复制并输出为 `.go`
- 模板内支持以下 token：
  - `__MODULE__`：`go.mod` 中的 module 名称
  - `__API_NAME__`：新项目名称（`--name`）
  - `__API_PATH__`：`api/<name>`

## 常见问题

### 1) 为什么 templates 里不直接放 .go？

因为模板中包含 `__MODULE__` / `__API_PATH__` 等 token，IDE 会把 `.go` 当成真实代码解析从而报错。使用 `.gotmpl` 可以避免 IDE 对模板进行 Go 语义解析。

### 2) api 目录为空是否可以生成？

可以。`create-api` 会自动创建 `api/` 目录。
