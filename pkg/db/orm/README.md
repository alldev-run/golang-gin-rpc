# pkg/db/orm

This package provides a lightweight, database-agnostic SQL query builder + small helpers (transactions, struct scanning).

It is **not** an ActiveRecord ORM (no schema migration, no relationship mapping). You typically:

- Create a `Dialect` (MySQL / PostgreSQL / SQLite / ClickHouse)
- Use builders to generate SQL
- Execute SQL via a `DB` implementation (e.g. `*sql.DB`, `*sql.Tx`, or adapters like `pkg/db/mysql.Client`)

## Quick start

### Use with `pkg/db/mysql`

```go
cfg := mysql.DefaultConfig()
cfg.Host = "127.0.0.1"
cfg.Port = 3306
cfg.Database = "demo"
cfg.Username = "root"
cfg.Password = "pwd"

cli, err := mysql.New(cfg)
if err != nil {
    return err
}
defer cli.Close()

repo := model.NewUserRepository(cli)
```

### Build SQL only (no DB required)

Builders can be used just to build SQL and args:

- `SelectBuilder.Build() (query string, args []any)`
- `InsertBuilder.Build() (query string, args []any, err error)`
- `UpdateBuilder.Build() (query string, args []any)`
- `DeleteBuilder.Build() (query string, args []any)`

See `example_test.go` for runnable examples.

## Common patterns

### SELECT

- `NewSelectBuilder(db, table)` (default dialect is MySQL-compatible)
- `NewSelectBuilderWithDialect(db, table, dialect)`

Features:

- Columns / joins / where / group by / having / order / limit / offset
- Locking (`FOR UPDATE`, `LOCK IN SHARE MODE`) depending on dialect

#### WITH (CTE)

```go
cte := orm.NewSelectBuilder(db, "orders").
    Columns("user_id", "COUNT(*)").
    Where("status = ?", "paid").
    GroupBy("user_id")

q, args := orm.NewSelectBuilder(db, "active_users").
    With("active_users", cte).
    Columns("user_id").
    Where("COUNT(*) > ?", 10).
    Build()
```

#### Subquery in FROM

```go
sub := orm.NewSelectBuilder(db, "users").Columns("id").Where("status = ?", "active")
q, args := orm.NewSelectBuilder(db, "ignored").FromSubquery(sub, "u").Columns("u.id").Build()
```

#### EXISTS / IN (subquery)

```go
sub := orm.NewSelectBuilder(db, "orders").Columns("1").Where("user_id = ?", 10)
sb := orm.NewSelectBuilder(db, "users").Columns("id")
sb.WhereBuilder().ExistsSubquery(sub)
q, args := sb.Build()
```

#### Join subquery

```go
sub := orm.NewSelectBuilder(db, "orders").Columns("user_id").Where("status = ?", "paid")
q, args := orm.NewSelectBuilder(db, "users u").
    Columns("u.id").
    JoinSubquery(sub, "o", "o.user_id = u.id").
    Build()
```

#### JoinOnBuilder (avoid raw ON but still allow raw)

```go
q, args := orm.NewSelectBuilder(db, "users u").
    Columns("u.id").
    JoinOn("profiles p", func(on *orm.JoinOnBuilder) {
        on.Eq("u.id", "p.user_id").And("p.status = ?", "active")
        // or: on.Raw("p.created_at > ?", time.Now().Add(-24*time.Hour))
    }).
    Build()
```

#### UNION / UNION ALL

```go
a := orm.NewSelectBuilder(db, "users").Columns("id").Where("status = ?", "active")
b := orm.NewSelectBuilder(db, "admins").Columns("id").Where("enabled = ?", true)
q, args := a.UnionAll(b).Build()
```

#### ORDER BY / LIMIT over UNION results

```go
outer := a.UnionAll(b).AsDerived("t").
    Columns("t.id").
    OrderByDesc("t.id").
    Limit(10)
q, args := outer.Build()
```

#### WITH RECURSIVE

```go
seed := orm.NewSelectBuilder(db, "nodes").Columns("id", "parent_id").Where("id = ?", 1)
rec := orm.NewSelectBuilder(db, "nodes n").Columns("n.id", "n.parent_id").Join("tree t", "n.parent_id = t.id")
q, args := orm.NewSelectBuilder(db, "tree").WithRecursive("tree", seed, rec).Columns("id").FromRaw("tree").Build()
```

### WHERE

Use `WhereBuilder` for typed predicates:

- `Eq/Ne/Gt/Gte/Lt/Lte`
- `Like/ILike`
- `In/NotIn`
- `IsNull/IsNotNull`
- `Between`

### INSERT

Use `InsertBuilder`:

- Single row: `Set` / `Sets`
- Bulk insert: `Values(columns, rows...)`

### UPDATE / DELETE

Use `UpdateBuilder` / `DeleteBuilder`.

## Struct scanning

- `StructScan(rows, &destStruct)` scans the first row into a struct
- `StructScanAll(rows, &[]*T)` scans all rows into a slice

Mapping rules:

- Use struct tag `db:"column_name"` when column differs
- Otherwise, exported fields map by `snake_case(fieldName)`

## Transactions

Use `ORM.Transaction(ctx, func(txORM *ORM) error { ... })` to run within a transaction.

## Dialects and support notes

### MySQL

- Identifier quoting: backticks (`` `col` ``)
- Placeholders: `?`
- LIMIT/OFFSET: `LIMIT n OFFSET m`
- Locks: `FOR UPDATE`, `LOCK IN SHARE MODE`

Known gaps:

- **PostgreSQL-style upsert** (`ON CONFLICT`) is not applicable to MySQL.
  - `InsertBuilder.OnConflict*` is designed around PostgreSQL-style `ON CONFLICT`.

Supported:

- **MySQL upsert** via `INSERT ... ON DUPLICATE KEY UPDATE ...`
  - Use `InsertBuilder.OnDuplicateKeyUpdate("col1", "col2", ...)`

- **INSERT IGNORE**
  - Use `InsertBuilder.Ignore()`

- **REPLACE INTO**
  - Use `InsertBuilder.Replace()`

- **FOR UPDATE 扩展**（MySQL 8 常用）
  - `SelectBuilder.ForUpdateNowait()` => `FOR UPDATE NOWAIT`
  - `SelectBuilder.ForUpdateSkipLocked()` => `FOR UPDATE SKIP LOCKED`

### ClickHouse

A minimal ClickHouse dialect is provided (`DialectClickHouse`).

- Identifier quoting: backticks
- Placeholders: `?` (most ClickHouse Go drivers support `?` placeholders)
- LIMIT/OFFSET: `LIMIT n OFFSET m`

Important differences:

- ClickHouse is columnar/analytical; **UPDATE/DELETE semantics differ** and may require special settings or are limited.
- Builders can still generate SQL, but whether it is accepted depends on:
  - ClickHouse server version
  - Table engine (e.g. MergeTree)
  - Driver (e.g. clickhouse-go)

If you need strict ClickHouse support for mutations (UPDATE/DELETE), we should add dedicated helpers that emit ClickHouse-specific mutation syntax.

## Notes on MySQL compatibility

- `FULL OUTER JOIN` is not supported by MySQL.
  - When using MySQL-like dialects (Default/MySQL), calling `FullOuterJoin(...)` will be degraded to `LEFT JOIN` so the generated SQL remains executable.
  - If you need true full outer join semantics in MySQL, use `UNION` with two queries.
