package middleware

import (
	"net/http"
	"strconv"
	"time"

	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/httplog"
	"alldev-gin-rpc/pkg/tracing"

	"github.com/gin-gonic/gin"
)

// RequestID 请求ID中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否已有请求ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// 生成新的请求ID
			requestID = generateRequestID()
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

// Logging 日志中间件
func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		start := time.Now()
		
		// 处理请求
		c.Next()
		
		// 计算延迟
		latency := time.Since(start)
		
		// 获取请求ID
		requestID, _ := c.Get("request_id")
		requestIDStr, _ := requestID.(string)
		if requestIDStr == "" {
			requestIDStr = ""
		}
		
		// 使用增强的日志函数，根据状态码记录不同级别的日志
		httplog.LogWithLevel(httplog.Fields{
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Status:    c.Writer.Status(),
			Latency:   latency,
			RequestID: requestIDStr,
		})
		
		// 如果是慢请求，额外记录慢请求日志
		if latency > 1*time.Second {
			httplog.LogSlowRequest(httplog.Fields{
				Method:    c.Request.Method,
				Path:      c.Request.URL.Path,
				ClientIP:  c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
				Status:    c.Writer.Status(),
				Latency:   latency,
				RequestID: requestIDStr,
			}, 1*time.Second)
		}
	}
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
