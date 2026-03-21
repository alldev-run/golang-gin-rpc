package gateway

import (
	"fmt"
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

// GatewayAuth provides gateway authentication middleware
type GatewayAuth struct {
	config AuthConfig
}

// NewGatewayAuth creates a new gateway authentication middleware
func NewGatewayAuth(config AuthConfig) *GatewayAuth {
	return &GatewayAuth{config: config}
}

// Name returns the middleware name
func (a *GatewayAuth) Name() string {
	return "gateway_auth"
}

// Execute executes the gateway authentication middleware
func (a *GatewayAuth) Execute() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !a.config.Enabled {
			c.Next()
			return
		}

		// 检查是否为 RPC 协议的路由
		if !a.isRPCRoute(c) {
			// 非 RPC 路由，跳过认证
			c.Next()
			return
		}

		// 检查是否路径应该跳过认证
		if a.shouldSkipPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 检查是否方法应该跳过认证
		if a.shouldSkipMethod(c.Request.Method) {
			c.Next()
			return
		}

		// 提取 API key from request
		apiKey, err := a.extractAPIKey(c)
		if err != nil {
			logger.Warn("API key extraction failed",
				logger.String("path", c.Request.URL.Path),
				logger.String("method", c.Request.Method),
				logger.String("client_ip", c.ClientIP()),
				logger.Error(err))
			
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "API key required",
				"code":  "AUTH_REQUIRED",
			})
			c.Abort()
			return
		}

		// 验证 API key
		if !a.validateAPIKey(apiKey) {
			logger.Warn("Invalid API key provided",
				logger.String("path", c.Request.URL.Path),
				logger.String("method", c.Request.Method),
				logger.String("client_ip", c.ClientIP()),
				logger.String("api_key", apiKey[:min(len(apiKey), 8)]+"..."))
			
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
				"code":  "AUTH_INVALID",
			})
			c.Abort()
			return
		}

		// 设置认证信息到上下文
		c.Set("api_key", apiKey)
		c.Set("api_user", a.config.APIKeys[apiKey])
		c.Set("authenticated", true)

		logger.Debug("RPC API key authentication successful",
			logger.String("path", c.Request.URL.Path),
			logger.String("method", c.Request.Method),
			logger.String("api_user", a.config.APIKeys[apiKey]))

		c.Next()
	}
}

// isRPCRoute checks if the current route is an RPC route
func (a *GatewayAuth) isRPCRoute(c *gin.Context) bool {
	// 从上下文获取路由信息
	routePath := c.Request.URL.Path
	routeMethod := c.Request.Method
	
	// 检查是否有路由协议信息
	if protocol, exists := c.Get("protocol"); exists {
		protocolStr, ok := protocol.(string)
		if ok {
			return protocolStr == "grpc" || protocolStr == "jsonrpc"
		}
	}
	
	// 通过路径模式判断 RPC 路由
	// gRPC 路由通常以 /grpc/ 或包含 rpc 关键词
	// JSON-RPC 路由通常以 /rpc/ 或包含 rpc 关键词
	if a.isGRPCRoute(routePath, routeMethod) || a.isJSONRPCRoute(routePath, routeMethod) {
		return true
	}
	
	return false
}

// isGRPCRoute checks if the route is a gRPC route
func (a *GatewayAuth) isGRPCRoute(path, method string) bool {
	// gRPC 路由特征
	grpcPatterns := []string{
		"/grpc/",
		"/api/grpc/",
		"/v1/",  // 通常 gRPC API 使用 /v1/ 前缀
		"/v2/",  // 通常 gRPC API 使用 /v2/ 前缀
	}
	
	for _, pattern := range grpcPatterns {
		if strings.HasPrefix(path, pattern) {
			return true
		}
	}
	
	// 检查路径中是否包含 gRPC 相关关键词，但要避免过度匹配
	// 只检查明确的关键词
	if strings.Contains(path, "grpc") {
		return true
	}
	
	return false
}

// isJSONRPCRoute checks if the route is a JSON-RPC route
func (a *GatewayAuth) isJSONRPCRoute(path, method string) bool {
	// JSON-RPC 路由特征
	jsonrpcPatterns := []string{
		"/rpc/",
		"/api/rpc/",
		"/jsonrpc/",
		"/api/jsonrpc/",
	}
	
	hasPattern := false
	for _, pattern := range jsonrpcPatterns {
		if strings.HasPrefix(path, pattern) {
			hasPattern = true
			break
		}
	}
	
	// 如果路径不匹配 JSON-RPC 模式，返回 false
	if !hasPattern {
		return false
	}
	
	// JSON-RPC 通常使用 POST 方法
	if method != "POST" {
		return false
	}
	
	return true
}

// shouldSkipPath checks if the path should skip authentication
func (a *GatewayAuth) shouldSkipPath(path string) bool {
	for _, skipPath := range a.config.SkipPaths {
		if a.matchPath(path, skipPath) {
			return true
		}
	}
	return false
}

// shouldSkipMethod checks if the method should skip authentication
func (a *GatewayAuth) shouldSkipMethod(method string) bool {
	for _, skipMethod := range a.config.SkipMethods {
		if strings.EqualFold(method, skipMethod) {
			return true
		}
	}
	return false
}

// matchPath checks if the path matches the skip pattern
func (a *GatewayAuth) matchPath(path, pattern string) bool {
	if pattern == path {
		return true
	}
	
	// Support wildcard patterns
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix)
	}
	
	return false
}

// extractAPIKey extracts API key from request
func (a *GatewayAuth) extractAPIKey(c *gin.Context) (string, error) {
	// Try to get from header first
	if apiKey := c.GetHeader(a.config.HeaderName); apiKey != "" {
		return apiKey, nil
	}

	// Try to get from query parameter
	if apiKey := c.Query(a.config.QueryName); apiKey != "" {
		return apiKey, nil
	}

	return "", fmt.Errorf("API key not found in header '%s' or query parameter '%s'", a.config.HeaderName, a.config.QueryName)
}

// validateAPIKey validates the API key
func (a *GatewayAuth) validateAPIKey(key string) bool {
	for validKey := range a.config.APIKeys {
		if key == validKey {
			return true
		}
	}
	return false
}

// AddAPIKey adds an API key to the configuration
func (a *GatewayAuth) AddAPIKey(key, description string) {
	if a.config.APIKeys == nil {
		a.config.APIKeys = make(map[string]string)
	}
	a.config.APIKeys[key] = description
}

// RemoveAPIKey removes an API key from the configuration
func (a *GatewayAuth) RemoveAPIKey(key string) {
	if a.config.APIKeys != nil {
		delete(a.config.APIKeys, key)
	}
}

// HasAPIKey checks if an API key exists in the configuration
func (a *GatewayAuth) HasAPIKey(key string) bool {
	if a.config.APIKeys == nil {
		return false
	}
	_, exists := a.config.APIKeys[key]
	return exists
}

// ShouldSkipAuth checks if a path should skip authentication (public path)
func (a *GatewayAuth) ShouldSkipAuth(path string) bool {
	return a.shouldSkipPath(path)
}

// IsAuthenticated checks if the request is authenticated
func (a *GatewayAuth) IsAuthenticated(c *gin.Context) bool {
	authenticated, exists := c.Get("authenticated")
	if !exists {
		return false
	}
	return authenticated.(bool)
}

// GetAPIKeyFromContext gets the API key from gin context
func GetAPIKeyFromContext(c *gin.Context) (string, bool) {
	apiKey, exists := c.Get("api_key")
	if !exists {
		return "", false
	}
	return apiKey.(string), true
}

// IsRPCRoutePublic is a public method for testing RPC route detection
func (a *GatewayAuth) IsRPCRoutePublic(path, method, protocol string) bool {
	// 检查协议
	if protocol == "grpc" || protocol == "jsonrpc" {
		return true
	}
	
	// 检查路径模式
	if a.isGRPCRoute(path, method) || a.isJSONRPCRoute(path, method) {
		return true
	}
	
	return false
}

// mockContext 模拟 gin.Context for testing
type mockContext struct {
	path     string
	method   string
	protocol string
}

func (m *mockContext) Request() interface{} {
	return &mockRequest{
		path:   m.path,
		method: m.method,
	}
}

func (m *mockContext) Get(key string) (interface{}, bool) {
	if key == "protocol" {
		return m.protocol, true
	}
	return nil, false
}

// mockRequest 模拟 HTTP 请求
type mockRequest struct {
	path   string
	method string
}

func (m *mockRequest) URL() interface{} {
	return &mockURL{path: m.path}
}

func (m *mockRequest) Method() string {
	return m.method
}

// mockURL 模拟 URL
type mockURL struct {
	path string
}

func (m *mockURL) Path() string {
	return m.path
}

// GetAPIUserFromContext gets the API user from gin context
func GetAPIUserFromContext(c *gin.Context) (string, bool) {
	apiUser, exists := c.Get("api_user")
	if !exists {
		return "", false
	}
	return apiUser.(string), true
}

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
