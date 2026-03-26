// +build integration

package orm

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// getTestDB 连接本地 MySQL 数据库
func getTestDB(t *testing.T) *sql.DB {
	// 默认使用 root/q1w2e3r4@localhost:3306/test_orm
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:q1w2e3r4@tcp(localhost:3306)/test_orm?charset=utf8mb4&parseTime=True&loc=Local"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		// 如果 test_orm 数据库不存在，尝试先创建
		if err.Error() == "Error 1049 (42000): Unknown database 'test_orm'" {
			dsnNoDB := "root:q1w2e3r4@tcp(localhost:3306)/?charset=utf8mb4&parseTime=True&loc=Local"
			db2, err2 := sql.Open("mysql", dsnNoDB)
			if err2 != nil {
				t.Fatalf("failed to open db without database: %v", err2)
			}
			_, _ = db2.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS test_orm CHARACTER SET utf8mb4")
			db2.Close()
			
			// 重新连接
			db, err = sql.Open("mysql", dsn)
			if err != nil {
				t.Fatalf("failed to open db after creating database: %v", err)
			}
			if err := db.PingContext(ctx); err != nil {
				t.Fatalf("failed to ping db after creating database: %v", err)
			}
		} else {
			t.Fatalf("failed to ping db: %v (DSN: %s)", err, dsn)
		}
	}

	return db
}

func TestIntegration_CreateAndDropTable(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	dialect := NewMySQLDialect()
	ctx := context.Background()

	// 1. 创建测试表
	tableName := "test_orm_ddl_" + fmt.Sprintf("%d", time.Now().Unix())

	tb := CreateTable(NewDBWrapper(db), dialect, tableName).
		IfNotExists().
		ID("id").
		Varchar("name", 100, true). // nullable
		Int("age", true).
		DateTime("created_at", true).
		Index("idx_name", "name")

	sqlStr, err := tb.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	t.Logf("CREATE TABLE SQL: %s", sqlStr)

	// 执行建表
	if err := tb.Exec(ctx); err != nil {
		t.Fatalf("Exec CREATE TABLE failed: %v", err)
	}
	t.Log("✅ CREATE TABLE success")

	// 2. 验证表存在
	orm := NewORMWithDB(db, dialect)
	exists, err := orm.HasTable(ctx, tableName)
	if err != nil {
		t.Logf("HasTable warning: %v", err)
	}
	t.Logf("Table exists: %v", exists)

	// 3. 插入测试数据验证表正常工作
	_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, age) VALUES (?, ?)", tableName), "测试用户", 25)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	t.Log("✅ INSERT data success")

	// 4. 查询数据
	var count int
	err = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
	t.Logf("✅ SELECT count=%d success", count)

	// 5. 删除表
	dropTb := DropTable(NewDBWrapper(db), dialect, tableName).IfExists()
	dropSQL, _ := dropTb.Build()
	t.Logf("DROP TABLE SQL: %s", dropSQL)

	if err := dropTb.Exec(ctx); err != nil {
		t.Fatalf("Exec DROP TABLE failed: %v", err)
	}
	t.Log("✅ DROP TABLE success")

	// 6. 验证表已删除
	_, err = db.QueryContext(ctx, fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", tableName))
	if err == nil {
		t.Error("expected error after dropping table, but query succeeded")
	} else {
		t.Logf("✅ Table %s successfully dropped (query error: %v)", tableName, err)
	}
}

func TestIntegration_TableBuilder_FullFeatures(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	dialect := NewMySQLDialect()
	ctx := context.Background()

	tableName := "test_full_features_" + fmt.Sprintf("%d", time.Now().Unix())

	// 创建包含多种字段类型的表
	tb := CreateTable(NewDBWrapper(db), dialect, tableName).
		IfNotExists().
		ID("id").
		Varchar("username", 50).
		Varchar("email", 100, true).
		Text("bio", true).
		Int("age", true).
		BigInt("score", true).
		Boolean("active", true).
		Decimal("balance", 10, 2, true).
		DateTime("created_at", true).
		Timestamp("updated_at", true).
		UniqueIndex("idx_email", "email").
		Index("idx_username", "username")

	sqlStr, err := tb.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	t.Logf("Full CREATE TABLE SQL:\n%s", sqlStr)

	if err := tb.Exec(ctx); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	t.Log("✅ Full features table created")

	// 清理
	DropTable(NewDBWrapper(db), dialect, tableName).IfExists().Exec(ctx)
}

func TestIntegration_CompositePrimaryKey(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	dialect := NewMySQLDialect()
	ctx := context.Background()

	tableName := "test_composite_pk_" + fmt.Sprintf("%d", time.Now().Unix())

	// 复合主键表（订单项）
	tb := CreateTable(NewDBWrapper(db), dialect, tableName).
		IfNotExists().
		Int("order_id").
		Int("product_id").
		Int("quantity", true).
		Decimal("price", 10, 2, true).
		PrimaryKey("order_id", "product_id")

	sqlStr, err := tb.Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	t.Logf("Composite PK SQL: %s", sqlStr)

	if err := tb.Exec(ctx); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	t.Log("✅ Composite primary key table created")

	// 验证复合主键约束
	_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (order_id, product_id, quantity) VALUES (1, 100, 2)", tableName))
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// 同样的主键应该报错
	_, err = db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (order_id, product_id, quantity) VALUES (1, 100, 3)", tableName))
	if err == nil {
		t.Error("Expected duplicate key error for composite PK")
	} else {
		t.Logf("✅ Composite PK constraint works: %v", err)
	}

	// 清理
	DropTable(NewDBWrapper(db), dialect, tableName).IfExists().Exec(ctx)
}

func TestIntegration_TruncateTable(t *testing.T) {
	db := getTestDB(t)
	defer db.Close()

	dialect := NewMySQLDialect()
	orm := NewORMWithDB(db, dialect)
	ctx := context.Background()

	tableName := "test_truncate_" + fmt.Sprintf("%d", time.Now().Unix())

	// 创建表并插入数据
	tb := CreateTable(NewDBWrapper(db), dialect, tableName).
		IfNotExists().
		ID("id").
		Varchar("name", 50, true)
	tb.Exec(ctx)

	db.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name) VALUES ('a'), ('b'), ('c')", tableName))

	var count int
	db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if count != 3 {
		t.Fatalf("expected 3 rows before truncate, got %d", count)
	}

	// 清空表
	if err := orm.TruncateTable(ctx, tableName); err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}
	t.Log("✅ TRUNCATE TABLE success")

	db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after truncate, got %d", count)
	}

	// 清理
	DropTable(NewDBWrapper(db), dialect, tableName).IfExists().Exec(ctx)
}
