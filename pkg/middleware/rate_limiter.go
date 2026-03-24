package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alldev-run/golang-gin-rpc/pkg/ratelimit"
)

// RateLimiter creates a rate limiting middleware
func RateLimiter(config RateLimiterConfig) gin.HandlerFunc {
	if config.RequestsPerMinute <= 0 {
		config.RequestsPerMinute = 60 // Default: 60 requests per minute
	}
	if config.BurstSize <= 0 {
		config.BurstSize = config.RequestsPerMinute / 4 // Default: 25% of rate limit
	}
	if config.KeyGenerator == nil {
		config.KeyGenerator = func(ip string) string {
			return ip
		}
	}
	if config.Message == "" {
		config.Message = "Rate limit exceeded"
	}

	limiter := ratelimit.NewTokenBucketLimiter(config.RequestsPerMinute, config.BurstSize)

	return func(c *gin.Context) {
		key := config.KeyGenerator(c.ClientIP())

		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": config.Message,
				"code":  "RATE_LIMIT_EXCEEDED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimiterByUser creates a rate limiter that limits by user ID
// Requires authentication middleware to set user_id in context
func RateLimiterByUser(requestsPerMinute int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		KeyGenerator: func(ip string) string {
			return ip
		},
	}
	return RateLimiter(config)
}

// RateLimiterByIP creates a rate limiter that limits by IP address
func RateLimiterByIP(requestsPerMinute int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		KeyGenerator: func(ip string) string {
			return ip
		},
	}
	return RateLimiter(config)
}

// RateLimiterByEndpoint creates a rate limiter that limits by endpoint + IP
func RateLimiterByEndpoint(requestsPerMinute int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		KeyGenerator: func(ip string) string {
			return ip + ":endpoint"
		},
	}
	return RateLimiter(config)
}
