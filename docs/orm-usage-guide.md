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
    db, err := sql.Open("mysql", "root:q1w2e3r4@tcp(localhost:3306)/myblog")
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

**注意**: 以下示例中的 `o` 变量表示已创建的 ORM 实例：

```go
// 假设已经创建了 ORM 实例
// o := orm.NewORMWithDB(db, orm.NewMySQLDialect())
// 或者从 bootstrap 获取
// o := orm.NewORMWithDB(mysqlClient.DB(), orm.NewMySQLDialect())
```

### SELECT 查询

#### 基础查询

```go
// 基本查询
query, args := o.Select("users").
    Columns("id", "name", "email").
    Eq("status", "active").
    Build()

// 执行查询
rows, err := o.DB().Query(ctx, query, args...)
defer rows.Close()

// 或者直接执行
rows, err := o.Select("users").
    Columns("id", "name", "email").
    Eq("status", "active").
    Query(ctx)
```

#### WHERE 条件构建

```go
// 简单条件
o.Select("users").Eq("status", "active")

// 多条件
o.Select("users").
    Eq("status", "active").
    Where("age > ?", 18).
    Where("name LIKE ?", "%John%")

// 复杂条件
o.Select("users").
    Where("(age BETWEEN ? AND ?) OR (status = ?)", 18, 65, "vip").
    Where("department IN ?", []string{"IT", "HR"})

// OR 条件
o.Select("users").
    Where("age > ?", 30).
    Or("department = ?", "IT").
    Or("experience > ?", 5)
```

#### JOIN 操作

```go
// 基本 JOIN
rows, err := o.Select("users u").
    Join("profiles p", "u.id = p.user_id").
    Columns("u.name", "p.bio").
    Query(ctx)

// LEFT JOIN
rows, err := o.Select("users u").
    LeftJoin("orders o", "u.id = o.user_id").
    Columns("u.name", "COUNT(o.id) as order_count").
    GroupBy("u.id").
    Query(ctx)

// 多表 JOIN
rows, err := o.Select("users u").
    Join("profiles p", "u.id = p.user_id").
    Join("departments d", "u.department_id = d.id").
    Columns("u.name", "p.bio", "d.name as department").
    Query(ctx)
```

#### 高级查询功能

```go
// CTE (Common Table Expression)
cte := o.Select("users").
    Columns("id", "name").
    Eq("status", "active")

rows, err := o.Select("active_users").
    With("active_users", cte).
    Columns("name", "COUNT(*) as order_count").
    Join("orders", "active_users.id = orders.user_id").
    GroupBy("active_users.id").
    Query(ctx)

// 子查询
subQuery := o.Select("orders").
    Columns("user_id").
    Where("total > ?", 1000)

subQuerySQL, subQueryArgs := subQuery.Build()
rows, err := o.Select("users").
    Columns("name").
    Where("id IN ("+subQuerySQL+")", subQueryArgs...).
    Query(ctx)

// UNION
query1 := o.Select("users").Columns("name", "email").Where("status = ?", "active")
query2 := o.Select("users").Columns("name", "email").Where("status = ?", "vip")

rows, err := query1.Union(query2).Query(ctx)
```

#### 分页和排序

```go
// 基础分页
rows, err := o.Select("users").
    Columns("id", "name", "email").
    OrderBy("created_at DESC").
    Limit(20).
    Offset(0).
    Query(ctx)

// 多字段排序
rows, err := o.Select("users").
    Columns("id", "name", "email").
    OrderBy("status ASC").
    OrderBy("name ASC").
    OrderBy("created_at DESC").
    Query(ctx)

// 动态排序
sortField := "name"
sortOrder := "DESC"
rows, err := o.Select("users").
    Columns("id", "name", "email").
    OrderBy(fmt.Sprintf("%s %s", sortField, sortOrder)).
    Query(ctx)
```

### INSERT 操作

#### 单行插入

```go
// 基础插入
result, err := o.Insert("users").
    Set("name", "Alice").
    Set("email", "alice@example.com").
    Set("age", 25).
    Exec(ctx)

id, err := result.LastInsertId()
affected, err := result.RowsAffected()
```

#### 批量插入

```go
// 批量插入
columns := []string{"name", "email", "age"}
rows := [][]interface{}{
    {"Bob", "bob@example.com", 30},
    {"Charlie", "charlie@example.com", 28},
    {"David", "david@example.com", 35},
}

result, err := o.Insert("users").
    Values(columns, rows...).
    Exec(ctx)

affected, err := result.RowsAffected()
fmt.Printf("Inserted %d rows\n", affected)
```

#### MySQL 特殊插入

```go
// INSERT IGNORE (忽略重复)
result, err := o.Insert("users").
    Set("name", "Alice").
    Set("email", "alice@example.com").
    Set("age", 25).
    Ignore().
    Exec(ctx)

// INSERT ... ON DUPLICATE KEY UPDATE (UPSERT)
result, err := o.Insert("users").
    Set("name", "Alice").
    Set("email", "alice@example.com").
    Set("age", 25).
    OnDuplicateKeyUpdate("age").
    Set("age", 26).
    Set("updated_at", "NOW()").
    Exec(ctx)

// REPLACE INTO
result, err := o.Insert("users").
    Set("name", "Alice").
    Set("email", "alice@example.com").
    Set("age", 25).
    Replace().
    Exec(ctx)
```

### UPDATE 操作

```go
// 基础更新
result, err := o.Update("users").
    Set("status", "inactive").
    Set("updated_at", time.Now()).
    Eq("id", 1).
    Exec(ctx)

// 条件更新
result, err := o.Update("users").
    Set("last_login", time.Now()).
    Where("last_login < ?", time.Now().AddDate(0, 0, -30)).
    Exec(ctx)

// 批量更新
result, err := o.Update("users").
    Set("status", "active").
    Where("department IN ?", []string{"IT", "HR"}).
    Exec(ctx)

// 使用辅助函数
affected, err := orm.Update(ctx, o.DB(), "UPDATE users SET status = ? WHERE id = ?", "active", 1)
```

### DELETE 操作

```go
// 基础删除
result, err := o.Delete("users").
    Eq("id", 1).
    Exec(ctx)

// 条件删除
result, err := o.Delete("users").
    Where("status = ?", "inactive").
    Where("last_login < ?", time.Now().AddDate(0, 0, -90)).
    Exec(ctx)

// 限制删除数量
result, err := o.Delete("users").
    Where("status = ?", "inactive").
    Limit(100).
    Exec(ctx)

affected, err := result.RowsAffected()
fmt.Printf("Deleted %d rows\n", affected)
```

## 事务管理

### 基础事务

```go
err := o.Transaction(ctx, func(txORM *orm.ORM) error {
    // 在事务中执行操作
    _, err := txORM.Insert("users").
        Set("name", "Alice").
        Set("email", "alice@example.com").
        Exec(ctx)
    if err != nil {
        return err // 自动回滚
    }

    _, err = txORM.Update("accounts").
        Set("balance", "balance - 100").
        Eq("user_id", 1).
        Exec(ctx)
    if err != nil {
        return err // 自动回滚
    }

    return nil // 自动提交
})

if err != nil {
    // 事务失败
    log.Printf("Transaction failed: %v", err)
} else {
    // 事务成功
    log.Println("Transaction completed successfully")
}
```

**注意**: 更高级的事务管理功能（如嵌套事务、保存点、重试机制）需要使用企业级的 `TransactionManager`，不在基础 ORM 范围内。

## 结构体扫描

### 定义结构体

```go
type User struct {
    ID        int       `db:"id" json:"id"`
    Name      string    `db:"name" json:"name"`
    Email     string    `db:"email" json:"email"`
    Age       int       `db:"age" json:"age"`
    Status    string    `db:"status" json:"status"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
```

### 扫描单条记录

```go
// 扫描到结构体
row := o.Select("users").
    Columns("id", "name", "email", "age", "status", "created_at", "updated_at").
    Eq("id", 1).
    QueryRow(ctx)

var user User
err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Age, &user.Status, &user.CreatedAt, &user.UpdatedAt)
if err != nil {
    return err
}

fmt.Printf("User: %+v\n", user)
```

### 扫描多条记录

```go
// 扫描到切片
rows, err := o.Select("users").
    Columns("id", "name", "email", "age", "status", "created_at", "updated_at").
    Eq("status", "active").
    Query(ctx)
if err != nil {
    return err
}
defer rows.Close()

var users []User
err = orm.StructScanAll(rows, &users)
if err != nil {
    return err
}

fmt.Printf("Found %d users\n", len(users))
```

### 自定义扫描

```go
// 扫描到 map
rows, err := o.Select("users").
    Columns("id", "name", "email").
    Query(ctx)
if err != nil {
    return err
}
defer rows.Close()

var results []map[string]interface{}
for rows.Next() {
    var id int
    var name, email string
    if err := rows.Scan(&id, &name, &email); err != nil {
        return err
    }
    
    results = append(results, map[string]interface{}{
        "id":    id,
        "name":  name,
        "email": email,
    })
}

// 扫描聚合结果
row := o.Select("users").
    Columns("COUNT(*) as total", "AVG(age) as avg_age").
    QueryRow(ctx)

var summary struct {
    Total  int     `db:"total"`
    AvgAge float64 `db:"avg_age"`
}
err = row.Scan(&summary.Total, &summary.AvgAge)
```

## 高级功能

### 软删除（企业级功能）

软删除功能需要使用独立的 `SoftDelete` 结构体：

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"

// 创建软删除实例
softDelete := orm.NewSoftDelete()

// 软删除记录
result, err := softDelete.SoftDelete(ctx, o.DB(), "users", "id", 1, 2, 3)
if err != nil {
    return err
}

// 恢复记录
result, err := softDelete.Restore(ctx, o.DB(), "users", "id", 1, 2)
if err != nil {
    return err
}

// 查询未删除的记录
rows, err := o.Select("users").
    Where("deleted_at IS NULL").
    Query(ctx)

// 查询已删除的记录
rows, err := o.Select("users").
    Where("deleted_at IS NOT NULL").
    Query(ctx)

// 清理软删除记录
result, err = softDelete.CleanSoftDeleted(ctx, o.DB(), "users")
```

### 乐观锁

```go
// 乐观锁更新
result, err := o.Update("products").
    Set("price", 99.99).
    Set("version", "version + 1").
    Eq("id", 1).
    Where("version = ?", currentVersion).
    Exec(ctx)

affected, err := result.RowsAffected()
if affected == 0 {
    return fmt.Errorf("optimistic lock conflict")
}
```

### 查询缓存（企业级功能）

查询缓存需要使用独立的缓存实现：

```go
import (
    "fmt"
    "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

// 创建缓存实例
cache := orm.NewMemoryCache()

// 创建带缓存的数据库包装
cachedDB := orm.NewCachedDB(o.DB(), cache)

// 使用缓存查询
rows, err := cachedDB.Query(ctx, "SELECT * FROM users WHERE status = ?", "active")

// 清除缓存
cache.Clear()
```

### SQL 日志（基础功能）

```go
// ORM 本身支持基础的查询日志
query, args := o.Select("users").
    Columns("id", "name").
    Eq("status", "active").
    Build()

fmt.Printf("SQL: %s\n", query)
fmt.Printf("Args: %v\n", args)

rows, err := o.DB().Query(ctx, query, args...)
```

## 数据库方言

### MySQL

```go
// MySQL 方言
dialect := orm.NewMySQLDialect()
o := orm.NewORMWithDB(db, dialect)

// MySQL 特定功能
result, err := o.Insert("users").
    Set("name", "Alice").
    Ignore(). // MySQL 特有
    Exec(ctx)
```

### PostgreSQL

```go
// PostgreSQL 方言
dialect := orm.NewPostgreSQLDialect()
o := orm.NewORMWithDB(db, dialect)

// PostgreSQL 特定功能
rows, err := o.Select("users").
    Columns("id", "name").
    Where("created_at > ?", time.Now().AddDate(0, 0, -7)).
    Build()
```

### ClickHouse

```go
// ClickHouse 方言
dialect := orm.NewClickHouseDialect()
o := orm.NewORMWithDB(db, dialect)

// ClickHouse 特定功能
rows, err := o.Select("events").
    Columns("event_type", "COUNT(*)").
    Where("timestamp >= ?", startDate).
    GroupBy("event_type").
    Build()
```

## 性能优化

### 连接池配置

```go
// 优化连接池
db.SetMaxOpenConns(25)                // 最大连接数
db.SetMaxIdleConns(10)                // 最大空闲连接数
db.SetConnMaxLifetime(time.Hour)      // 连接最大生存时间
db.SetConnMaxIdleTime(time.Minute * 30) // 空闲连接超时
```

### 批量操作

```go
// 批量插入优化
columns := []string{"name", "email", "age"}
batchSize := 100
for i := 0; i < len(users); i += batchSize {
    end := i + batchSize
    if end > len(users) {
        end = len(users)
    }
    
    rows := make([][]interface{}, end-i)
    for j, user := range users[i:end] {
        rows[j] = []interface{}{user.Name, user.Email, user.Age}
    }
    
    _, err := o.Insert("users").Values(columns, rows...).Exec(ctx)
}
```

### 预编译语句

```go
// 预编译常用查询
stmt, err := db.PrepareContext(ctx, "SELECT id, name FROM users WHERE id = ?")
if err != nil {
    return err
}
defer stmt.Close()

row := stmt.QueryRowContext(ctx, 1)
var user User
err = row.Scan(&user.ID, &user.Name)
```

## 错误处理和最佳实践

### 错误处理

```go
// 统一错误处理
result, err := o.Insert("users").
    Set("name", "Alice").
    Exec(ctx)
if err != nil {
    // 检查错误类型
    if strings.Contains(err.Error(), "Duplicate entry") {
        return fmt.Errorf("user already exists")
    }
    return fmt.Errorf("failed to create user: %w", err)
}

id, err := result.LastInsertId()
if err != nil {
    return fmt.Errorf("failed to get insert ID: %w", err)
}
```

### 最佳实践

1. **使用参数化查询**: 避免字符串拼接，防止 SQL 注入
2. **合理使用事务**: 保持事务简短，避免长时间锁定
3. **正确关闭资源**: 使用 `defer rows.Close()`
4. **批量操作**: 对于大量数据，使用批量插入/更新
5. **索引优化**: 根据查询模式创建合适的数据库索引
6. **连接池管理**: 合理配置连接池参数
7. **监控日志**: 记录慢查询和错误信息
8. **结构体映射**: 使用合适的 db tag 进行字段映射

## 完整示例集合

### 示例文件位置

所有完整的示例代码都可以在 `examples/mysql_usage/` 目录下找到：

- **`examples/mysql_usage/orm_crud_operations.go`** - 基础 CRUD 操作完整示例
- **`examples/mysql_usage/orm_advanced_operations.go`** - 高级操作完整示例  
- **`examples/mysql_usage/main_fixed.go`** - MySQL 集成完整示例
- **`examples/mysql_usage/test_examples.sh`** - 编译测试脚本

### 运行示例

```bash
# 进入示例目录
cd examples/mysql_usage

# 运行基础 CRUD 示例
go run orm_crud_operations.go

# 运行高级操作示例
go run orm_advanced_operations.go

# 运行 MySQL 集成示例
go run main_fixed.go

# 验证所有示例编译
./test_examples.sh
```

### ✅ 验证的示例特性

#### 1. `orm_crud_operations.go` - 基础 CRUD 操作
- ✅ **CREATE**: 单条插入、批量插入
- ✅ **READ**: 查询单条记录、查询多条记录、复杂条件查询、聚合查询
- ✅ **UPDATE**: 更新单个字段、更新多个字段、条件更新
- ✅ **DELETE**: 根据ID删除、条件删除

**特点**:
- 完整的错误处理
- 正确的单行扫描（`row.Scan()`）
- 正确的多行扫描（`orm.StructScanAll()`）
- 正确的 `defer rows.Close()` 使用
- 详细的注释和输出

#### 2. `orm_advanced_operations.go` - 高级操作
- ✅ **JOIN 查询**: INNER JOIN
- ✅ **子查询**: EXISTS、IN、FROM 子查询（使用正确的 Build() 方法）
- ✅ **分页和排序**: 基础分页、多字段排序、条件分页
- ✅ **聚合和分组**: 基础聚合、分组聚合
- ✅ **事务管理**: 基础事务、事务回滚

**特点**:
- 多表关联查询
- 复杂的 SQL 构建技巧
- 事务的完整生命周期管理
- 实际业务场景模拟
- 正确的子查询语法

#### 3. `main_fixed.go` - MySQL 集成示例
- ✅ **直接 MySQL 客户端**: 原生 SQL 操作
- ✅ **工厂模式**: 使用 db.Factory 创建客户端
- ✅ **ORM 集成**: MySQL + ORM 完整示例
- ✅ **事务管理**: ORM 事务使用
- ✅ **连接池**: 多连接池管理

**特点**:
- 多种连接方式的对比
- 完整的配置示例
- 连接池的使用方法
- 错误处理最佳实践
- 修正了原文件中的所有 API 错误

### 🔧 修正的问题

原文件中的错误已修正：
1. **API 调用错误**: `ormInstance.DB().ExecContext()` → `ormInstance.DB().Exec()`
2. **结构体扫描错误**: `orm.StructScan(row)` → `row.Scan()`（单行）
3. **子查询语法错误**: `WhereBuilder().ExistsSubquery()` → `Where("EXISTS (?)", subQuery)`
4. **原始 SQL 错误**: `orm.Raw()` → 直接字符串（Update 中）
5. **导入错误**: 移除未使用的 `database/sql` 导入

### 🚀 快速验证

运行测试脚本验证所有示例编译正常：

```bash
cd examples/mysql_usage
./test_examples.sh
```

预期输出：
```
=== 测试 ORM 示例文件编译 ===
1. 测试基础 CRUD 操作示例...
   ✓ orm_crud_operations.go 编译成功
2. 测试高级操作示例...
   ✓ orm_advanced_operations.go 编译成功
3. 测试 MySQL 集成示例...
   ✓ main_fixed.go 编译成功
```

### 📋 关键知识点总结

#### 1. 正确的单行查询
```go
row := ormInstance.Select("users").Eq("id", 1).QueryRow(ctx)
var user User
err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Age)
```

#### 2. 正确的多行查询
```go
rows, err := ormInstance.Select("users").Query(ctx)
defer rows.Close() // ✅ 确保关闭

var users []User
err := orm.StructScanAll(rows, &users)
```

#### 3. 正确的子查询
```go
subQuerySQL, subQueryArgs := ormInstance.Select("order_items").
    Columns("1").
    Eq("product_id", 1).
    Build()

rows, err := ormInstance.Select("products").
    Where("EXISTS ("+subQuerySQL+")", subQueryArgs...).
    Query(ctx)
```

#### 4. 正确的事务使用
```go
err := ormInstance.Transaction(ctx, func(txORM *orm.ORM) error {
    // 在事务中操作
    _, err := txORM.Insert("users").Set("name", "Alice").Exec(ctx)
    if err != nil {
        return err // 自动回滚
    }
    return nil // 自动提交
})
```

### ⚠️ 注意事项

- 所有示例都已通过编译验证
- 示例中的数据库连接配置需要根据实际环境调整（默认：root/q1w2e3r4@localhost:3306/myblog）
- 建议在测试环境中运行示例
- 如遇到问题，请检查 MySQL 服务状态和连接配置

### 📚 扩展学习

完成基础示例后，可以进一步学习：
1. 了解企业级功能（软删除、查询缓存、高级事务）
2. 学习性能优化技巧
3. 掌握错误处理最佳实践
4. 查看更多实际应用场景

---

*本指南涵盖了 ORM 的核心功能和最佳实践，帮助开发者快速上手并正确使用 ORM 进行数据库操作。*
