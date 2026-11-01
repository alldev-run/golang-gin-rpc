# Enterprise Transaction Manager Guide

## Overview

The Enterprise Transaction Manager provides production-ready transaction management with advanced features including distributed transactions, comprehensive monitoring, intelligent retry logic, and enterprise-grade configuration options.

## Key Features

### 🔧 Core Features
- **Intelligent Retry Logic**: Exponential backoff with jitter and error classification
- **Distributed Transactions**: Support for multi-database transactions
- **Connection Pool Management**: Enterprise-grade connection pooling
- **Nested Transactions**: Savepoint support for complex transaction flows
- **Timeout Management**: Configurable timeouts at multiple levels

### 📊 Monitoring & Observability
- **Real-time Metrics**: Transaction success rates, durations, retry counts
- **Distributed Tracing**: Integration with Jaeger and other tracing systems
- **Slow Query Detection**: Automatic logging of slow operations
- **Health Monitoring**: Transaction status and system health tracking

### 🛡️ Enterprise Features
- **Error Classification**: Intelligent error categorization and handling
- **Circuit Breaker Pattern**: Protection against cascading failures
- **Resource Management**: Memory and connection resource optimization
- **Security**: Transaction isolation and access control

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Enterprise Transaction Manager            │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Retry     │  │   Monitor   │  │   Tracer    │        │
│  │  Strategy   │  │             │  │             │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Error     │  │ Connection  │  │ Distributed │        │
│  │ Classifier  │  │    Pool     │  │ Coordinator │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
├─────────────────────────────────────────────────────────────┤
│                     Transaction Layer                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Primary    │  │   Nested    │  │   Retry     │        │
│  │ Transaction │  │ Transaction │  │   Logic     │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
├─────────────────────────────────────────────────────────────┤
│                    Database Layer                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │    MySQL    │  │ PostgreSQL  │  │   Redis     │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```go
config := orm.TransactionConfig{
    // Retry configuration
    MaxRetries:         5,
    RetryDelay:         50 * time.Millisecond,
    RetryBackoffFactor: 2.0,
    MaxRetryDelay:      10 * time.Second,
    
    // Transaction configuration
    IsolationLevel: sql.LevelReadCommitted,
    ReadOnly:       false,
    Timeout:        30 * time.Second,
    
    // Monitoring
    EnableMetrics:      true,
    EnableTracing:      true,
    LogSlowQueries:     true,
    SlowQueryThreshold: 500 * time.Millisecond,
    
    // Connection pool
    MaxOpenConns:    50,
    MaxIdleConns:    10,
    ConnMaxLifetime: 1 * time.Hour,
    ConnMaxIdleTime: 30 * time.Minute,
}

tm := orm.NewTransactionManager(config)
```

### High-Throughput Configuration

```go
highThroughputConfig := orm.TransactionConfig{
    MaxRetries:         2,
    RetryDelay:         25 * time.Millisecond,
    RetryBackoffFactor: 1.5,
    MaxRetryDelay:      2 * time.Second,
    IsolationLevel:     sql.LevelReadCommitted,
    Timeout:            10 * time.Second,
    EnableMetrics:      true,
    LogSlowQueries:     true,
    SlowQueryThreshold: 200 * time.Millisecond,
    MaxOpenConns:       100,
    MaxIdleConns:       20,
    ConnMaxLifetime:    30 * time.Minute,
    ConnMaxIdleTime:    5 * time.Minute,
}
```

### Distributed Transaction Configuration

```go
distributedConfig := orm.TransactionConfig{
    MaxRetries:         3,
    RetryDelay:         100 * time.Millisecond,
    RetryBackoffFactor: 2.0,
    MaxRetryDelay:      5 * time.Second,
    IsolationLevel:     sql.LevelSerializable,
    Timeout:            60 * time.Second,
    EnableDistributed:  true,
    CoordinatorURL:     "http://tx-coordinator:8080",
    EnableTracing:      true,
    EnableMetrics:      true,
}
```

## Usage Examples

### 1. Basic Transaction with Retry

```go
tm := orm.NewDefaultTransactionManager()
defer tm.Close()

// 注意:第二参数须为 orm.DB 接口实现(如 orm.NewDBWrapper(sqlDB) 或已有 ORM 的 o.DB());
// 传入裸 *sql.DB 会在运行时断言失败。
result, err := tm.WithTransaction(ctx, ormDB, func(txORM *orm.ORM) error {
    // ORM 是 SQL 构建器(无 model 式 Create/Update/Delete),用 builder 风格:
    if err := txORM.Insert("users").Sets(map[string]interface{}{
        "name":  "John",
        "email": "john@example.com",
    }).Exec(ctx); err != nil {
        return err
    }

    // More operations...
    return nil
})

if err != nil {
    log.Printf("Transaction failed: %v", err)
} else {
    log.Printf("Transaction succeeded in %v with %d retries", 
        result.Duration, result.Retries)
}
```

### 2. Nested Transactions with Savepoints

```go
err := tm.WithNestedTransaction(ctx, orm, "user_update_sp", func(nestedORM *orm.ORM) error {
    // Update user(builder 风格)
    if err := nestedORM.Update("users").
        Set("name", "Updated Name").
        Eq("id", 123).
        Exec(ctx); err != nil {
        return err
    }

    // Nested savepoint for audit
    err = tm.WithNestedTransaction(ctx, nestedORM, "audit_sp", func(auditORM *orm.ORM) error {
        return auditORM.Insert("audit_logs").Sets(map[string]interface{}{
            "user_id": 123,
            "action":  "update",
        }).Exec(ctx)
    })
    
    if err != nil {
        // Audit failed, but user update will be committed
        log.Printf("Audit failed: %v", err)
    }
    
    return nil
})
```

### 3. Enterprise Retry Logic

```go
err := tm.WithRetry(ctx, func() error {
    // Operation that might fail temporarily
    return externalAPICall()
})

if err != nil {
    log.Printf("Operation failed after retries: %v", err)
}
```

### 4. Monitoring and Metrics

```go
// Get current metrics
metrics := tm.GetMetrics()
log.Printf("Total transactions: %d", metrics.TotalTransactions)
log.Printf("Success rate: %.2f%%", 
    float64(metrics.SuccessfulTransactions)/float64(metrics.TotalTransactions)*100)
log.Printf("Average duration: %v", metrics.AvgDuration)
log.Printf("Active transactions: %d", tm.GetActiveTransactionCount())

// Reset metrics periodically
if time.Since(metrics.LastReset) > 24*time.Hour {
    tm.ResetMetrics()
}
```

## Error Classification

The enterprise transaction manager automatically classifies errors and applies appropriate retry strategies:

### Error Types

| Type | Retryable | Transient | Severity | Example |
|------|-----------|-----------|----------|---------|
| Connection | ✅ | ✅ | High | "connection refused" |
| Timeout | ✅ | ✅ | Medium | "query timeout exceeded" |
| Deadlock | ✅ | ✅ | High | "deadlock detected" |
| Constraint | ❌ | ❌ | Medium | "unique constraint violation" |
| Permission | ❌ | ❌ | Critical | "permission denied" |
| Resource | ❌ | ❌ | Critical | "out of memory" |
| Logic | ❌ | ❌ | Low | "invalid input data" |

### Retry Strategies

#### Exponential Backoff with Jitter
```go
delay = baseDelay * (backoffFactor ^ attempt) + jitter
```

#### Error-Specific Strategies
- **Deadlock**: Longer backoff (2^attempt * 100ms)
- **Timeout**: Moderate backoff (2^attempt * 200ms)
- **Connection**: Conservative backoff (1.5^attempt * 100ms)

## Monitoring and Observability

### Metrics Collection

The transaction manager automatically collects and reports metrics:

```go
// Prometheus metrics (if enabled)
orm_transactions_total
orm_transactions_success_total
orm_transactions_failed_total
orm_transactions_retried_total
orm_transaction_duration_seconds
```

### Distributed Tracing

Integration with Jaeger for distributed tracing:

```go
// Trace spans are automatically created
// Tags include:
// - transaction.id
// - transaction.isolation_level
// - transaction.read_only
// - retry.attempt
// - retry.delay
// - retry.error
```

### Slow Query Detection

Automatic logging of slow operations:

```go
// Configuration
LogSlowQueries: true
SlowQueryThreshold: 500 * time.Millisecond

// Output
WARN: Slow query detected transaction_id=tx_123 duration=1.2s query="SELECT * FROM users WHERE id = ?"
```

## Performance Optimization

### Connection Pool Tuning

```go
// High-throughput application
MaxOpenConns:    100,
MaxIdleConns:    20,
ConnMaxLifetime: 30 * time.Minute,
ConnMaxIdleTime: 5 * time.Minute,

// Low-latency application
MaxOpenConns:    25,
MaxIdleConns:    5,
ConnMaxLifetime: 1 * time.Hour,
ConnMaxIdleTime: 30 * time.Minute,
```

### Retry Optimization

```go
// For latency-sensitive operations
MaxRetries:         2,
RetryDelay:         25 * time.Millisecond,
RetryBackoffFactor: 1.5,
MaxRetryDelay:      2 * time.Second,

// For reliability-critical operations
MaxRetries:         5,
RetryDelay:         100 * time.Millisecond,
RetryBackoffFactor: 2.0,
MaxRetryDelay:      10 * time.Second,
```

### Isolation Level Selection

| Level | Performance | Consistency | Use Case |
|-------|-------------|-------------|----------|
| Read Uncommitted | Highest | Lowest | Analytics, reporting |
| Read Committed | High | Medium | General purpose |
| Repeatable Read | Medium | High | Financial operations |
| Serializable | Lowest | Highest | Critical operations |

## Best Practices

### 1. Configuration Management

```go
// Environment-specific configurations
func getTransactionConfig(env string) orm.TransactionConfig {
    switch env {
    case "production":
        return orm.TransactionConfig{
            MaxRetries:         3,
            RetryDelay:         100 * time.Millisecond,
            IsolationLevel:     sql.LevelReadCommitted,
            Timeout:            30 * time.Second,
            EnableMetrics:      true,
            EnableTracing:      true,
            LogSlowQueries:     true,
            SlowQueryThreshold: 1 * time.Second,
            MaxOpenConns:       50,
            MaxIdleConns:       10,
        }
    case "development":
        return orm.TransactionConfig{
            MaxRetries:         1,
            RetryDelay:         50 * time.Millisecond,
            IsolationLevel:     sql.LevelReadCommitted,
            Timeout:            10 * time.Second,
            EnableMetrics:      true,
            LogSlowQueries:     true,
            SlowQueryThreshold: 500 * time.Millisecond,
            MaxOpenConns:       10,
            MaxIdleConns:       2,
        }
    default:
        return orm.TransactionConfig{} // Use defaults
    }
}
```

### 2. Error Handling

```go
result, err := tm.WithTransaction(ctx, db, func(txORM *orm.ORM) error {
    if err := validateInput(data); err != nil {
        // Validation errors are not retryable
        return fmt.Errorf("validation failed: %w", err)
    }
    
    if err := processData(txORM, data); err != nil {
        // Let the transaction manager handle retry logic
        return err
    }
    
    return nil
})

if err != nil {
    // Check error classification(DefaultErrorClassifier 可独立构造)
    classifier := orm.NewDefaultErrorClassifier()
    if classifier.IsRetryable(err) {
        log.Printf("Retryable error: %v", err)
    } else {
        log.Printf("Non-retryable error: %v", err)
    }
    
    // Log detailed transaction information
    log.Printf("Transaction failed: tx_id=%s, retries=%d, duration=%v",
        result.TransactionID, result.Retries, result.Duration)
}
```

### 3. Resource Management

```go
// Proper cleanup
func handleRequest(w http.ResponseWriter, r *http.Request) {
    tm := orm.NewTransactionManager(getConfig())
    defer tm.Close()
    
    // Use transaction manager...
}

// Connection pool monitoring
func monitorConnectionPool(tm *orm.TransactionManager) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := tm.GetMetrics()
        if active := tm.GetActiveTransactionCount(); active > 100 {
            log.Warn("High number of active transactions", "count", active)
        }
        
        avgDuration := metrics.AvgDuration
        if avgDuration > 5*time.Second {
            log.Warn("High average transaction duration", "duration", avgDuration)
        }
    }
}
```

### 4. Distributed Transactions

```go
// Two-phase commit pattern
func executeDistributedTransaction(ctx context.Context, tm *orm.TransactionManager) error {
    result, err := tm.WithTransaction(ctx, primaryDB, func(primaryORM *orm.ORM) error {
        // Phase 1: Prepare
        if err := prepareSecondaryDB(ctx, secondaryDB); err != nil {
            return err
        }
        
        // Execute primary operations
        if err := executePrimaryOperations(primaryORM); err != nil {
            rollbackSecondaryDB(ctx, secondaryDB)
            return err
        }
        
        // Phase 2: Commit
        if err := commitSecondaryDB(ctx, secondaryDB); err != nil {
            return err
        }
        
        return nil
    })
    
    return err
}
```

## Troubleshooting

### Common Issues

1. **High Retry Rates**
   ```go
   // Check error classification(errorClassifier 为非导出字段,包外自行构造)
   classifier := orm.NewDefaultErrorClassifier()
   classification := classifier.ClassifyError(err)
   log.Printf("Error type: %s, retryable: %v", classification.Type, classification.Retryable)
   
   // Consider adjusting retry strategy
   config := tm.GetConfig()
   config.MaxRetries = 2 // Reduce retries
   tm.SetConfig(config)
   ```

2. **Slow Transactions**
   ```go
   // Enable slow query logging
   config := tm.GetConfig()
   config.LogSlowQueries = true
   config.SlowQueryThreshold = 500 * time.Millisecond
   tm.SetConfig(config)
   
   // Monitor metrics
   metrics := tm.GetMetrics()
   if metrics.AvgDuration > 1*time.Second {
       log.Warn("High average transaction duration", "avg", metrics.AvgDuration)
   }
   ```

3. **Connection Pool Exhaustion**
   ```go
   // Monitor active transactions
   active := tm.GetActiveTransactionCount()
   if active > 80 { // 80% of MaxOpenConns
       log.Warn("High connection pool usage", "active", active)
   }
   
   // Consider increasing pool size
   config := tm.GetConfig()
   config.MaxOpenConns = 100
   tm.SetConfig(config)
   ```

### Debug Mode

```go
// Enable comprehensive logging
config := orm.TransactionConfig{
    EnableMetrics:      true,
    EnableTracing:      true,
    LogSlowQueries:     true,
    SlowQueryThreshold: 100 * time.Millisecond, // Lower threshold for debugging
}

tm := orm.NewTransactionManager(config)
// monitor/errorClassifier 为非导出字段,包外无法注入自定义实现;
// 调试观测通过 EnableMetrics/EnableTracing/LogSlowQueries 配置项开启。
```

## Integration Examples

### With Web Services

```go
type UserService struct {
    tm *orm.TransactionManager
}

func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
    var user *User
    
    result, err := s.tm.WithTransaction(ctx, s.db, func(txORM *orm.ORM) error {
        // Validate input
        if err := validateUserRequest(req); err != nil {
            return err
        }
        
        // Create user(builder 风格)
        if err := txORM.Insert("users").Sets(map[string]interface{}{
            "name":  req.Name,
            "email": req.Email,
        }).Exec(ctx); err != nil {
            return err
        }

        // Create audit log
        return txORM.Insert("audit_logs").Sets(map[string]interface{}{
            "action": "create_user",
        }).Exec(ctx)
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }
    
    return user, nil
}
```

### With Microservices

```go
type OrderService struct {
    tm           *orm.TransactionManager
    paymentClient PaymentClient
    inventoryClient InventoryClient
}

func (s *OrderService) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    // WithTransaction 的 fn 签名固定为 func(*orm.ORM) error;需要的结果
    // (如 *Order)通过外部变量捕获。
    var order *Order

    err := s.tm.WithTransaction(ctx, s.db, func(txORM *orm.ORM) error {
        // Reserve inventory
        if err := s.inventoryClient.Reserve(ctx, req.Items); err != nil {
            return err
        }

        // Create order(builder 风格;ORM 无 model 式 Create/Delete)
        if err := txORM.Insert("orders").Sets(map[string]interface{}{
            "user_id": req.UserID,
            "items":   req.Items,
            "status":  "pending",
        }).Exec(ctx); err != nil {
            s.inventoryClient.Release(ctx, req.Items)
            return err
        }

        // Process payment
        payment, err := s.paymentClient.Process(ctx, &PaymentRequest{
            OrderID: order.ID,
            Amount:  req.Total(),
        })

        if err != nil {
            // fn 返回 error 即整体回滚,无需手工"删除已插入的行"
            s.inventoryClient.Release(ctx, req.Items)
            return err
        }

        // Update order status
        return txORM.Update("orders").
            Set("status", "paid").
            Set("payment_id", payment.ID).
            Eq("id", order.ID).
            Exec(ctx)
    })
    if err != nil {
        return nil, err
    }
    return order, nil
}
}
```

This enterprise transaction manager provides production-ready transaction management with comprehensive monitoring, intelligent retry logic, and support for complex transaction scenarios.
