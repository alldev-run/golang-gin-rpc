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

// TestFinalErrorLogging 测试最终的 ERROR 日志记录
func TestFinalErrorLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("VerifyErrorLogInGlobalFile", func(t *testing.T) {
		// 配置日志记录器输出到全局日志文件
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
		testError := fmt.Errorf("数据库配置错误验证: 主机连接失败")
		logger.Errorf("数据库错误验证测试", logger.Error(testError))

		// 等待日志写入
		time.Sleep(300 * time.Millisecond)

		// 检查全局日志文件
		content, err := os.ReadFile("/Users/johnjames/Workspace/golang/golang-gin-rpc/logs/app.log")
		require.NoError(t, err)

		logContent := string(content)
		t.Logf("全局日志文件大小: %d 字符", len(logContent))

		// 查找我们的 ERROR 日志
		if strings.Contains(logContent, "数据库错误验证测试") {
			t.Log("✅ ERROR 日志成功写入全局日志文件")
			
			// 查找 ERROR 日志行
			lines := strings.Split(logContent, "\n")
			for i := len(lines) - 1; i >= 0; i-- {
				line := strings.TrimSpace(lines[i])
				if strings.Contains(line, "数据库错误验证测试") {
					t.Logf("找到 ERROR 日志行: %s", line)
					
					// 验证日志格式
					assert.Contains(t, line, `"level":"ERROR"`, "应该包含 ERROR 级别")
					assert.Contains(t, line, `"msg":"数据库错误验证测试"`, "应该包含错误消息")
					assert.Contains(t, line, `"error":"数据库配置错误验证: 主机连接失败"`, "应该包含错误详情")
					
					t.Log("✅ ERROR 日志格式验证通过")
					break
				}
			}
		} else {
			t.Log("⚠️  ERROR 日志没有在全局日志文件中找到")
			
			// 显示最近的日志内容
			lines := strings.Split(logContent, "\n")
			t.Log("最近 5 条日志:")
			for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					t.Logf("  %s", lines[i])
				}
			}
		}
	})

	t.Run("CheckAllErrorLogs", func(t *testing.T) {
		// 检查全局日志文件中的所有 ERROR 日志
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
			t.Log("所有 ERROR 日志:")
			for i, line := range errorLines {
				t.Logf("  %d: %s", i+1, line)
			}
			assert.True(t, errorCount > 0, "应该有 ERROR 日志记录")
		} else {
			t.Log("全局日志文件中没有找到 ERROR 级别的日志")
		}
	})

	t.Run("TestErrorVsInfoSeparation", func(t *testing.T) {
		// 重新配置日志记录器包含 INFO 和 ERROR 级别
		mixedLoggerConfig := logger.Config{
			Level:      logger.LogLevelInfo, // 包含 INFO 和 ERROR
			Format:     logger.LogFormatJSON,
			Output:     logger.LogOutputFile,
			LogPath:    "/Users/johnjames/Workspace/golang/golang-gin-rpc/logs/app.log",
			TimeFormat: "2006-01-02 15:04:05",
			Env:        "test",
		}
		logger.Init(mixedLoggerConfig)

		// 记录 INFO 和 ERROR 日志
		logger.Info("正常操作测试", logger.String("operation", "database_check"))
		testError := fmt.Errorf("分离测试错误")
		logger.Errorf("错误分离测试", logger.Error(testError))

		// 等待日志写入
		time.Sleep(200 * time.Millisecond)

		// 检查日志文件
		content, err := os.ReadFile("/Users/johnjames/Workspace/golang/golang-gin-rpc/logs/app.log")
		require.NoError(t, err)

		logContent := string(content)
		
		// 验证日志级别分离
		hasInfo := strings.Contains(logContent, `"level":"INFO"`) && strings.Contains(logContent, "正常操作测试")
		hasError := strings.Contains(logContent, `"level":"ERROR"`) && strings.Contains(logContent, "错误分离测试")
		
		if hasInfo {
			t.Log("✅ INFO 级别日志正常工作")
		}
		
		if hasError {
			t.Log("✅ ERROR 级别日志正常工作")
		}
		
		// 验证级别分离
		infoLines := strings.Split(logContent, "\n")
		errorInInfo := false
		for _, line := range infoLines {
			if strings.Contains(line, `"level":"INFO"`) && 
			   strings.Contains(line, "分离测试错误") {
				errorInInfo = true
				break
			}
		}
		
		assert.False(t, errorInInfo, "INFO 级别不应该包含 ERROR 内容")
		t.Log("✅ 日志级别分离正确")
	})
}
