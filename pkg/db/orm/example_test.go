package orm

import (
	"context"
	"database/sql"
	"fmt"
)

type noopDB struct{}

func (n *noopDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (n *noopDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

func (n *noopDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (n *noopDB) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, nil
}

func (n *noopDB) Ping(ctx context.Context) error {
	return nil
}

func (n *noopDB) Stats() sql.DBStats {
	return sql.DBStats{}
}

func (n *noopDB) Close() error {
	return nil
}

func ExampleSelectBuilder_mysql() {
	db := &noopDB{}
	sb := NewSelectBuilderWithDialect(db, "users", NewMySQLDialect()).
		Columns("id", "name").
		Eq("id", 1).
		OrderByDesc("id").
		Limit(10)

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `id`, `name` FROM `users` WHERE `id` = ? ORDER BY `id` DESC LIMIT 10
	// [1]
}

func ExampleSelectBuilder_clickhouse() {
	db := &noopDB{}
	sb := NewSelectBuilderWithDialect(db, "events", NewClickHouseDialect()).
		Columns("event_id", "ts").
		Gte("ts", "2026-01-01").
		OrderByDesc("ts").
		Limit(5)

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `event_id`, `ts` FROM `events` WHERE `ts` >= ? ORDER BY `ts` DESC LIMIT 5
	// [2026-01-01]
}

func ExampleInsertBuilder_mysql() {
	db := &noopDB{}
	ib := NewInsertBuilderWithDialect(db, "users", NewMySQLDialect()).
		Set("name", "alice").
		Set("age", 18)

	q, args, err := ib.Build()
	fmt.Println(q)
	fmt.Println(args)
	fmt.Println(err == nil)

	// Output:
	// INSERT INTO `users` (`age`, `name`) VALUES (?, ?)
	// [18 alice]
	// true
}

func ExampleInsertBuilder_mysqlIgnore() {
	db := &noopDB{}
	ib := NewInsertBuilderWithDialect(db, "users", NewMySQLDialect()).
		Ignore().
		Set("id", 1).
		Set("name", "alice")

	q, args, err := ib.Build()
	fmt.Println(q)
	fmt.Println(args)
	fmt.Println(err == nil)

	// Output:
	// INSERT IGNORE INTO `users` (`id`, `name`) VALUES (?, ?)
	// [1 alice]
	// true
}

func ExampleInsertBuilder_mysqlReplace() {
	db := &noopDB{}
	ib := NewInsertBuilderWithDialect(db, "users", NewMySQLDialect()).
		Replace().
		Set("id", 1).
		Set("name", "alice")

	q, args, err := ib.Build()
	fmt.Println(q)
	fmt.Println(args)
	fmt.Println(err == nil)

	// Output:
	// REPLACE INTO `users` (`id`, `name`) VALUES (?, ?)
	// [1 alice]
	// true
}

func ExampleInsertBuilder_mysqlUpsert() {
	db := &noopDB{}
	ib := NewInsertBuilderWithDialect(db, "users", NewMySQLDialect()).
		Set("id", 1).
		Set("name", "alice").
		OnDuplicateKeyUpdate("name")

	q, args, err := ib.Build()
	fmt.Println(q)
	fmt.Println(args)
	fmt.Println(err == nil)

	// Output:
	// INSERT INTO `users` (`id`, `name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)
	// [1 alice]
	// true
}

func ExampleUpdateBuilder_mysql() {
	db := &noopDB{}
	ub := NewUpdateBuilderWithDialect(db, "users", NewMySQLDialect()).
		Set("name", "bob").
		Eq("id", 1)

	q, args := ub.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// UPDATE `users` SET `name` = ? WHERE `id` = ?
	// [bob 1]
}

func ExampleDeleteBuilder_mysql() {
	db := &noopDB{}
	db2 := NewDeleteBuilderWithDialect(db, "users", NewMySQLDialect()).
		Eq("id", 1).
		Limit(1)

	q, args := db2.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// DELETE FROM `users` WHERE `id` = ? LIMIT 1
	// [1]
}

func ExampleWhereBuilder_grouped() {
	wb := NewWhereBuilder(NewMySQLDialect())
	wb.AndWhere(func(w *WhereBuilder) {
		w.Eq("status", "active").Or("role = ?", "admin")
	})

	where, args := wb.Build()
	fmt.Println(where)
	fmt.Println(args)

	// Output:
	// WHERE (`status` = ? OR role = ?)
	// [active admin]
}
