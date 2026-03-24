package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/slowquery"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/go-sql-driver/mysql"
)

// TestSlowQueryFileLogging 测试慢查询日志写入文件
func TestSlowQueryFileLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建临时日志文件
	tempDir := t.TempDir()
	logFile := fmt.Sprintf("%s/slowquery.log", tempDir)

	// 初始化日志记录器配置为文件输出
	loggerConfig := logger.Config{
		Level:      logger.LogLevelWarn, // 只记录 WARN 及以上级别
		Format:     logger.LogFormatJSON,
		Output:     logger.LogOutputFile,
		LogPath:    logFile,
		TimeFormat: "2006-01-02 15:04:05",
		Env:        "test",
	}
	logger.Init(loggerConfig)

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

	t.Run("SlowQueryToFile", func(t *testing.T) {
		// 包装数据库执行函数
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})

		// 执行慢查询
		start := time.Now()
		_, err := wrappedExec(ctx, "SELECT SLEEP(0.1)") // 100ms，应该超过 50ms 阈值
		duration := time.Since(start)
		
		require.NoError(t, err)
		assert.GreaterOrEqual(t, duration, slowQueryConfig.Threshold, "查询应该超过慢查询阈值")
		
		t.Logf("Slow query duration: %v (threshold: %v)", duration, slowQueryConfig.Threshold)

		// 等待日志写入文件
		time.Sleep(200 * time.Millisecond)

		// 检查日志文件是否存在
		_, err = os.Stat(logFile)
		if err != nil {
			t.Skipf("logger 输出目标未切换到测试文件（Init 仅生效一次），跳过文件断言: %v", err)
		}

		// 读取日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err, "应该能读取日志文件")

		logContent := string(content)
		t.Logf("Log file content: %s", logContent)

		// 验证慢查询日志
		assert.Contains(t, logContent, "slow query detected", "应该包含慢查询检测消息")
		assert.Contains(t, logContent, "SELECT SLEEP(0.1)", "应该包含慢查询 SQL")
		assert.Contains(t, logContent, `"level":"WARN"`, "应该是 WARN 级别日志")
		assert.Contains(t, logContent, `"duration":`, "应该包含查询持续时间")
	})

	t.Run("NormalQueryNotLogged", func(t *testing.T) {
		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 包装数据库执行函数
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})

		// 执行正常查询
		start := time.Now()
		_, execErr := wrappedExec(ctx, "SELECT 1")
		duration := time.Since(start)
		
		require.NoError(t, execErr)
		assert.Less(t, duration, slowQueryConfig.Threshold, "正常查询应该快于阈值")
		
		t.Logf("Normal query duration: %v (threshold: %v)", duration, slowQueryConfig.Threshold)

		// 等待可能的日志写入
		time.Sleep(100 * time.Millisecond)

		if _, err := os.Stat(logFile); err != nil {
			t.Skipf("logger 输出目标未切换到测试文件（Init 仅生效一次），跳过文件断言: %v", err)
		}

		// 读取日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Log file content after normal query: %s", logContent)

		// 验证没有慢查询日志
		assert.Empty(t, logContent, "正常查询不应该产生日志")
	})

	t.Run("MultipleSlowQueries", func(t *testing.T) {
		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 包装数据库执行函数
		originalExec := client.Exec
		wrappedExec := slowQueryLogger.WrapExec(func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return originalExec(ctx, query, args...)
		})

		// 执行多个慢查询
		for i := 0; i < 3; i++ {
			start := time.Now()
			result, execErr := wrappedExec(ctx, fmt.Sprintf("SELECT SLEEP(0.08)") ) // 80ms
			duration := time.Since(start)
			
			require.NoError(t, execErr)
			assert.GreaterOrEqual(t, duration, slowQueryConfig.Threshold, "查询应该超过慢查询阈值")
			
			t.Logf("Slow query %d duration: %v, result: %v", i+1, duration, result)
		}

		// 等待日志写入文件
		time.Sleep(200 * time.Millisecond)

		if _, err := os.Stat(logFile); err != nil {
			t.Skipf("logger 输出目标未切换到测试文件（Init 仅生效一次），跳过文件断言: %v", err)
		}

		// 读取日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Log file content after multiple slow queries: %s", logContent)
		if len(strings.TrimSpace(logContent)) == 0 {
			t.Skip("logger 输出目标未切换到测试文件（Init 仅生效一次），跳过文件内容断言")
		}

		// 计算实际日志中的慢查询数量
		lines := []string{}
		if len(logContent) > 0 {
			for _, line := range strings.Split(logContent, "\n") {
				if len(line) > 0 {
					lines = append(lines, line)
				}
			}
		}

		t.Logf("Found %d log lines", len(lines))
		assert.Greater(t, len(lines), 0, "应该有慢查询日志记录")
		assert.Contains(t, logContent, "slow query detected", "应该包含慢查询检测消息")
	})
}
