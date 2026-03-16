# golang-gin-rpc

A lightweight Go project showcasing a MySQL client with fluent query builders, connection pooling, and transaction support.

## Usage

### 1) Create a MySQL client

```go
import (
    "time"

    "github.com/your/repo/pkg/db/mysql"
)

cfg := mysql.DefaultConfig()
cfg.Host = "127.0.0.1"
cfg.Port = 3306
cfg.Username = "root"
cfg.Password = "password"
cfg.Database = "example"

client, err := mysql.New(cfg)
if err != nil {
    // handle error
}
defer client.Close()
```

### 2) Build queries with `SelectBuilder`

The query builder supports `Where`, `And`, `Or` and automatically binds arguments.

> **Note:** SQL `AND` has higher precedence than `OR`. If you need `(a OR b) AND c`, wrap the `OR` part in parentheses.

```go
rows, err := client.NewSelectBuilder("users").
    Where("(status = ? OR status = ?)", "active", "pending").
    And("tenant_id = ?", 42).
    OrderBy("created_at DESC").
    Limit(10).
    Query(ctx)
```

### 3) Transactions

```go
err := client.Transaction(ctx, func(tx *sql.Tx) error {
    // use tx.QueryContext, tx.ExecContext, etc.
    return nil
})
```
