package gateway

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/pkg/logger"
)

var requestIDCounter uint64

// corsMiddleware provides CORS middleware
func corsMiddleware(config CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range config.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}
		
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
		
		if len(config.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
		}
		
		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		
		if config.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
		}
		
		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// rateLimitMiddleware provides rate limiting middleware
func rateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc {
	// Simple in-memory rate limiter
	// In production, you'd want to use Redis or similar
	clients := make(map[string]*clientBucket)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		
		bucket, exists := clients[clientIP]
		if !exists {
			bucket = &clientBucket{
				requests: config.Requests,
				window:   parseDuration(config.Window),
				lastReset: time.Now(),
			}
			clients[clientIP] = bucket
		}
		
		if bucket.allow() {
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
			requestID = generateRequestID()
		}
		
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// loggingMiddleware provides request logging
func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Info("HTTP Request",
			logger.String("method", param.Method),
			logger.String("path", param.Path),
			logger.String("client_ip", param.ClientIP),
			logger.Int("status", param.StatusCode),
			logger.Duration("latency", param.Latency),
			logger.String("request_id", param.Keys["request_id"].(string)),
		)
		return ""
	})
}

// clientBucket implements simple rate limiting
type clientBucket struct {
	requests int
	window   time.Duration
	lastReset time.Time
	count    int
}

func (b *clientBucket) allow() bool {
	now := time.Now()
	if now.Sub(b.lastReset) > b.window {
		b.lastReset = now
		b.count = 0
	}
	
	if b.count >= b.requests {
		return false
	}
	
	b.count++
	return true
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
	ts := time.Now().UnixNano()
	seq := atomic.AddUint64(&requestIDCounter, 1)
	return strconv.FormatInt(ts, 36) + strconv.FormatUint(seq, 36)
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
