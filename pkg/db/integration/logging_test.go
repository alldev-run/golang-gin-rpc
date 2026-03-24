package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/slowquery"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/go-sql-driver/mysql"
)

// TestDatabaseLogging 测试数据库日志记录
func TestDatabaseLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 初始化全局日志记录器
	loggerConfig := logger.Config{
		Level:      logger.LogLevelInfo,
		Format:     logger.LogFormatJSON,
		Output:     logger.LogOutputStdout,
		TimeFormat: "2006-01-02 15:04:05",
	}
	logger.Init(loggerConfig)

	// 配置 MySQL 客户端
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

	ctx := context.Background()

	// 创建测试表
	createTestTable(ctx, t, client)
	defer cleanupTestTable(ctx, t, client)

	// 1. 测试正常查询日志记录
	t.Run("NormalQueryLogging", func(t *testing.T) {
		// 执行正常查询
		_, err := client.Exec(ctx, "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())", "log-test", 123)
		require.NoError(t, err)

		// 查询数据
		rows, err := client.Query(ctx, "SELECT id, name, value FROM test_integration WHERE name = ?", "log-test")
		require.NoError(t, err)
		rows.Close()

		t.Log("✅ Normal query executed successfully")
	})

	// 2. 测试慢查询日志记录
	t.Run("SlowQueryLogging", func(t *testing.T) {
		// 配置慢查询检测器
		slowQueryConfig := slowquery.Config{
			Threshold:   100 * time.Millisecond, // 100ms 阈值
			MaxQueryLen: 1000,
			IncludeArgs: false,
			SampleRate:  1,
		}
		
		slowQueryLogger := slowquery.New(slowQueryConfig)

		// 包装数据库执行函数
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})

		// 执行一个相对慢的查询（使用 SLEEP 模拟）
		start := time.Now()
		_, err := wrappedExec(ctx, "SELECT SLEEP(0.2)") // 200ms
		duration := time.Since(start)
		
		require.NoError(t, err)
		assert.GreaterOrEqual(t, duration, slowQueryConfig.Threshold, "查询应该超过慢查询阈值")
		
		t.Logf("Slow query duration: %v (threshold: %v)", duration, slowQueryConfig.Threshold)
		
		// 等待日志写入
		time.Sleep(100 * time.Millisecond)
		t.Log("✅ Slow query test completed")
	})

	// 3. 测试错误查询日志记录
	t.Run("ErrorQueryLogging", func(t *testing.T) {
		// 执行会出错的查询
		_, err := client.Exec(ctx, "INSERT INTO nonexistent_table (name) VALUES (?)", "test")
		require.Error(t, err)

		t.Logf("Expected error: %v", err)
		t.Log("✅ Error query test completed")
	})

	// 4. 测试连接池日志记录
	t.Run("ConnectionPoolLogging", func(t *testing.T) {
		// 并发执行多个查询以触发连接池活动
		concurrency := 5
		done := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				defer func() { done <- true }()
				
				// 执行查询
				_, err := client.Exec(ctx, "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())", fmt.Sprintf("pool-test-%d", id), id)
				require.NoError(t, err)
				
				// 查询数据
				rows, err := client.Query(ctx, "SELECT COUNT(*) FROM test_integration")
				require.NoError(t, err)
				rows.Close()
			}(i)
		}

		// 等待所有 goroutine 完成
		for i := 0; i < concurrency; i++ {
			<-done
		}

		t.Log("✅ Connection pool test completed")
	})

	// 5. 测试事务日志记录
	t.Run("TransactionLogging", func(t *testing.T) {
		// 开始事务
		tx, err := client.Begin(ctx, nil)
		require.NoError(t, err)

		// 在事务中执行操作
		_, err = tx.ExecContext(ctx, "INSERT INTO test_integration (name, value, created_at) VALUES (?, ?, NOW())", "tx-test", 456)
		require.NoError(t, err)

		// 提交事务
		err = tx.Commit()
		require.NoError(t, err)

		t.Log("✅ Transaction test completed")
	})
}

// TestLogLevelValidation 测试日志级别验证
func TestLogLevelValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 测试不同日志级别
	testLevels := []logger.LogLevel{
		logger.LogLevelDebug,
		logger.LogLevelInfo,
		logger.LogLevelWarn,
		logger.LogLevelError,
	}

	for _, level := range testLevels {
		t.Run(fmt.Sprintf("LogLevel_%s", level), func(t *testing.T) {
			// 重新初始化日志记录器
			loggerConfig := logger.Config{
				Level:      level,
				Format:     logger.LogFormatJSON,
				Output:     logger.LogOutputStdout,
				TimeFormat: "2006-01-02 15:04:05",
			}
			logger.Init(loggerConfig)

			// 配置 MySQL 客户端
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

			client, err := mysql.New(cfg)
			require.NoError(t, err)
			defer client.Close()

			ctx := context.Background()

			// 执行正常查询
			_, err = client.Exec(ctx, "SELECT 1")
			require.NoError(t, err)

			t.Logf("✅ Log level %s test completed", level)
		})
	}
}

// TestSlowQueryDetection 测试慢查询检测功能
func TestSlowQueryDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 配置慢查询检测器
	slowQueryConfig := slowquery.Config{
		Threshold:   50 * time.Millisecond, // 50ms 阈值
		MaxQueryLen: 1000,
		IncludeArgs: false,
		SampleRate:  1,
	}
	
	slowQueryLogger := slowquery.New(slowQueryConfig)

	// 配置 MySQL 客户端
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

	client, err := mysql.New(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// 测试正常查询（不应该被记录为慢查询）
	t.Run("NormalQuery", func(t *testing.T) {
		// 包装数据库执行函数用于慢查询检测
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})
		
		start := time.Now()
		_, err := wrappedExec(ctx, "SELECT 1")
		duration := time.Since(start)
		
		require.NoError(t, err)
		assert.Less(t, duration, slowQueryConfig.Threshold, "正常查询应该快于阈值")
		
		t.Logf("Normal query duration: %v (threshold: %v)", duration, slowQueryConfig.Threshold)
	})

	// 测试慢查询（应该被记录）
	t.Run("SlowQuery", func(t *testing.T) {
		// 包装数据库执行函数用于慢查询检测
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})
		
		start := time.Now()
		_, err := wrappedExec(ctx, "SELECT SLEEP(0.1)") // 100ms
		duration := time.Since(start)
		
		require.NoError(t, err)
		assert.GreaterOrEqual(t, duration, slowQueryConfig.Threshold, "慢查询应该超过阈值")
		
		t.Logf("Slow query duration: %v (threshold: %v)", duration, slowQueryConfig.Threshold)
		
		// 等待慢查询日志写入
		time.Sleep(100 * time.Millisecond)
	})
}

// TestDatabaseErrorLogging 测试数据库错误日志记录
func TestDatabaseErrorLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 初始化日志记录器为 ERROR 级别以确保能捕获错误
	loggerConfig := logger.Config{
		Level:      logger.LogLevelError,
		Format:     logger.LogFormatJSON,
		Output:     logger.LogOutputStdout,
		TimeFormat: "2006-01-02 15:04:05",
	}
	logger.Init(loggerConfig)

	// 配置 MySQL 客户端
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

	client, err := mysql.New(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	t.Run("DatabaseErrors", func(t *testing.T) {
		// 测试各种数据库错误
		testCases := []struct {
			name     string
			query    string
			args     []interface{}
			wantErr  bool
		}{
			{
				name:    "InvalidTable",
				query:   "INSERT INTO nonexistent_table (name) VALUES (?)",
				args:    []interface{}{"test"},
				wantErr: true,
			},
			{
				name:    "InvalidSyntax",
				query:   "INVALID SQL SYNTAX",
				args:    nil,
				wantErr: true,
			},
			{
				name:    "ValidQuery",
				query:   "SELECT 1",
				args:    nil,
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := client.Exec(ctx, tc.query, tc.args...)
				
				if tc.wantErr {
					require.Error(t, err)
					t.Logf("Expected error for %s: %v", tc.name, err)
				} else {
					require.NoError(t, err)
					t.Logf("✅ %s executed successfully", tc.name)
				}
			})
		}
	})
}
