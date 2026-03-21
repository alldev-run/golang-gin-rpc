package gateway

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/pkg/cors"
	"alldev-gin-rpc/pkg/httplog"
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/ratelimit"
	"alldev-gin-rpc/pkg/requestid"
)

// corsMiddleware provides CORS middleware
func corsMiddleware(config CORSConfig) gin.HandlerFunc {
	cc := cors.Config{
		AllowedOrigins:   config.AllowedOrigins,
		AllowedMethods:   config.AllowedMethods,
		AllowedHeaders:   config.AllowedHeaders,
		ExposedHeaders:   config.ExposedHeaders,
		AllowCredentials: config.AllowCredentials,
		MaxAge:           config.MaxAge,
		OptionsPassthrough: false,
	}

	return func(c *gin.Context) {
		if handled := cors.Apply(c.Writer, c.Request, cc); handled {
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// rateLimitMiddleware provides rate limiting middleware
func rateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	limiter := ratelimit.NewMemoryFixedWindow(config.Requests, parseDuration(config.Window))
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		
		if limiter.Allow(clientIP) {
			c.Next()
		} else {
			logger.Warn("Rate limit exceeded",
				logger.String("client", clientIP),
				logger.Int("limit", config.Requests))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
		}
	}
}

// requestIDMiddleware adds request ID to each request
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = requestid.MustNew()
		}
		
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// loggingMiddleware provides request logging
func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		requestID, _ := param.Keys["request_id"].(string)
		httplog.Log(httplog.Fields{
			Method:    param.Method,
			Path:      param.Path,
			ClientIP:  param.ClientIP,
			UserAgent: param.Request.UserAgent(),
			Status:    param.StatusCode,
			Latency:   param.Latency,
			RequestID: requestID,
		})
		return ""
	})
}

// parseDuration parses duration string
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Minute // default to 1 minute
	}
	return d
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return requestid.MustNew()
}

// IsTimeout checks if error is a timeout error
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timeout") || 
		   strings.Contains(err.Error(), "deadline exceeded")
}

// IsTooManyRetries checks if error is due to too many retries
func IsTooManyRetries(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "too many retries")
}
