package httplog

import (
	"time"

	"alldev-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

type Fields struct {
	Method    string
	Path      string
	ClientIP  string
	UserAgent string
	Status    int
	Latency   time.Duration
	RequestID string
}

// Log 记录 HTTP 请求日志，使用 pkg/logger 包
func Log(f Fields) {
	// 创建带有请求ID的子 logger（如果有的话）
	var log *zap.Logger
	if f.RequestID != "" {
		log = logger.With(
			logger.String("request_id", f.RequestID),
		)
	} else {
		log = logger.L()
	}

	// 记录日志
	log.Info("HTTP Request",
		logger.String("method", f.Method),
		logger.String("path", f.Path),
		logger.String("client_ip", f.ClientIP),
		logger.Int("status", f.Status),
		logger.Duration("latency", f.Latency),
		logger.String("user_agent", f.UserAgent),
	)
}

// LogWithLevel 根据状态码记录不同级别的日志
func LogWithLevel(f Fields) {
	var logFunc func(string, ...zap.Field)
	
	// 根据状态码确定日志级别
	switch {
	case f.Status >= 500:
		logFunc = logger.Errorf
	case f.Status >= 400:
		logFunc = logger.Warn
	case f.Status >= 300:
		logFunc = logger.Info
	default:
		logFunc = logger.Info
	}

	// 创建带有请求ID的字段列表
	fields := []zap.Field{
		logger.String("method", f.Method),
		logger.String("path", f.Path),
		logger.String("client_ip", f.ClientIP),
		logger.Int("status", f.Status),
		logger.Duration("latency", f.Latency),
		logger.String("user_agent", f.UserAgent),
	}
	
	// 如果有请求ID，添加到字段中
	if f.RequestID != "" {
		fields = append(fields, logger.String("request_id", f.RequestID))
	}

	// 使用对应的日志级别记录
	logFunc("HTTP Request", fields...)
}

// LogError 记录错误级别的 HTTP 日志
func LogError(f Fields, errorMsg string) {
	var log *zap.Logger
	if f.RequestID != "" {
		log = logger.With(
			logger.String("request_id", f.RequestID),
		)
	} else {
		log = logger.L()
	}

	log.Error("HTTP Request Error",
		logger.String("error", errorMsg),
		logger.String("method", f.Method),
		logger.String("path", f.Path),
		logger.String("client_ip", f.ClientIP),
		logger.Int("status", f.Status),
		logger.Duration("latency", f.Latency),
		logger.String("user_agent", f.UserAgent),
	)
}

// LogSlowRequest 记录慢请求日志
func LogSlowRequest(f Fields, threshold time.Duration) {
	var log *zap.Logger
	if f.RequestID != "" {
		log = logger.With(
			logger.String("request_id", f.RequestID),
		)
	} else {
		log = logger.L()
	}

	log.Warn("HTTP Slow Request",
		logger.Duration("threshold", threshold),
		logger.String("method", f.Method),
		logger.String("path", f.Path),
		logger.String("client_ip", f.ClientIP),
		logger.Int("status", f.Status),
		logger.Duration("latency", f.Latency),
		logger.String("user_agent", f.UserAgent),
	)
}
