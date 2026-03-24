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

// TestGlobalSlowQueryLogging 测试全局慢查询日志记录
func TestGlobalSlowQueryLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 使用默认日志配置（应该写入到全局日志文件）
	loggerConfig := logger.DefaultConfig()
	loggerConfig.Level = logger.LogLevelWarn // 只记录 WARN 及以上级别
	logger.Init(loggerConfig)

	// 配置慢查询检测器
	slowQueryConfig := slowquery.DefaultConfig()
	slowQueryConfig.Threshold = 50 * time.Millisecond // 50ms 阈值
	
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

	t.Run("GlobalSlowQueryLog", func(t *testing.T) {
		// 包装数据库执行函数
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})

		// 执行多个慢查询来确保日志被记录
		for i := 0; i < 3; i++ {
			start := time.Now()
			_, err := wrappedExec(ctx, fmt.Sprintf("SELECT SLEEP(0.08)")) // 80ms
			duration := time.Since(start)
			
			require.NoError(t, err)
			assert.GreaterOrEqual(t, duration, slowQueryConfig.Threshold, "查询应该超过慢查询阈值")
			
			t.Logf("Slow query %d duration: %v", i+1, duration)
		}

		// 等待日志写入
		time.Sleep(200 * time.Millisecond)

		t.Log("✅ 全局慢查询日志测试完成")
		t.Logf("慢查询日志应该写入到: /Users/johnjames/Workspace/golang/golang-gin-rpc/pkg/db/slowquery/logs/app.log")
	})
}
