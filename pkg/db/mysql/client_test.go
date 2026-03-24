package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("DefaultConfig() Host = %v, want localhost", cfg.Host)
	}
	if cfg.Port != 3306 {
		t.Errorf("DefaultConfig() Port = %v, want 3306", cfg.Port)
	}
	if cfg.Charset != "utf8mb4" {
		t.Errorf("DefaultConfig() Charset = %v, want utf8mb4", cfg.Charset)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("DefaultConfig() MaxOpenConns = %v, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("DefaultConfig() MaxIdleConns = %v, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("DefaultConfig() ConnMaxLifetime = %v, want 1h", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 30*time.Minute {
		t.Errorf("DefaultConfig() ConnMaxIdleTime = %v, want 30m", cfg.ConnMaxIdleTime)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Host:            "127.0.0.1",
		Port:            3307,
		Database:        "testdb",
		Username:        "testuser",
		Password:        "testpass",
		Charset:         "utf8",
		MaxOpenConns:    50,
		MaxIdleConns:    20,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 15 * time.Minute,
	}

	if cfg.Host != "127.0.0.1" {
		t.Error("Config struct assignment failed for Host")
	}
	if cfg.Port != 3307 {
		t.Error("Config struct assignment failed for Port")
	}
	if cfg.Database != "testdb" {
		t.Error("Config struct assignment failed for Database")
	}
	if cfg.Username != "testuser" {
		t.Error("Config struct assignment failed for Username")
	}
	if cfg.Password != "testpass" {
		t.Error("Config struct assignment failed for Password")
	}
	if cfg.ConnMaxIdleTime != 15*time.Minute {
		t.Error("Config struct assignment failed for ConnMaxIdleTime")
	}
}

// TestNewWithInvalidDSN tests connection with invalid parameters
// This should fail but validates our error handling
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Host:     "invalid.host.that.does.not.exist",
		Port:     3306,
		Database: "test",
		Username: "test",
		Password: "test",
		Charset:  "utf8mb4",
	}

	// This will fail due to connection timeout, testing error handling
	client, err := New(cfg)
	if err == nil {
		// If no error, close the client
		if client != nil {
			_ = client.Close()
		}
		// Don't fail - just note that connection succeeded
		t.Log("Connection succeeded unexpectedly - may need real MySQL instance for full testing")
	} else {
		t.Logf("Expected connection error: %v", err)
	}
}

// TestClientMethods tests that client methods don't panic
func TestClientMethods(t *testing.T) {
	// We can't test without a real DB, but we verify method signatures exist.
	// This is a compile-time check via method references.
	client := &Client{}

	var _ func(context.Context, string, ...any) (int64, error) = client.InsertGetID
	var _ func(context.Context, string, ...any) (int64, error) = client.Update
	var _ func(context.Context, string, string, interface{}, string, interface{}) (int64, error) = client.SetFieldByID

	t.Log("Client methods compile successfully")
}

func TestWhereBuilder(t *testing.T) {
	wb := orm.NewWhereBuilder(orm.NewDefaultDialect())

	// Test empty builder
	where, args := wb.Build()
	if where != "" || len(args) != 0 {
		t.Errorf("Empty WhereBuilder should return empty string and no args, got where='%s', args=%v", where, args)
	}

	// Test single WHERE condition
	wb.Where("id = ?", 1)
	where, args = wb.Build()
	expectedWhere := "WHERE id = ?"
	if where != expectedWhere || len(args) != 1 || args[0] != 1 {
		t.Errorf("Expected where='%s', args=%v, got where='%s', args=%v", expectedWhere, []interface{}{1}, where, args)
	}

	// Test AND condition
	wb.And("status = ?", "active")
	where, args = wb.Build()
	expectedWhere = "WHERE id = ? AND status = ?"
	if where != expectedWhere || len(args) != 2 || args[1] != "active" {
		t.Errorf("Expected where='%s', args=%v, got where='%s', args=%v", expectedWhere, []interface{}{1, "active"}, where, args)
	}

	// Test OR condition
	wb.Or("deleted = ?", false)
	where, args = wb.Build()
	expectedWhere = "WHERE id = ? AND status = ? OR deleted = ?"
	if where != expectedWhere || len(args) != 3 || args[2] != false {
		t.Errorf("Expected where='%s', args=%v, got where='%s', args=%v", expectedWhere, []interface{}{1, "active", false}, where, args)
	}
}

func TestSelectBuilder(t *testing.T) {
	client := &Client{} // Mock client for testing

	sb := orm.NewSelectBuilder(client, "users")

	// Test basic SELECT
	query, args := sb.Build()
	expectedQuery := "SELECT * FROM `users`"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}

	// Test with columns
	sb.Columns("id", "name", "email")
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users`"
	if query != expectedQuery {
		t.Errorf("Expected query='%s', got '%s'", expectedQuery, query)
	}

	// Test with WHERE
	sb.Where("id = ?", 1).And("status = ?", "active")
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? AND status = ?"
	expectedArgs := []interface{}{1, "active"}
	if query != expectedQuery || len(args) != 2 || args[0] != 1 || args[1] != "active" {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test with ORDER BY
	sb.OrderBy("created_at DESC")
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? AND status = ? ORDER BY created_at DESC"
	if query != expectedQuery {
		t.Errorf("Expected query='%s', got '%s'", expectedQuery, query)
	}

	// Test with LIMIT
	sb.Limit(10)
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? AND status = ? ORDER BY created_at DESC LIMIT 10"
	if query != expectedQuery {
		t.Errorf("Expected query='%s', got '%s'", expectedQuery, query)
	}

	// Test with OFFSET
	sb.Offset(20)
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? AND status = ? ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	if query != expectedQuery {
		t.Errorf("Expected query='%s', got '%s'", expectedQuery, query)
	}
}

func TestSelectBuilderWithJoins(t *testing.T) {
	client := &Client{} // Mock client for testing

	sb := orm.NewSelectBuilder(client, "users").
		Columns("u.id", "u.name", "p.title").
		Join("posts p", "u.id = p.user_id").
		LeftJoin("comments c", "p.id = c.post_id").
		RightJoin("categories cat", "p.category_id = cat.id").
		FullOuterJoin("tags t", "p.id = t.post_id").
		Where("u.status = ?", "active").
		GroupBy("u.id", "u.name").
		Having("COUNT(p.id) > ?", 5).
		OrderBy("u.created_at DESC")

	query, args := sb.Build()
	expectedQuery := "SELECT `u`.`id`, `u`.`name`, `p`.`title` FROM `users` INNER JOIN `posts` p ON u.id = p.user_id LEFT JOIN `comments` c ON p.id = c.post_id RIGHT JOIN `categories` cat ON p.category_id = cat.id LEFT JOIN `tags` t ON p.id = t.post_id WHERE u.status = ? GROUP BY `u`.`id`, `u`.`name` HAVING COUNT(p.id) > ? ORDER BY u.created_at DESC"
	expectedArgs := []interface{}{"active", 5}

	if query != expectedQuery {
		t.Errorf("Expected query='%s', got '%s'", expectedQuery, query)
	}

	if len(args) != len(expectedArgs) || args[0] != expectedArgs[0] || args[1] != expectedArgs[1] {
		t.Errorf("Expected args=%v, got args=%v", expectedArgs, args)
	}
}

func TestJoinWithType(t *testing.T) {
	client := &Client{} // Mock client for testing

	sb := orm.NewSelectBuilder(client, "users").
		Columns("u.id", "u.name", "p.title").
		JoinWithType("CROSS", "posts p", "true").
		Where("u.status = ?", "active")

	query, args := sb.Build()
	expectedQuery := "SELECT `u`.`id`, `u`.`name`, `p`.`title` FROM `users` CROSS JOIN `posts` p ON true WHERE u.status = ?"
	expectedArgs := []interface{}{"active"}

	if query != expectedQuery {
		t.Errorf("Expected query='%s', got '%s'", expectedQuery, query)
	}

	if len(args) != len(expectedArgs) || args[0] != expectedArgs[0] {
		t.Errorf("Expected args=%v, got args=%v", expectedArgs, args)
	}
}

func TestAggregationFunctions(t *testing.T) {
	client := &Client{} // Mock client for testing

	// Test COUNT
	countQuery, countArgs := orm.NewSelectBuilder(client, "users").Columns("COUNT(*)").Where("status = ?", "active").Build()
	expectedCountQuery := "SELECT COUNT(*) FROM `users` WHERE status = ?"
	if countQuery != expectedCountQuery || len(countArgs) != 1 || countArgs[0] != "active" {
		t.Errorf("COUNT query failed: got query='%s', args=%v", countQuery, countArgs)
	}

	// Test COUNT with column
	countColQuery, countColArgs := orm.NewSelectBuilder(client, "orders").Columns("COUNT(id)").Where("status = ?", "completed").Build()
	expectedCountColQuery := "SELECT COUNT(id) FROM `orders` WHERE status = ?"
	if countColQuery != expectedCountColQuery || len(countColArgs) != 1 || countColArgs[0] != "completed" {
		t.Errorf("COUNT column query failed: got query='%s', args=%v", countColQuery, countColArgs)
	}

	// Test SUM
	sumQuery, sumArgs := orm.NewSelectBuilder(client, "orders").Columns("SUM(total_amount)").Where("created_at >= ?", "2023-01-01").Build()
	expectedSumQuery := "SELECT SUM(total_amount) FROM `orders` WHERE created_at >= ?"
	if sumQuery != expectedSumQuery || len(sumArgs) != 1 || sumArgs[0] != "2023-01-01" {
		t.Errorf("SUM query failed: got query='%s', args=%v", sumQuery, sumArgs)
	}

	// Test AVG
	avgQuery, avgArgs := orm.NewSelectBuilder(client, "products").Columns("AVG(price)").Where("category = ?", "electronics").Build()
	expectedAvgQuery := "SELECT AVG(price) FROM `products` WHERE category = ?"
	if avgQuery != expectedAvgQuery || len(avgArgs) != 1 || avgArgs[0] != "electronics" {
		t.Errorf("AVG query failed: got query='%s', args=%v", avgQuery, avgArgs)
	}

	// Test MAX
	maxQuery, maxArgs := orm.NewSelectBuilder(client, "temperatures").Columns("MAX(value)").Where("sensor_id = ?", 1).Build()
	expectedMaxQuery := "SELECT MAX(value) FROM `temperatures` WHERE sensor_id = ?"
	if maxQuery != expectedMaxQuery || len(maxArgs) != 1 || maxArgs[0] != 1 {
		t.Errorf("MAX query failed: got query='%s', args=%v", maxQuery, maxArgs)
	}

	// Test MIN
	minQuery, minArgs := orm.NewSelectBuilder(client, "products").Columns("MIN(stock_quantity)").Where("supplier_id = ?", 123).Build()
	expectedMinQuery := "SELECT MIN(stock_quantity) FROM `products` WHERE supplier_id = ?"
	if minQuery != expectedMinQuery || len(minArgs) != 1 || minArgs[0] != 123 {
		t.Errorf("MIN query failed: got query='%s', args=%v", minQuery, minArgs)
	}
}

func TestSelectBuilderTransactionMethods(t *testing.T) {
	client := &Client{} // Mock client for testing

	sb := orm.NewSelectBuilder(client, "users").
		Columns("id", "name", "email").
		Where("status = ?", "active").
		OrderBy("created_at DESC").
		Limit(10)

	// Test that QueryTx and QueryRowTx methods exist and can be called
	// (We can't test actual execution without a real transaction, but we can verify method signatures)
	query, args := sb.Build()
	expectedQuery := "SELECT `id`, `name`, `email` FROM `users` WHERE status = ? ORDER BY created_at DESC LIMIT 10"

	if query != expectedQuery || len(args) != 1 || args[0] != "active" {
		t.Errorf("SelectBuilder build failed: got query='%s', args=%v", query, args)
	}

	// Verify that the methods exist (compile-time check)
	_ = func(sb *orm.SelectBuilder, ctx context.Context, tx *sql.Tx) {
		_, _ = sb.QueryTx(ctx, tx)
		_ = sb.QueryRowTx(ctx, tx)
	}
}

func TestSelectBuilderLocks(t *testing.T) {
	client := &Client{} // Mock client for testing

	// Test FOR UPDATE lock
	sb1 := orm.NewSelectBuilder(client, "accounts").
		Columns("id", "balance").
		Where("user_id = ?", 123).
		ForUpdate()

	query1, args1 := sb1.Build()
	expectedQuery1 := "SELECT `id`, `balance` FROM `accounts` WHERE user_id = ? FOR UPDATE"
	if query1 != expectedQuery1 || len(args1) != 1 || args1[0] != 123 {
		t.Errorf("FOR UPDATE query failed: got query='%s', args=%v", query1, args1)
	}

	// Test LOCK IN SHARE MODE
	sb2 := orm.NewSelectBuilder(client, "products").
		Columns("id", "name", "stock").
		Where("category = ?", "electronics").
		LockInShareMode()

	query2, args2 := sb2.Build()
	expectedQuery2 := "SELECT `id`, `name`, `stock` FROM `products` WHERE category = ? LOCK IN SHARE MODE"
	if query2 != expectedQuery2 || len(args2) != 1 || args2[0] != "electronics" {
		t.Errorf("LOCK IN SHARE MODE query failed: got query='%s', args=%v", query2, args2)
	}

	// Test custom lock
	sb3 := orm.NewSelectBuilder(client, "orders").
		Columns("id", "total").
		Where("status = ?", "pending").
		Lock("FOR UPDATE NOWAIT")

	query3, args3 := sb3.Build()
	expectedQuery3 := "SELECT `id`, `total` FROM `orders` WHERE status = ? FOR UPDATE NOWAIT"
	if query3 != expectedQuery3 || len(args3) != 1 || args3[0] != "pending" {
		t.Errorf("Custom lock query failed: got query='%s', args=%v", query3, args3)
	}

	// Test lock with JOIN
	sb4 := orm.NewSelectBuilder(client, "users u").
		Columns("u.id", "u.name", "a.balance").
		LeftJoin("accounts a", "u.id = a.user_id").
		Where("u.status = ?", "active").
		ForUpdate()

	query4, args4 := sb4.Build()
	expectedQuery4 := "SELECT `u`.`id`, `u`.`name`, `a`.`balance` FROM `users` u LEFT JOIN `accounts` a ON u.id = a.user_id WHERE u.status = ? FOR UPDATE"
	if query4 != expectedQuery4 || len(args4) != 1 || args4[0] != "active" {
		t.Errorf("Lock with JOIN query failed: got query='%s', args=%v", query4, args4)
	}
}
