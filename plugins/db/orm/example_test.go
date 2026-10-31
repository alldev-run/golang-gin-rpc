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

func ExampleWhereBuilder_groupedAndOr() {
	wb := NewWhereBuilder(NewMySQLDialect())
	wb.Where("a = ?", 1).AndGroup(func(g *WhereBuilder) {
		g.Where("b = ?", 2).Or("c = ?", 3)
	})

	where, args := wb.Build()
	fmt.Println(where)
	fmt.Println(args)

	// Output:
	// WHERE a = ? AND (b = ? OR c = ?)
	// [1 2 3]
}

func ExampleSelectBuilder_withCTE() {
	db := &noopDB{}

	cte := NewSelectBuilder(db, "orders").
		Columns("user_id", "COUNT(*)").
		Where("status = ?", "paid").
		GroupBy("user_id")

	sb := NewSelectBuilder(db, "active_users").
		With("active_users", cte).
		Columns("user_id").
		Where("COUNT(*) > ?", 10)

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// WITH `active_users` AS (SELECT `user_id`, COUNT(*) FROM `orders` WHERE status = ? GROUP BY `user_id`) SELECT `user_id` FROM `active_users` WHERE COUNT(*) > ?
	// [paid 10]
}

func ExampleSelectBuilder_fromSubquery() {
	db := &noopDB{}

	sub := NewSelectBuilder(db, "users").
		Columns("id").
		Where("status = ?", "active")

	sb := NewSelectBuilder(db, "ignored").
		FromSubquery(sub, "u").
		Columns("u.id")

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `u`.`id` FROM (SELECT `id` FROM `users` WHERE status = ?) u
	// [active]
}

func ExampleSelectBuilder_unionAll() {
	db := &noopDB{}

	a := NewSelectBuilder(db, "users").Columns("id").Where("status = ?", "active")
	b := NewSelectBuilder(db, "admins").Columns("id").Where("enabled = ?", true)

	q, args := a.UnionAll(b).Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `id` FROM `users` WHERE status = ? UNION ALL SELECT `id` FROM `admins` WHERE enabled = ?
	// [active true]
}

func ExampleWhereBuilder_existsSubquery() {
	db := &noopDB{}

	sub := NewSelectBuilder(db, "orders").
		Columns("1").
		Where("user_id = ?", 10)

	sb := NewSelectBuilder(db, "users").
		Columns("id")
	sb.WhereBuilder().ExistsSubquery(sub)

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `id` FROM `users` WHERE EXISTS (SELECT `1` FROM `orders` WHERE user_id = ?)
	// [10]
}

func ExampleSelectBuilder_unionOuterOrderLimit() {
	db := &noopDB{}

	a := NewSelectBuilder(db, "users").Columns("id").Where("status = ?", "active")
	b := NewSelectBuilder(db, "admins").Columns("id").Where("enabled = ?", true)

	outer := a.UnionAll(b).AsDerived("t").
		Columns("t.id").
		OrderByDesc("t.id").
		Limit(10)

	q, args := outer.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `t`.`id` FROM (SELECT `id` FROM `users` WHERE status = ? UNION ALL SELECT `id` FROM `admins` WHERE enabled = ?) t ORDER BY `t`.`id` DESC LIMIT 10
	// [active true]
}

func ExampleSelectBuilder_withRecursive() {
	db := &noopDB{}

	seed := NewSelectBuilder(db, "nodes").Columns("id", "parent_id").Where("id = ?", 1)
	rec := NewSelectBuilder(db, "nodes n").
		Columns("n.id", "n.parent_id").
		Join("tree t", "n.parent_id = t.id")

	sb := NewSelectBuilder(db, "tree").
		WithRecursive("tree", seed, rec).
		Columns("id").
		FromRaw("tree")

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// WITH RECURSIVE `tree` AS (SELECT `id`, `parent_id` FROM `nodes` WHERE id = ? UNION ALL SELECT `n`.`id`, `n`.`parent_id` FROM `nodes` n INNER JOIN `tree` t ON n.parent_id = t.id) SELECT `id` FROM tree
	// [1]
}

func ExampleSelectBuilder_joinSubquery() {
	db := &noopDB{}

	sub := NewSelectBuilder(db, "orders").Columns("user_id").Where("status = ?", "paid")
	sb := NewSelectBuilder(db, "users u").
		Columns("u.id").
		JoinSubquery(sub, "o", "o.user_id = u.id")

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `u`.`id` FROM `users` u INNER JOIN (SELECT `user_id` FROM `orders` WHERE status = ?) o ON o.user_id = u.id
	// [paid]
}

func ExampleSelectBuilder_joinOnBuilder() {
	db := &noopDB{}

	sb := NewSelectBuilder(db, "users u").
		Columns("u.id").
		JoinOn("profiles p", func(on *JoinOnBuilder) {
			on.Eq("u.id", "p.user_id").And("p.status = ?", "active")
		})

	q, args := sb.Build()
	fmt.Println(q)
	fmt.Println(args)

	// Output:
	// SELECT `u`.`id` FROM `users` u INNER JOIN `profiles` p ON `u`.`id` = `p`.`user_id` AND p.status = ?
	// [active]
}
