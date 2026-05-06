package postgres

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("DefaultConfig() Host = %v, want localhost", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("DefaultConfig() Port = %v, want 5432", cfg.Port)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("DefaultConfig() SSLMode = %v, want disable", cfg.SSLMode)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("DefaultConfig() MaxOpenConns = %v, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("DefaultConfig() MaxIdleConns = %v, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("DefaultConfig() ConnMaxLifetime = %v, want 1h", cfg.ConnMaxLifetime)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Host:            "127.0.0.1",
		Port:            5433,
		Database:        "testdb",
		Username:        "testuser",
		Password:        "testpass",
		SSLMode:         "require",
		MaxOpenConns:    50,
		MaxIdleConns:    20,
		ConnMaxLifetime: 30 * time.Minute,
	}

	if cfg.Host != "127.0.0.1" {
		t.Error("Config struct assignment failed for Host")
	}
	if cfg.Port != 5433 {
		t.Error("Config struct assignment failed for Port")
	}
	if cfg.Database != "testdb" {
		t.Error("Config struct assignment failed for Database")
	}
	if cfg.SSLMode != "require" {
		t.Error("Config struct assignment failed for SSLMode")
	}
}

func TestConfigWithLogging(t *testing.T) {
	cfg := Config{
		Host:               "localhost",
		Port:               5432,
		Database:           "testdb",
		Username:           "testuser",
		Password:           "testpass",
		LogEnabled:         true,
		LogLevel:           "debug",
		SlowQueryThreshold: 200 * time.Millisecond,
	}

	if !cfg.LogEnabled {
		t.Error("LogEnabled should be true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.SlowQueryThreshold != 200*time.Millisecond {
		t.Errorf("SlowQueryThreshold = %v, want 200ms", cfg.SlowQueryThreshold)
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Host:     "invalid.host.that.does.not.exist",
		Port:     5432,
		Database: "test",
		Username: "test",
		Password: "test",
		SSLMode:  "disable",
	}

	client, err := New(cfg)
	if err == nil {
		if client != nil {
			_ = client.Close()
		}
		t.Log("Connection succeeded unexpectedly")
	} else {
		t.Logf("Expected connection error: %v", err)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"error", LogLevelError},
		{"ERROR", LogLevelError},
		{"Error", LogLevelError},
		{"warn", LogLevelWarn},
		{"WARN", LogLevelWarn},
		{"info", LogLevelInfo},
		{"INFO", LogLevelInfo},
		{"debug", LogLevelDebug},
		{"DEBUG", LogLevelDebug},
		{"trace", LogLevelTrace},
		{"TRACE", LogLevelTrace},
		{"invalid", LogLevelInfo}, // default
		{"", LogLevelInfo},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelError, "ERROR"},
		{LogLevelWarn, "WARN"},
		{LogLevelInfo, "INFO"},
		{LogLevelDebug, "DEBUG"},
		{LogLevelTrace, "TRACE"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("LogLevel(%v).String() = %q, want %q", tt.level, result, tt.expected)
			}
		})
	}
}

func TestNewSQLLogger(t *testing.T) {
	logger := NewSQLLogger("info", 100*time.Millisecond)
	if logger == nil {
		t.Fatal("NewSQLLogger returned nil")
	}
	if logger.level != LogLevelInfo {
		t.Errorf("logger.level = %v, want LogLevelInfo", logger.level)
	}
	if logger.slowQueryThreshold != 100*time.Millisecond {
		t.Errorf("logger.slowQueryThreshold = %v, want 100ms", logger.slowQueryThreshold)
	}
}

func TestSQLLoggerLogQuery(t *testing.T) {
	// Test with info level (should log)
	logger := NewSQLLogger("info", 100*time.Millisecond)

	// This test verifies the logger doesn't panic
	// Actual log output is handled by the logger package
	logger.LogQuery("SELECT * FROM users", []interface{}{1, 2}, 50*time.Millisecond, nil)
	logger.LogQuery("SELECT * FROM users", []interface{}{1, 2}, 150*time.Millisecond, nil) // slow query
	logger.LogQuery("SELECT * FROM users", []interface{}{1, 2}, 50*time.Millisecond, errors.New("test error"))

	// Test with error level (should not log info queries)
	errorLogger := NewSQLLogger("error", 100*time.Millisecond)
	errorLogger.LogQuery("SELECT * FROM users", []interface{}{1, 2}, 50*time.Millisecond, nil)
}
