# HTTP Gateway 目录结构

## 📁 目录结构

```
api/http-gateway/
├── main.go                    # 主入口文件
├── features_test.go           # 功能测试文件
├── config/
│   └── config.yaml           # 配置文件
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

## 📝 文件说明

### 🚀 **主要文件**
- **main.go** - 应用程序入口（加载配置、初始化网关、优雅关闭）
- **features_test.go** - 功能验证测试
- **config/config.yaml** - 示例配置文件

### 🔧 **内部模块**
- **internal/httpapi/router.go** - RouterBuilder 适配层
- **internal/mw/** - 中间件集合
  - **tracing.go** - 链路追踪中间件
  - **demo.go** - 示例中间件
  - **registry.go** - 中间件注册机制
- **internal/routes/** - 业务路由注册
- **internal/model/user.go** - 用户接口数据结构
- **internal/service/hello_service.go** - 示例业务服务
- **internal/model/hello.go** - Hello 数据模型

## 🌟 **特性**

### ✅ **当前示例能力**
- 🔍 链路追踪接入示例
- 🌐 基于 `pkg/gateway` 的多协议路由示例
- 🛡️ 可扩展中间件注册机制
- 🧪 基础功能测试

### 🚀 **使用方式**
```bash
# 启动服务
go run ./api/http-gateway

# 运行测试
go test ./api/http-gateway

# 构建应用
go build ./api/http-gateway
```

## 📈 **测试结果**
可运行以下测试进行验证：

```bash
go test ./api/http-gateway -v
```

## 🔗 相关文档

- `docs/gateway.md`
- `pkg/gateway/README.md`
