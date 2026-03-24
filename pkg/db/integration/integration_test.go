package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	internalbootstrap "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/db"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/go-sql-driver/mysql"
)

// TestRealMySQLConnection 测试真实的 MySQL 连接
func TestRealMySQLConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 配置本地 MySQL 连接
	cfg := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "myblog",
		Username:        "root",
		Password:        "q1w2e3r4",
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	// 创建 MySQL 客户端
	client, err := mysql.New(cfg)
	require.NoError(t, err)
	defer client.Close()

	// 测试连接
	ctx := context.Background()
	err = client.Ping(ctx)
	require.NoError(t, err, "Failed to ping MySQL database")

	t.Log("✅ MySQL connection successful")
}

// TestMySQLConnectionPool 测试 MySQL 连接池
func TestMySQLConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "myblog",
		Username:        "root",
		Password:        "q1w2e3r4",
		Charset:         "utf8mb4",
		MaxOpenConns:    5,  // 较小的连接池用于测试
		MaxIdleConns:    2,
		ConnMaxLifetime: 30 * time.Second,
		ConnMaxIdleTime: 10 * time.Second,
	}

	client, err := mysql.New(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// 创建测试表
	createTestTable(ctx, t, client)
	defer cleanupTestTable(ctx, t, client)

	// 并发测试连接池
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// 执行多次查询
			for j := 0; j < 5; j++ {
				// 插入数据
				query := "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())"
				result, err := client.Exec(ctx, query, fmt.Sprintf("worker-%d", id), j*100+id)
				require.NoError(t, err)
				
				lastID, err := result.LastInsertId()
				require.NoError(t, err)
				require.Greater(t, lastID, int64(0))
				
				// 查询数据
				selectQuery := "SELECT id, name, value FROM test_integration WHERE name = ?"
				rows, err := client.Query(ctx, selectQuery, fmt.Sprintf("worker-%d", id))
				require.NoError(t, err)
				
				count := 0
				for rows.Next() {
					var id int64
					var name string
					var value int
					err := rows.Scan(&id, &name, &value)
					require.NoError(t, err)
					count++
				}
				rows.Close()
				
				assert.Greater(t, count, 0)
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 验证数据总数
	var total int
	err = client.QueryRow(ctx, "SELECT COUNT(*) FROM test_integration").Scan(&total)
	require.NoError(t, err)
	assert.Equal(t, concurrency*5, total)

	t.Log("✅ MySQL connection pool test successful")
}

// TestFactoryIntegration 测试工厂模式集成
func TestFactoryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建工厂
	factory := db.NewFactory()

	// 创建 MySQL 配置
	mysqlCfg := db.Config{
		Type: db.TypeMySQL,
		MySQL: mysql.Config{
			Host:            "localhost",
			Port:            3306,
			Database:        "myblog",
			Username:        "root",
			Password:        "q1w2e3r4",
			Charset:         "utf8mb4",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 30 * time.Minute,
		},
	}

	// 通过工厂创建客户端
	client, err := factory.Create(mysqlCfg)
	require.NoError(t, err)
	defer client.Close()

	// 测试客户端
	ctx := context.Background()
	err = client.Ping(ctx)
	require.NoError(t, err)

	// 测试获取特定类型的客户端
	mysqlClient, err := factory.GetMySQL()
	require.NoError(t, err)
	require.NotNil(t, mysqlClient)

	sqlClient, err := factory.GetMySQLSQLClient()
	require.NoError(t, err)
	require.NotNil(t, sqlClient)

	// 测试 SQL 接口
	createTestTableSQL(ctx, t, sqlClient)
	defer cleanupTestTableSQL(ctx, t, sqlClient)

	// 插入测试数据
	query := "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())"
	result, err := sqlClient.Exec(ctx, query, "factory-test", 42)
	require.NoError(t, err)

	lastID, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, lastID, int64(0))

	t.Log("✅ Factory integration test successful")
}

// TestBootstrapIntegration 测试 Bootstrap 集成
func TestBootstrapIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建数据库配置文件
	dbConfig := `mysql_primary:
  type: mysql
  mysql:
    host: "localhost"
    port: 3306
    database: "myblog"
    username: "root"
    password: "q1w2e3r4"
    charset: "utf8mb4"
    max_open_conns: 25
    max_idle_conns: 10
    conn_max_lifetime: "1h"
    conn_max_idle_time: "30m"`

	tmpDir := t.TempDir()
	dbConfigFile := fmt.Sprintf("%s/database.yml", tmpDir)
	err := os.WriteFile(dbConfigFile, []byte(dbConfig), 0644)
	require.NoError(t, err)

	// 使用 bootstrap 加载配置
	boot, err := internalbootstrap.NewBootstrap("")
	require.NoError(t, err)
	defer boot.Close()

	// 加载数据库配置
	err = bootstrap.LoadDatabaseConfig(boot, dbConfigFile)
	require.NoError(t, err)

	// 启动数据库初始化
	err = boot.InitializeDatabases()
	require.NoError(t, err)

	// 获取数据库客户端
	mysqlClient, err := boot.GetMySQLClient()
	require.NoError(t, err)
	require.NotNil(t, mysqlClient)

	// 测试数据库操作
	ctx := context.Background()
	err = mysqlClient.Ping(ctx)
	require.NoError(t, err)

	// 创建测试表
	createTestTable(ctx, t, mysqlClient)
	defer cleanupTestTable(ctx, t, mysqlClient)

	// 插入测试数据
	query := "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())"
	result, err := mysqlClient.Exec(ctx, query, "bootstrap-test", 100)
	require.NoError(t, err)

	lastID, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, lastID, int64(0))

	t.Log("✅ Bootstrap integration test successful")
}

// TestORMIntegration 测试 ORM 集成
func TestORMIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建 MySQL 客户端
	mysqlCfg := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "myblog",
		Username:        "root",
		Password:        "q1w2e3r4",
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	client, err := mysql.New(mysqlCfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// 创建测试表
	createTestTable(ctx, t, client)
	defer cleanupTestTable(ctx, t, client)

	// 创建 ORM 实例
	ormInstance := orm.NewORMWithDB(client.DB(), orm.NewMySQLDialect())
	require.NotNil(t, ormInstance)

	// 测试 ORM 操作
	testORMOperations(ctx, t, ormInstance)

	t.Log("✅ ORM integration test successful")
}

// 辅助函数

func createTestTable(ctx context.Context, t *testing.T, client *mysql.Client) {
	query := `
		CREATE TABLE IF NOT EXISTS test_integration (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			value INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`
	_, err := client.Exec(ctx, query)
	require.NoError(t, err)
}

func createTestTableSQL(ctx context.Context, t *testing.T, client db.SQLClient) {
	query := `
		CREATE TABLE IF NOT EXISTS test_integration (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			value INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`
	_, err := client.Exec(ctx, query)
	require.NoError(t, err)
}

func cleanupTestTable(ctx context.Context, t *testing.T, client *mysql.Client) {
	_, err := client.Exec(ctx, "DROP TABLE IF EXISTS test_integration")
	require.NoError(t, err)
}

func cleanupTestTableSQL(ctx context.Context, t *testing.T, client db.SQLClient) {
	_, err := client.Exec(ctx, "DROP TABLE IF EXISTS test_integration")
	require.NoError(t, err)
}

func testORMOperations(ctx context.Context, t *testing.T, ormInstance *orm.ORM) {
	// 使用原生 SQL 进行 ORM 测试
	db := ormInstance.DB()
	
	// 测试插入
	result, err := db.Exec(ctx, "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())", "orm-test", 123)
	require.NoError(t, err)

	lastID, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, lastID, int64(0))

	// 测试查询
	rows, err := db.Query(ctx, "SELECT id, name, value FROM test_integration WHERE name = ?", "orm-test")
	require.NoError(t, err)
	defer rows.Close()

	var records []struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value int    `db:"value"`
	}

	for rows.Next() {
		var record struct {
			ID    int64  `db:"id"`
			Name  string `db:"name"`
			Value int    `db:"value"`
		}
		err := rows.Scan(&record.ID, &record.Name, &record.Value)
		require.NoError(t, err)
		records = append(records, record)
	}

	assert.Len(t, records, 1)
	assert.Equal(t, "orm-test", records[0].Name)
	assert.Equal(t, 123, records[0].Value)

	// 测试事务
	err = ormInstance.Transaction(ctx, func(tx *orm.ORM) error {
		// 在事务中插入数据
		_, err := tx.DB().Exec(ctx, "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())", "orm-tx-test", 456)
		return err
	})
	require.NoError(t, err)

	// 验证事务数据
	txRows, err := db.Query(ctx, "SELECT COUNT(*) FROM test_integration WHERE name = ?", "orm-tx-test")
	require.NoError(t, err)
	defer txRows.Close()

	var count int
	if txRows.Next() {
		err := txRows.Scan(&count)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, count)

	// 测试完整的 ORM 查询构建器功能
	testORMQueryBuilder(ctx, t, ormInstance)
}

// testORMQueryBuilder 测试 ORM 查询构建器的所有功能
func testORMQueryBuilder(ctx context.Context, t *testing.T, ormInstance *orm.ORM) {
	db := ormInstance.DB()
	
	// 清理测试数据
	_, err := db.Exec(ctx, "DELETE FROM test_integration")
	require.NoError(t, err)

	// 1. 测试 INSERT 查询构建器
	insertResult, err := ormInstance.Insert("test_integration").
		Set("name", "query-builder-test").
		Set("value", 999).
		Set("created_at", time.Now().Format("2006-01-02 15:04:05")).
		Exec(ctx)
	require.NoError(t, err)
	
	insertID, err := insertResult.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, insertID, int64(0))

	// 2. 测试 SELECT 查询构建器 - 单条记录
	var singleRecord struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value int    `db:"value"`
	}
	
	err = ormInstance.Select("test_integration").
		Columns("id", "name", "value").
		Eq("name", "query-builder-test").
		QueryRow(ctx).
		Scan(&singleRecord.ID, &singleRecord.Name, &singleRecord.Value)
	require.NoError(t, err)
	assert.Equal(t, "query-builder-test", singleRecord.Name)
	assert.Equal(t, 999, singleRecord.Value)

	// 3. 测试 SELECT 查询构建器 - 多条记录
	// 插入更多测试数据
	_, err = ormInstance.Insert("test_integration").
		Set("name", "query-builder-test-2").
		Set("value", 888).
		Set("created_at", time.Now().Format("2006-01-02 15:04:05")).
		Exec(ctx)
	require.NoError(t, err)

	_, err = ormInstance.Insert("test_integration").
		Set("name", "query-builder-test-3").
		Set("value", 777).
		Set("created_at", time.Now().Format("2006-01-02 15:04:05")).
		Exec(ctx)
	require.NoError(t, err)

	// 查询多条记录
	rows, err := ormInstance.Select("test_integration").
		Columns("id", "name", "value").
		Like("name", "query-builder-test%").
		OrderByDesc("value").
		Query(ctx)
	require.NoError(t, err)
	defer rows.Close()

	var multiRecords []struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value int    `db:"value"`
	}

	for rows.Next() {
		var record struct {
			ID    int64  `db:"id"`
			Name  string `db:"name"`
			Value int    `db:"value"`
		}
		err := rows.Scan(&record.ID, &record.Name, &record.Value)
		require.NoError(t, err)
		multiRecords = append(multiRecords, record)
	}

	assert.Len(t, multiRecords, 3) // 只有这三个测试记录
	assert.Equal(t, 999, multiRecords[0].Value) // 按值降序排列

	// 4. 测试 UPDATE 查询构建器
	updateResult, err := ormInstance.Update("test_integration").
		Set("value", 555).
		Eq("name", "query-builder-test-2").
		Exec(ctx)
	require.NoError(t, err)
	
	affectedRows, err := updateResult.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affectedRows)

	// 验证更新结果
	var updatedRecord struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value int    `db:"value"`
	}
	
	err = ormInstance.Select("test_integration").
		Columns("id", "name", "value").
		Eq("name", "query-builder-test-2").
		QueryRow(ctx).
		Scan(&updatedRecord.ID, &updatedRecord.Name, &updatedRecord.Value)
	require.NoError(t, err)
	assert.Equal(t, 555, updatedRecord.Value)

	// 5. 测试 DELETE 查询构建器
	deleteResult, err := ormInstance.Delete("test_integration").
		Eq("name", "query-builder-test-3").
		Exec(ctx)
	require.NoError(t, err)
	
	deletedRows, err := deleteResult.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedRows)

	// 验证删除结果
	var deletedCount int
	err = ormInstance.Select("test_integration").
		Columns("COUNT(*)").
		Eq("name", "query-builder-test-3").
		QueryRow(ctx).
		Scan(&deletedCount)
	require.NoError(t, err)
	assert.Equal(t, 0, deletedCount)

	// 6. 测试简单 WHERE 条件
	complexRows, err := ormInstance.Select("test_integration").
		Columns("id", "name", "value").
		Where("value > ?", 500).
		Query(ctx)
	require.NoError(t, err)
	defer complexRows.Close()

	var complexRecords []struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value int    `db:"value"`
	}

	for complexRows.Next() {
		var record struct {
			ID    int64  `db:"id"`
			Name  string `db:"name"`
			Value int    `db:"value"`
		}
		err := complexRows.Scan(&record.ID, &record.Name, &record.Value)
		require.NoError(t, err)
		complexRecords = append(complexRecords, record)
	}

	assert.Greater(t, len(complexRecords), 0)

	// 7. 测试 LIMIT 和 OFFSET
	limitRows, err := ormInstance.Select("test_integration").
		Columns("id", "name", "value").
		OrderBy("id").
		Limit(2).
		Offset(1).
		Query(ctx)
	require.NoError(t, err)
	defer limitRows.Close()

	var limitRecords []struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value int    `db:"value"`
	}

	for limitRows.Next() {
		var record struct {
			ID    int64  `db:"id"`
			Name  string `db:"name"`
			Value int    `db:"value"`
		}
		err := limitRows.Scan(&record.ID, &record.Name, &record.Value)
		require.NoError(t, err)
		limitRecords = append(limitRecords, record)
	}

	assert.LessOrEqual(t, len(limitRecords), 2)

	// 8. 测试原始 SQL 查询
	rawRows, err := db.Query(ctx, "SELECT COUNT(*) as count, AVG(value) as avg_value FROM test_integration WHERE value > ?", 600)
	require.NoError(t, err)
	defer rawRows.Close()

	var rawStats struct {
		Count     int     `db:"count"`
		AvgValue  float64 `db:"avg_value"`
	}

	if rawRows.Next() {
		err := rawRows.Scan(&rawStats.Count, &rawStats.AvgValue)
		require.NoError(t, err)
		assert.Greater(t, rawStats.Count, 0)
		assert.Greater(t, rawStats.AvgValue, 600.0)
	}

	// 9. 测试原始 SQL 执行（INSERT, UPDATE, DELETE）
	execResult, err := db.Exec(ctx, "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())", "raw-sql-test", 333)
	require.NoError(t, err)
	
	rawInsertID, err := execResult.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, rawInsertID, int64(0))

	// 原始 SQL UPDATE
	updateExecResult, err := db.Exec(ctx, "UPDATE test_integration SET value = ? WHERE name = ?", 444, "raw-sql-test")
	require.NoError(t, err)
	
	updateAffectedRows, err := updateExecResult.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), updateAffectedRows)

	// 原始 SQL DELETE
	deleteExecResult, err := db.Exec(ctx, "DELETE FROM test_integration WHERE name = ?", "raw-sql-test")
	require.NoError(t, err)
	
	deleteAffectedRows, err := deleteExecResult.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleteAffectedRows)

	// 10. 测试批量操作
	batchValues := []struct {
		Name  string
		Value int
	}{
		{"batch-test-1", 111},
		{"batch-test-2", 222},
		{"batch-test-3", 333},
	}

	for _, batch := range batchValues {
		_, err := ormInstance.Insert("test_integration").
			Set("name", batch.Name).
			Set("value", batch.Value).
			Set("created_at", time.Now().Format("2006-01-02 15:04:05")).
			Exec(ctx)
		require.NoError(t, err)
	}

	// 验证批量插入结果
	var batchCount int
	err = ormInstance.Select("test_integration").
		Columns("COUNT(*)").
		Like("name", "batch-test%").
		QueryRow(ctx).
		Scan(&batchCount)
	require.NoError(t, err)
	assert.Equal(t, 3, batchCount)

	// 11. 测试事务中的复杂操作
	err = ormInstance.Transaction(ctx, func(tx *orm.ORM) error {
		// 在事务中执行多个操作
		_, err := tx.Insert("test_integration").
			Set("name", "transaction-test").
			Set("value", 666).
			Set("created_at", time.Now().Format("2006-01-02 15:04:05")).
			Exec(ctx)
		if err != nil {
			return err
		}

		// 更新刚插入的记录
		_, err = tx.Update("test_integration").
			Set("value", 777).
			Eq("name", "transaction-test").
			Exec(ctx)
		if err != nil {
			return err
		}

		// 查询并验证
		var txValue int
		err = tx.Select("test_integration").
			Columns("value").
			Eq("name", "transaction-test").
			QueryRow(ctx).
			Scan(&txValue)
		if err != nil {
			return err
		}

		// 验证值是否正确更新
		if txValue != 777 {
			return fmt.Errorf("transaction test failed: expected 777, got %d", txValue)
		}

		return nil
	})
	require.NoError(t, err)

	// 验证事务结果
	var finalTxValue int
	err = ormInstance.Select("test_integration").
		Columns("value").
		Eq("name", "transaction-test").
		QueryRow(ctx).
		Scan(&finalTxValue)
	require.NoError(t, err)
	assert.Equal(t, 777, finalTxValue)

	t.Log("✅ ORM query builder operations test successful")
}
