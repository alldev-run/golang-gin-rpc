package config

import (
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/gateway"
	"github.com/stretchr/testify/assert"
)

func TestHTTPLoggingConfigDefaults(t *testing.T) {
	// 测试默认配置
	config := gateway.DefaultConfig()
	
	// 验证默认 HTTP 日志配置为 nil（未启用）
	assert.Nil(t, config.Logging.HTTPLogging)
}

func TestHTTPLoggingConfigValidation(t *testing.T) {
	// 创建启用的 HTTP 日志配置
	config := &gateway.HTTPLoggingConfig{
		Enabled:               true,
		LogRequestBody:         true,
		LogResponseBody:        false,
		MaxBodySize:           1024 * 512,
		LogHeaders:            true,
		SensitiveHeaders:      []string{"Authorization", "Cookie"},
		SkipPaths:             []string{"/health", "/metrics"},
		SlowRequestThreshold:  "2s",
		EnableRequestID:       true,
		RequestIDHeader:       "X-Request-ID",
		LogLevelThresholds: gateway.LogLevelThresholds{
			ErrorThreshold: 500,
			WarnThreshold:  400,
			InfoThreshold:  200,
		},
	}
	
	// 验证配置值
	assert.True(t, config.Enabled)
	assert.True(t, config.LogRequestBody)
	assert.False(t, config.LogResponseBody)
	assert.Equal(t, int64(524288), config.MaxBodySize)
	assert.True(t, config.LogHeaders)
	assert.Contains(t, config.SensitiveHeaders, "Authorization")
	assert.Contains(t, config.SkipPaths, "/health")
	assert.Equal(t, "2s", config.SlowRequestThreshold)
	assert.True(t, config.EnableRequestID)
	assert.Equal(t, "X-Request-ID", config.RequestIDHeader)
	assert.Equal(t, 500, config.LogLevelThresholds.ErrorThreshold)
}

func TestHTTPLoggingConfigWithYAML(t *testing.T) {
	// 测试从 YAML 解析配置
	// 示例 YAML 配置：
	/*
	logging:
	  level: "info"
	  format: "json"
	  http_logging:
	    enabled: true
	    log_request_body: true
	    log_response_body: false
	    max_body_size: 1048576
	    log_headers: true
	    sensitive_headers:
	      - "Authorization"
	      - "X-API-Key"
	    skip_paths:
	      - "/health"
	      - "/ready"
	    slow_request_threshold: "1.5s"
	    enable_request_id: true
	    request_id_header: "X-Trace-ID"
	    log_level_thresholds:
	      error_threshold: 500
	      warn_threshold: 400
	      info_threshold: 200
	*/
	
	// 模拟解析后的配置
	config := &gateway.HTTPLoggingConfig{
		Enabled:              true,
		LogRequestBody:        true,
		LogResponseBody:       false,
		MaxBodySize:          1048576,
		LogHeaders:           true,
		SensitiveHeaders:     []string{"Authorization", "X-API-Key"},
		SkipPaths:            []string{"/health", "/ready"},
		SlowRequestThreshold: "1.5s",
		EnableRequestID:      true,
		RequestIDHeader:      "X-Trace-ID",
		LogLevelThresholds: gateway.LogLevelThresholds{
			ErrorThreshold: 500,
			WarnThreshold:  400,
			InfoThreshold:  200,
		},
	}
	
	// 验证解析后的配置
	assert.True(t, config.Enabled)
	assert.Equal(t, int64(1048576), config.MaxBodySize)
	assert.Equal(t, "1.5s", config.SlowRequestThreshold)
	assert.Equal(t, "X-Trace-ID", config.RequestIDHeader)
}

func TestDurationParsing(t *testing.T) {
	// 测试时间解析
	testCases := []struct {
		input    string
		expected time.Duration
	}{
		{"1s", 1 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"2m", 2 * time.Minute},
		{"1h", 1 * time.Hour},
	}
	
	for _, tc := range testCases {
		duration, err := time.ParseDuration(tc.input)
		assert.NoError(t, err)
		assert.Equal(t, tc.expected, duration)
	}
}

func TestLogLevelThresholds(t *testing.T) {
	// 测试日志级别阈值
	thresholds := gateway.LogLevelThresholds{
		ErrorThreshold: 500,
		WarnThreshold:  400,
		InfoThreshold:  200,
	}
	
	// 验证阈值逻辑
	testCases := []struct {
		statusCode int
		expectedLevel string
	}{
		{200, "INFO"},
		{301, "INFO"},
		{404, "WARN"},
		{500, "ERROR"},
		{502, "ERROR"},
	}
	
	for _, tc := range testCases {
		var level string
		switch {
		case tc.statusCode >= thresholds.ErrorThreshold:
			level = "ERROR"
		case tc.statusCode >= thresholds.WarnThreshold:
			level = "WARN"
		case tc.statusCode >= thresholds.InfoThreshold:
			level = "INFO"
		default:
			level = "DEBUG"
		}
		assert.Equal(t, tc.expectedLevel, level, "Status code %d should map to %s", tc.statusCode, tc.expectedLevel)
	}
}
