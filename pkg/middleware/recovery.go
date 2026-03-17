package middleware

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RecoveryConfig holds configuration for recovery middleware
type RecoveryConfig struct {
	// StackSize is the stack size to be printed
	StackSize int
	// Logger is the logger to use for logging panics
	Logger func(c *gin.Context, err interface{})
	// LogAllRequests logs all requests, not just panics
	LogAllRequests bool
	// RequestBodyLimit limits the size of request body to log
	RequestBodyLimit int64
}

// DefaultRecoveryConfig returns a default recovery configuration
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		StackSize:        4 * 1024, // 4KB
		Logger:           defaultLogger,
		LogAllRequests:   false,
		RequestBodyLimit: 1024, // 1KB
	}
}

// Recovery creates a recovery middleware that recovers from any panics
func Recovery(config ...RecoveryConfig) gin.HandlerFunc {
	var cfg RecoveryConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRecoveryConfig()
	}

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				cfg.Logger(c, err)

				// Return error response
				response.Error(c, "Internal server error", map[string]interface{}{
					"error": fmt.Sprintf("%v", err),
					"time":  time.Now().Unix(),
				})
				c.Abort()
			}
		}()

		if cfg.LogAllRequests {
			logRequest(c, cfg.RequestBodyLimit)
		}

		c.Next()
	}
}

// defaultLogger is the default logger function
func defaultLogger(c *gin.Context, err interface{}) {
	// Get request information
	method := c.Request.Method
	path := c.Request.URL.Path
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()
	
	// Get user information if available
	var userID string
	if uid, exists := c.Get("user_id"); exists {
		userID = uid.(string)
	}

	// Get request ID if available
	var requestID string
	if rid, exists := c.Get("request_id"); exists {
		requestID = rid.(string)
	}

	// Log the panic with context
	logger.Errorf("Panic recovered",
		zap.String("error", fmt.Sprintf("%v", err)),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("ip", ip),
		zap.String("user_agent", userAgent),
		zap.String("user_id", userID),
		zap.String("request_id", requestID),
		zap.String("stack", string(debug.Stack())),
	)
}

// logRequest logs the request details
func logRequest(c *gin.Context, bodyLimit int64) {
	start := time.Now()

	// Read request body if it exists
	var body []byte
	if c.Request.Body != nil {
		body, _ = io.ReadAll(io.LimitReader(c.Request.Body, bodyLimit))
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	c.Next()

	// Log response details
	duration := time.Since(start)
	statusCode := c.Writer.Status()

	logger.Info("Request completed",
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
		zap.String("ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
		zap.ByteString("request_body", body),
		zap.Int("response_size", c.Writer.Size()),
	)
}

// RecoveryWithLogger creates a recovery middleware with a custom logger
func RecoveryWithLogger(loggerFunc func(c *gin.Context, err interface{})) gin.HandlerFunc {
	config := DefaultRecoveryConfig()
	config.Logger = loggerFunc
	return Recovery(config)
}

// RecoveryWithRequestLogging creates a recovery middleware that logs all requests
func RecoveryWithRequestLogging(bodyLimit int64) gin.HandlerFunc {
	config := DefaultRecoveryConfig()
	config.LogAllRequests = true
	config.RequestBodyLimit = bodyLimit
	return Recovery(config)
}

// HandlePanic is a helper function to handle panics in goroutines
func HandlePanic(operation string) {
	if err := recover(); err != nil {
		logger.Errorf("Panic in goroutine",
			zap.String("operation", operation),
			zap.String("error", fmt.Sprintf("%v", err)),
			zap.String("stack", string(debug.Stack())),
		)
	}
}

// SafeGo runs a function in a goroutine with panic recovery
func SafeGo(fn func()) {
	go func() {
		defer HandlePanic("goroutine")
		fn()
	}()
}

// ErrorHandler is a custom error handler that can be used with gin.Error()
func ErrorHandler(c *gin.Context, err error) {
	logger.Errorf("Request error",
		zap.String("error", err.Error()),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
	)

	response.Error(c, err.Error(), nil)
}

// ErrorHandlerWithStatus creates an error handler with custom status code
func ErrorHandlerWithStatus(statusCode int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(statusCode, gin.H{
			"error": "Internal server error",
			"code":  "INTERNAL_ERROR",
		})
	}
}
