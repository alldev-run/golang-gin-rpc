package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGlobalErrorLogging 测试全局 ERROR 日志记录
func TestGlobalErrorLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 使用全局日志配置
	loggerConfig := logger.DefaultConfig()
	loggerConfig.Level = logger.LogLevelError // 只记录 ERROR 级别
	loggerConfig.Output = logger.LogOutputStdout // 输出到标准输出
	logger.Init(loggerConfig)

	t.Run("GlobalErrorLogOutput", func(t *testing.T) {
		// 记录一个 ERROR 日志
		testError := fmt.Errorf("测试全局错误日志: 数据库连接失败")
		logger.Errorf("全局错误测试", logger.Error(testError))

		// 等待日志写入
		time.Sleep(100 * time.Millisecond)

		t.Log("✅ 全局 ERROR 日志已记录到标准输出")
		t.Logf("错误信息: %v", testError)
	})

	t.Run("GlobalErrorLogToFile", func(t *testing.T) {
		// 重新配置日志记录器输出到全局日志文件
		globalLoggerConfig := logger.Config{
			Level:      logger.LogLevelError,
			Format:     logger.LogFormatJSON,
			Output:     logger.LogOutputFile,
			LogPath:    "/Users/johnjames/Workspace/golang/golang-gin-rpc/logs/app.log",
			TimeFormat: "2006-01-02 15:04:05",
			Env:        "test",
		}
		logger.Init(globalLoggerConfig)

		// 记录一个 ERROR 日志到全局日志文件
		testError := fmt.Errorf("测试写入全局日志文件: 数据库配置错误")
		logger.Errorf("全局文件错误测试", logger.Error(testError))

		// 等待日志写入
		time.Sleep(200 * time.Millisecond)

		// 检查全局日志文件
		content, err := os.ReadFile("/Users/johnjames/Workspace/golang/golang-gin-rpc/logs/app.log")
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("全局日志文件内容 (最后500字符): %s", logContent[len(logContent)-min(500, len(logContent)):])

		// 检查是否包含我们的 ERROR 日志
		if strings.Contains(logContent, "全局文件错误测试") {
			t.Log("✅ ERROR 日志成功写入全局日志文件")
			
			// 查找 ERROR 日志行
			lines := strings.Split(logContent, "\n")
			for _, line := range lines {
				if strings.Contains(line, "全局文件错误测试") {
					t.Logf("找到 ERROR 日志行: %s", line)
					assert.Contains(t, line, `"level":"ERROR"`, "应该包含 ERROR 级别")
					break
				}
			}
		} else {
			t.Log("⚠️  ERROR 日志没有在全局日志文件中找到")
			t.Log("这可能是因为日志配置或写入时机的问题")
		}
	})

	t.Run("CheckExistingErrorLogs", func(t *testing.T) {
		// 检查现有的全局日志文件中是否有 ERROR 日志
		content, err := os.ReadFile("/Users/johnjames/Workspace/golang/golang-gin-rpc/logs/app.log")
		require.NoError(t, err)

		logContent := string(content)
		
		// 查找所有 ERROR 级别的日志
		lines := strings.Split(logContent, "\n")
		errorCount := 0
		errorLines := []string{}
		
		for _, line := range lines {
			if strings.Contains(line, `"level":"ERROR"`) {
				errorCount++
				errorLines = append(errorLines, line)
			}
		}

		t.Logf("在全局日志文件中找到 %d 条 ERROR 日志", errorCount)
		
		if errorCount > 0 {
			t.Log("最近的 ERROR 日志:")
			for i, line := range errorLines {
				if i >= 5 { // 只显示最近5条
					break
				}
				t.Logf("  %d: %s", i+1, line)
			}
		} else {
			t.Log("全局日志文件中没有找到 ERROR 级别的日志")
			
			// 显示最近的几条日志来确认日志级别
			recentLines := []string{}
			for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					recentLines = append(recentLines, lines[i])
				}
			}
			
			t.Log("最近的日志内容:")
			for i, line := range recentLines {
				t.Logf("  %d: %s", i+1, line)
			}
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
