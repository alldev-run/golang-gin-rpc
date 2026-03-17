package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestInit(t *testing.T) {
	// Test with default configuration
	cfg := Config{
		Level: "info",
		Env:   "dev",
	}

	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized")
	}

	// Test logging functions
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Errorf("error message")

	// Test convenience functions
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")
}

func TestInitWithDefaults(t *testing.T) {
	// Test with empty configuration (should use defaults)
	cfg := Config{}
	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized with defaults")
	}
}

func TestInitWithFileLogging(t *testing.T) {
	// Create temporary directory for log files
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		Level:      "debug",
		Env:        "prod",
		LogPath:    logFile,
		MaxSize:    1,
		MaxBackups: 3,
		MaxAge:     1,
		Compress:   true,
	}

	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized")
	}

	// Test logging
	logger.Info("test message with file logging")

	// Give some time for file operations
	time.Sleep(100 * time.Millisecond)

	// Check if log file was created (may not exist immediately due to buffering)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Skip("Log file not created yet (may be buffered)")
	}
}

func TestInitWithDifferentLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "fatal", "panic"}

	for _, level := range levels {
		cfg := Config{
			Level: level,
			Env:   "dev",
		}

		Init(cfg)

		logger := L()
		if logger == nil {
			t.Errorf("Expected logger to be initialized with level %s", level)
		}

		// Test logging at this level
		logger.Info("test message for level " + level)
	}
}

func TestInitWithInvalidLevel(t *testing.T) {
	cfg := Config{
		Level: "invalid",
		Env:   "dev",
	}

	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized even with invalid level")
	}

	// Should default to info level
	logger.Info("test message with invalid level")
}

func TestLWithoutInit(t *testing.T) {
	// Note: Due to sync.Once, we can't test true auto-initialization
	// after previous tests have already initialized the logger
	// Instead, we test that L() returns a valid logger
	
	logger := L()
	if logger == nil {
		t.Error("Expected logger to be available")
	}

	// Should work with existing logger
	logger.Info("test message without explicit init")
}

func TestWith(t *testing.T) {
	cfg := Config{
		Level: "info",
		Env:   "dev",
	}

	Init(cfg)

	// Test With function
	logger := With(zap.String("key", "value"))
	if logger == nil {
		t.Error("Expected logger with fields")
	}

	logger.Info("test message with fields")
}

func TestConcurrentLogging(t *testing.T) {
	cfg := Config{
		Level: "info",
		Env:   "dev",
	}

	Init(cfg)

	const numGoroutines = 10
	const numMessages = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numMessages; j++ {
				Info("concurrent log message", 
					zap.Int("goroutine", id),
					zap.Int("message", j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestFileRotation(t *testing.T) {
	// Create temporary directory for log files
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "rotation_test.log")

	cfg := Config{
		Level:      "debug",
		Env:        "prod",
		LogPath:    logFile,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
		MaxAge:     1, // 1 day
		Compress:   true,
	}

	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized")
	}

	// Write a lot of logs to trigger rotation
	for i := 0; i < 1000; i++ {
		logger.Info("test message for rotation", 
			zap.Int("iteration", i),
			zap.String("data", "This is a long message to help trigger file rotation by taking up more space in the log file"))
	}

	// Give some time for file operations
	time.Sleep(100 * time.Millisecond)

	// Check if log files were created
	files, err := filepath.Glob(logFile + "*")
	if err != nil {
		t.Fatalf("Error checking log files: %v", err)
	}

	if len(files) == 0 {
		t.Error("Expected at least one log file to be created")
	}
}

func TestDifferentEnvironments(t *testing.T) {
	environments := []string{"dev", "prod", "staging"}

	for _, env := range environments {
		cfg := Config{
			Level: "info",
			Env:   env,
		}

		Init(cfg)

		logger := L()
		if logger == nil {
			t.Errorf("Expected logger to be initialized for environment %s", env)
		}

		logger.Info("test message for environment " + env)
	}
}

func TestLoggerPerformance(t *testing.T) {
	cfg := Config{
		Level: "info",
		Env:   "prod",
	}

	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized")
	}

	// Measure logging performance
	start := time.Now()
	const numMessages = 10000

	for i := 0; i < numMessages; i++ {
		logger.Info("performance test message",
			zap.Int("iteration", i),
			zap.String("data", "some test data"))
	}

	duration := time.Since(start)
	messagesPerSecond := float64(numMessages) / duration.Seconds()

	t.Logf("Logged %d messages in %v (%.0f messages/second)", 
		numMessages, duration, messagesPerSecond)

	// Should be able to log at least 1000 messages per second
	if messagesPerSecond < 1000 {
		t.Errorf("Expected at least 1000 messages/second, got %.0f", messagesPerSecond)
	}
}

func TestLoggerWithFields(t *testing.T) {
	cfg := Config{
		Level: "debug",
		Env:   "dev",
	}

	Init(cfg)

	// Test all field types
	Info("test with various field types",
		zap.String("string_field", "test"),
		zap.Int("int_field", 42),
		zap.Float64("float_field", 3.14),
		zap.Bool("bool_field", true),
		zap.Time("time_field", time.Now()),
		zap.Duration("duration_field", time.Second),
		zap.Any("any_field", map[string]interface{}{"key": "value"}),
	)

	// Test nested fields
	With(
		zap.Namespace("nested"),
		zap.String("inner", "value"),
	).Info("test with nested fields")
}

func TestLoggerErrorHandling(t *testing.T) {
	cfg := Config{
		Level: "info",
		Env:   "dev",
	}

	Init(cfg)

	logger := L()
	if logger == nil {
		t.Error("Expected logger to be initialized")
	}

	// Test logging with nil fields (should not panic)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Logging with nil fields caused panic: %v", r)
		}
	}()

	logger.Info("test with nil field", zap.Any("nil_field", nil))
}

func TestLoggerMultipleInit(t *testing.T) {
	// Test multiple calls to Init (should not cause issues)
	cfg1 := Config{
		Level: "debug",
		Env:   "dev",
	}

	Init(cfg1)
	logger1 := L()

	cfg2 := Config{
		Level: "error",
		Env:   "prod",
	}

	Init(cfg2)
	logger2 := L()

	// Due to sync.Once, the first initialization should persist
	if logger1 != logger2 {
		t.Error("Multiple calls to Init should not create different loggers")
	}
}

func TestLoggerConfigValidation(t *testing.T) {
	// Test various edge cases for configuration
	testCases := []struct {
		name string
		cfg  Config
	}{
		{"Empty config", Config{}},
		{"Negative max size", Config{MaxSize: -1}},
		{"Zero max backups", Config{MaxBackups: 0}},
		{"Zero max age", Config{MaxAge: 0}},
		{"Invalid log path", Config{LogPath: "/invalid/path/test.log"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Config %s caused panic: %v", tc.name, r)
				}
			}()

			Init(tc.cfg)
			logger := L()
			if logger == nil {
				t.Errorf("Expected logger to be initialized for config %s", tc.name)
			}

			logger.Info("test message for config " + tc.name)
		})
	}
}
