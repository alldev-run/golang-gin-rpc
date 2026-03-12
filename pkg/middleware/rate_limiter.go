package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig holds configuration for rate limiting middleware
type RateLimiterConfig struct {
	// RequestsPerMinute is the maximum number of requests allowed per minute
	RequestsPerMinute int
	// BurstSize is the maximum number of requests allowed in a short burst
	BurstSize int
	// KeyGenerator function to generate rate limit keys (default: by IP)
	KeyGenerator func(c *gin.Context) string
	// SkipSuccessful skips counting successful requests (2xx status codes)
	SkipSuccessful bool
	// Message to return when rate limited
	Message string
}

// tokenBucket represents a token bucket for rate limiting
type tokenBucket struct {
	tokens       int
	maxTokens    int
	refillRate   int
	lastRefill   time.Time
	mutex        sync.Mutex
}

// rateLimiter manages multiple token buckets
type rateLimiter struct {
	buckets map[string]*tokenBucket
	config  RateLimiterConfig
	mutex   sync.RWMutex
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(config RateLimiterConfig) *rateLimiter {
	if config.RequestsPerMinute <= 0 {
		config.RequestsPerMinute = 60 // Default: 60 requests per minute
	}
	if config.BurstSize <= 0 {
		config.BurstSize = config.RequestsPerMinute / 4 // Default: 25% of rate limit
	}
	if config.KeyGenerator == nil {
		config.KeyGenerator = func(c *gin.Context) string {
			return c.ClientIP()
		}
	}
	if config.Message == "" {
		config.Message = "Rate limit exceeded"
	}

	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		config:  config,
	}
}

// getBucket gets or creates a token bucket for the given key
func (rl *rateLimiter) getBucket(key string) *tokenBucket {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	if bucket, exists := rl.buckets[key]; exists {
		return bucket
	}

	bucket := &tokenBucket{
		tokens:     rl.config.BurstSize,
		maxTokens:  rl.config.BurstSize,
		refillRate: rl.config.RequestsPerMinute / 60, // tokens per second
		lastRefill: time.Now(),
	}

	rl.buckets[key] = bucket
	return bucket
}

// takeToken attempts to take a token from the bucket
func (tb *tokenBucket) takeToken() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := time.Now()
	// Refill tokens based on time elapsed
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * tb.refillRate

	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.maxTokens {
			tb.tokens = tb.maxTokens
		}
		tb.lastRefill = now
	}

	// Try to take a token
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// RateLimiter creates a rate limiting middleware
func RateLimiter(config RateLimiterConfig) gin.HandlerFunc {
	limiter := newRateLimiter(config)

	return func(c *gin.Context) {
		key := config.KeyGenerator(c)
		bucket := limiter.getBucket(key)

		if !bucket.takeToken() {
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
		KeyGenerator: func(c *gin.Context) string {
			if userID, exists := c.Get("user_id"); exists {
				return userID.(string)
			}
			return c.ClientIP() // Fallback to IP if user not authenticated
		},
	}
	return RateLimiter(config)
}

// RateLimiterByIP creates a rate limiter that limits by IP address
func RateLimiterByIP(requestsPerMinute int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
	return RateLimiter(config)
}

// RateLimiterByEndpoint creates a rate limiter that limits by endpoint + IP
func RateLimiterByEndpoint(requestsPerMinute int) gin.HandlerFunc {
	config := RateLimiterConfig{
		RequestsPerMinute: requestsPerMinute,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP() + ":" + c.Request.URL.Path
		},
	}
	return RateLimiter(config)
}
