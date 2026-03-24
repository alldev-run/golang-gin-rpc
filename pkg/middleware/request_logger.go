package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	
	"alldev-gin-rpc/pkg/logger"
)

// RequestLoggerConfig holds configuration for request logging middleware
type RequestLoggerConfig struct {
	// SkipPaths is a list of paths to skip logging
	SkipPaths []string
	
	// MaxBodySize limits the size of request/response body to log
	MaxBodySize int64
	
	// LogRequestBody enables logging request body
	LogRequestBody bool
	
	// LogResponseBody enables logging response body
	LogResponseBody bool
	
	// LogHeaders enables logging request headers
	LogHeaders bool
	
	// SensitiveHeaders is a list of header names to mask or skip
	SensitiveHeaders []string
}

// DefaultRequestLoggerConfig returns default configuration
func DefaultRequestLoggerConfig() RequestLoggerConfig {
	return RequestLoggerConfig{
		SkipPaths: []string{
			"/health",
			"/ready", 
			"/metrics",
		},
		MaxBodySize:      1024 * 1024, // 1MB
		LogRequestBody:   true,
		LogResponseBody:  false, // Response body logging can be expensive
		LogHeaders:       true,
		SensitiveHeaders: []string{
			"Authorization",
			"Cookie",
			"X-API-Key",
			"X-Auth-Token",
		},
	}
}

// RequestLogger returns a middleware that logs HTTP requests with request_id
func RequestLogger(config ...RequestLoggerConfig) gin.HandlerFunc {
	cfg := DefaultRequestLoggerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	// Use framework logger
	frameworkLogger := logger.L()

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Skip logging for specified paths
		for _, skipPath := range cfg.SkipPaths {
			if path == skipPath {
				c.Next()
				return
			}
		}

		// Generate or get existing request_id
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// Set request_id in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		// Read and potentially replace request body
		var requestBody []byte
		if cfg.LogRequestBody && c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Create response writer wrapper to capture response
		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Prepare log fields
		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", raw),
			zap.String("remote_addr", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("status_code", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.Int("response_size", c.Writer.Size()),
		}

		// Add request headers if enabled
		if cfg.LogHeaders {
			for name, values := range c.Request.Header {
				if !isSensitiveHeader(name, cfg.SensitiveHeaders) {
					fields = append(fields, zap.Strings("req_header_"+name, values))
				} else {
					fields = append(fields, zap.Strings("req_header_"+name, []string{"***"}))
				}
			}
		}

		// Add request body if enabled and not too large
		if cfg.LogRequestBody && len(requestBody) > 0 {
			if int64(len(requestBody)) <= cfg.MaxBodySize {
				// Try to format as JSON if possible
				var formattedBody interface{}
				if err := json.Unmarshal(requestBody, &formattedBody); err == nil {
					fields = append(fields, zap.Any("request_body", formattedBody))
				} else {
					fields = append(fields, zap.String("request_body", string(requestBody)))
				}
			} else {
				fields = append(fields, zap.String("request_body", "[TOO LARGE]"))
			}
		}

		// Add response body if enabled and not too large
		if cfg.LogResponseBody && responseWriter.body.Len() > 0 {
			responseBody := responseWriter.body.Bytes()
			if int64(len(responseBody)) <= cfg.MaxBodySize {
				// Try to format as JSON if possible
				var formattedBody interface{}
				if err := json.Unmarshal(responseBody, &formattedBody); err == nil {
					fields = append(fields, zap.Any("response_body", formattedBody))
				} else {
					fields = append(fields, zap.String("response_body", string(responseBody)))
				}
			} else {
				fields = append(fields, zap.String("response_body", "[TOO LARGE]"))
			}
		}

		// Add error information if any
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
		}

		// Log based on status code
		switch {
		case c.Writer.Status() >= 500:
			frameworkLogger.Error("HTTP Request - Server Error", fields...)
		case c.Writer.Status() >= 400:
			frameworkLogger.Warn("HTTP Request - Client Error", fields...)
		case c.Writer.Status() >= 300:
			frameworkLogger.Info("HTTP Request - Redirect", fields...)
		default:
			frameworkLogger.Info("HTTP Request - Success", fields...)
		}
	}
}

// responseBodyWriter wraps gin.ResponseWriter to capture response body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// isSensitiveHeader checks if a header should be masked
func isSensitiveHeader(name string, sensitiveHeaders []string) bool {
	for _, sensitive := range sensitiveHeaders {
		if name == sensitive {
			return true
		}
	}
	return false
}

// GetRequestID returns the request ID from the Gin context
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// LogRequestWithID logs a message with the current request ID
func LogRequestWithID(c *gin.Context, level string, message string, fields ...zap.Field) {
	requestID := GetRequestID(c)
	if requestID != "" {
		fields = append([]zap.Field{zap.String("request_id", requestID)}, fields...)
	}
	
	frameworkLogger := logger.L()
	switch level {
	case "debug":
		frameworkLogger.Debug(message, fields...)
	case "info":
		frameworkLogger.Info(message, fields...)
	case "warn":
		frameworkLogger.Warn(message, fields...)
	case "error":
		frameworkLogger.Error(message, fields...)
	default:
		frameworkLogger.Info(message, fields...)
	}
}
