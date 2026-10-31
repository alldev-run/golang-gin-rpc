# __APP_NAME__

由 `scaffold init` 生成的应用骨架,基于插件化框架(内核 + 按需组件)。

## 运行

```bash
# 在仓库根目录执行
go run ./__API_PATH__
```

默认监听 `config/config.yaml` 中 `server.http.port` 指定的端口,优雅退出依赖 SIGINT/SIGTERM。

## 已启用组件

组件选择记录在 `config/config.yaml`,两部分共同控制:

- `plugins.<name>.enabled`:内核插件开关(db/cache/discovery/tracing/auth/metrics/health/errors/upload)
- `framework.services`:托管服务列表(api-gateway/rpc/websocket)

## 调整组件

编辑 `config/config.yaml`:把不需要的组件 `enabled` 改为 `false`,或在 `framework.services` 中增删服务。未 import 的插件不参与链接,未启用的插件不参与运行。

## 下一步

- 业务代码建议放在 `internal/` 下,通过 `bootstrap.RegisterAPIGatewayServiceFactory` 等注册自定义服务
- 数据库等外部设施的连接信息在 `config/config.yaml` 对应段中修改
