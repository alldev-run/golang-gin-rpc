# HTTP Gateway 目录结构

## 目录结构

```
__API_PATH__/
├── main.go                    # 主入口文件
├── features_test.go           # 功能测试文件
├── config/
│   ├── config.yaml            # 主配置文件
│   └── http-logging-example.yaml # HTTP 日志配置示例
└── internal/
    ├── httpapi/
    │   └── router.go          # HTTP 路由器
    ├── model/
    │   ├── hello.go           # Hello 示例模型
    │   └── user.go            # 用户模型
    ├── mw/
    │   ├── demo.go            # 示例中间件
    │   ├── registry.go        # 中间件注册
    │   └── tracing.go         # 追踪中间件
    ├── routes/
    │   ├── registry.go        # 业务路由注册入口
    │   └── user_routes.go     # 用户路由示例
    └── service/
        └── hello_service.go   # 业务服务
```

## 文件说明

### 主要文件
- **main.go** - 应用程序入口（加载配置、初始化网关、优雅关闭）
- **features_test.go** - 功能验证测试
- **config/config.yaml** - 示例配置文件

### 内部模块
- **internal/httpapi/router.go** - RouterBuilder 适配层
- **internal/mw/** - 中间件集合
  - **tracing.go** - 链路追踪中间件
  - **demo.go** - 示例中间件
  - **registry.go** - 中间件注册机制
- **internal/routes/** - 业务路由注册
- **internal/model/user.go** - 用户接口数据结构
- **internal/service/hello_service.go** - 示例业务服务
- **internal/model/hello.go** - Hello 数据模型

## 功能

### 当前能力
- 链路追踪接入示例
- 基于 `pkg/gateway` 的多协议路由示例
- 自动化 HTTP 请求日志记录
- 请求 ID 自动生成与传递
- 支持通过配置控制请求/响应日志行为

### HTTP 日志配置
可在 `config/config.yaml` 的 `logging.http_logging` 下配置：

- `enabled`：是否启用 HTTP 请求日志
- `log_request_body` / `log_response_body`：是否记录请求体和响应体
- `max_body_size`：记录体积上限（字节）
- `log_headers`：是否记录请求头
- `sensitive_headers`：敏感请求头过滤列表
- `skip_paths`：跳过日志记录的路径
- `slow_request_threshold`：慢请求阈值
- `enable_request_id` / `request_id_header`：请求 ID 配置
- `log_level_thresholds`：按状态码映射日志级别

### 配置文件
- `config/config.yaml` - 主配置文件
- `config/http-logging-example.yaml` - HTTP 日志配置示例
- `LOGGING.md` - 日志相关说明

### 使用方式
```bash
# 启动服务
go run ./__API_PATH__

# 运行测试
go test ./__API_PATH__

# 构建应用
go build ./__API_PATH__
```

## 测试
可运行以下测试进行验证：

```bash
go test ./__API_PATH__ -v
```

## 相关文档

- `docs/gateway.md`
- `pkg/gateway/README.md`
