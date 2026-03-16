# API Usage Guide

This guide provides detailed examples for using the database client packages.

## Table of Contents

1. [Factory Pattern](#factory-pattern)
2. [Connection Pooling](#connection-pooling)
3. [Read-Write Splitting](#read-write-splitting)
4. [Circuit Breaker](#circuit-breaker)
5. [Slow Query Logging](#slow-query-logging)
6. [SQL Injection Prevention](#sql-injection-prevention)
7. [Database Migration](#database-migration)
8. [Metrics Collection](#metrics-collection)

## Factory Pattern

### Basic Client Creation

```go
package main

import (
    "context"
    "golang-gin-rpc/pkg/db"
    "golang-gin-rpc/pkg/mysql"
    "golang-gin-rpc/pkg/redis"
)

func main() {
    factory := db.NewFactory()
    ctx := context.Background()

    // Create MySQL client
    mysqlClient, err := factory.Create(db.Config{
        Type: db.TypeMySQL,
        MySQL: mysql.Config{
            Host:     "localhost",
            Port:     3306,
            Database: "myapp",
            Username: "root",
            Password: "secret",
        },
    })
    if err != nil {
        panic(err)
    }
    defer mysqlClient.Close()

    // Create Redis client
    redisClient, err := factory.Create(db.Config{
        Type: db.TypeRedis,
        Redis: redis.Config{
            Host: "localhost",
            Port: 6379,
        },
    })
    if err != nil {
        panic(err)
    }
    defer redisClient.Close()

    // Ping both
    if err := mysqlClient.Ping(ctx); err != nil {
        log.Fatal("MySQL ping failed:", err)
    }
    if err := redisClient.Ping(ctx); err != nil {
        log.Fatal("Redis ping failed:", err)
    }
}
```

### Creating All Supported Clients

```go
// MongoDB
mongoClient, _ := factory.Create(db.Config{
    Type: db.TypeMongoDB,
    MongoDB: mongodb.Config{
        URI:      "mongodb://localhost:27017",
        Database: "myapp",
    },
})

// Memcached
cacheClient, _ := factory.Create(db.Config{
    Type: db.TypeMemcache,
    Memcache: memcache.Config{
        Hosts: []string{"localhost:11211"},
    },
})

// ClickHouse
chClient, _ := factory.Create(db.Config{
    Type: db.TypeClickHouse,
    CH: clickhouse.Config{
        Hosts:    []string{"localhost:9000"},
        Database: "analytics",
    },
})

// Elasticsearch
esClient, _ := factory.Create(db.Config{
    Type: db.TypeES,
    ES: elasticsearch.Config{
        Addresses: []string{"http://localhost:9200"},
    },
})
```

## Connection Pooling

### Insert / Update Helpers (MySQL)

The MySQL client provides convenience helpers for common write operations.

```go
// Insert and get inserted ID
id, err := client.InsertGetID(ctx, "INSERT INTO users (name, email) VALUES (?, ?)", "Alice", "alice@example.com")

// Update and get rows affected
affected, err := client.Update(ctx, "UPDATE users SET email = ? WHERE id = ?", "alice@new.com", id)

// Set a single field by ID
affected, err = client.SetFieldByID(ctx, "users", "id", id, "email", "alice@new.com")

// Save (insert or update based on ID)
newID, err := client.Save(ctx, "users", "id", 0, map[string]interface{}{
    "name": "Bob",
    "email": "bob@example.com",
})
if err != nil {
    return err
}

// Update existing row
affected, err = client.Save(ctx, "users", "id", newID, map[string]interface{}{
    "email": "bob@new.com",
})

// Save with optimistic locking (version field for concurrency control)
affected, err = client.Save(ctx, "users", "id", newID, map[string]interface{}{
    "email": "bob@new.com",
    "version": 1, // current version value
})

// DELETE queries using DeleteBuilder
result, err := client.NewDeleteBuilder("users").
    Where("status = ?", "inactive").
    And("created_at < ?", time.Now().Add(-30*24*time.Hour)).
    Exec(ctx)

affected, err := result.RowsAffected()
```

## Connection Pooling

### Basic Pool Usage

```go
import (
    "golang-gin-rpc/pkg/db/pool"
    "golang-gin-rpc/pkg/db"
)

// Configure pool
poolConfig := pool.Config{
    MaxSize:           50,
    InitialSize:       5,
    MaxIdleTime:       30 * time.Minute,
    HealthCheckPeriod: 30 * time.Second,
    MaxFailures:       3,
    AcquireTimeout:    5 * time.Second,
}

// Create pool
p := pool.New(poolConfig, db.NewFactory())
defer p.Close()

// Register connections
p.Register("mysql-main", db.Config{
    Type: db.TypeMySQL,
    MySQL: mysql.Config{
        Host: "localhost",
        Port: 3306,
    },
})

p.Register("redis-cache", db.Config{
    Type: db.TypeRedis,
    Redis: redis.Config{
        Host: "localhost",
        Port: 6379,
    },
})

// Acquire and use
client, err := p.Acquire(ctx, "mysql-main")
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Get statistics
stats := p.GetStats()
for name, stat := range stats {
    fmt.Printf("%s: state=%d, use_count=%d\n", name, stat.State, stat.UseCount)
}
```

### Pool Statistics

```go
// Get pool size
size := p.Size()
fmt.Printf("Pool has %d connections\n", size)

// Get connection names
names := p.GetConnectionNames()
for _, name := range names {
    fmt.Println("Registered:", name)
}

// Unregister connection
if err := p.Unregister("mysql-main"); err != nil {
    log.Println("Unregister failed:", err)
}
```

## Read-Write Splitting

### Read-Write Proxy Client

```go
import (
    "database/sql"
    "golang-gin-rpc/pkg/db/rwproxy"
)

// Create master connection
masterDB, _ := sql.Open("mysql", "user:pass@tcp(master:3306)/db")

// Create replica connections
replica1, _ := sql.Open("mysql", "user:pass@tcp(replica1:3306)/db")
replica2, _ := sql.Open("mysql", "user:pass@tcp(replica2:3306)/db")

// Create RW proxy
config := rwproxy.Config{
    Master:   masterDB,
    Replicas: []*sql.DB{replica1, replica2},
    Strategy: rwproxy.LBStrategyRoundRobin,
}

client := rwproxy.New(config)
defer client.Close()

// Query (goes to replica)
rows, _ := client.Query(ctx, "SELECT * FROM users")
defer rows.Close()

// QueryRow (goes to replica)
row := client.QueryRow(ctx, "SELECT name FROM users WHERE id=?", 1)
var name string
row.Scan(&name)

// Exec (goes to master)
result, _ := client.Exec(ctx, "INSERT INTO users (name) VALUES (?)", "John")
lastID, _ := result.LastInsertId()

// Transaction (goes to master)
err := client.Transaction(ctx, func(tx *sql.Tx) error {
    _, err := tx.Exec("INSERT INTO logs VALUES (?)", "action")
    return err
})
```

### Forcing Master for Consistency

```go
// Check if forced
if client.IsMasterForced() {
    fmt.Println("All queries going to master")
}

// Force master mode
client.ForceMaster(true)

// Now even SELECT goes to master
row := client.QueryRow(ctx, "SELECT balance FROM accounts WHERE id=?", 1)

// Disable force master
client.ForceMaster(false)
```

### Managing Replicas

```go
// Get replica count
count := client.GetReplicaCount()
fmt.Printf("Has %d replicas\n", count)

// Add new replica
newReplica, _ := sql.Open("mysql", "...")
client.AddReplica(newReplica)

// Remove replica
if err := client.RemoveReplica(0); err != nil {
    log.Println("Remove failed:", err)
}
```

## Circuit Breaker

### Basic Circuit Breaker Usage

```go
import "golang-gin-rpc/pkg/db/circuitbreaker"

// Create breaker
config := circuitbreaker.Config{
    MaxFailures:         5,
    ResetTimeout:        30 * time.Second,
    HalfOpenMaxRequests: 3,
    SuccessThreshold:    2,
    Name:                "main-db",
}

breaker := circuitbreaker.New(config)

// Execute with protection
err := breaker.Execute(ctx, func() error {
    // Your database operation
    return db.QueryRow(...).Scan(&result)
})

if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
    // Service unavailable
    return fallbackResult, nil
}
```

### Executing with Results

```go
result, err := breaker.ExecuteWithResult(ctx, func() (any, error) {
    var user User
    err := db.QueryRow("SELECT * FROM users WHERE id=?", id).Scan(&user.ID, &user.Name)
    return user, err
})

if err != nil {
    if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
        // Use cached result
        return getCachedUser(id), nil
    }
    return nil, err
}

user := result.(User)
```

### Monitoring Breaker State

```go
// Get current state
state := breaker.State()
switch state {
case circuitbreaker.StateClosed:
    fmt.Println("Service healthy")
case circuitbreaker.StateOpen:
    fmt.Println("Service failing")
case circuitbreaker.StateHalfOpen:
    fmt.Println("Testing recovery")
}

// Get statistics
stats := breaker.GetStats()
fmt.Printf("State: %s, Failures: %d, Successes: %d\n",
    stats.State, stats.Failures, stats.Successes)
```

### Manual Control

```go
// Force open (maintenance mode)
breaker.ForceOpen()

// All requests will now fail fast
// Useful during database maintenance

// Force close (recovery mode)
breaker.ForceClosed()

// Normal operation resumes
```

## Slow Query Logging

### Wrapping Database Operations

```go
import "golang-gin-rpc/pkg/db/slowquery"

// Configure logger
config := slowquery.Config{
    Threshold:   100 * time.Millisecond,
    MaxQueryLen: 500,
    IncludeArgs: false, // Don't log sensitive data
    SampleRate:  1,    // Log all slow queries
}

logger := slowquery.New(config)

// Wrap existing functions
slowQuery := logger.WrapQuery(func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
    return db.QueryContext(ctx, query, args...)
})

slowExec := logger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
    return db.ExecContext(ctx, query, args...)
})

// Use wrapped functions
rows, _ := slowQuery(ctx, "SELECT * FROM users")
result, _ := slowExec(ctx, "UPDATE users SET name=?", "John")
```

### SQL Interceptor

```go
// Create interceptor with all operations
interceptor := slowquery.NewSQLInterceptor(
    logger,
    func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
        return db.QueryContext(ctx, query, args...)
    },
    func(ctx context.Context, query string, args ...any) *sql.Row {
        return db.QueryRowContext(ctx, query, args...)
    },
    func(ctx context.Context, query string, args ...any) (sql.Result, error) {
        return db.ExecContext(ctx, query, args...)
    },
)

// Use interceptor
rows, _ := interceptor.Query(ctx, "SELECT * FROM users")
result, _ := interceptor.Exec(ctx, "INSERT INTO logs VALUES (?)", "action")
```

### Manual Logging

```go
// Log custom operations
start := time.Now()
result, err := performComplexOperation()
duration := time.Since(start)

logger.LogManual("complex_operation", duration, err,
    zap.String("operation_id", "12345"),
    zap.Int("items_processed", count),
)
```

## Database Migration

### Creating Migrations

```go
import "golang-gin-rpc/pkg/db/migration"

// Create migrator
m := migration.New(db)

// Migration 1: Create users table
m.Add(1, "create_users",
    `CREATE TABLE users (
        id INT PRIMARY KEY AUTO_INCREMENT,
        name VARCHAR(255) NOT NULL,
        email VARCHAR(255) UNIQUE,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`,
    `DROP TABLE users;`)

// Migration 2: Create posts table with foreign key
m.Add(2, "create_posts",
    `CREATE TABLE posts (
        id INT PRIMARY KEY AUTO_INCREMENT,
        user_id INT NOT NULL,
        title VARCHAR(255) NOT NULL,
        content TEXT,
        FOREIGN KEY (user_id) REFERENCES users(id)
    );`,
    `DROP TABLE posts;`)

// Migration 3: Add index
m.Add(3, "add_email_index",
    `CREATE INDEX idx_email ON users(email);`,
    `DROP INDEX idx_email ON users;`)
```

### Running Migrations

```go
ctx := context.Background()

// Initialize migration table
if err := m.Init(ctx); err != nil {
    log.Fatal(err)
}

// Get current version
version, _ := m.GetCurrentVersion(ctx)
fmt.Printf("Current version: %d\n", version)

// Run all pending migrations
if err := m.Up(ctx); err != nil {
    log.Fatal("Migration failed:", err)
}

// Check status
statuses, _ := m.Status(ctx)
for _, s := range statuses {
    status := "pending"
    if s.Applied {
        status = "applied"
    }
    fmt.Printf("Migration %d (%s): %s\n", s.Version, s.Name, status)
}
```

### Rolling Back

```go
// Rollback last migration
if err := m.Down(ctx); err != nil {
    log.Fatal("Rollback failed:", err)
}
```

## Metrics Collection

### Recording Metrics

```go
import (
    "golang-gin-rpc/pkg/db/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Start metrics server
http.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":9090", nil)

// Record query duration
start := time.Now()
rows, err := db.Query(ctx, "SELECT * FROM users")
duration := time.Since(start).Seconds()

metrics.DBQueryDuration.With(prometheus.Labels{
    "database":  "mysql-main",
    "operation": "select",
    "table":     "users",
}).Observe(duration)

// Record query count
status := "success"
if err != nil {
    status = "error"
}
metrics.DBQueryTotal.With(prometheus.Labels{
    "database":  "mysql-main",
    "operation": "select",
    "status":    status,
}).Inc()

// Record slow query
if duration > 0.1 { // 100ms threshold
    metrics.DBSlowQueryTotal.With(prometheus.Labels{
        "database":  "mysql-main",
        "threshold": "100ms",
    }).Inc()
}
```

### Circuit Breaker Metrics

```go
// Record breaker state
metrics.CircuitBreakerState.With(prometheus.Labels{
    "name": "main-db",
}).Set(float64(breaker.State()))

// Record failures
if err != nil {
    metrics.CircuitBreakerFailures.With(prometheus.Labels{
        "name": "main-db",
    }).Inc()
}
```

### Connection Pool Metrics

```go
// Record pool size
metrics.DBConnectionPoolSize.With(prometheus.Labels{
    "database": "mysql-main",
    "type":     "master",
}).Set(float64(pool.Size()))

// Record active connections
stats := db.Stats()
metrics.DBConnectionActive.With(prometheus.Labels{
    "database": "mysql-main",
    "type":     "master",
}).Set(float64(stats.InUse + stats.Idle))
```

## Best Practices

### Error Handling

```go
client, err := pool.Acquire(ctx, "main-db")
if err != nil {
    if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
        // Use fallback
        return getFromCache(key), nil
    }
    return nil, fmt.Errorf("database error: %w", err)
}
defer client.Close()
```

### Context Usage

```go
// Set timeout for operations
timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

rows, err := client.Query(timeoutCtx, "SELECT * FROM large_table")
```

### Resource Cleanup

```go
// Always close resources
func processUser(ctx context.Context, id int) error {
    client, err := pool.Acquire(ctx, "main-db")
    if err != nil {
        return err
    }
    defer client.Close() // Important!

    rows, err := client.Query(ctx, "SELECT * FROM users WHERE id=?", id)
    if err != nil {
        return err
    }
    defer rows.Close() // Important!

    // Process rows...
    return nil
}
```

### Configuration Management

```go
// Use environment variables for sensitive data
dbPassword := os.Getenv("DB_PASSWORD")

// Load config from file
cfg, _ := db.LoadConfigFromYAML("configs/database.yaml")

// Override with environment
if password := os.Getenv("DB_PASSWORD"); password != "" {
    cfg.MySQL.Password = password
}
```

## Complete Example: Web Application

```go
package main

import (
    "context"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "golang-gin-rpc/pkg/db"
    "golang-gin-rpc/pkg/db/poolcb"
    "golang-gin-rpc/pkg/mysql"
)

type Server struct {
    dbPool *poolcb.PoolWithBreaker
}

func NewServer() (*Server, error) {
    // Load config
    cfg, err := db.LoadConfigFromYAML("configs/database.yaml")
    if err != nil {
        return nil, err
    }
    
    // Create pool with circuit breaker
    poolCfg := poolcb.Config{
        PoolConfig: pool.Config{
            MaxSize:           50,
            HealthCheckPeriod: 30 * time.Second,
        },
        BreakerConfig: circuitbreaker.Config{
            MaxFailures:  5,
            ResetTimeout: 30 * time.Second,
        },
    }
    
    p := poolcb.New(poolCfg, db.NewFactory())
    
    // Register main database
    if err := p.Register("main-db", cfg); err != nil {
        return nil, err
    }
    
    return &Server{dbPool: p}, nil
}

func (s *Server) GetUser(c *gin.Context) {
    id := c.Param("id")
    
    ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
    defer cancel()
    
    client, err := s.dbPool.Acquire(ctx, "main-db")
    if err != nil {
        if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
            c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer client.Close()
    
    sqlClient, ok := client.(db.SQLClient)
    if !ok {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid client type"})
        return
    }
    
    row := sqlClient.QueryRow(ctx, "SELECT id, name FROM users WHERE id=?", id)
    
    var user struct {
        ID   int
        Name string
    }
    if err := row.Scan(&user.ID, &user.Name); err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    
    c.JSON(http.StatusOK, user)
}

func main() {
    server, err := NewServer()
    if err != nil {
        panic(err)
    }
    defer server.dbPool.Close()
    
    r := gin.Default()
    r.GET("/users/:id", server.GetUser)
    r.Run(":8080")
}
```

---

For more examples, see the `examples/` directory in the repository.
