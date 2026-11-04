# plugins/db

`plugins/db` 提供数据库访问与治理相关能力，覆盖：

- 多数据库客户端封装
- 工厂模式创建数据库实例
- ORM / 查询构建 / 分页
- 连接池与池级治理
- 读写分离与代理能力
- 数据库迁移
- 指标采集、慢查询观测
- SQL 风险防护
- 数据库访问熔断能力

它适合作为业务开发中的统一数据库基础层，用于支撑常规 CRUD、分页查询、连接管理、迁移与观测等日常数据库开发需求。

## 目录结构

- `helper.go`
  - 全局数据库访问助手，提供简洁的 ORM 和 SQL 查询接口
- `config.go`
  - 数据库配置定义
- `factory.go`
  - 统一数据库工厂与实例创建入口
- `mysql/`
  - MySQL 相关封装
- `postgres/`
  - PostgreSQL 相关封装
- `mongodb/`
  - MongoDB 相关封装
- `clickhouse/`
  - ClickHouse 相关封装
- `orm/`
  - ORM、查询构建、分页、扫描等能力
- `pool/`
  - 基础连接池封装
- `poolcb/`
  - 带熔断能力的连接池
- `poolrw/`
  - 读写池能力
- `rwproxy/`
  - 读写代理能力
- `migration/`
  - 数据库迁移能力
- `metrics/`
  - 数据库指标采集
- `slowquery/`
  - 慢查询观测
- `sqlprevention/`
  - SQL 风险防护
- `circuitbreaker/`
  - 数据库侧熔断能力

## 适用场景

`plugins/db` 当前适合：

- 常规业务系统数据库开发
- CRUD 与分页查询
- MySQL / PostgreSQL / MongoDB / ClickHouse 接入
- 基础连接池管理
- 读写分离开发
- 上线前迁移与数据库观测接入
- 慢查询监控与基础 SQL 防护

## 核心能力

### 1. 全局 Helper（推荐）

`helper.go` 提供全局数据库访问助手，在 bootstrap 初始化后，可直接在业务代码中使用，无需手动传递数据库实例。

#### 基本使用

```go
import "github.com/alldev-run/golang-gin-rpc/plugins/db"

// MySQL 查询（默认）
rows, err := db.Select("users").
    Columns("id", "name", "email").
    Eq("status", 1).
    Query(ctx)

// 插入
_, err := db.Insert("users").Sets(map[string]interface{}{
    "name":  "John",
    "email": "john@example.com",
}).Exec(ctx)

// 更新
_, err := db.Update("users").
    Set("name", "Jane").
    Eq("id", 1).
    Exec(ctx)

// 删除
_, err := db.Delete("users").
    Eq("id", 1).
    Exec(ctx)
```

#### 多数据库支持

全局 helper 默认面向 MySQL；PostgreSQL 通过 `Postgres()` / `InsertOn()` 显式指定：

```go
// MySQL（默认，无需指定）
rows, err := db.Select("users").Eq("id", 1).Query(ctx)

// PostgreSQL：SQLClient 级别使用
pgClient, err := db.Postgres()
rows, err := pgClient.Query(ctx, "SELECT * FROM orders WHERE status = $1", "pending")

// PostgreSQL：ORM builder 级别使用
_, err = db.InsertOn(db.TypePostgres, "orders").Sets(map[string]interface{}{
    "status": "pending",
}).Exec(ctx)

// 跨库访问：On 选择器为 builder 指定库名（MySQL）
rows, err = db.On("tenant_a").Select("users").Eq("id", 1).Query(ctx)
```

#### 原始 SQL 查询

```go
// 查询
rows, err := db.Query(ctx, "SELECT * FROM users WHERE id = ?", 1)

// 执行
result, err := db.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "John")

// 事务
err := db.Transaction(ctx, func(tx *sql.Tx) error {
    _, err := tx.Exec("INSERT INTO ...")
    return err
})
```

#### 初始化

在 main.go 中初始化 bootstrap，helper 会自动可用：

```go
boot, err := bootstrap.NewBootstrap("config.yaml")
boot.InitializeAll() // 自动设置全局 factory，db.Select 立即可用
```

### 2. 数据库工厂

通过工厂模式统一创建数据库实例，适合在 bootstrap 阶段按配置加载。

适合：

- 按配置初始化数据库资源
- 在多数据库场景下统一入口
- 屏蔽具体数据库初始化差异

```go
factory := db.NewFactory()

// 按类型创建（同一配置自动去重/复用，singleflight 防击穿）
client, err := factory.Create(db.Config{Type: db.TypeMySQL, MySQL: mysqlCfg})
pgClient, err := factory.Create(db.Config{Type: db.TypePostgres, PG: pgCfg})

// 按类型取缓存实例
client, err := factory.GetClient(db.TypeMySQL)

// 驱动注册表自检
linked := db.LinkedDrivers()   // 当前二进制已链接的驱动列表
ok := db.DriverLinked(db.TypeMongoDB)
```

#### 驱动注册表（重型驱动自注册）

`mysql/redis/postgres/memcache` 由本包 `init()` 内置注册；重型驱动
（`clickhouse`、`mongodb`）通过独立子包 **import 即注册**——不 import 不链接：

```go
import (
    _ "github.com/alldev-run/golang-gin-rpc/plugins/db/clickhouse" // 注册 ClickHouse 驱动
    _ "github.com/alldev-run/golang-gin-rpc/plugins/db/mongodb"    // 注册 MongoDB 驱动
)

client, err := factory.Create(db.Config{Type: db.TypeMongoDB, MongoDB: mongoCfg})
// 具体类型经驱动子包断言取回：见各子包 register.go
```

未链接的驱动会得到明确错误，提示 import 对应的 `plugins/db/<driver>` 包。

#### 各驱动配置参考

配置结构体以 `db.Config` 聚合，字段与 yaml/json tag 一致：
`MySQL`/`Redis`/`PG`/`Memcache` 为具体结构体，`CH`/`ES`/`MongoDB` 以 `any`
携带、由对应驱动子包解析（`decodeConfig` 支持 struct 直传与 map 兜底）。
字段明细见各子包 `Config` 结构体的 yaml tag（如 `postgres.Config`：
`host` `port` `database` `username` `password` `ssl_mode` `max_open_conns`
`max_idle_conns` `conn_max_lifetime` `log_enabled` `log_level`
`slow_query_threshold`）。

### 3. ORM / 查询构建

`orm/` 提供常见数据库开发所需能力：

- 查询构建
- 分页
- 结构体扫描
- 基础 SQL 组合

适合：

- 管理后台
- 常规服务查询接口
- 列表分页与过滤查询

#### 事务管理器（企业特性）

`orm.TransactionManager` 提供重试、错误分类、指标与监控回调：

```go
tm := orm.NewTransactionManager(orm.TransactionConfig{
    MaxRetries:         3,
    RetryDelay:         100 * time.Millisecond,
    RetryBackoffFactor: 2.0,
    MaxRetryDelay:      5 * time.Second,
    EnableMetrics:      true,
    Timeout:            30 * time.Second,
})
defer tm.Close()

// 事务 + 可重试错误自动重试（死锁/连接类错误）
result, err := tm.WithTransaction(ctx, orm.NewDBWrapper(sqlDB), func(tx *orm.ORM) error {
    _, err := tx.Insert("orders").Sets(map[string]interface{}{"id": 1}).Exec(ctx)
    return err
})
if result != nil && result.Retries > 0 {
    // result.RetryAttempts 记录每次尝试
}

// 嵌套事务（SAVEPOINT）
err = tm.WithNestedTransaction(ctx, o, "sp_order", func(tx *orm.ORM) error {
    // 出错自动 ROLLBACK TO SAVEPOINT
    return nil
})

// 指标
m := tm.GetMetrics() // Total/Success/Failed/Retried、耗时分布
```

错误分类器（`NewDefaultErrorClassifier`）识别死锁、超时、连接、约束、权限、
资源类错误并给出重试建议；`WithRetry` 可为任意函数附加同一套重试策略。

### 4. 连接池与读写分离

连接池相关子包包括：

- `pool`
- `poolcb`
- `poolrw`
- `rwproxy`

适合：

- 基础连接池管理
- 池级别保护
- 读写分离的业务开发

#### pool：命名连接池

```go
p := pool.New(pool.DefaultConfig(), factory)
defer p.Close()

_ = p.Register("main", db.Config{Type: db.TypeMySQL, MySQL: cfg})
client, err := p.Acquire(ctx, "main")
stats := p.GetStats() // 各连接 UseCount/FailCount/State
```

#### poolcb：池 + 熔断

```go
sp := poolcb.NewSQL(poolcb.Config{
    PoolConfig:    pool.DefaultConfig(),
    BreakerConfig: circuitbreaker.DefaultConfig(),
}, factory)
defer sp.Close()

_ = sp.Register("main", db.Config{Type: db.TypeMySQL, MySQL: cfg})
rows, err := sp.Query(ctx, "main", "SELECT ...")   // Query/QueryRow/Exec/Transaction 均受熔断保护
_ = sp.GetBreakerStats()
```

#### poolrw：读写分离池

```go
p, err := poolrw.New(poolrw.RWPoolConfig{
    MasterConfig:   db.Config{Type: db.TypeMySQL, MySQL: masterCfg},
    ReplicaConfigs: []db.Config{{Type: db.TypeMySQL, MySQL: replicaCfg}},
    Strategy:       rwproxy.LBStrategyRoundRobin,
}, factory)
defer p.Close()

rows, _ := p.Query(ctx, "SELECT ...")   // 路由到 replica
_, err = p.Exec(ctx, "UPDATE ...")      // 路由到 master
p.ForceMaster(true)                     // 会话级强制走主库
stats := p.GetStats()
```

#### rwproxy：读写代理

```go
rw := rwproxy.New(rwproxy.Config{
    Master:   masterDB,            // *sql.DB
    Replicas: []*sql.DB{replicaDB},
    Strategy: rwproxy.LBStrategyRandom,
})
rows, err := rw.Query(ctx, "SELECT ...")
```

### 5. 可观测性与治理

数据库相关治理与观测能力包括：

- `metrics`
- `slowquery`
- `sqlprevention`
- `circuitbreaker`

适合：

- 暴露数据库指标
- 观测慢查询
- 基础 SQL 风险拦截
- 故障时的保护性熔断

#### metrics：Prometheus 指标

import 即注册到默认 registry（`db_query_duration_seconds`、
`db_query_total`、`db_connection_pool_size` 等），配合 `plugins/metrics`
的 /metrics 端点暴露即可。

#### slowquery：慢查询拦截

```go
logger := slowquery.New(slowquery.Config{Threshold: 500 * time.Millisecond})

rowsFn := logger.WrapQuery(rawQueryFn)       // 包一层即获得慢查询告警日志
rowFn := logger.WrapQueryRow(rawQueryRowFn)
execFn := logger.WrapExec(rawExecFn)
```

#### sqlprevention：SQL 注入防护

```go
v := sqlprevention.New(sqlprevention.DefaultConfig())
result := sqlprevention.DetectInjection(userInput) // 静态检测
if result.IsInjection { /* 拦截 */ }

pq := sqlprevention.NewParameterizedQuery("SELECT * FROM users WHERE id = ?").AddParam(id)
rows, err := pq.Execute(sqlDB) // 强制参数化执行
```

#### circuitbreaker：数据库熔断

```go
cb := circuitbreaker.New(circuitbreaker.Config{Name: "db-main"})
err := cb.Execute(ctx, func() error { return doQuery() })
if errors.Is(err, circuitbreaker.ErrCircuitOpen) { /* 熔断中，走降级 */ }
_ = cb.GetStats()
```

### 6. 迁移

`migration/` 适合做：

- schema 变更
- 初始化脚本执行
- 上线前迁移管理

```go
m := migration.New(sqlDB)
m.Add(1, "create_users",
    "CREATE TABLE users (id BIGINT PRIMARY KEY, name VARCHAR(64))",
    "DROP TABLE users")
_ = m.Init(ctx)   // 建 schema_migrations 表
err = m.Up(ctx)   // 执行到最新版本
err = m.Down(ctx) // 回滚一个版本
```

## 日常开发建议

如果你的目标是**日常数据库开发**，推荐优先这样使用：

- 常规业务查询与分页：`orm/`
- 基础数据库实例创建：`factory.go`
- 多数据库接入：`mysql/`、`postgres/`、`mongodb/`、`clickhouse/`
- 连接池：`pool/`
- 读写分离：`poolrw/` 或 `rwproxy/`
- 迁移：`migration/`
- 观测：`metrics/` + `slowquery/`
- 风险控制：`sqlprevention/`

## 当前状态判断

按当前仓库状态，`plugins/db` 已经**基本满足日常开发中的数据库相关操作**，包括：

- 常规增删改查
- 分页
- 多数据库接入
- 基础连接池
- 读写分离
- 数据库观测
- 慢查询与 SQL 基础防护

如果你的目标是：

- 常规业务系统开发
- 中小型服务数据库层封装
- 联调、灰度、上线准备

那么当前 `plugins/db` 是可用的。

## 当前保留项

当前仍建议保留预期，不直接把 `plugins/db` 定义为完全封顶的平台级数据库基础设施。还可以继续加强的方向包括：

- 更复杂的主从切换与故障编排
- 更成熟的分布式事务治理
- 更细粒度的数据库审计能力
- 更复杂的多租户数据库治理
- 更强的高并发数据库访问策略与压测验证

## 回归建议

修改 `plugins/db` 后，建议至少执行：

```bash
cd plugins/db && go test ./...
```

如果改动影响了更广的业务基础设施层，建议再执行：

```bash
go test ./pkg/...
```

## 测试与覆盖率

模块内所有子包均带单元测试（`database/sql` 层用 go-sqlmock，MongoDB/驱动层
用注入缝 + 离线错误路径，集成路径以 `*_INTEGRATION=1` 环境变量门控）：

```bash
# 单包覆盖率
cd plugins/db && go test ./orm/ -cover

# 模块级 statement coverage（GA 门槛 >= 50%，CI 强制）
bash scripts/coverage_gate.sh gate db

# 全部插件模块覆盖率报表（docs/PLUGIN_ARCHITECTURE.md 表格数据来源）
bash scripts/coverage_gate.sh report --all
```

带真实后端的集成测试（可选，默认跳过）：

```bash
MONGODB_INTEGRATION=1 MONGODB_URI=mongodb://localhost:27017 go test ./mongodb/ -run Integration
```

## 总结

`plugins/db` 当前已经可以作为业务开发中的数据库基础层使用，适合支撑：

- 日常数据库开发
- 常规生产开发准备
- 联调与回归阶段的数据库访问能力

但如果目标是高要求的平台级数据库治理，还建议继续做后续加固。
