package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestServiceLogger(t *testing.T) {
	// Clean up any existing test logs
	testDir := "./logs/test-service"
	os.RemoveAll(testDir)

	// Create service logger config
	config := DefaultServiceLoggerConfig("test-service")
	config.BaseDir = "./logs"
	config.EnableDateFolder = true
	config.SeparateByLevel = false
	config.InheritGlobalConfig = false
	config.OverrideConfig = TestConfig()

	// Test service logger creation
	logger := GetServiceLoggerInstance("test-service", config)
	if logger == nil {
		t.Fatal("Failed to create service logger")
	}

	// Test logging
	logger.Info("Test info message", zap.String("key", "value"))
	logger.Error("Test error message", zap.Error(os.ErrNotExist))

	// Test service logger wrapper
	serviceLogger := NewServiceLoggerFromConfig("test-service", config)
	serviceLogger.Info("Test from wrapper", zap.String("wrapper", "true"))

	// Verify log file was created
	logPath := GetServiceLogPath("test-service", config)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file was not created at %s", logPath)
	}

	// Clean up
	os.RemoveAll("./logs/test-service")
}

func TestServiceLoggerWithDateFolders(t *testing.T) {
	// Clean up any existing test logs
	testDir := "./logs/date-test"
	os.RemoveAll(testDir)

	// Create service logger config with date folders
	config := DefaultServiceLoggerConfig("date-test")
	config.BaseDir = "./logs"
	config.EnableDateFolder = true
	config.SeparateByLevel = false
	config.InheritGlobalConfig = false
	config.OverrideConfig = TestConfig()

	logger := GetServiceLoggerInstance("date-test", config)
	logger.Info("Test message with date folder")

	// Verify date folder structure
	today := time.Now().Format("2006-01-02")
	expectedDir := filepath.Join("./logs", "date-test", today)
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Date folder was not created at %s", expectedDir)
	}

	// Verify log file in date folder
	expectedFile := filepath.Join(expectedDir, "date-test.log")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("Log file was not created at %s", expectedFile)
	}

	// Clean up
	os.RemoveAll("./logs/date-test")
}

func TestServiceLoggerWithSeparateLevels(t *testing.T) {
	// Clean up any existing test logs
	testDir := "./logs/separate-test"
	os.RemoveAll(testDir)

	// Create service logger config with separate levels
	config := DefaultServiceLoggerConfig("separate-test")
	config.BaseDir = "./logs"
	config.EnableDateFolder = false
	config.SeparateByLevel = true
	config.InheritGlobalConfig = false
	config.OverrideConfig = TestConfig()
	config.OverrideConfig.Level = LogLevelDebug

	logger := GetServiceLoggerInstance("separate-test", config)
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	// Verify separate log files
	expectedFiles := []string{
		"separate-test.debug.log",
		"separate-test.info.log",
		"separate-test.warn.log",
		"separate-test.error.log",
	}

	for _, filename := range expectedFiles {
		expectedFile := filepath.Join("./logs", "separate-test", filename)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Log file was not created at %s", expectedFile)
		}
	}

	// Clean up
	os.RemoveAll("./logs/separate-test")
}

func TestMultipleServices(t *testing.T) {
	// Clean up any existing test logs
	os.RemoveAll("./logs/multi-test")

	// Test multiple services
	services := []string{"api-gateway", "rpc", "websocket"}

	for _, serviceName := range services {
		config := DefaultServiceLoggerConfig(serviceName)
		config.BaseDir = "./logs"
		config.EnableDateFolder = true
		config.SeparateByLevel = false
		config.InheritGlobalConfig = false
		config.OverrideConfig = TestConfig()

		logger := GetServiceLoggerInstance(serviceName, config)
		logger.Info("Message from "+serviceName, zap.String("service", serviceName))

		// Verify service-specific directory
		today := time.Now().Format("2006-01-02")
		expectedDir := filepath.Join("./logs", serviceName, today)
		if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
			t.Errorf("Service directory was not created at %s", expectedDir)
		}
	}

	// Clean up
	for _, serviceName := range services {
		os.RemoveAll(filepath.Join("./logs", serviceName))
	}
}

func TestServiceLoggerWithFields(t *testing.T) {
	// Clean up any existing test logs
	testDir := "./logs/fields-test"
	os.RemoveAll(testDir)

	// Create service logger
	config := DefaultServiceLoggerConfig("fields-test")
	config.BaseDir = "./logs"
	config.InheritGlobalConfig = false
	config.OverrideConfig = TestConfig()

	serviceLogger := NewServiceLoggerFromConfig("fields-test", config)
	
	// Test with additional fields
	loggerWithFields := serviceLogger.With(
		zap.String("component", "test"),
		zap.Int("request_id", 12345),
	)
	
	loggerWithFields.Info("Message with fields")

	// Clean up
	os.RemoveAll("./logs/fields-test")
}

func TestCleanupOldLogs(t *testing.T) {
	// Create test directory structure
	testDir := "./logs/cleanup-test"
	os.RemoveAll(testDir)

	// Create old log directory (3 days ago)
	oldDate := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	oldDir := filepath.Join(testDir, "test-service", oldDate)
	os.MkdirAll(oldDir, 0755)
	
	// Create a file in old directory with old modification time
	oldFile := filepath.Join(oldDir, "test-service.log")
	file, _ := os.Create(oldFile)
	file.Close()
	
	// Set file modification time to 3 days ago
	oldTime := time.Now().AddDate(0, 0, -3)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Create recent log directory (today)
	today := time.Now().Format("2006-01-02")
	recentDir := filepath.Join(testDir, "test-service", today)
	os.MkdirAll(recentDir, 0755)
	
	// Create a file in recent directory
	recentFile := filepath.Join(recentDir, "test-service.log")
	file, _ = os.Create(recentFile)
	file.Close()

	// Run cleanup with max age of 2 days
	err := CleanupOldLogs(testDir, 2)
	if err != nil {
		t.Errorf("CleanupOldLogs failed: %v", err)
	}

	// Verify old directory was removed
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("Old directory was not removed: %s", oldDir)
	}

	// Verify recent directory still exists
	if _, err := os.Stat(recentDir); os.IsNotExist(err) {
		t.Errorf("Recent directory was incorrectly removed: %s", recentDir)
	}

	// Clean up
	os.RemoveAll(testDir)
}

func TestServiceLoggerConfigValidation(t *testing.T) {
	// Test default config
	config := DefaultServiceLoggerConfig("test")
	if config.ServiceName != "test" {
		t.Errorf("Expected service name 'test', got '%s'", config.ServiceName)
	}
	if config.BaseDir != "./logs" {
		t.Errorf("Expected base dir './logs', got '%s'", config.BaseDir)
	}
	if !config.EnableDateFolder {
		t.Error("Expected EnableDateFolder to be true")
	}
	if config.SeparateByLevel {
		t.Error("Expected SeparateByLevel to be false")
	}
	if !config.InheritGlobalConfig {
		t.Error("Expected InheritGlobalConfig to be true")
	}

	// Test custom config
	customConfig := ServiceLoggerConfig{
		ServiceName:       "custom",
		BaseDir:           "/custom/logs",
		EnableDateFolder:  false,
		SeparateByLevel:   true,
		InheritGlobalConfig: false,
	}

	if customConfig.ServiceName != "custom" {
		t.Errorf("Expected service name 'custom', got '%s'", customConfig.ServiceName)
	}
	if customConfig.BaseDir != "/custom/logs" {
		t.Errorf("Expected base dir '/custom/logs', got '%s'", customConfig.BaseDir)
	}
	if customConfig.EnableDateFolder {
		t.Error("Expected EnableDateFolder to be false")
	}
	if !customConfig.SeparateByLevel {
		t.Error("Expected SeparateByLevel to be true")
	}
	if customConfig.InheritGlobalConfig {
		t.Error("Expected InheritGlobalConfig to be false")
	}
}
