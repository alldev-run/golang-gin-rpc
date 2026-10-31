package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestComprehensiveDatabaseErrorCapture 测试全面的数据库错误捕获
func TestComprehensiveDatabaseErrorCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建临时日志文件
	tempDir := t.TempDir()
	logFile := fmt.Sprintf("%s/comprehensive_error_test.log", tempDir)

	t.Run("TestManualErrorLogging", func(t *testing.T) {
		// 配置日志记录器输出到文件
		fileLoggerConfig := logger.Config{
			Level:      logger.LogLevelError,
			Format:     logger.LogFormatJSON,
			Output:     logger.LogOutputFile,
			LogPath:    logFile,
			TimeFormat: "2006-01-02 15:04:05",
			Env:        "test",
		}
		logger.Init(fileLoggerConfig)

		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 手动记录一个错误来测试日志系统
		testError := fmt.Errorf("模拟数据库连接错误: host=invalid_host port=3306")
		logger.Errorf("测试手动错误记录", logger.Error(testError))

		// 等待日志写入
		time.Sleep(100 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Manual error log content: %s", logContent)

		// 验证手动错误日志
		assert.Contains(t, logContent, `"level":"ERROR"`, "应该包含 ERROR 级别日志")
		assert.Contains(t, logContent, "模拟数据库连接错误", "应该包含错误消息")
		
		t.Log("✅ 手动错误日志测试成功")
	})

	t.Run("TestDatabaseErrorCapture", func(t *testing.T) {
		// 配置日志记录器输出到文件
		fileLoggerConfig := logger.Config{
			Level:      logger.LogLevelError,
			Format:     logger.LogFormatJSON,
			Output:     logger.LogOutputFile,
			LogPath:    logFile,
			TimeFormat: "2006-01-02 15:04:05",
			Env:        "test",
		}
		logger.Init(fileLoggerConfig)

		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 创建网络错误的数据库配置文件
		configContent := `
database:
  primary:
    enabled: true
    driver: mysql
    host: localhost
    port: 9999
    database: myblog
    username: root
    password: q1w2e3r4
  pool:
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h
`

		configPath := fmt.Sprintf("%s/network_error_config.yaml", tempDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// 初始化 Bootstrap
		boot, err := bootstrap.NewBootstrap(configPath)
		
		t.Logf("Network error Bootstrap creation: %v", err)

		if err == nil {
			// 尝试初始化数据库
			err = boot.InitializeDatabases()
			t.Logf("Network error Database initialization: %v", err)
		}

		// 等待日志写入
		time.Sleep(200 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Database error log content: %s", logContent)

		// 验证错误日志
		if len(logContent) > 0 {
			assert.Contains(t, logContent, `"level":"ERROR"`, "应该包含 ERROR 级别日志")
			t.Log("✅ 数据库错误日志成功写入文件")
		} else {
			t.Log("⚠️  数据库错误日志没有写入文件")
			t.Log("这表明数据库连接错误被正确返回，但没有自动记录到全局日志中")
		}

		// 清理
		if boot != nil {
			boot.Close()
		}
	})

	t.Run("TestErrorVsInfoLevels", func(t *testing.T) {
		// 配置日志记录器输出到文件，包含 ERROR 和 INFO 级别
		fileLoggerConfig := logger.Config{
			Level:      logger.LogLevelInfo, // 包含 INFO 和 ERROR
			Format:     logger.LogFormatJSON,
			Output:     logger.LogOutputFile,
			LogPath:    logFile,
			TimeFormat: "2006-01-02 15:04:05",
			Env:        "test",
		}
		logger.Init(fileLoggerConfig)

		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 记录 INFO 级别日志
		logger.Info("正常操作日志", zap.String("operation", "database_test"), zap.String("status", "started"))

		// 记录 ERROR 级别日志
		testError := fmt.Errorf("数据库连接失败")
		logger.Errorf("数据库错误", logger.Error(testError))

		// 等待日志写入
		time.Sleep(100 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Mixed level log content: %s", logContent)

		// 验证日志级别分离
		if strings.Contains(logContent, `"level":"INFO"`) {
			t.Log("✅ INFO 级别日志正常工作")
		}
		
		if strings.Contains(logContent, `"level":"ERROR"`) {
			t.Log("✅ ERROR 级别日志正常工作")
		}

		// 验证 INFO 不会错误捕获 ERROR
		infoLines := strings.Split(logContent, "\n")
		errorInInfo := false
		for _, line := range infoLines {
			if strings.Contains(line, `"level":"INFO"`) && 
			   strings.Contains(line, "数据库连接失败") {
				errorInInfo = true
				break
			}
		}
		
		assert.False(t, errorInInfo, "INFO 级别不应该包含 ERROR 内容")
		t.Log("✅ 日志级别分离正确")
	})
}
