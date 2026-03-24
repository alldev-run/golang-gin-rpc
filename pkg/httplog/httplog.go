package httplog

import (
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

type Fields struct {
	Method         string
	Path           string
	Query          string
	PathParams     map[string]string
	QueryParams    map[string][]string
	ClientIP       string
	UserAgent      string
	Status         int
	Latency        time.Duration
	RequestID      string
	RequestBody    *interface{} `json:"request_body,omitempty"`
	RequestBodyStr *string       `json:"request_body_str,omitempty"`
	ResponseBody   *interface{} `json:"response_body,omitempty"`
	ResponseBodyStr *string      `json:"response_body_str,omitempty"`
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
	// 根据状态码确定日志级别
	switch {
	case f.Status >= 500:
		logger.Errorf("HTTP Request",
			logger.String("method", f.Method),
			logger.String("path", f.Path),
			logger.String("client_ip", f.ClientIP),
			logger.Int("status", f.Status),
			logger.Duration("latency", f.Latency),
			logger.String("user_agent", f.UserAgent),
			logger.String("request_id", f.RequestID),
		)
	case f.Status >= 400:
		logger.Warn("HTTP Request",
			logger.String("method", f.Method),
			logger.String("path", f.Path),
			logger.String("client_ip", f.ClientIP),
			logger.Int("status", f.Status),
			logger.Duration("latency", f.Latency),
			logger.String("user_agent", f.UserAgent),
			logger.String("request_id", f.RequestID),
		)
	default:
		logger.Info("HTTP Request",
			logger.String("method", f.Method),
			logger.String("path", f.Path),
			logger.String("client_ip", f.ClientIP),
			logger.Int("status", f.Status),
			logger.Duration("latency", f.Latency),
			logger.String("user_agent", f.UserAgent),
			logger.String("request_id", f.RequestID),
		)
	}
}

// LogWithLevelEnhanced 增强版日志记录，支持请求体和响应体
func LogWithLevelEnhanced(f Fields) {
	// 使用默认阈值
	LogWithLevelEnhancedWithThresholds(f, 500, 400, 200)
}

// LogWithLevelEnhancedWithThresholds 增强版日志记录，支持自定义阈值
func LogWithLevelEnhancedWithThresholds(f Fields, errorThreshold, warnThreshold, infoThreshold int) {
	// 构建基础日志字段
	baseFields := []zap.Field{
		zap.String("method", f.Method),
		zap.String("path", f.Path),
		zap.String("query", f.Query),
		zap.String("client_ip", f.ClientIP),
		zap.Int("status", f.Status),
		zap.Duration("latency", f.Latency),
		zap.String("user_agent", f.UserAgent),
		zap.String("request_id", f.RequestID),
	}

	if len(f.PathParams) > 0 {
		baseFields = append(baseFields, zap.Any("path_param", f.PathParams))
		baseFields = append(baseFields, zap.Any("path_params", f.PathParams))
	}

	if len(f.QueryParams) > 0 {
		baseFields = append(baseFields, zap.Any("query_param", f.QueryParams))
		baseFields = append(baseFields, zap.Any("query_params", f.QueryParams))
	}
	
	// 添加请求体字段
	if f.RequestBody != nil {
		baseFields = append(baseFields, zap.Any("request_body", *f.RequestBody))
	} else if f.RequestBodyStr != nil {
		baseFields = append(baseFields, zap.String("request_body", *f.RequestBodyStr))
	}
	
	// 添加响应体字段
	if f.ResponseBody != nil {
		baseFields = append(baseFields, zap.Any("response_body", *f.ResponseBody))
	} else if f.ResponseBodyStr != nil {
		baseFields = append(baseFields, zap.String("response_body", *f.ResponseBodyStr))
	}
	
	// 根据自定义阈值确定日志级别并记录
	switch {
	case f.Status >= errorThreshold:
		logger.Errorf("HTTP Request", baseFields...)
	case f.Status >= warnThreshold:
		logger.Warn("HTTP Request", baseFields...)
	case f.Status >= infoThreshold:
		logger.Info("HTTP Request", baseFields...)
	default:
		// 低于 info 阈值的不记录，或者记录为 debug
		logger.Debug("HTTP Request", baseFields...)
	}
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

	log.Warn("Slow HTTP Request",
		logger.String("method", f.Method),
		logger.String("path", f.Path),
		logger.String("client_ip", f.ClientIP),
		logger.Int("status", f.Status),
		logger.Duration("latency", f.Latency),
		logger.Duration("threshold", threshold),
		logger.String("user_agent", f.UserAgent),
	)
}

// LogSlowRequestEnhanced 增强版慢请求日志记录
func LogSlowRequestEnhanced(f Fields, threshold time.Duration) {
	// 构建基础日志字段
	baseFields := []zap.Field{
		zap.String("method", f.Method),
		zap.String("path", f.Path),
		zap.String("query", f.Query),
		zap.String("client_ip", f.ClientIP),
		zap.Int("status", f.Status),
		zap.Duration("latency", f.Latency),
		zap.Duration("threshold", threshold),
		zap.String("user_agent", f.UserAgent),
		zap.String("request_id", f.RequestID),
	}

	if len(f.PathParams) > 0 {
		baseFields = append(baseFields, zap.Any("path_param", f.PathParams))
		baseFields = append(baseFields, zap.Any("path_params", f.PathParams))
	}

	if len(f.QueryParams) > 0 {
		baseFields = append(baseFields, zap.Any("query_param", f.QueryParams))
		baseFields = append(baseFields, zap.Any("query_params", f.QueryParams))
	}
	
	// 添加请求体字段（慢请求通常需要上下文信息）
	if f.RequestBody != nil {
		baseFields = append(baseFields, zap.Any("request_body", *f.RequestBody))
	} else if f.RequestBodyStr != nil {
		baseFields = append(baseFields, zap.String("request_body", *f.RequestBodyStr))
	}
	
	// 记录慢请求日志
	logger.Warn("Slow HTTP Request", baseFields...)
}
