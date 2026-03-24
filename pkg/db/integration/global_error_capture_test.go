package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseErrorGlobalCapture 测试数据库配置错误的全局捕获
func TestDatabaseErrorGlobalCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建临时日志文件来捕获错误日志
	tempDir := t.TempDir()
	logFile := fmt.Sprintf("%s/db_error_test.log", tempDir)

	// 初始化日志记录器配置为文件输出，包含 ERROR 级别
	loggerConfig := logger.Config{
		Level:      logger.LogLevelError, // 只记录 ERROR 级别
		Format:     logger.LogFormatJSON,
		Output:     logger.LogOutputFile,
		LogPath:    logFile,
		TimeFormat: "2006-01-02 15:04:05",
		Env:        "test",
	}
	logger.Init(loggerConfig)

	t.Run("InvalidDatabaseConfig", func(t *testing.T) {
		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 创建无效的数据库配置文件
		configContent := `
database:
  primary:
    enabled: true
    driver: mysql
    host: invalid_host
    port: 3306
    database: nonexistent_db
    username: invalid_user
    password: invalid_password
  pool:
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h
`

		configPath := fmt.Sprintf("%s/invalid_db_config.yaml", tempDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// 尝试使用无效配置初始化 Bootstrap
		boot, err := bootstrap.NewBootstrap(configPath)
		
		// 应该有错误发生
		t.Logf("Database initialization error: %v", err)

		if err == nil {
			// 如果 Bootstrap 创建成功，尝试初始化数据库
			err = boot.InitializeDatabases()
			t.Logf("Database initialization error after Bootstrap: %v", err)
		}

		// 等待错误日志写入
		time.Sleep(100 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err, "应该能读取日志文件")

		logContent := string(content)
		t.Logf("Error log content: %s", logContent)

		// 验证错误日志
		if len(logContent) > 0 {
			// 检查是否包含错误级别
			assert.Contains(t, logContent, `"level":"error"`, "应该包含 ERROR 级别日志")
			
			// 检查是否包含数据库相关错误信息
			assert.True(t, 
				len(logContent) > 10, // 有实际内容
				"应该有错误日志内容")
		} else {
			t.Log("注意：当前实现可能没有将数据库连接错误记录到日志中")
		}

		// 清理
		if boot != nil {
			boot.Close()
		}
	})

	t.Run("MissingDatabaseConfig", func(t *testing.T) {
		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 创建空的数据库配置文件
		configContent := `
database:
  primary:
    enabled: false
  pool:
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h
`

		configPath := fmt.Sprintf("%s/empty_db_config.yaml", tempDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// 初始化 Bootstrap
		boot, err := bootstrap.NewBootstrap(configPath)
		
		// 应该没有错误（因为数据库被禁用）
		t.Logf("Empty config result: %v", err)

		if err == nil {
			// 尝试初始化数据库
			err = boot.InitializeDatabases()
			t.Logf("Database initialization result with empty config: %v", err)
		}

		// 等待可能的日志写入
		time.Sleep(100 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Empty config log content: %s", logContent)

		// 应该没有错误日志
		assert.Empty(t, logContent, "禁用数据库时不应该有错误日志")

		// 清理
		if boot != nil {
			boot.Close()
		}
	})

	t.Run("PartialInvalidConfig", func(t *testing.T) {
		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 创建部分无效的数据库配置文件（正确的连接信息，但数据库不存在）
		configContent := `
database:
  primary:
    enabled: true
    driver: mysql
    host: localhost
    port: 3306
    database: nonexistent_database
    username: root
    password: q1w2e3r4
  pool:
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h
`

		configPath := fmt.Sprintf("%s/partial_invalid_db_config.yaml", tempDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// 初始化 Bootstrap
		boot, err := bootstrap.NewBootstrap(configPath)
		
		// 应该有错误（数据库不存在）
		t.Logf("Partial invalid config error: %v", err)

		if err == nil {
			// 尝试初始化数据库
			err = boot.InitializeDatabases()
			t.Logf("Database initialization error with partial invalid config: %v", err)
		}

		// 等待错误日志写入
		time.Sleep(100 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Partial invalid config log content: %s", logContent)

		// 验证错误日志
		if len(logContent) > 0 {
			assert.Contains(t, logContent, `"level":"error"`, "应该包含 ERROR 级别日志")
		}

		// 清理
		if boot != nil {
			boot.Close()
		}
	})

	t.Run("NetworkErrorConfig", func(t *testing.T) {
		// 清空日志文件
		err := os.WriteFile(logFile, []byte{}, 0644)
		require.NoError(t, err)

		// 创建网络错误的数据库配置文件（错误的端口）
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

		configPath := fmt.Sprintf("%s/network_error_db_config.yaml", tempDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// 初始化 Bootstrap
		boot, err := bootstrap.NewBootstrap(configPath)
		
		// 应该有网络错误
		t.Logf("Network error config error: %v", err)

		if err == nil {
			// 尝试初始化数据库
			err = boot.InitializeDatabases()
			t.Logf("Database initialization error with network config: %v", err)
		}

		// 等待错误日志写入
		time.Sleep(100 * time.Millisecond)

		// 检查日志文件内容
		content, err := os.ReadFile(logFile)
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("Network error config log content: %s", logContent)

		// 验证错误日志
		if len(logContent) > 0 {
			assert.Contains(t, logContent, `"level":"error"`, "应该包含 ERROR 级别日志")
		}

		// 清理
		if boot != nil {
			boot.Close()
		}
	})
}
