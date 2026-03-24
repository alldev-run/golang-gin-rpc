package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/gateway"
	"github.com/alldev-run/golang-gin-rpc/pkg/httplog"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDFromConfig 从配置创建请求ID中间件
func RequestIDFromConfig(cfg *gateway.Config) gin.HandlerFunc {
	// 检查 HTTP 日志配置是否存在
	if cfg.Logging.HTTPLogging == nil {
		return RequestID()
	}
	
	httpConfig := cfg.Logging.HTTPLogging
	if !httpConfig.EnableRequestID {
		// 如果禁用了 Request ID，返回空中间件
		return func(c *gin.Context) {
			c.Next()
		}
	}
	
	return func(c *gin.Context) {
		// 检查是否已有请求ID
		headerName := "X-Request-ID"
		if httpConfig.RequestIDHeader != "" {
			headerName = httpConfig.RequestIDHeader
		}
		
		requestID := c.GetHeader(headerName)
		if requestID == "" {
			// 生成新的请求ID
			requestID = uuid.New().String()
		}
		
		// 设置到上下文和响应头
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// RequestID 请求ID中间件（默认行为）
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否已有请求ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// 生成新的请求ID
			requestID = uuid.New().String()
		}
		
		// 设置到上下文和响应头
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Recovery 恢复中间件
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}

// Logging 日志中间件 - 增强版支持请求体和响应体记录
func Logging() gin.HandlerFunc {
	return LoggingWithConfig(LoggingConfig{
		LogRequestBody:  false,  // 默认不记录请求体，避免性能影响
		LogResponseBody: false,  // 默认不记录响应体，避免性能影响
		MaxBodySize:     1024 * 512, // 512KB
		SkipPaths:       []string{"/health", "/ready", "/metrics"},
	})
}

// LoggingFromConfig 从网关配置创建日志中间件
func LoggingFromConfig(cfg *gateway.Config) gin.HandlerFunc {
	if cfg.Logging.HTTPLogging == nil || !cfg.Logging.HTTPLogging.Enabled {
		// 如果未启用 HTTP 日志，返回一个空的中间件
		return func(c *gin.Context) {
			c.Next()
		}
	}
	
	httpConfig := cfg.Logging.HTTPLogging
	
	// 设置默认值
	maxBodySize := httpConfig.MaxBodySize
	if maxBodySize <= 0 {
		maxBodySize = 1024 * 512 // 默认 512KB
	}
	
	slowThreshold := 1 * time.Second
	if httpConfig.SlowRequestThreshold != "" {
		if duration, err := time.ParseDuration(httpConfig.SlowRequestThreshold); err == nil {
			slowThreshold = duration
		}
	}
	
	return LoggingWithConfig(LoggingConfig{
		LogRequestBody:  httpConfig.LogRequestBody,
		LogResponseBody: httpConfig.LogResponseBody,
		MaxBodySize:     maxBodySize,
		LogHeaders:      httpConfig.LogHeaders,
		SkipPaths:       httpConfig.SkipPaths,
		SlowThreshold:   slowThreshold,
		EnableRequestID: httpConfig.EnableRequestID,
		RequestIDHeader: httpConfig.RequestIDHeader,
		SensitiveHeaders: httpConfig.SensitiveHeaders,
		ErrorThreshold:  httpConfig.LogLevelThresholds.ErrorThreshold,
		WarnThreshold:   httpConfig.LogLevelThresholds.WarnThreshold,
		InfoThreshold:   httpConfig.LogLevelThresholds.InfoThreshold,
	})
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	LogRequestBody      bool
	LogResponseBody     bool
	MaxBodySize         int64
	LogHeaders          bool
	SkipPaths           []string
	SlowThreshold       time.Duration
	EnableRequestID     bool
	RequestIDHeader     string
	SensitiveHeaders    []string
	ErrorThreshold      int
	WarnThreshold       int
	InfoThreshold       int
}

// LoggingWithConfig 带配置的日志中间件
func LoggingWithConfig(config LoggingConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		
		// 检查是否跳过日志记录
		for _, skipPath := range config.SkipPaths {
			if path == skipPath {
				c.Next()
				return
			}
		}
		
		// 处理 Request ID
		var requestID string
		if config.EnableRequestID {
			headerName := "X-Request-ID"
			if config.RequestIDHeader != "" {
				headerName = config.RequestIDHeader
			}
			
			requestID = c.GetHeader(headerName)
			if requestID == "" {
				requestID = uuid.New().String()
			}
			
			c.Set("request_id", requestID)
			c.Header("X-Request-ID", requestID)
		}
		
		// 读取请求体（如果需要）
		var requestBody []byte
		if config.LogRequestBody && c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}
		
		// 包装响应写入器以捕获响应体（如果需要）
		var responseWriter *responseBodyWriter
		if config.LogResponseBody {
			responseWriter = &responseBodyWriter{
				ResponseWriter: c.Writer,
				body:           &bytes.Buffer{},
			}
			c.Writer = responseWriter
		}
		
		// 处理请求
		c.Next()
		
		// 计算延迟
		latency := time.Since(start)
		
		// 构建基础字段
		pathParams := make(map[string]string, len(c.Params))
		for _, p := range c.Params {
			pathParams[p.Key] = p.Value
		}
		queryParams := c.Request.URL.Query()

		fields := httplog.Fields{
			Method:    c.Request.Method,
			Path:      path,
			Query:     c.Request.URL.RawQuery,
			PathParams: pathParams,
			QueryParams: queryParams,
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Status:    c.Writer.Status(),
			Latency:   latency,
			RequestID: requestID,
		}
		
		// 添加请求头字段（如果启用）
		if config.LogHeaders {
			for name := range c.Request.Header {
				if isSensitiveHeader(name, config.SensitiveHeaders) {
					// 跳过敏感头或添加掩码
					continue
				}
				// 这里可以添加请求头到日志中
			}
		}
		
		// 添加请求体字段（如果启用且有内容）
		if config.LogRequestBody && len(requestBody) > 0 {
			if int64(len(requestBody)) <= config.MaxBodySize {
				// 尝试格式化为JSON
				var formattedBody interface{}
				if err := json.Unmarshal(requestBody, &formattedBody); err == nil {
					fields.RequestBody = &formattedBody
				} else {
					bodyStr := string(requestBody)
					fields.RequestBodyStr = &bodyStr
				}
			} else {
				largeMsg := "[REQUEST BODY TOO LARGE]"
				fields.RequestBodyStr = &largeMsg
			}
		}
		
		// 添加响应体字段（如果启用且有内容）
		if config.LogResponseBody && responseWriter != nil && responseWriter.body.Len() > 0 {
			responseBody := responseWriter.body.Bytes()
			if int64(len(responseBody)) <= config.MaxBodySize {
				// 尝试格式化为JSON
				var formattedBody interface{}
				if err := json.Unmarshal(responseBody, &formattedBody); err == nil {
					fields.ResponseBody = &formattedBody
				} else {
					bodyStr := string(responseBody)
					fields.ResponseBodyStr = &bodyStr
				}
			} else {
				largeMsg := "[RESPONSE BODY TOO LARGE]"
				fields.ResponseBodyStr = &largeMsg
			}
		}
		
		// 使用增强的日志函数记录
		httplog.LogWithLevelEnhancedWithThresholds(fields, config.ErrorThreshold, config.WarnThreshold, config.InfoThreshold)
		
		// 如果是慢请求，额外记录慢请求日志
		if latency > config.SlowThreshold {
			httplog.LogSlowRequestEnhanced(fields, config.SlowThreshold)
		}
	}
}

// isSensitiveHeader 检查是否为敏感头
func isSensitiveHeader(name string, sensitiveHeaders []string) bool {
	for _, sensitive := range sensitiveHeaders {
		if name == sensitive {
			return true
		}
	}
	return false
}

// responseBodyWriter 包装 gin.ResponseWriter 以捕获响应体
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// CORS 跨域中间件
func CORSFromGatewayConfig(cfg *gateway.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// 检查是否允许的源
		allowed := false
		for _, allowedOrigin := range cfg.CORS.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}
		
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		// 设置其他CORS头
		if len(cfg.CORS.AllowedMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", joinStrings(cfg.CORS.AllowedMethods, ", "))
		}
		if len(cfg.CORS.AllowedHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", joinStrings(cfg.CORS.AllowedHeaders, ", "))
		}
		if len(cfg.CORS.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", joinStrings(cfg.CORS.ExposedHeaders, ", "))
		}
		
		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// RateLimit 限流中间件
func RateLimitFromGatewayConfig(cfg *gateway.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 简单的限流实现（实际应该使用更复杂的算法）
		if cfg.RateLimit.Enabled {
			// 这里可以集成 Redis 或其他限流器
			// 目前只是示例
			c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.RateLimit.Requests))
			c.Header("X-RateLimit-Remaining", "59") // 示例值
		}
		c.Next()
	}
}

// Tracing 追踪中间件
func TracingFromGatewayConfig(cfg *gateway.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Tracing != nil && cfg.Tracing.Enabled {
			httpMiddleware := tracing.NewHTTPMiddleware(tracing.GlobalTracer())
			
			// 创建一个包装的 http.Handler
			wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.Next()
			})
			
			tracedHandler := httpMiddleware.Wrap("http-gateway", wrappedHandler)
			
			// 执行追踪中间件
			tracedHandler.ServeHTTP(c.Writer, c.Request)
		}
		c.Next()
	}
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// joinStrings 连接字符串数组
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
