# Database Client Package Documentation

## Overview

The `pkg/db` package provides a comprehensive, production-ready database client solution for Go applications. It supports multiple databases with advanced features including connection pooling, read-write splitting, circuit breaker patterns, slow query logging, and comprehensive metrics collection.

## Supported Databases

| Database | Package | Type |
|----------|---------|------|
| MySQL | `pkg/mysql` | SQL |
| PostgreSQL | `pkg/postgres` | SQL |
| Redis | `pkg/redis` | Key-Value |
| ClickHouse | `pkg/clickhouse` | Column Store |
| Elasticsearch | `pkg/elasticsearch` | Search Engine |
| Memcached | `pkg/memcache` | Cache |
| MongoDB | `pkg/mongodb` | Document Store |

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

MIT License - See LICENSE file for details.

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
