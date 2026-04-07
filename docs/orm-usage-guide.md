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

## 全局 Helper 快速使用（推荐）

`pkg/db/helper.go` 提供了更简洁的全局数据库访问方式，无需手动创建 ORM 实例，在 bootstrap 初始化后即可直接使用。

### 3. 使用全局 Helper

```go
package main

import (
    "context"
    
    bootstrap "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/db"
)

func main() {
    // 初始化 bootstrap（自动设置全局 factory）
    boot, err := bootstrap.NewBootstrap("configs/config.yaml")
    if err != nil {
        panic(err)
    }
    defer boot.Close()

    if err := boot.InitializeAll(); err != nil {
        panic(err)
    }

    // 直接使用全局 helper，无需创建 ORM 实例
    useHelper()
}

func useHelper() {
    ctx := context.Background()
    
    // 查询示例
    rows, err := db.Select("users").
        Columns("id", "name", "email").
        Eq("status", 1).
        Query(ctx)
    if err != nil {
        panic(err)
    }
    defer rows.Close()
    
    // 插入示例
    _, err = db.Insert("users").Sets(map[string]interface{}{
        "name":  "Alice",
        "email": "alice@example.com",
    }).Exec(ctx)
    
    // 更新示例
    _, err = db.Update("users").
        Set("name", "Bob").
        Eq("id", 1).
        Exec(ctx)
    
    // 删除示例
    _, err = db.Delete("users").
        Eq("id", 1).
        Exec(ctx)
}
```

### 多数据库支持

同时访问 MySQL 和 PostgreSQL：

```go
// MySQL（默认，无需指定）
users, err := db.Select("users").
    Columns("id", "name").
    Query(ctx)

// PostgreSQL（显式指定）
orders, err := db.Using(db.DBTypePostgres).Select("orders").
    Columns("id", "amount").
    Query(ctx)

// 或使用专用函数
orders, err := db.PostgresSelect("orders").
    Eq("status", "pending").
    Query(ctx)
```

### 同一 MySQL 实例多数据库访问

配置中的 `database` 作为默认库。对于多数据库访问，**推荐使用新的 goroutine-safe API**，避免并发问题。

```go
// 配置示例 (database.yml):
// mysql_primary:
//   type: mysql
//   mysql:
//     host: localhost
//     database: userdb  // 默认库

// 默认使用配置中的 database
users, err := db.Select("users").Query(ctx)  // 查询 userdb.users

// ✅ 推荐：goroutine-safe 方式，直接指定数据库
orders, err := db.SelectDB("orderdb", "orders").Query(ctx)  // 查询 orderdb.orders
logs, err := db.InsertDB("logdb", "logs").Sets(...).Exec(ctx)

// ✅ 推荐：使用 On() 链式调用（fluent API）
rows, err := db.On("orderdb").Select("orders").Where("status = ?", "pending").Query(ctx)

// ✅ 推荐：Context 模式（适合在 middleware 中设置数据库）
ctx := db.WithDBContext(r.Context(), "orderdb")
// 后续查询会自动使用该数据库（需要配合支持 context 的方法）
```

#### ⚠️ 旧版 Use() / UseDefault()（极不推荐用于并发环境）

```go
// 🚨 极度危险：以下方式在并发环境下（HTTP handlers、goroutines）会有严重竞态条件！
// 仅适用于单协程脚本或初始化流程，绝对不要在生产环境使用！

// 切换到 orderdb（影响全局状态）
db.Use("orderdb")
orders, err := db.Select("orders").Query(ctx)  // 可能查询到错误的数据库！

// 写日志到 logdb
db.Use("logdb")
_, err = db.Insert("logs").Sets(...).Exec(ctx)

// 恢复默认库
db.UseDefault()  // 或 db.Use("")
users, err = db.Select("users").Query(ctx)  // 可能查询到错误的数据库！
```

**为什么 `Use()` 在并发下极度危险？**

`db.Use()` 修改全局变量 `currentDB`，所有 goroutine 共享这个状态：

```go
// 🚨 严重竞态条件示例：
// Goroutine A: db.Use("orderdb")
// Goroutine B: db.Use("userdb")  // 覆盖了 A 的设置！
// Goroutine A: db.Select("orders") // 实际查询的是 userdb.orders ❌ 数据错乱！
// Goroutine C: db.Select("users")  // 可能查询到 orderdb.users ❌ 数据错乱！

// 🔴 生产环境后果：
// - 用户数据错乱
// - 订单查询到错误数据
// - 财务数据不一致
// - 无法复现的间歇性错误
```

**并发风险等级评估：**

| API | 风险等级 | 后果 | 适用场景 |
|-----|---------|------|----------|
| `Use()` + `Select()` | 🔴 极高 | 数据错乱，生产事故 | 仅单协程脚本 |
| `GetDB()` | 🟡 中等 | 数据不一致 | 调试用途 |
| `SelectDB()` | 🟢 低 | 无问题 | 生产推荐 |
| `On()` | 🟢 低 | 无问题 | 生产推荐 |

### Repository 模式中的多库切换

**推荐：使用 goroutine-safe 的 SelectDB/InsertDB/UpdateDB/DeleteDB 或 On() API**

```go
type UserRepository struct{}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*User, error) {
    // ✅ 推荐：显式指定数据库，无全局状态修改
    rows, err := db.SelectDB("userdb", "users").Eq("id", id).Query(ctx)
    // ...
}

type OrderRepository struct{}

func (r *OrderRepository) FindByID(ctx context.Context, id int64) (*Order, error) {
    // ✅ 推荐：使用 On() fluent API
    rows, err := db.On("orderdb").Select("orders").Eq("id", id).Query(ctx)
    // ...
}

// ✅ 完整 Repository 示例
type ProductRepository struct {
    dbName string // 可在初始化时指定数据库
}

func NewProductRepository(dbName string) *ProductRepository {
    return &ProductRepository{dbName: dbName}
}

func (r *ProductRepository) FindByID(ctx context.Context, id int64) (*Product, error) {
    rows, err := db.SelectDB(r.dbName, "products").
        Columns("id", "name", "price", "stock").
        Eq("id", id).
        Limit(1).
        Query(ctx)
    // ...
}

func (r *ProductRepository) Create(ctx context.Context, p *Product) error {
    _, err := db.InsertDB(r.dbName, "products").Sets(map[string]interface{}{
        "name":  p.Name,
        "price": p.Price,
        "stock": p.Stock,
    }).Exec(ctx)
    return err
}
```

**旧版不推荐的方式（Use/UseDefault）**

```go
// ⚠️ 警告：以下方式在并发环境中有竞态条件，不推荐在 Repository 中使用
func (r *UserRepository) FindByIDUnsafe(ctx context.Context, id int64) (*User, error) {
    db.Use("userdb")        // ❌ 修改全局状态
    defer db.UseDefault()   // ❌ defer 在并发下不能解决问题
    
    rows, err := db.Select("users").Eq("id", id).Query(ctx)
    // ...
}
```

### 跨库 JOIN（MySQL 原生支持）

**注意**: 跨库 JOIN 是 **MySQL 原生支持**的特性，只要表在同一 MySQL 实例内，就可以用 `database.table` 语法跨库关联：

```sql
-- MySQL 原生支持的跨库 JOIN
SELECT u.name, o.amount 
FROM userdb.users AS u 
JOIN orderdb.orders AS o ON u.id = o.user_id
```

在框架中使用：

```go
// 方式1: 完整表名（推荐，清晰明确）
rows, err := db.Select("userdb.users AS u").
    Join("orderdb.orders AS o", "u.id = o.user_id").
    Columns("u.name", "o.amount").
    Query(ctx)

// 方式2: Use 一个库，另一个写完整名
db.Use("userdb")
rows, err := db.Select("users AS u").
    Join("orderdb.orders AS o", "u.id = o.user_id").
    Query(ctx)
```

**限制**: 跨库 JOIN 只能在**同一 MySQL 实例**内进行。如果 `userdb` 和 `orderdb` 在不同 MySQL 服务器上，则无法直接 JOIN，需要通过应用层关联或使用分布式查询方案。

### 多数据库使用注意事项（并发安全 - 极重要）

在并发环境（HTTP handlers、goroutines）中，**绝对避免使用 `Use()/UseDefault()` 模式**，因为它会导致严重的数据错乱和竞态条件。

```go
// ❌ 极度危险：Use() 在并发环境下会导致生产事故
func Handler(w http.ResponseWriter, r *http.Request) {
    db.Use("orderdb")  // 会干扰所有其他正在处理的请求！
    defer db.UseDefault()  // defer 完全不能解决并发问题
    // ... 可能查询到错误的数据库
}

// ✅ 安全：使用 goroutine-safe 的 API
func Handler(w http.ResponseWriter, r *http.Request) {
    // 方式1：显式指定数据库
    rows, err := db.SelectDB("orderdb", "orders").
        Where("status = ?", "pending").
        Query(r.Context())
    
    // 方式2：使用 On() fluent API
    rows, err := db.On("orderdb").Select("orders").
        Where("user_id = ?", userID).
        Query(r.Context())
    // ...
}
```

**⚠️ 关键警告：**

1. **`defer db.UseDefault()` 无法解决并发问题** - defer 只在当前函数结束时执行，期间其他 goroutine 已经修改了全局状态
2. **竞态条件难以测试** - 在低负载下可能正常，高负载下必然出错
3. **数据错乱后果严重** - 可能导致用户数据泄露、订单错乱、财务数据不一致
4. **难以调试** - 间歇性错误，难以复现和定位

### 多数据库查询函数对照表

| 场景 | 用法 | 示例 | 并发安全 |
|------|------|------|----------|
| 默认库（配置中的 database）| 直接使用 | `db.Select("users")` | ✅ 是 |
| 切换到其他库（推荐） | `SelectDB()` / `On()` | `db.SelectDB("orderdb", "orders")` | ✅ 是 |
| 切换到其他库（旧版） | `Use()` + 查询 | `db.Use("orderdb"); db.Select("orders")` | ❌ 否 |
| 跨库 JOIN | 完整表名 | `db.Select("db1.table1 AS t1").Join(...)` | ✅ 是 |
| 恢复默认 | `UseDefault()` | `db.UseDefault()` | ❌ 否（仅单协程） |

### 新版 goroutine-safe API 汇总

| 操作 | 旧方式（不推荐并发） | 新方式（goroutine-safe） |
|------|---------------------|------------------------|
| SELECT | `db.Use("db"); db.Select("table")` | `db.SelectDB("db", "table")` 或 `db.On("db").Select("table")` |
| INSERT | `db.Use("db"); db.Insert("table")` | `db.InsertDB("db", "table")` 或 `db.On("db").Insert("table")` |
| UPDATE | `db.Use("db"); db.Update("table")` | `db.UpdateDB("db", "table")` 或 `db.On("db").Update("table")` |
| DELETE | `db.Use("db"); db.Delete("table")` | `db.DeleteDB("db", "table")` 或 `db.On("db").Delete("table")` |
| Context 设置 | - | `ctx := db.WithDBContext(ctx, "db")` |

### 多数据库 Repository 模式示例

**推荐做法：使用 goroutine-safe 的 `SelectDB`/`On()` API，避免并发问题**

```go
// UserRepository 使用 MySQL userdb
type UserRepository struct{}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*User, error) {
    // ✅ 推荐：显式指定数据库，无并发问题
    rows, err := db.SelectDB("userdb", "users").
        Columns("id", "name", "email").
        Eq("id", id).
        Limit(1).
        Query(ctx)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    if !rows.Next() {
        return nil, sql.ErrNoRows
    }
    
    var user User
    err = rows.Scan(&user.ID, &user.Name, &user.Email)
    return &user, err
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    _, err := db.InsertDB("userdb", "users").Sets(map[string]interface{}{
        "name":  user.Name,
        "email": user.Email,
    }).Exec(ctx)
    return err
}

// OrderRepository 使用 PostgreSQL
type OrderRepository struct{}

func (r *OrderRepository) FindByID(ctx context.Context, id int64) (*Order, error) {
    // PostgreSQL 使用 Using 链式调用
    rows, err := db.Using(db.DBTypePostgres).Select("orders").
        Columns("id", "user_id", "amount", "status").
        Eq("id", id).
        Limit(1).
        Query(ctx)
    // ...
}

func (r *OrderRepository) Create(ctx context.Context, order *Order) error {
    _, err := db.PostgresInsert("orders").Sets(map[string]interface{}{
        "user_id": order.UserID,
        "amount":  order.Amount,
        "status":  order.Status,
    }).Exec(ctx)
    return err
}

// ProductRepository 使用可配置数据库名
type ProductRepository struct {
    dbName string
}

func NewProductRepository(dbName string) *ProductRepository {
    return &ProductRepository{dbName: dbName}
}

func (r *ProductRepository) FindByID(ctx context.Context, id int64) (*Product, error) {
    // 使用 On() fluent API
    rows, err := db.On(r.dbName).Select("products").
        Columns("id", "name", "price").
        Eq("id", id).
        Limit(1).
        Query(ctx)
    // ...
}
```

**不推荐：旧版 Use/UseDefault 方式（有并发风险）**

```go
// ⚠️ 警告：以下方式在并发环境中有竞态条件，仅适用于单协程脚本
func (r *UserRepository) FindByIDUnsafe(ctx context.Context, id int64) (*User, error) {
    db.Use("userdb")        // ❌ 修改全局状态
    defer db.UseDefault()   // ❌ defer 在并发下不能解决问题
    
    rows, err := db.Select("users").Eq("id", id).Query(ctx)
    // ...
}
```

### Service 层组合多个数据库

```go
type OrderService struct {
    userRepo  *UserRepository  // MySQL
    orderRepo *OrderRepository // PostgreSQL
}

func (s *OrderService) CreateOrder(ctx context.Context, userID int64, amount float64) error {
    // 1. 从 MySQL 验证用户
    user, err := s.userRepo.FindByID(ctx, userID)
    if err != nil {
        return fmt.Errorf("user not found: %w", err)
    }
    
    // 2. 在 PostgreSQL 创建订单
    order := &Order{
        UserID: userID,
        Amount: amount,
        Status: "pending",
    }
    
    if err := s.orderRepo.Create(ctx, order); err != nil {
        return fmt.Errorf("failed to create order: %w", err)
    }
    
    // 3. 在 MySQL 更新用户统计
    _, err = db.Update("users").
        SetExpr("order_count", "`order_count` + ?", 1).
        Eq("id", userID).
        Exec(ctx)
    
    return err
}
```

### 配置多数据库

```yaml
# database.yml - 服务级配置
mysql_primary:
  type: mysql
  mysql:
    host: localhost
    port: 3306
    database: userdb
    username: root
    password: secret
    
postgres_primary:
  type: postgres
  postgres:
    host: localhost
    port: 5432
    database: orderdb
    username: postgres
    password: secret
```

### ⚠️ 跨数据库事务注意事项

框架的 Helper **不支持跨数据库事务**（MySQL 和 PostgreSQL 事务不能混合）：

```go
// ❌ 错误：不能在同一个事务中操作两个数据库
err := db.Transaction(ctx, func(tx *sql.Tx) error {
    // 这是 MySQL 事务
    tx.Exec("UPDATE users SET ...") 
    
    // ❌ PostgreSQL 操作不能在这个事务中！
    // 下面的代码不会工作，因为它属于不同数据库
    return nil
})

// ✅ 正确：分开处理，或使用分布式事务（Saga/TCC 模式）
func (s *OrderService) CreateOrderWithCompensation(ctx context.Context, userID int64, amount float64) error {
    // 步骤1：创建订单（PostgreSQL）
    order, err := s.createOrderInPostgres(ctx, userID, amount)
    if err != nil {
        return err
    }
    
    // 步骤2：更新用户（MySQL）
    if err := s.updateUserInMySQL(ctx, userID); err != nil {
        // 补偿：回滚订单创建
        s.cancelOrderInPostgres(ctx, order.ID)
        return err
    }
    
    return nil
}
```

### 推荐的目录结构

```
internal/
├── repository/
│   ├── user_repo.go      # MySQL 操作
│   ├── order_repo.go     # PostgreSQL 操作
│   └── product_repo.go    # MySQL 操作
├── service/
│   └── order_service.go   # 组合多个 Repository
└── model/
    ├── user.go
    ├── order.go
    └── product.go
```

### 多数据库查询函数对照表

| 操作 | MySQL (默认) | PostgreSQL (显式) |
|------|-------------|------------------|
| 查询 | `db.Select("table")` | `db.Using(db.DBTypePostgres).Select("table")` |
| 插入 | `db.Insert("table")` | `db.PostgresInsert("table")` |
| 更新 | `db.Update("table")` | `db.PostgresUpdate("table")` |
| 删除 | `db.Delete("table")` | `db.PostgresDelete("table")` |
| 原始SQL | `db.Query(ctx, sql)` | `db.Postgres().Query(ctx, sql)` |

### 原始 SQL 查询

```go
// 直接执行 SQL
rows, err := db.Query(ctx, "SELECT * FROM users WHERE id = ?", 1)

// 执行 INSERT/UPDATE/DELETE
result, err := db.Exec(ctx, "UPDATE users SET name = ? WHERE id = ?", "John", 1)

// 事务
err := db.Transaction(ctx, func(tx *sql.Tx) error {
    _, err := tx.Exec("INSERT INTO accounts ...")
    if err != nil {
        return err
    }
    _, err = tx.Exec("UPDATE balances ...")
    return err
})
```

### Helper vs ORM 选择

| 场景 | 推荐方式 | 说明 |
|------|---------|------|
| 快速开发 | `db.Select/Insert/Update/Delete` | 无需初始化，直接调用 |
| 多数据库（并发安全） | `db.SelectDB/On()` | 推荐的新方式，goroutine-safe |
| 多数据库（旧版） | `db.Use()` + 查询 | 不推荐，仅适用于单协程脚本 |
| 不同数据库类型 | `db.Using(dbType)` | MySQL/PostgreSQL 切换 |
| 复杂查询 | `orm.ORM` | 需要更多控制时使用 |
| 事务 | 两者都支持 | Helper 用 `db.Transaction`，ORM 用 `o.Transaction` |

---

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

// 显式 Raw（仅用于可信 SQL 片段）
o.Select("users").
    WhereRaw("JSON_EXTRACT(meta, '$.vip') = ?", true).
    AndRaw("created_at >= ?", fromTime)
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

// 显式 Raw JOIN（仅用于可信 SQL 片段）
rows, err := o.Select("users u").
    JoinWithTypeRaw("INNER", "(SELECT user_id, MAX(score) AS score FROM rankings GROUP BY user_id) r", "u.id = r.user_id").
    Columns("u.name", "r.score").
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
    OrderBy(sortField + " " + sortOrder).
    Query(ctx)

// 如需复杂表达式排序，使用显式 Raw（仅可信输入）
rows, err := o.Select("users").
    Columns("id", "name", "email").
    OrderByRaw("FIELD(status, 'vip', 'active', 'inactive')", "created_at DESC").
    Query(ctx)
```

> 说明：`OrderBy(...)` 现在会校验排序项格式（`column` / `column ASC|DESC` / `table.column ASC|DESC`）。
> 若需要函数表达式或自定义排序片段，请使用 `OrderByRaw(...)` 并确保输入可信。

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
    SetExpr("updated_at", "NOW()").
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
    SetExpr("updated_at", "NOW()").
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

### 安全迁移指南（旧写法 -> 新写法）

```go
// 1) UPDATE 表达式
// 旧：字符串表达式（不推荐）
o.Update("users").Set("score", "score + 1")

// 新：显式表达式（推荐）
o.Update("users").SetExpr("score", "`score` + ?", 1)

// 2) ORDER BY 动态拼接
// 旧：直接拼接字符串
o.Select("users").OrderBy(sortField + " " + sortOrder)

// 新：优先用 OrderBy（内置格式校验）
o.Select("users").OrderBy("created_at DESC")

// 新：复杂函数排序时，显式使用 Raw
o.Select("users").OrderByRaw("FIELD(status, 'vip', 'active', 'inactive')")

// 3) WHERE/HAVING 原始条件
// 旧：Where/Having 直接写原始 SQL
o.Select("users").Where("JSON_EXTRACT(meta, '$.vip') = ?", true)
o.Select("users").Having("SUM(amount) > ?", 1000)

// 新：显式 Raw API，语义更清晰
o.Select("users").WhereRaw("JSON_EXTRACT(meta, '$.vip') = ?", true)
o.Select("users").HavingRaw("SUM(amount) > ?", 1000)

// 4) JOIN 自定义语句
// 旧：JoinWithType 里放复杂 table 表达式（可能被过滤）
o.Select("users u").JoinWithType("INNER", "(SELECT ... ) r", "u.id = r.user_id")

// 新：复杂 JOIN 明确走 Raw API
o.Select("users u").JoinWithTypeRaw("INNER", "(SELECT ... ) r", "u.id = r.user_id")
```

> 建议：
> - 默认使用 `Eq/In/Like/Where/OrderBy/JoinWithType` 这类安全 API。
> - 只有在必须写函数表达式、子查询片段时才使用 `*Raw` 或 `SetExpr`，并确保输入是可信来源。

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
        SetExpr("balance", "`balance` - ?", 100).
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
result, err := o.Update("users").
    Set("price", 99.99).
    SetExpr("version", "`version` + ?", 1).
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

## Scopes（查询作用域）完整指南 [已更新匹配实际实现]

### 概述

Scopes 是 ORM 中用于构建可重用查询模式的强大功能。它允许你将常用的查询条件封装成函数，然后在查询中链式调用，提高代码的可读性和复用性。

### 核心 Scopes 类型

#### 1. 基础业务 Scopes

```go
// 状态过滤
scope := orm.Active("status")           // WHERE status = 'active'

// 软删除过滤  
scope := orm.NotDeleted()               // WHERE deleted_at IS NULL
```

#### 2. 查询控制 Scopes

```go
// 分页
scope := orm.Paginate(2, 20)            // LIMIT 20 OFFSET 20 (page=2, size=20)

// 排序
scope := orm.OrderByDesc("created_at")  // ORDER BY created_at DESC

// 日期过滤
scope := orm.CreatedAfter(time.Now().AddDate(0, 0, -7))  // WHERE created_at >= ?
```

#### 3. 分片 Scopes

```go
// Hash 表路由
scope := orm.HashTable("orders", 12345, 8)   // 路由到 orders_{12345 % 8}

// 用户分片
scope := orm.ShardByUser(12345)            // WHERE user_id = 12345
```

#### 4. 元信息 Scopes

```go
// Trace 用于追踪（不修改查询）
scope := orm.Trace("query-name")
```

### 使用方式

#### 1. 基础使用

```go
// 创建 Builder
builder := orm.NewBuilder(ctx, "users").
    Scope(orm.Active("status")).           // 添加状态过滤
    Scope(orm.NotDeleted()).                // 添加软删除过滤
    Scope(orm.Paginate(1, 10)).            // 添加分页
    Scope(orm.OrderByDesc("created_at")).   // 添加排序
    ApplyScopes()                           // 应用所有 scopes

// 生成 SQL
sql, args := builder.Build()
// 输出: SELECT * FROM users WHERE status = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 10 OFFSET 0
// args: ["active"]
```

#### 2. Scope 分类注册

```go
builder := orm.NewBuilder(ctx, "orders")

// 注册 Routing scope（先执行）
builder.Routing(func(c context.Context, b *orm.Builder) *orm.Builder {
    return b.Table(fmt.Sprintf("orders_%d", userID % 8))
})

// 注册普通 Query scope
builder.Scope(func(c context.Context, b *orm.Builder) *orm.Builder {
    return b.Where("user_id = ?", userID)
})

// 注册 Meta scope（后执行，用于追踪）
builder.Meta(func(c context.Context, b *orm.Builder) *orm.Builder {
    // 可以在这里记录日志或追踪信息
    return b
})

// 应用所有 scopes（按 Routing -> Query -> Meta 顺序执行）
builder.ApplyScopes()
```

#### 3. 命名 Scope 追踪

```go
// 创建命名 scope
namedScope := orm.Named("UserFilter", orm.ScopeQuery, func(c context.Context, b *orm.Builder) *orm.Builder {
    return b.Where("user_type = ?", "vip")
})

builder := orm.NewBuilder(ctx, "users").
    Add(namedScope).
    ApplyScopes()

// 查看已应用的 scopes
applied := builder.AppliedScopes()
// 输出: ["UserFilter"]
```

### Helper 函数

#### 1. Compose - 组合多个 Scopes

```go
// 将多个 scope 组合成一个
combinedScope := orm.Compose(
    orm.Active("status"),
    orm.NotDeleted(),
    orm.OrderByDesc("created_at"),
)

builder := orm.NewBuilder(ctx, "users").
    Scope(combinedScope).
    ApplyScopes()
```

#### 2. If - 条件应用 Scope

```go
// 根据条件决定是否应用 scope
isActive := true
scope := orm.If(isActive, orm.Active("status"))

builder := orm.NewBuilder(ctx, "users").
    Scope(scope).
    ApplyScopes()
// 如果 isActive=false，则 Active scope 不会生效
```

#### 3. IfNotZero - 非零值应用 Scope

```go
// 当值不为零时应用 scope
userID := int64(123)

userScope := func(id int64) orm.Scope {
    return func(c context.Context, b *orm.Builder) *orm.Builder {
        return b.Where("user_id = ?", id)
    }
}

builder := orm.NewBuilder(ctx, "orders").
    Scope(orm.IfNotZero(userID, userScope)).
    ApplyScopes()

// 如果 userID=0，则不会添加 WHERE 条件
```

### 实际应用场景

#### 1. 电商系统订单查询

```go
func GetUserOrders(ctx context.Context, userID int64, page, pageSize int) (string, []interface{}) {
    builder := orm.NewBuilder(ctx, "orders").
        // 路由到用户分片表
        Scope(orm.HashTable("orders", userID, 8)).
        // 添加用户过滤
        Scope(orm.ShardByUser(userID)).
        // 未删除订单
        Scope(orm.NotDeleted()).
        // 分页
        Scope(orm.Paginate(page, pageSize)).
        // 排序
        Scope(orm.OrderByDesc("created_at")).
        ApplyScopes()
    
    return builder.Build()
}
```

#### 2. 动态条件查询

```go
func SearchUsers(ctx context.Context, filters map[string]interface{}) (string, []interface{}) {
    builder := orm.NewBuilder(ctx, "users")
    
    // 基础条件：活跃用户
    builder.Scope(orm.Active("status"))
    
    // 可选条件：根据传入参数动态添加
    if name, ok := filters["name"].(string); ok && name != "" {
        builder.Scope(func(c context.Context, b *orm.Builder) *orm.Builder {
            return b.Where("name LIKE ?", "%"+name+"%")
        })
    }
    
    if age, ok := filters["age"].(int); ok && age > 0 {
        builder.Scope(func(c context.Context, b *orm.Builder) *orm.Builder {
            return b.Where("age = ?", age)
        })
    }
    
    // 使用 IfNotZero 处理可选的用户类型
    if userType, ok := filters["user_type"].(int); ok {
        typeScope := func(t int) orm.Scope {
            return func(c context.Context, b *orm.Builder) *orm.Builder {
                return b.Where("user_type = ?", t)
            }
        }
        builder.Scope(orm.IfNotZero(userType, typeScope))
    }
    
    // 分页和排序
    builder.
        Scope(orm.Paginate(1, 20)).
        Scope(orm.OrderByDesc("created_at")).
        ApplyScopes()
    
    return builder.Build()
}
```

#### 3. Repository 模式

```go
type UserRepository struct{}

// 定义可复用的 scope 组合
func (r *UserRepository) ActiveUsersScope() orm.Scope {
    return orm.Compose(
        orm.Active("status"),
        orm.NotDeleted(),
    )
}

func (r *UserRepository) FindActiveUsers(ctx context.Context, page, pageSize int) (*sql.Rows, error) {
    builder := orm.NewBuilder(ctx, "users").
        Scope(r.ActiveUsersScope()).
        Scope(orm.Paginate(page, pageSize)).
        Scope(orm.OrderByDesc("created_at")).
        ApplyScopes()
    
    sql, args := builder.Build()
    return db.Query(ctx, sql, args...)
}
```

### 性能优化建议

#### 1. Scope 设计原则

```go
// 好的设计：单一职责
scope := orm.Active("status")

// 好的设计：可组合
userScopes := orm.Compose(
    orm.Active("status"),
    orm.NotDeleted(),
)

// 避免：过于复杂的 scope（应该拆分成多个简单 scope）
badScope := func(c context.Context, b *orm.Builder) *orm.Builder {
    return b.
        Where("status = ?", "active").
        Where("deleted_at IS NULL").
        Limit(100).
        OrderBy("created_at DESC")
}
```

#### 2. 执行顺序

```go
// Scopes 执行顺序：Routing -> Query -> Meta
builder := orm.NewBuilder(ctx, "data").
    Routing(routingFn).  // 先执行：路由到正确的表
    Scope(queryFn).      // 再执行：添加查询条件
    Meta(metaFn).        // 最后执行：追踪记录
    ApplyScopes()
```

#### 3. 调试追踪

```go
builder := orm.NewBuilder(ctx, "users")

// 添加命名 scope 便于追踪
builder.Add(orm.Named("StatusFilter", orm.ScopeQuery, func(c context.Context, b *orm.Builder) *orm.Builder {
    return b.Where("status = ?", "active")
}))

builder.Add(orm.Named("Pagination", orm.ScopeQuery, func(c context.Context, b *orm.Builder) *orm.Builder {
    return b.Limit(10).Offset(0)
}))

builder.ApplyScopes()

// 查看哪些 scopes 被应用了
applied := builder.AppliedScopes()
fmt.Printf("Applied scopes: %v\n", applied)
// 输出: ["StatusFilter", "Pagination"]
```

### 最佳实践

#### 1. 命名规范

```go
// 清晰的命名
func ActiveUsersScope() orm.Scope {
    return orm.Compose(
        orm.Active("status"),
        orm.NotDeleted(),
    )
}

// 参数化命名
func UsersByStatusScope(status string) orm.Scope {
    return func(c context.Context, b *orm.Builder) *orm.Builder {
        return b.Where("status = ?", status)
    }
}
```

#### 2. 错误处理

```go
// 验证 scope 参数
func ValidatedPaginateScope(page, pageSize int) orm.Scope {
    if page < 1 {
        page = 1
    }
    if pageSize <= 0 || pageSize > 1000 {
        pageSize = 100
    }
    return orm.Paginate(page, pageSize)
}
```

#### 3. 测试友好

```go
// 可测试的 scope 设计
func TestActiveScope(t *testing.T) {
    ctx := context.Background()
    builder := orm.NewBuilder(ctx, "users")
    
    builder.Scope(orm.Active("status")).ApplyScopes()
    
    sql, args := builder.Build()
    
    // 验证生成的 SQL
    if !strings.Contains(sql, "status = ?") {
        t.Error("Active scope not properly applied")
    }
    if len(args) != 1 || args[0] != "active" {
        t.Error("Active scope args incorrect")
    }
}
```

### 与 SelectBuilder 的关系

注意：`scopes.go` 中的 `Builder` 与 `select_builder.go` 中的 `SelectBuilder` 是两个独立的类型：

- **`Builder` (scopes.go)**: 轻量级，专注于 scope 链式应用
- **`SelectBuilder` (select_builder.go)**: 完整 SQL 构建器，带独立的 `appliedScopes` 追踪

两者可以配合使用：

```go
// 使用 SelectBuilder 构建复杂查询
query := o.Select("users").
    Columns("id", "name", "email").
    Eq("status", "active")

// 使用 Builder 的 scope 概念封装条件
scope := orm.Compose(
    orm.Active("status"),
    orm.NotDeleted(),
)

// 应用到 Builder 进行简单查询
builder := orm.NewBuilder(ctx, "users").
    Scope(scope).
    ApplyScopes()

sql, args := builder.Build()
rows, err := db.Query(ctx, sql, args...)
```

### 总结

- **类型定义**: `Scope` 是 `func(context.Context, *Builder) *Builder`
- **注册方法**: `Scope()`, `Routing()`, `Meta()`, `Add()`
- **应用方法**: `ApplyScopes()` 按 Routing -> Query -> Meta 顺序执行
- **追踪功能**: 使用 `Named()` 和 `AppliedScopes()` 追踪已应用的 scopes
- **组合工具**: `Compose()`, `If()`, `IfNotZero()` 提供灵活的 scope 组合

通过合理使用 Scopes，可以显著提高查询代码的可读性和复用性。

---

*注意：本文档已根据 `pkg/db/orm/scopes.go` 的实际实现更新，确保所有示例代码均可编译运行。之前版本的文档描述了不存在或名称不符的 API（如 `ActiveScope`、`NotDeletedScope`、`PaginationScope` 等），现已修正为正确的 API（`Active`、`NotDeleted`、`Paginate` 等）。*
