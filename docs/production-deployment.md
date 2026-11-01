# 生产部署指南

本文档详细介绍如何将 github.com/alldev-run/golang-gin-rpc 的数据库客户端组件部署到生产环境。

## 目录

1. [架构概述](#架构概述)
2. [环境准备](#环境准备)
3. [配置说明](#配置说明)
4. [部署步骤](#部署步骤)
5. [监控与告警](#监控与告警)
6. [故障排查](#故障排查)
7. [性能优化](#性能优化)

## 架构概述

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│  plugins/db/poolcb (连接池 + 断路器)                              │
├─────────────────────────────────────────────────────────────┤
│  plugins/db/rwproxy (读写分离)                                   │
├─────────────────────────────────────────────────────────────┤
│  plugins/db/pool (连接池)                                        │
├─────────────────────────────────────────────────────────────┤
│  plugins/db/mysql, plugins/db/postgres, plugins/cache/redis … │
└─────────────────────────────────────────────────────────────┘
```

## 环境准备

### 1. 系统要求

- Go 1.25+
- Docker 20.10+ (可选，用于容器化部署)
- 数据库服务器：MySQL 8.0+ / PostgreSQL 14+ / Redis 7+ / ClickHouse / Elasticsearch 8+

### 2. 安装依赖

```bash
go mod download
```

### 3. 数据库初始化

#### MySQL

```sql
CREATE DATABASE IF NOT EXISTS myapp CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'app_user'@'%' IDENTIFIED BY 'strong_password';
GRANT ALL PRIVILEGES ON myapp.* TO 'app_user'@'%';
FLUSH PRIVILEGES;
```

#### PostgreSQL

```sql
CREATE DATABASE myapp;
CREATE USER app_user WITH ENCRYPTED PASSWORD 'strong_password';
GRANT ALL PRIVILEGES ON DATABASE myapp TO app_user;
```

#### Redis

无需初始化，直接使用。

## 配置说明

### 数据库配置文件

创建 `configs/database.production.yaml`：

```yaml
# 主库配置
mysql_master:
  type: mysql
  mysql:
    host: "mysql-primary.internal"
    port: 3306
    database: "myapp"
    username: "app_user"
    password: "${MYSQL_PASSWORD}"  # 使用环境变量
    charset: "utf8mb4"
    max_open_conns: 50
    max_idle_conns: 25
    conn_max_lifetime: "1h"

# 从库配置
mysql_replica:
  type: mysql
  mysql:
    host: "mysql-replica.internal"
    port: 3306
    database: "myapp"
    username: "app_user"
    password: "${MYSQL_PASSWORD}"
    charset: "utf8mb4"
    max_open_conns: 50
    max_idle_conns: 25
    conn_max_lifetime: "1h"

# Redis配置
redis_cache:
  type: redis
  redis:
    host: "redis.internal"
    port: 6379
    password: "${REDIS_PASSWORD}"
    database: 0
    pool_size: 50
    min_idle_conns: 10

# PostgreSQL配置
postgres_main:
  type: postgres
  postgres:
    host: "postgres.internal"
    port: 5432
    database: "myapp"
    username: "app_user"
    password: "${POSTGRES_PASSWORD}"
    ssl_mode: "require"
    max_open_conns: 50
```

### 断路器配置

```go
breakerConfig := circuitbreaker.Config{
    MaxFailures:         10,               // 10次失败开启断路器
    ResetTimeout:        30 * time.Second, // 30秒后尝试恢复
    HalfOpenMaxRequests: 5,                // 半开状态测试请求数
    SuccessThreshold:    3,                // 3次成功关闭断路器
    Name:                "main-db",
}
```

### 慢查询配置

```go
slowQueryConfig := slowquery.Config{
    Threshold:   200 * time.Millisecond,  // 超过200ms视为慢查询
    MaxQueryLen: 1000,                   // 日志中查询最大长度
    IncludeArgs: false,                  // 生产环境不建议记录参数
    SampleRate:  1,                      // 记录所有慢查询
}
```

## 部署步骤

### 1. 编译

```bash
# 开发环境
go build -o bin/app .

# 生产环境（静态链接）
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/app .
```

### 2. 运行数据库迁移

迁移能力由 `plugins/db/migration` 提供,命令入口在 scaffold(无独立 cmd/migrate):

```bash
# 生成迁移文件
go run ./cmd/scaffold gen-migration --dir migrations --name add_users_table

# 执行迁移(DSN 按环境传入)
go run ./cmd/scaffold run-migration --dir migrations --driver mysql     --dsn "user:pass@tcp(localhost:3306)/myapp" --action up

# 查看状态
go run ./cmd/scaffold run-migration --dir migrations --driver mysql     --dsn "user:pass@tcp(localhost:3306)/myapp" --action status
```

### 3. Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/app .
COPY configs/ ./configs/
EXPOSE 8080
CMD ["./app"]
```

构建和运行：

```bash
docker build -t myapp:latest .
docker run -d \
  -p 8080:8080 \
  -e MYSQL_PASSWORD=secret \
  -e REDIS_PASSWORD=secret \
  -v $(pwd)/configs:/root/configs \
  myapp:latest
```

### 4. Kubernetes 部署

```yaml
# 参考现成清单:deploy/k8s/(gateway/user/websocket-game);
# 以下为通用 Deployment 骨架
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:latest
        ports:
        - containerPort: 8080
        env:
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-secrets
              key: mysql-password
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-secrets
              key: redis-password
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## 监控与告警

### Prometheus 指标

```go
// 在应用中暴露指标
import "github.com/prometheus/client_golang/prometheus/promhttp"

http.Handle("/metrics", promhttp.Handler())
```

### 关键指标

| 指标 | 告警阈值 | 说明 |
|------|----------|------|
| db_query_duration_seconds | P99 > 500ms | 查询延迟 |
| db_slow_query_total | > 100/分钟 | 慢查询数量 |
| circuit_breaker_state | = 1 | 断路器开启 |
| db_connection_pool_size | > 80% | 连接池使用率 |
| db_connection_active | 持续增长 | 连接泄漏 |

### Grafana Dashboard

1. 导入提供的 dashboard 配置
2. 配置数据源指向 Prometheus
3. 设置告警规则

## 故障排查

### 1. 断路器开启

**症状**: 大量 `ErrCircuitOpen` 错误

**排查步骤**:
1. 检查数据库连接状态
2. 查看慢查询日志
3. 确认网络连接正常
4. 临时手动关闭断路器：`breaker.ForceClosed()`

### 2. 连接池耗尽

**症状**: `AcquireTimeout` 错误

**排查步骤**:
1. 检查 `db_connection_active` 指标
2. 确认连接是否正确释放（是否调用 `Close()`）
3. 增加 `MaxOpenConns` 配置
4. 检查是否存在慢查询占用连接

### 3. 慢查询

**症状**: 大量慢查询日志

**排查步骤**:
1. 分析慢查询日志找出问题SQL
2. 检查是否缺少索引
3. 优化查询语句
4. 考虑添加缓存

## 性能优化

### 1. 连接池调优

```go
// 生产环境推荐配置
poolConfig := pool.Config{
    MaxSize:           100,              // 根据数据库配置调整
    MaxIdleTime:       10 * time.Minute, // 空闲连接超时
    HealthCheckPeriod: 15 * time.Second, // 健康检查频率
    MaxFailures:       3,                // 快速标记不健康
    AcquireTimeout:    5 * time.Second,  // 获取连接超时
}
```

### 2. 读写分离

```go
// 配置主从结构
rwConfig := poolrw.RWPoolConfig{
    MasterConfig: masterCfg,
    ReplicaConfigs: []db.Config{replica1Cfg, replica2Cfg},
    Strategy: rwproxy.LBStrategyRoundRobin,
}
```

### 3. 缓存策略

```go
// 读多写少场景启用缓存
if data, err := redisClient.Get(ctx, key); err == nil {
    return data, nil
}

// 查询数据库
data, err := db.Query(ctx, query)
if err != nil {
    return nil, err
}

// 写入缓存
redisClient.Set(ctx, key, data, 5*time.Minute)
```

## 安全建议

1. **密码管理**: 使用环境变量或密钥管理服务，不要硬编码
2. **SSL/TLS**: 生产环境必须启用 SSL 连接
3. **最小权限**: 数据库用户只授予必要权限
4. **网络隔离**: 数据库服务器应在私有子网
5. **审计日志**: 启用数据库审计功能

## 压测与验证

```bash
# WebSocket 机器人压测(连接数/速率/时长可调,输出 RTT 分位与吞吐)
go run ./cmd/loadbot --url ws://localhost:8091/ws --bots 100 --rate 10 --duration 30s
```

## 附录

### 常用命令

```bash
# 注意:框架默认不提供 /debug/pool、/debug/breaker 路由,以下为
# Prometheus 指标端点(启用 metrics 插件后可用):
curl http://localhost:9090/metrics
# 连接池/断路器状态可从 db_connection_pool_size、circuit_breaker_state 等指标读取
```

### 配置文件模板

参考 `configs/database.example.yaml`(生产配置需按该模板自行创建)。

---

**技术支持**: 如遇问题，请查看日志或联系开发团队。
