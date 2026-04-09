# Database Module Documentation

## Overview

This document focuses on the `pkg/db` module in `github.com/alldev-run/golang-gin-rpc`.

`pkg/db` provides reusable database access components for Go services, including client factory, connection pooling extensions, read-write split helpers, ORM builder, migration utilities, and related observability helpers.

## Supported Databases (`pkg/db`)

| Database | Package | Type |
|----------|---------|------|
| MySQL | `pkg/db/mysql` | SQL |
| PostgreSQL | `pkg/db/postgres` | SQL |
| ClickHouse | `pkg/db/clickhouse` | Column Store |
| MongoDB | `pkg/db/mongodb` | Document Store |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/sqlprevention (SQL Injection Prevention)           │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/poolcb (Connection Pool + Circuit Breaker)         │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/poolrw (Pool + Read-Write Split)                    │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/rwproxy (Read-Write Proxy)                          │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/pool (Connection Pool)                              │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/slowquery (Slow Query Logger)                       │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/circuitbreaker (Circuit Breaker)                  │
├─────────────────────────────────────────────────────────────┤
│  pkg/db/metrics (Prometheus Metrics)                        │
├─────────────────────────────────────────────────────────────┤
│  Individual Database Clients                               │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Basic Usage

```go
import (
    "github.com/alldev-run/golang-gin-rpc/pkg/db"
    "github.com/alldev-run/golang-gin-rpc/pkg/db/poolcb"
)
```

### Global MySQL / Global Client Usage (Bootstrap)

When using framework bootstrap, the recommended way is:

1. create bootstrap
2. load database yaml into bootstrap
3. initialize databases
4. get global MySQL client from bootstrap helpers

```go
package main

import (
    "context"
    "log"

    bootstrap "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
)

func main() {
    boot, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        log.Fatalf("new bootstrap failed: %v", err)
    }
    defer boot.Close()

    // Optional: load external database yaml (mysql_primary/mysql_replica etc.)
    if err := bootstrap.LoadDatabaseConfig(boot, "configs/database.yaml"); err != nil {
        log.Fatalf("load database config failed: %v", err)
    }

    // Must initialize DB before GetMySQLClient/GetMySQLSQLClient
    if err := boot.InitializeDatabases(); err != nil {
        log.Fatalf("initialize databases failed: %v", err)
    }

    // Typed MySQL client
    mysqlClient, err := bootstrap.GetMySQLClient(boot)
    if err != nil {
        log.Fatalf("get mysql client failed: %v", err)
    }

    ctx := context.Background()
    if err := mysqlClient.Ping(ctx); err != nil {
        log.Fatalf("mysql ping failed: %v", err)
    }
}
```

If you need a database-agnostic SQL interface (for easier mocking/abstraction), use `GetMySQLSQLClient`:

```go
sqlClient, err := bootstrap.GetMySQLSQLClient(boot)
if err != nil {
    return err
}

rows, err := sqlClient.Query(ctx, "SELECT id, name FROM users WHERE id = ?", 1)
if err != nil {
    return err
}
defer rows.Close()
```

You can also get the global factory and retrieve client by type:

```go
factory := bootstrap.GetDatabaseFactory(boot)
if factory == nil {
    return fmt.Errorf("database factory not initialized")
}

client, err := factory.GetClient(db.TypeMySQL)
if err != nil {
    return err
}

_ = client // db.Client
```

Common errors and meanings:

- `bootstrap instance is nil`: bootstrap pointer is nil when calling helper methods.
- `database factory not initialized`: forgot to call `InitializeDatabases()` first.
- `MySQL client not found`: MySQL is not enabled/loaded in your DB config.

### Recommended DI Pattern (Service / Repository)

To avoid coupling business code with bootstrap helpers, initialize global client in startup layer and inject `db.SQLClient` into repository/service.

```go
package repository

import (
    "context"

    "github.com/alldev-run/golang-gin-rpc/pkg/db"
)

type UserRepository struct {
    db db.SQLClient
}

func NewUserRepository(dbClient db.SQLClient) *UserRepository {
    return &UserRepository{db: dbClient}
}

func (r *UserRepository) GetNameByID(ctx context.Context, id int64) (string, error) {
    row := r.db.QueryRow(ctx, "SELECT name FROM users WHERE id = ?", id)
    var name string
    if err := row.Scan(&name); err != nil {
        return "", err
    }
    return name, nil
}
```

```go
package service

import (
    "context"
)

type UserRepo interface {
    GetNameByID(ctx context.Context, id int64) (string, error)
}

type UserService struct {
    repo UserRepo
}

func NewUserService(repo UserRepo) *UserService {
    return &UserService{repo: repo}
}

func (s *UserService) GetUserName(ctx context.Context, id int64) (string, error) {
    return s.repo.GetNameByID(ctx, id)
}
```

```go
package main

import (
    "log"

    bootstrap "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
)

func wire() {
    boot, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    if err := bootstrap.LoadDatabaseConfig(boot, "configs/database.yaml"); err != nil {
        log.Fatal(err)
    }

    if err := boot.InitializeDatabases(); err != nil {
        log.Fatal(err)
    }

    sqlClient, err := bootstrap.GetMySQLSQLClient(boot)
    if err != nil {
        log.Fatal(err)
    }

    userRepo := repository.NewUserRepository(sqlClient)
    _ = service.NewUserService(userRepo)
}
```

Benefits of this pattern:

- Business layer depends on interface (`UserRepo`), not bootstrap/global state.
- Repository can be unit-tested by mocking `db.SQLClient`.
- Bootstrap concerns are isolated in startup wiring only.

### Gateway Route Wiring Pattern (Recommended)

For HTTP gateway projects, keep the same DI principle:

- Assemble dependencies in `main` (startup layer).
- Pass service dependencies into router/route registrars.
- Do not inject `*bootstrap.Bootstrap` into route handlers directly.
- Do not use request context (`gin.Context`) to store global dependencies.

```go
// main.go
const gatewayServiceName = "api-gateway.http-gateway"

routeServices := routes.NewServices()
bizHandler := httpapi.NewRouter(mergedGwCfg, routeServices).Handler()
```

```go
// internal/routes/registry.go
type Services struct {
    HelloService *service.HelloService
}

func NewServices() *Services {
    return &Services{
        HelloService: service.NewHelloService(),
    }
}

func RegisterAll(registry *router.RouteRegistry, services *Services) {
    RegisterUserRoutes(registry, services)
}
```

```go
// Example: blog routes receive service dependency (preferred)
func RegisterBlogRoutes(registry *router.RouteRegistry, topicService *service.TopicService) {
    blogGroup := registry.Group("blog", "/api/blog")
    blogGroup.GET("/list", topicService.TopicList, "获取主题列表")
}
```

This keeps routing layer simple and testable, while allowing service construction to use global MySQL/ORM clients in startup code.

### ORM Usage Example (with Global MySQL Client)

After getting global MySQL SQL client from bootstrap, you can build ORM instance once and inject it to repositories.

```go
package main

import (
    "log"

    bootstrap "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

func initORM() *orm.ORM {
    boot, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    if err := bootstrap.LoadDatabaseConfig(boot, "configs/database.yaml"); err != nil {
        log.Fatal(err)
    }

    if err := boot.InitializeDatabases(); err != nil {
        log.Fatal(err)
    }

    mysqlClient, err := bootstrap.GetMySQLClient(boot)
    if err != nil {
        log.Fatal(err)
    }

    return orm.NewORMWithDB(mysqlClient.DB(), orm.NewMySQLDialect())
}
```

CRUD with query builder:

```go
// INSERT
_, err := ormInstance.Insert("users").
    Set("name", "alice").
    Set("status", 1).
    Exec(ctx)

// SELECT one
var id int64
var name string
err = ormInstance.Select("users").
    Columns("id", "name").
    Eq("status", 1).
    QueryRow(ctx).
    Scan(&id, &name)

// UPDATE
_, err = ormInstance.Update("users").
    Set("status", 2).
    Eq("id", id).
    Exec(ctx)

// DELETE
_, err = ormInstance.Delete("users").
    Eq("id", id).
    Exec(ctx)
```

Transaction example:

```go
err := ormInstance.Transaction(ctx, func(tx *orm.ORM) error {
    if _, err := tx.Insert("orders").Set("user_id", 1001).Set("amount", 99).Exec(ctx); err != nil {
        return err
    }
    if _, err := tx.Update("accounts").Set("balance", 1000).Eq("user_id", 1001).Exec(ctx); err != nil {
        return err
    }
    return nil
})
```

Repository injection style:

```go
type OrderRepository struct {
    orm *orm.ORM
}

func NewOrderRepository(ormInstance *orm.ORM) *OrderRepository {
    return &OrderRepository{orm: ormInstance}
}
```

Recommended practice:

- Build ORM once in startup/wire layer and inject it.
- Keep SQL/table details inside repository layer.
- Use `Transaction` for multi-step write operations.
- Combine ORM builder with raw SQL only when necessary (complex aggregation/reporting).

### End-to-End: Global DB Client + ORM Together

Use `GetMySQLClient` for ORM initialization, and keep `GetMySQLSQLClient` if you still need occasional raw SQL.

```go
package main

import (
    "context"
    "log"

    bootstrap "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/db"
    "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

type UserRepository struct {
    orm       *orm.ORM
    sqlClient db.SQLClient
}

func NewUserRepository(ormInstance *orm.ORM, sqlClient db.SQLClient) *UserRepository {
    return &UserRepository{orm: ormInstance, sqlClient: sqlClient}
}

func (r *UserRepository) Create(ctx context.Context, name string) error {
    _, err := r.orm.Insert("users").Set("name", name).Set("status", 1).Exec(ctx)
    return err
}

func (r *UserRepository) CountActive(ctx context.Context) (int, error) {
    // Example: raw SQL for aggregation/report style query
    row := r.sqlClient.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE status = ?", 1)
    var n int
    if err := row.Scan(&n); err != nil {
        return 0, err
    }
    return n, nil
}

func wire() *UserRepository {
    boot, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    if err := bootstrap.LoadDatabaseConfig(boot, "configs/database.yaml"); err != nil {
        log.Fatal(err)
    }

    if err := boot.InitializeDatabases(); err != nil {
        log.Fatal(err)
    }

    mysqlClient, err := bootstrap.GetMySQLClient(boot)
    if err != nil {
        log.Fatal(err)
    }

    sqlClient, err := bootstrap.GetMySQLSQLClient(boot)
    if err != nil {
        log.Fatal(err)
    }

    ormInstance := orm.NewORMWithDB(mysqlClient.DB(), orm.NewMySQLDialect())
    return NewUserRepository(ormInstance, sqlClient)
}
```

When to use which:

- ORM query builder: standard CRUD and maintainable repository logic.
- SQLClient raw SQL: complex aggregation/report queries and highly tuned SQL.

### MySQL Helper Methods

The MySQL client also provides helper methods for common write operations:

- `InsertGetID` - run an INSERT and return the last inserted ID
- `Update` - run an UPDATE and return affected rows
- `SetFieldByID` - update a single field by primary key
- `Save` - insert or update based on whether the ID is zero. Supports optimistic locking with version field.

```go
id, err := client.InsertGetID(ctx, "INSERT INTO users (name,email) VALUES (?, ?)", "alice", "alice@example.com")
_, err = client.Update(ctx, "UPDATE users SET email = ? WHERE id = ?", "alice@new.com", id)
_, err = client.SetFieldByID(ctx, "users", "id", id, "status", "active")
_, err = client.Save(ctx, "users", "id", id, map[string]interface{}{"status": "active"})

// Optimistic locking with version
_, err = client.Save(ctx, "users", "id", id, map[string]interface{}{"status": "active", "version": 1})

// DELETE queries
result, err := orm.NewDeleteBuilder(client, "users").Where("status = ?", "inactive").Exec(ctx)
```

## Connection Pooling

// Load configuration
cfg := poolcb.Config{
    PoolConfig: pool.Config{
        MaxSize:           50,
        HealthCheckPeriod: 30 * time.Second,
    },
    BreakerConfig: circuitbreaker.Config{
        MaxFailures:  5,
        ResetTimeout: 30 * time.Second,
    },
}

// Create pool with circuit breaker
pool := poolcb.New(cfg, db.NewFactory())
defer pool.Close()

// Register database
pool.Register("main-db", db.Config{
    Type: db.TypeMySQL,
    MySQL: mysql.Config{
        Host:     "localhost",
        Port:     3306,
        Database: "myapp",
    },
})

// Acquire and use
client, err := pool.Acquire(ctx, "main-db")
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### SQL Operations with Read-Write Split

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/db/poolrw"

// Configure master and replicas
cfg := poolrw.RWPoolConfig{
    MasterConfig: db.Config{
        Type: db.TypeMySQL,
        MySQL: mysql.Config{
            Host: "master.db.internal",
            Port: 3306,
        },
    },
    ReplicaConfigs: []db.Config{
        {
            Type: db.TypeMySQL,
            MySQL: mysql.Config{
                Host: "replica1.db.internal",
                Port: 3306,
            },
        },
    },
}

pool, _ := poolrw.New(cfg, db.NewFactory())
defer pool.Close()

// SELECT goes to replica
rows, _ := pool.Query(ctx, "SELECT * FROM users")

// INSERT/UPDATE/DELETE goes to master
result, _ := pool.Exec(ctx, "INSERT INTO logs VALUES (?)", data)

// Transaction always on master
tx, _ := pool.Begin(ctx, nil)
```

## Features

### 1. Connection Pooling

- Max connection limit
- Idle connection timeout
- Health checking
- Automatic reconnection

```go
poolConfig := pool.Config{
    MaxSize:           100,
    MaxIdleTime:       30 * time.Minute,
    HealthCheckPeriod: 30 * time.Second,
    MaxFailures:       3,
    AcquireTimeout:    5 * time.Second,
}
```

### 2. Read-Write Splitting

- Automatic query routing
- Round-robin load balancing
- Force master mode for consistency

```go
// Query goes to replica (SELECT)
rows, _ := pool.Query(ctx, "SELECT * FROM users")

// Exec goes to master (INSERT/UPDATE/DELETE)
result, _ := pool.Exec(ctx, "UPDATE users SET name=?", name)

// Force master for critical reads
pool.ForceMaster(true)
rows, _ := pool.Query(ctx, "SELECT balance FROM accounts")
pool.ForceMaster(false)
```

### 3. Circuit Breaker

- Automatic failure detection
- Fast fail when service is unhealthy
- Automatic recovery

```go
// Configure circuit breaker
breakerConfig := circuitbreaker.Config{
    MaxFailures:         5,
    ResetTimeout:        30 * time.Second,
    HalfOpenMaxRequests: 3,
    SuccessThreshold:    2,
}

// Returns ErrCircuitOpen when breaker is open
client, err := pool.Acquire(ctx, "main-db")
if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
    // Service unavailable, use fallback
}
```

### 4. Slow Query Logging

- Configurable threshold
- Query truncation for safety
- Sampling support

```go
slowQueryConfig := slowquery.Config{
    Threshold:   100 * time.Millisecond,
    MaxQueryLen: 1000,
    IncludeArgs: false, // Don't log args in production
    SampleRate:  1,
}

logger := slowquery.New(slowQueryConfig)
wrappedQuery := logger.WrapQuery(db.Query)
```

### 5. Metrics Collection

Prometheus metrics exported:

```
db_query_duration_seconds{database, operation, table}
db_query_total{database, operation, status}
db_connection_pool_size{database, type}
db_slow_query_total{database, threshold}
circuit_breaker_state{name}
circuit_breaker_failures_total{name}
```

### 6. SQL Injection Prevention

- Input validation and sanitization
- Pattern-based injection detection
- Parameterized query builder
- Safe identifier validation

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/db/sqlprevention"

// Validate input
validator := sqlprevention.New(sqlprevention.DefaultConfig())
if err := validator.ValidateInput(userInput); err != nil {
    return errors.New("invalid input")
}

// Detect injection patterns
result := sqlprevention.DetectInjection("' OR '1'='1")
if result.IsInjected {
    log.Printf("SQL injection detected: %s (severity: %s)", result.Pattern, result.Severity)
}

// Use parameterized queries
pq := sqlprevention.NewParameterizedQuery("SELECT * FROM users WHERE id = ?")
pq.AddParam(userID)
query, params := pq.Build()
rows, err := db.Query(query, params...)

// Sanitize LIKE patterns
safePattern := sqlprevention.CleanLikePattern("test%") 
// Result: test\% (escapes special characters)
```

## Configuration

### YAML Configuration

```yaml
# configs/database.yaml
mysql_primary:
  type: mysql
  mysql:
    host: "localhost"
    port: 3306
    database: "myapp"
    username: "app_user"
    password: "${MYSQL_PASSWORD}"
    max_open_conns: 50
    max_idle_conns: 25

redis_cache:
  type: redis
  redis:
    host: "localhost"
    port: 6379
    pool_size: 50

mongodb_primary:
  type: mongodb
  mongodb:
    uri: "mongodb://localhost:27017"
    database: "myapp"
```

### Loading Configuration

```go
// Load from YAML
cfg, err := db.LoadConfigFromYAML("configs/database.yaml")

// Load multiple configs
cfgs, err := db.LoadConfigsFromYAML("configs/databases.yaml")
for name, cfg := range cfgs {
    pool.Register(name, cfg)
}
```

## Database Migration

Generate migration SQL files with scaffold CLI (standalone, not coupled to api-gateway):

```bash
# create migrations/20260324183000_create_users_table.up.sql
# create migrations/20260324183000_create_users_table.down.sql
go run ./cmd/scaffold gen-migration --name create_users_table

# custom output directory
go run ./cmd/scaffold gen-migration --name add_user_status --dir db/migrations

# custom version prefix
go run ./cmd/scaffold gen-migration --name add_user_index --version 20260324190000
```

The generated files are plain SQL templates. Fill in `UP` and `DOWN` SQL, then register/run them via `pkg/db/migration`.

Run SQL migration files directly with scaffold CLI:

```bash
# apply all pending migrations from directory
go run ./cmd/scaffold run-migration \
  --driver mysql \
  --dsn "root:password@tcp(127.0.0.1:3306)/myblog?parseTime=true" \
  --dir migrations \
  --action up

# rollback one step
go run ./cmd/scaffold run-migration \
  --driver mysql \
  --dsn "root:password@tcp(127.0.0.1:3306)/myblog?parseTime=true" \
  --dir migrations \
  --action down \
  --steps 1

# view migration status
go run ./cmd/scaffold run-migration \
  --driver mysql \
  --dsn "root:password@tcp(127.0.0.1:3306)/myblog?parseTime=true" \
  --dir migrations \
  --action status
```

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/db/migration"

// Create migrator
m := migration.New(db)

// Add migrations
m.Add(1, "create_users",
    "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(255));",
    "DROP TABLE users;")

m.Add(2, "create_posts",
    "CREATE TABLE posts (id INT PRIMARY KEY, user_id INT, title VARCHAR(255));",
    "DROP TABLE posts;")

// Run migrations
err := m.Up(ctx)

// Check status
statuses, _ := m.Status(ctx)
for _, s := range statuses {
    fmt.Printf("Version %d: %s (applied: %v)\n", s.Version, s.Name, s.Applied)
}
```

## Production Checklist

- [ ] Use environment variables for passwords
- [ ] Enable SSL/TLS for database connections
- [ ] Configure connection pool sizes based on database capacity
- [ ] Set up circuit breaker with appropriate thresholds
- [ ] Enable slow query logging (threshold: 100-200ms)
- [ ] Configure Prometheus metrics export
- [ ] Set up database migration system
- [ ] Use read-write split for read-heavy workloads
- [ ] **Enable SQL injection validation for all user inputs**
- [ ] Use parameterized queries instead of string concatenation
- [ ] Monitor connection pool usage
- [ ] Test failover scenarios

## API Reference

### Core Interfaces

```go
// Client - Base database client interface
type Client interface {
    Ping(ctx context.Context) error
    Close() error
}

// SQLClient - Extended interface for SQL databases
type SQLClient interface {
    Client
    DB() *sql.DB
    Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
    QueryRow(ctx context.Context, query string, args ...any) *sql.Row
    Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
    Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
    Transaction(ctx context.Context, fn func(*sql.Tx) error) error
}
```

### Factory

```go
factory := db.NewFactory()

// Create individual clients
mysqlClient, _ := factory.Create(db.Config{
    Type: db.TypeMySQL,
    MySQL: mysql.Config{...},
})

// Supported types: TypeMySQL, TypePostgres, TypeRedis, 
// TypeClickHouse, TypeES, TypeMemcache, TypeMongoDB
```

## Testing

```bash
# Run all tests
go test ./pkg/db/...

# Run specific package tests
go test ./pkg/db/poolcb/... -v

# Integration tests (requires Docker)
docker-compose -f deploy/docker-compose.integration.yml up -d
go test ./pkg/... -tags=integration
```

## Troubleshooting

### Circuit Breaker Open

**Symptom**: `ErrCircuitOpen` errors

**Solution**:
1. Check database connectivity
2. Review slow query logs
3. Consider adjusting `MaxFailures` threshold

### Connection Pool Exhaustion

**Symptom**: `AcquireTimeout` errors

**Solution**:
1. Increase `MaxSize`
2. Check for connection leaks (ensure `Close()` is called)
3. Reduce `MaxIdleTime`

### High Query Latency

**Symptom**: Slow query logs triggered frequently

**Solution**:
1. Review and optimize slow queries
2. Add database indexes
3. Enable read replicas
4. Consider caching

## License

Apache License 2.0 - See LICENSE file for details.

## Author

- John James
- Email: `nbjohn999@gmail.com`

## Contributing

1. Fork the repository
2. Create your feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

---

For more details, see:
- [Production Deployment Guide](docs/production-deployment.md)
- [API Usage Examples](examples/)
- [Integration Tests](deploy/docker-compose.integration.yml)
