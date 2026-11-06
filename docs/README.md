# 文档索引

按主题查找项目指南。插件实现细节和配置项也可查看各插件的 README。

## 架构

| 文档 | 说明 |
|---|---|
| [插件架构](architecture/plugin-architecture.md) | 插件生命周期、注册机制与成熟度分级。 |
| [配置治理](architecture/config-governance.md) | 配置加载、校验与治理约定。 |
| [Gin 路由](architecture/gin-router.md) | Gin 路由注册与组织方式。 |

## 数据库

| 文档 | 说明 |
|---|---|
| [数据库插件](database/README.md) | 数据库驱动、ORM、迁移与 PostgreSQL 批量写入概览。 |
| [数据库 API 用法](database/api-usage.md) | 客户端、连接池、读写分离与常用数据库能力示例。 |
| [ORM 指南](database/orm-guide.md) | ORM 查询、模型与事务用法。 |
| [事务管理器](database/transaction-manager.md) | 分布式事务协调器协议及配置说明。 |

## 缓存

| 文档 | 说明 |
|---|---|
| [故障转移缓存](cache/failover-cache.md) | 多后端缓存和故障切换配置。 |
| [文件缓存](cache/file-cache.md) | 本地文件缓存的使用与配置。 |

## RPC

| 文档 | 说明 |
|---|---|
| [RPC 指南](rpc/README.md) | gRPC 与 JSON-RPC 的客户端、服务端用法。 |
| [RPC 认证](rpc/authentication.md) | RPC 请求认证配置与示例。 |
| [JSON-RPC 追踪](rpc/json-rpc-tracing.md) | JSON-RPC 专属的追踪接入与上下文传播。 |

## 网关

| 文档 | 说明 |
|---|---|
| [Gateway 指南](gateway/README.md) | 快速启动、端点、静态/Consul 服务发现与排障。 |
| [Gateway 认证](gateway/authentication.md) | 网关认证模式、配置与安全建议。 |

## 服务发现

| 文档 | 说明 |
|---|---|
| [服务发现指南](discovery/README.md) | 注册中心、服务注册发现及运行时配置。 |

## 可观测性

| 文档 | 说明 |
|---|---|
| [分布式追踪](observability/tracing.md) | OpenTelemetry、Zipkin、Jaeger/OTLP 与采样配置。 |

## 消息

| 文档 | 说明 |
|---|---|
| [消息系统](messaging/README.md) | Kafka、NATS、RabbitMQ 的接入与使用。 |

## 上传

| 文档 | 说明 |
|---|---|
| [文件上传](upload/README.md) | 文件存储、校验、命名策略与安全注意事项。 |

## 安全

| 文档 | 说明 |
|---|---|
| [JWTX](security/jwtx.md) | JWT 令牌签发、验证、刷新与撤销。 |
| [IP 过滤与 GeoIP](security/ip-filter-geoip.md) | IP 过滤中间件与 GeoIP 数据使用。 |
| [安全审计](security/audit.md) | 审计事件、存储与安全运维说明。 |

## 运维

| 文档 | 说明 |
|---|---|
| [本地开发与启动](operations/local-development.md) | 启动方式、配置兜底、热重载与 Docker 本地运行。 |
| [生产部署](operations/production-deployment.md) | 环境准备、部署、监控、排障与验证。 |
| [审计排障手册](operations/audit-runbook.md) | 审计日志与安全事件排查步骤。 |

## 项目与插件参考

- [贡献指南](../CONTRIBUTING.md) · [安全策略](../SECURITY.md) · [变更日志](../CHANGELOG.md)

| 插件 | README | 说明 |
|---|---|---|
| actor | [README](../plugins/actor/README.md) | Actor 消息、监督与调度能力。 |
| alert | [README](../plugins/alert/README.md) | 告警渠道与消息投递。 |
| audit | [README](../plugins/audit/README.md) | 审计事件与持久化。 |
| auth | [README](../plugins/auth/README.md) | 认证管理与令牌能力。 |
| bloomfilter | [README](../plugins/bloomfilter/README.md) | 布隆过滤器。 |
| cache | [README](../plugins/cache/README.md) | 缓存抽象与后端。 |
| circuitbreaker | [README](../plugins/circuitbreaker/README.md) | 熔断器。 |
| configcenter | [README](../plugins/configcenter/README.md) | 配置中心客户端。 |
| db | [README](../plugins/db/README.md) | 数据库插件 API。 |
| discovery | [README](../plugins/discovery/README.md) | 服务发现 API。 |
| gameserver | [README](../plugins/gameserver/README.md) | 实时游戏服务与 AOI。 |
| gateway | [README](../plugins/gateway/README.md) | HTTP/RPC 网关。 |
| geoip | [README](../plugins/geoip/README.md) | IP 地理位置查询。 |
| health | [README](../plugins/health/README.md) | 健康检查服务。 |
| loadbalancer | [README](../plugins/loadbalancer/README.md) | 负载均衡策略。 |
| messaging | [README](../plugins/messaging/README.md) | 消息队列适配器。 |
| metrics | [README](../plugins/metrics/README.md) | 指标采集与导出。 |
| middleware | [README](../plugins/middleware/README.md) | HTTP 中间件集合。 |
| push | [README](../plugins/push/README.md) | APNs 与 FCM 推送。 |
| ratelimit | [README](../plugins/ratelimit/README.md) | 请求限流器。 |
| rbac | [README](../plugins/rbac/README.md) | 基于角色的访问控制。 |
| rpc | [README](../plugins/rpc/README.md) | gRPC 与 JSON-RPC。 |
| scheduler | [README](../plugins/scheduler/README.md) | 定时任务调度。 |
| search | [README](../plugins/search/README.md) | Elasticsearch 搜索驱动。 |
| tracing | [README](../plugins/tracing/README.md) | 分布式追踪集成。 |
| upload | [README](../plugins/upload/README.md) | 文件上传插件。 |
| websocket | [README](../plugins/websocket/README.md) | WebSocket 会话与房间。 |
