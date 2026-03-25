# ORM 使用详细指南

## 概述

`pkg/db/orm` 是一个轻量级、数据库无关的 SQL 查询构建器，提供事务管理和结构体扫描功能。它不是 ActiveRecord ORM（无模式迁移、无关系映射），而是专注于构建安全的 SQL 查询。

## 核心特性

- **数据库无关**: 支持 MySQL、PostgreSQL、SQLite、ClickHouse
- **类型安全**: 构建器模式避免 SQL 注入
- **查询构建**: SELECT、INSERT、UPDATE、DELETE 完整支持
- **高级查询**: CTE、子查询、JOIN、UNION 等
- **事务管理**: 简化事务处理
- **结构体扫描**: 自动映射查询结果到 Go 结构体

## 快速开始

### 1. 基本初始化

```go
package main

import (
    "context"
    "database/sql"
    
    bootstrap "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

func main() {
    // 方式1: 使用 bootstrap 获取 MySQL 客户端
    boot, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        panic(err)
    }
    defer boot.Close()

    if err := boot.InitializeDatabases(); err != nil {
        panic(err)
    }

    mysqlClient, err := bootstrap.GetMySQLClient(boot)
    if err != nil {
        panic(err)
    }

    // 创建 ORM 实例
    ormInstance := orm.NewORMWithDB(mysqlClient.DB(), orm.NewMySQLDialect())

    // 使用 ORM...
    useORM(ormInstance)
}

func useORM(o *orm.ORM) {
    ctx := context.Background()
    
    // 检查连接
    if err := o.Ping(ctx); err != nil {
        panic(err)
    }
    
    // 开始使用 ORM...
}
```

### 2. 直接使用 *sql.DB

```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

func directDB() {
    db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/dbname")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // 创建 ORM 实例
    o := orm.NewORMWithDB(db, orm.NewMySQLDialect())
    
    // 使用 ORM...
}
```

## 查询构建器详解

### SELECT 查询

#### 基础查询

```go
// 基本查询
query, args := o.Select("users").
    Columns("id", "name", "email").
    Eq("status", "active").
    OrderByDesc("created_at").
    Limit(10).
    Build()

// 生成的 SQL: SELECT `id`, `name`, `email` FROM `users` WHERE `status` = ? ORDER BY `created_at` DESC LIMIT 10
// 参数: [active]

// 执行查询
rows, err := o.Select("users").
    Columns("id", "name", "email").
    Eq("status", "active").
    Query(ctx)
if err != nil {
    return err
}
defer rows.Close()

// 扫描到结构体
var users []User
if err := orm.StructScanAll(rows, &users); err != nil {
    return err
}
```

#### WHERE 条件构建

```go
// 简单条件
sb := o.Select("users").
    Columns("id", "name").
    Eq("status", "active").           // WHERE status = ?
    Ne("role", "deleted").            // AND role != ?
    Gt("age", 18).                    // AND age > ?
    Like("name", "john%").            // AND name LIKE ?
    In("department", []string{"IT", "HR"}). // AND department IN (?, ?)
    IsNull("deleted_at").             // AND deleted_at IS NULL
    Between("created_at", "2024-01-01", "2024-12-31") // AND created_at BETWEEN ? AND ?

// 分组条件 (AND/OR 括号)
sb = o.Select("users").
    Columns("id", "name").
    Where("status = ?", "active").
    AndGroup(func(g *orm.WhereBuilder) {
        g.Where("role = ?", "admin").Or("level >= ?", 5)
    }).
    OrGroup(func(g *orm.WhereBuilder) {
        g.Where("department = ?", "IT").And("experience > ?", 3)
    })

// 生成的 SQL: 
// SELECT `id`, `name` FROM `users` 
// WHERE status = ? 
// AND (role = ? OR level >= ?) 
// OR (department = ? AND experience > ?)
```

#### JOIN 操作

```go
// 基本 JOIN
query, args := o.Select("users u").
    Columns("u.id", "u.name", "p.title").
    Join("profiles p", "u.id = p.user_id").
    LeftJoin("orders o", "u.id = o.user_id").
    Eq("u.status", "active").
    Build()

// JOIN ON 构建器（更安全）
query, args = o.Select("users u").
    Columns("u.id", "u.name").
    JoinOn("profiles p", func(on *orm.JoinOnBuilder) {
        on.Eq("u.id", "p.user_id").
           And("p.status = ?", "active").
           And("p.created_at > ?", "2024-01-01")
    }).
    Build()

// 子查询 JOIN
subQuery := o.Select("orders").
    Columns("user_id", "COUNT(*) as order_count").
    Where("status = ?", "completed").
    GroupBy("user_id")

query, args = o.Select("users u").
    Columns("u.id", "u.name", "o.order_count").
    JoinSubquery(subQuery, "o", "o.user_id = u.id").
    Build()
```

#### 高级查询功能

```go
// CTE (Common Table Expression)
cte := o.Select("orders").
    Columns("user_id", "COUNT(*) as order_count").
    Where("status = ?", "paid").
    GroupBy("user_id").
    Having("COUNT(*) > ?", 5)

query, args := o.Select("active_users").
    With("active_users", cte).
    Columns("user_id", "order_count").
    Where("order_count > ?", 10).
    OrderByDesc("order_count").
    Build()

// 子查询在 FROM 中
sub := o.Select("users").
    Columns("id").
    Where("status = ?", "active")

query, args := o.Select("ignored").
    FromSubquery(sub, "u").
    Columns("u.id").
    Where("u.id > ?", 100).
    Build()

// EXISTS 子查询
orderSub := o.Select("orders").
    Columns("1").
    Where("user_id = ?", 10).
    Where("status = ?", "pending")

query, args := o.Select("users").
    Columns("id", "name").
    WhereBuilder().ExistsSubquery(orderSub).
    Build()

// UNION / UNION ALL
usersQuery := o.Select("users").Columns("id", "name").Where("status = ?", "active")
adminsQuery := o.Select("admins").Columns("id", "name").Where("enabled = ?", true)

query, args := usersQuery.UnionAll(adminsQuery).
    AsDerived("t").
    Columns("t.id", "t.name").
    OrderByDesc("t.id").
    Limit(20).
    Build()

// 递归 CTE (层次查询)
seed := o.Select("categories").Columns("id", "parent_id", "name").Where("id = ?", 1)
recursive := o.Select("categories c").
    Columns("c.id", "c.parent_id", "c.name").
    Join("tree t", "c.parent_id = t.id")

query, args := o.Select("tree").
    WithRecursive("tree", seed, recursive).
    Columns("id", "parent_id", "name").
    FromRaw("tree").
    Build()
```

#### 分页和排序

```go
// 基础分页
query, args := o.Select("users").
    Columns("id", "name", "email").
    OrderBy("name ASC").           // ORDER BY name ASC
    OrderByDesc("created_at").     // ORDER BY created_at DESC
    Limit(20).                     // LIMIT 20
    Offset(40).                    // OFFSET 40
    Build()

// 使用分页构建器（更便捷）
paginator := orm.NewPaginator(1, 20) // page=1, pageSize=20
query, args := o.Select("users").
    Columns("id", "name", "email").
    OrderBy("name ASC").
    ApplyPagination(paginator).
    Build()

// 获取总数和分页信息
countQuery, countArgs := o.Select("users").
    Columns("COUNT(*) as total").
    Build()

total, err := getCount(countQuery, countArgs) // 执行查询获取总数
paginator.SetTotal(total)
```

### INSERT 操作

#### 单行插入

```go
// 基础插入
result, err := o.Insert("users").
    Set("name", "Alice").
    Set("email", "alice@example.com").
    Set("age", 25).
    Set("status", "active").
    Exec(ctx)

// 获取插入的 ID
insertID, err := result.LastInsertId()
if err != nil {
    return err
}

// 使用 InsertGetID 辅助函数
id, err := orm.InsertGetID(ctx, o.DB(), 
    "INSERT INTO users (name, email) VALUES (?, ?)", 
    "Alice", "alice@example.com")
```

#### 批量插入

```go
// 批量插入
columns := []string{"name", "email", "age", "status"}
rows := [][]interface{}{
    {"Alice", "alice@example.com", 25, "active"},
    {"Bob", "bob@example.com", 30, "active"},
    {"Charlie", "charlie@example.com", 28, "inactive"},
}

result, err := o.Insert("users").
    Values(columns, rows...).
    Exec(ctx)

// 检查插入的行数
affected, err := result.RowsAffected()
if err != nil {
    return err
}
fmt.Printf("Inserted %d rows\n", affected)
```

#### MySQL 特殊插入

```go
// INSERT IGNORE (忽略重复)
result, err := o.Insert("users").
    Ignore().
    Set("id", 1).
    Set("name", "Alice").
    Exec(ctx)

// REPLACE INTO (替换重复)
result, err := o.Insert("users").
    Replace().
    Set("id", 1).
    Set("name", "Alice Updated").
    Exec(ctx)

// ON DUPLICATE KEY UPDATE (Upsert)
result, err := o.Insert("users").
    Set("id", 1).
    Set("name", "Alice").
    Set("email", "alice@new.com").
    OnDuplicateKeyUpdate("name", "email"). // 更新指定字段
    Exec(ctx)

// 或者使用 VALUES() 函数
result, err = o.Insert("users").
    Set("id", 1).
    Set("name", "Alice").
    Set("counter", 1).
    OnDuplicateKeyUpdate("counter", "VALUES(counter) + 1").
    Exec(ctx)
```

### UPDATE 操作

```go
// 基础更新
result, err := o.Update("users").
    Set("name", "Alice Updated").
    Set("email", "alice@new.com").
    Eq("id", 1).
    Exec(ctx)

// 条件更新
result, err = o.Update("users").
    Set("status", "inactive").
    Set("updated_at", time.Now()).
    Where("last_login < ?", time.Now().AddDate(0, -6, 0)). // 6个月未登录
    Exec(ctx)

// 带排序和限制的更新（MySQL）
result, err = o.Update("users").
    Set("status", "expired").
    OrderBy("created_at ASC").
    Limit(100).
    Where("status = ?", "pending").
    Exec(ctx)

// 使用 Update 辅助函数
affected, err := orm.Update(ctx, o.DB(), 
    "UPDATE users SET status = ? WHERE created_at < ?", 
    "inactive", time.Now().AddDate(0, -1, 0))

// 单字段更新
affected, err = orm.SetFieldByID(ctx, o.DB(), 
    "users", "id", 1, "status", "active")
```

### DELETE 操作

```go
// 基础删除
result, err := o.Delete("users").
    Eq("id", 1).
    Exec(ctx)

// 条件删除
result, err = o.Delete("users").
    Where("status = ?", "inactive").
    Where("created_at < ?", time.Now().AddDate(-1, 0, 0)). // 1年前创建
    Exec(ctx)

// 限制删除数量
result, err = o.Delete("users").
    Where("status = ?", "spam").
    OrderBy("created_at ASC").
    Limit(1000).
    Exec(ctx)

// 检查删除的行数
affected, err := result.RowsAffected()
if err != nil {
    return err
}
fmt.Printf("Deleted %d rows\n", affected)
```

## 事务管理

### 基础事务

```go
err := o.Transaction(ctx, func(txORM *orm.ORM) error {
    // 在事务中执行操作
    
    // 插入用户
    _, err := txORM.Insert("users").
        Set("name", "Alice").
        Set("email", "alice@example.com").
        Exec(ctx)
    if err != nil {
        return err // 自动回滚
    }
    
    // 更新统计
    _, err = txORM.Update("user_stats").
        Set("total_users", orm.Raw("total_users + 1")).
        Exec(ctx)
    if err != nil {
        return err // 自动回滚
    }
    
    // 插入订单
    _, err = txORM.Insert("orders").
        Set("user_id", 1).
        Set("amount", 100.00).
        Set("status", "pending").
        Exec(ctx)
    if err != nil {
        return err // 自动回滚
    }
    
    return nil // 自动提交
})

if err != nil {
    fmt.Printf("Transaction failed: %v\n", err)
} else {
    fmt.Println("Transaction completed successfully")
}
```

### 嵌套事务和保存点

```go
// 使用事务管理器进行更复杂的操作
tm := orm.NewTransactionManager(o)

err := tm.ExecuteInTransaction(ctx, func(tx *orm.Transaction) error {
    // 主事务操作
    _, err := tx.Insert("users").
        Set("name", "Alice").
        Exec(ctx)
    if err != nil {
        return err
    }
    
    // 创建保存点
    sp, err := tx.CreateSavepoint("before_orders")
    if err != nil {
        return err
    }
    
    // 尝试插入订单（可能失败）
    _, err = tx.Insert("orders").
        Set("user_id", 1).
        Set("amount", -100.00). // 无效金额
        Exec(ctx)
    if err != nil {
        // 回滚到保存点，但不回滚整个事务
        if rollbackErr := tx.RollbackToSavepoint(sp); rollbackErr != nil {
            return rollbackErr
        }
        fmt.Println("Rolled back to savepoint, continuing transaction")
    }
    
    // 继续其他操作
    _, err = tx.Insert("user_profiles").
        Set("user_id", 1).
        Set("bio", "New user").
        Exec(ctx)
    if err != nil {
        return err
    }
    
    return nil
})
```

### 事务隔离级别

```go
// 设置事务隔离级别
err := o.Transaction(ctx, func(txORM *orm.ORM) error {
    // 使用自定义事务选项
    return nil
}, &sql.TxOptions{
    Isolation: sql.LevelSerializable,
    ReadOnly:  false,
})

// 或者使用事务管理器
tm := orm.NewTransactionManager(o)
err = tm.ExecuteInTransactionWithOptions(ctx, func(tx *orm.Transaction) error {
    return tx.Update("accounts").
        Set("balance", orm.Raw("balance - ?")).
        Eq("id", 1).
        Exec(ctx)
}, &sql.TxOptions{
    Isolation: sql.LevelReadCommitted,
})
```

## 结构体扫描

### 定义结构体

```go
type User struct {
    ID        int64     `db:"id" json:"id"`
    Name      string    `db:"name" json:"name"`
    Email     string    `db:"email" json:"email"`
    Age       int       `db:"age" json:"age"`
    Status    string    `db:"status" json:"status"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
    
    // 不映射到数据库的字段
    Password string `db:"-" json:"-"`
    TempInfo string `json:"temp_info"`
}

// 嵌套结构体
type UserProfile struct {
    User
    Bio        string    `db:"bio" json:"bio"`
    Avatar     string    `db:"avatar" json:"avatar"`
    LastLogin  time.Time `db:"last_login" json:"last_login"`
}
```

### 扫描单条记录

```go
// 扫描到结构体
var user User
row := o.Select("users").
    Columns("id", "name", "email", "age", "status", "created_at").
    Eq("id", 1).
    QueryRow(ctx)

err := orm.StructScan(row, &user)
if err != nil {
    if err == sql.ErrNoRows {
        return fmt.Errorf("user not found")
    }
    return err
}

fmt.Printf("User: %+v\n", user)
```

### 扫描多条记录

```go
// 扫描到切片
var users []User
rows, err := o.Select("users").
    Columns("id", "name", "email", "age", "status", "created_at").
    Eq("status", "active").
    OrderBy("name ASC").
    Query(ctx)
if err != nil {
    return err
}
defer rows.Close()

err = orm.StructScanAll(rows, &users)
if err != nil {
    return err
}

for _, user := range users {
    fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
}
```

### 自定义扫描

```go
// 扫描到 map
rows, err := o.Select("users").
    Columns("id", "name", "email").
    Limit(10).
    Query(ctx)
if err != nil {
    return err
}
defer rows.Close()

var results []map[string]interface{}
for rows.Next() {
    result := make(map[string]interface{})
    if err := rows.MapScan(result); err != nil {
        return err
    }
    results = append(results, result)
}

// 扫描到自定义字段
type UserSummary struct {
    ID       int64  `db:"id"`
    FullName string `db:"full_name"`
}

var summaries []UserSummary
rows, err := o.Select("users").
    Columns("id", orm.Raw("CONCAT(name, ' (', email, ')') as full_name")).
    Query(ctx)
if err != nil {
    return err
}
defer rows.Close()

err = orm.StructScanAll(rows, &summaries)
```

## 高级功能

### 软删除

```go
// 软删除构建器
softDelete := orm.NewSoftDeleteBuilder("users", "deleted_at")

// 软删除（设置 deleted_at）
result, err := softDelete.Delete(1).Exec(ctx) // WHERE id = 1

// 条件软删除
result, err = softDelete.
    Where("status = ?", "inactive").
    DeleteAll().
    Exec(ctx)

// 查询未删除记录（自动排除软删除）
query, args := softDelete.
    Select().
    Columns("id", "name").
    Eq("status", "active").
    Build()

// 查询包含软删除记录
query, args = softDelete.
    SelectWithDeleted().
    Columns("id", "name", "deleted_at").
    Build()

// 恢复软删除记录
result, err = softDelete.
    Restore(1).
    Exec(ctx)

// 物理删除
result, err = softDelete.
    ForceDelete(1).
    Exec(ctx)
```

### 乐观锁

```go
// 乐观锁更新
result, err := o.Update("users").
    Set("name", "Alice Updated").
    Set("email", "alice@new.com").
    Eq("id", 1).
    Eq("version", 5). // 指定当前版本
    Exec(ctx)

affected, err := result.RowsAffected()
if err != nil {
    return err
}

if affected == 0 {
    return fmt.Errorf("update failed: record not found or version mismatch")
}

// 使用 Save 函数（自动处理乐观锁）
data := map[string]interface{}{
    "name":    "Alice Updated",
    "email":   "alice@new.com",
    "version": 6, // 新版本
}

affected, err = orm.Save(ctx, o.DB(), "users", "id", 1, data)
if err != nil {
    return err
}

if affected == 0 {
    return fmt.Errorf("save failed: optimistic lock conflict")
}
```

### 查询缓存

```go
// 创建查询缓存
cache := orm.NewQueryCache(redisClient, 5*time.Minute)

// 缓存查询结果
key := "users:active:page:1"
var users []User

err := cache.GetOrSet(ctx, key, &users, func() (interface{}, error) {
    rows, err := o.Select("users").
        Columns("id", "name", "email").
        Eq("status", "active").
        OrderBy("name ASC").
        Limit(20).
        Query(ctx)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var result []User
    if err := orm.StructScanAll(rows, &result); err != nil {
        return nil, err
    }
    return result, nil
})

if err != nil {
    return err
}

// 清除缓存
err = cache.Delete(ctx, key)
if err != nil {
    return err
}

// 按模式清除缓存
err = cache.DeleteByPattern(ctx, "users:*")
```

### SQL 日志

```go
// 创建 SQL 日志器
logger := orm.NewSQLLogger(os.Stdout, orm.LogLevelInfo)

// 包装 ORM 实例
loggedORM := orm.NewLoggedORM(o, logger)

// 所有查询都会被记录
result, err := loggedORM.Insert("users").
    Set("name", "Alice").
    Exec(ctx)

// 自定义日志格式
customLogger := orm.NewSQLLoggerWithFormatter(
    os.Stdout,
    func(ctx context.Context, query string, args []interface{}, duration time.Duration, err error) {
        fmt.Printf("[%s] Query: %s, Args: %v, Duration: %v, Error: %v\n",
            time.Now().Format("2006-01-02 15:04:05"),
            query, args, duration, err)
    },
)
```

## 数据库方言

### MySQL

```go
// MySQL 方言
mysqlDialect := orm.NewMySQLDialect()

// MySQL 特定功能
query, args := o.Select("accounts").
    Columns("id", "balance").
    Eq("id", 1).
    ForUpdateNowait(). // FOR UPDATE NOWAIT
    Build()

// MySQL 8+ 的 SKIP LOCKED
query, args = o.Select("accounts").
    Columns("id", "balance").
    ForUpdateSkipLocked(). // FOR UPDATE SKIP LOCKED
    Build()

// MySQL 的 FULLTEXT 搜索
query, args = o.Select("articles").
    Columns("id", "title").
    Where("MATCH(title, content) AGAINST(?)", "search term").
    Build()
```

### PostgreSQL

```go
// PostgreSQL 方言
pgDialect := orm.NewPostgreSQLDialect()

// PostgreSQL 特定功能
query, args := o.Select("users").
    Columns("id", "name").
    Where("email ILIKE ?", "%@example.com"). // 不区分大小写的 LIKE
    Build()

// PostgreSQL 的 JSON 查询
query, args = o.Select("documents").
    Columns("id", "title").
    Where("metadata->>'type' = ?", "article").
    Build()

// PostgreSQL 的数组查询
query, args = o.Select("users").
    Columns("id", "name").
    Where("tags && ?", []string{"important", "urgent"}). // 数组交集
    Build()
```

### ClickHouse

```go
// ClickHouse 方言
chDialect := orm.NewClickHouseDialect()

// ClickHouse 特定功能
query, args := o.Select("events").
    Columns("event_id", "timestamp", "user_id").
    Gte("timestamp", "2024-01-01").
    Lt("timestamp", "2024-02-01").
    OrderBy("timestamp DESC").
    Limit(1000).
    Build()

// ClickHouse 的聚合查询
query, args = o.Select("events").
    Columns("user_id", "COUNT(*) as event_count", "SUM(duration) as total_duration").
    Gte("timestamp", "2024-01-01").
    GroupBy("user_id").
    Having("COUNT(*) > ?", 10).
    OrderByDesc("event_count").
    Build()
```

## 性能优化

### 连接池配置

```go
// 优化连接池
db.SetMaxOpenConns(100)        // 最大连接数
db.SetMaxIdleConns(20)         // 最大空闲连接数
db.SetConnMaxLifetime(time.Hour) // 连接最大生命周期
db.SetConnMaxIdleTime(time.Minute * 30) // 空闲连接超时
```

### 批量操作

```go
// 批量插入优化
batchSize := 1000
for i := 0; i < len(users); i += batchSize {
    end := i + batchSize
    if end > len(users) {
        end = len(users)
    }
    
    batch := users[i:end]
    _, err := o.Insert("users").
        Values([]string{"name", "email", "age"}, convertToInterfaceSlice(batch)...).
        Exec(ctx)
    if err != nil {
        return err
    }
}
```

### 预编译语句

```go
// 预编译常用查询
stmt, err := o.DB().PrepareContext(ctx, 
    "SELECT id, name, email FROM users WHERE status = ? ORDER BY name LIMIT ?")
if err != nil {
    return err
}
defer stmt.Close()

// 使用预编译语句
rows, err := stmt.QueryContext(ctx, "active", 50)
if err != nil {
    return err
}
defer rows.Close()
```

## 错误处理和最佳实践

### 错误处理

```go
// 统一错误处理
func handleDBError(err error) error {
    if err == nil {
        return nil
    }
    
    switch {
    case errors.Is(err, sql.ErrNoRows):
        return fmt.Errorf("record not found")
    case errors.Is(err, sql.ErrTxDone):
        return fmt.Errorf("transaction already completed")
    case errors.Is(err, sql.ErrConnDone):
        return fmt.Errorf("database connection closed")
    default:
        return fmt.Errorf("database error: %w", err)
    }
}

// 使用示例
var user User
err := orm.StructScan(row, &user)
if err := handleDBError(err); err != nil {
    return err
}
```

### 最佳实践

1. **使用参数化查询**: 避免字符串拼接，防止 SQL 注入
2. **合理使用事务**: 保持事务简短，避免长时间锁定
3. **连接池管理**: 根据应用负载调整连接池大小
4. **错误处理**: 区分不同类型的数据库错误
5. **结构体映射**: 使用 `db` 标签明确字段映射关系
6. **分页查询**: 对大数据集使用分页，避免内存溢出
7. **索引优化**: 根据查询模式创建合适的数据库索引
8. **监控日志**: 记录慢查询和错误信息

## 完整示例

### Repository 模式示例

```go
package repository

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"
    
    "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

type UserRepository struct {
    orm *orm.ORM
}

func NewUserRepository(ormInstance *orm.ORM) *UserRepository {
    return &UserRepository{orm: ormInstance}
}

type User struct {
    ID        int64     `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    Age       int       `db:"age"`
    Status    string    `db:"status"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    result, err := r.orm.Insert("users").
        Set("name", user.Name).
        Set("email", user.Email).
        Set("age", user.Age).
        Set("status", user.Status).
        Set("created_at", time.Now()).
        Exec(ctx)
    if err != nil {
        return fmt.Errorf("create user failed: %w", err)
    }
    
    id, err := result.LastInsertId()
    if err != nil {
        return fmt.Errorf("get insert id failed: %w", err)
    }
    
    user.ID = id
    return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
    row := r.orm.Select("users").
        Columns("id", "name", "email", "age", "status", "created_at", "updated_at").
        Eq("id", id).
        QueryRow(ctx)
    
    var user User
    err := orm.StructScan(row, &user)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, fmt.Errorf("user %d not found", id)
        }
        return nil, fmt.Errorf("scan user failed: %w", err)
    }
    
    return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *User) error {
    result, err := r.orm.Update("users").
        Set("name", user.Name).
        Set("email", user.Email).
        Set("age", user.Age).
        Set("status", user.Status).
        Set("updated_at", time.Now()).
        Eq("id", user.ID).
        Exec(ctx)
    if err != nil {
        return fmt.Errorf("update user failed: %w", err)
    }
    
    affected, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("get affected rows failed: %w", err)
    }
    
    if affected == 0 {
        return fmt.Errorf("user %d not found", user.ID)
    }
    
    return nil
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
    result, err := r.orm.Delete("users").
        Eq("id", id).
        Exec(ctx)
    if err != nil {
        return fmt.Errorf("delete user failed: %w", err)
    }
    
    affected, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("get affected rows failed: %w", err)
    }
    
    if affected == 0 {
        return fmt.Errorf("user %d not found", id)
    }
    
    return nil
}

func (r *UserRepository) List(ctx context.Context, opts ListOptions) ([]*User, error) {
    query := r.orm.Select("users").
        Columns("id", "name", "email", "age", "status", "created_at", "updated_at")
    
    // 应用过滤条件
    if opts.Status != "" {
        query = query.Eq("status", opts.Status)
    }
    if opts.MinAge > 0 {
        query = query.Gte("age", opts.MinAge)
    }
    if opts.MaxAge > 0 {
        query = query.Lte("age", opts.MaxAge)
    }
    if opts.Search != "" {
        query = query.Where("(name LIKE ? OR email LIKE ?)", 
            "%"+opts.Search+"%", "%"+opts.Search+"%")
    }
    
    // 应用排序
    if opts.SortBy != "" {
        direction := "ASC"
        if opts.SortDesc {
            direction = "DESC"
        }
        query = query.OrderBy(fmt.Sprintf("%s %s", opts.SortBy, direction))
    } else {
        query = query.OrderBy("created_at DESC")
    }
    
    // 应用分页
    if opts.Limit > 0 {
        query = query.Limit(opts.Limit)
        if opts.Offset > 0 {
            query = query.Offset(opts.Offset)
        }
    }
    
    rows, err := query.Query(ctx)
    if err != nil {
        return nil, fmt.Errorf("query users failed: %w", err)
    }
    defer rows.Close()
    
    var users []*User
    if err := orm.StructScanAll(rows, &users); err != nil {
        return nil, fmt.Errorf("scan users failed: %w", err)
    }
    
    return users, nil
}

func (r *UserRepository) Count(ctx context.Context, opts ListOptions) (int64, error) {
    query := r.orm.Select("users").Columns("COUNT(*)")
    
    // 应用相同的过滤条件
    if opts.Status != "" {
        query = query.Eq("status", opts.Status)
    }
    if opts.MinAge > 0 {
        query = query.Gte("age", opts.MinAge)
    }
    if opts.MaxAge > 0 {
        query = query.Lte("age", opts.MaxAge)
    }
    if opts.Search != "" {
        query = query.Where("(name LIKE ? OR email LIKE ?)", 
            "%"+opts.Search+"%", "%"+opts.Search+"%")
    }
    
    row := query.QueryRow(ctx)
    var count int64
    if err := row.Scan(&count); err != nil {
        return 0, fmt.Errorf("scan count failed: %w", err)
    }
    
    return count, nil
}

func (r *UserRepository) TransferBalance(ctx context.Context, fromID, toID int64, amount float64) error {
    return r.orm.Transaction(ctx, func(txORM *orm.ORM) error {
        // 扣除发送者余额
        result, err := txORM.Update("accounts").
            Set("balance", orm.Raw("balance - ?")).
            Set("updated_at", time.Now()).
            Eq("user_id", fromID).
            Where("balance >= ?", amount). // 检查余额充足
            Exec(ctx)
        if err != nil {
            return err
        }
        
        affected, err := result.RowsAffected()
        if err != nil {
            return err
        }
        if affected == 0 {
            return fmt.Errorf("insufficient balance for user %d", fromID)
        }
        
        // 增加接收者余额
        _, err = txORM.Update("accounts").
            Set("balance", orm.Raw("balance + ?")).
            Set("updated_at", time.Now()).
            Eq("user_id", toID).
            Exec(ctx)
        if err != nil {
            return err
        }
        
        // 记录转账记录
        _, err = txORM.Insert("transfers").
            Set("from_user_id", fromID).
            Set("to_user_id", toID).
            Set("amount", amount).
            Set("status", "completed").
            Set("created_at", time.Now()).
            Exec(ctx)
        if err != nil {
            return err
        }
        
        return nil
    })
}

type ListOptions struct {
    Status   string
    MinAge   int
    MaxAge   int
    Search   string
    SortBy   string
    SortDesc bool
    Limit    int
    Offset   int
}
```

这个详细的 ORM 使用指南涵盖了从基础使用到高级功能的各个方面，为开发者提供了完整的参考资料。
