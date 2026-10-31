package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGatewayAuth_Execute_Disabled(t *testing.T) {
	config := AuthConfig{
		Enabled: false,
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test route
	engine.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayAuth_Execute_Enabled_NoKey(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test route (JSON-RPC route)
	engine.POST("/rpc/test", func(c *gin.Context) {
		// 设置协议为 jsonrpc，这样会触发认证
		c.Set("protocol", "jsonrpc")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test request without API key
	req := httptest.NewRequest("POST", "/rpc/test", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGatewayAuth_Execute_Enabled_ValidKey(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test route (JSON-RPC route)
	engine.POST("/rpc/test", func(c *gin.Context) {
		// 设置协议为 jsonrpc，这样会触发认证
		c.Set("protocol", "jsonrpc")
		
		apiKey, exists := GetAPIKeyFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, "test-key", apiKey)
		
		apiUser, exists := GetAPIUserFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, "test-user", apiUser)
		
		authenticated, exists := c.Get("authenticated")
		assert.True(t, exists)
		assert.True(t, authenticated.(bool))
		
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test request with valid API key in header
	req := httptest.NewRequest("POST", "/rpc/test", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayAuth_Execute_Enabled_ValidKey_QueryParam(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test route (RPC route)
	engine.GET("/test", func(c *gin.Context) {
		// 设置协议为 grpc，这样会触发认证
		c.Set("protocol", "grpc")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test request with valid API key in query parameter
	req := httptest.NewRequest("GET", "/test?api_key=test-key", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayAuth_Execute_Enabled_InvalidKey(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test route (JSON-RPC route)
	engine.POST("/rpc/test", func(c *gin.Context) {
		// 设置协议为 jsonrpc，这样会触发认证
		c.Set("protocol", "jsonrpc")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test request with invalid API key
	req := httptest.NewRequest("POST", "/rpc/test", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGatewayAuth_SkipPath(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		SkipPaths:  []string{"/health", "/debug/*"},
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test routes
	engine.GET("/health", func(c *gin.Context) {
		// 设置协议为 jsonrpc，但应该跳过认证
		c.Set("protocol", "jsonrpc")
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	
	engine.GET("/debug/test", func(c *gin.Context) {
		// 设置协议为 grpc，但应该跳过认证
		c.Set("protocol", "grpc")
		c.JSON(http.StatusOK, gin.H{"debug": "info"})
	})
	
	engine.POST("/rpc/protected", func(c *gin.Context) {
		// 设置协议为 jsonrpc，需要认证
		c.Set("protocol", "jsonrpc")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Test skipped path
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Test skipped wildcard path
	req = httptest.NewRequest("GET", "/debug/test", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Test protected path
	req = httptest.NewRequest("POST", "/rpc/protected", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGatewayAuth_SkipMethod(t *testing.T) {
	config := AuthConfig{
		Enabled:     true,
		HeaderName:  "X-API-Key",
		QueryName:   "api_key",
		SkipMethods: []string{"OPTIONS"},
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add test route
	engine.OPTIONS("/test", func(c *gin.Context) {
		// 设置协议为 jsonrpc，但应该跳过认证
		c.Set("protocol", "jsonrpc")
		c.JSON(http.StatusOK, gin.H{"message": "options"})
	})
	
	// Test OPTIONS request (should skip auth)
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayAuth_APIKeyManagement(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"existing-key": "existing-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	
	// Test HasAPIKey
	assert.True(t, auth.HasAPIKey("existing-key"))
	assert.False(t, auth.HasAPIKey("non-existing-key"))
	
	// Test AddAPIKey
	auth.AddAPIKey("new-key", "new-user")
	assert.True(t, auth.HasAPIKey("new-key"))
	assert.Equal(t, "new-user", auth.config.APIKeys["new-key"])
	
	// Test RemoveAPIKey
	auth.RemoveAPIKey("existing-key")
	assert.False(t, auth.HasAPIKey("existing-key"))
	assert.True(t, auth.HasAPIKey("new-key")) // Check that new-key still exists
}

func TestGatewayAuth_Execute_NonRPCRoute(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add HTTP route (non-RPC)
	engine.GET("/api/users", func(c *gin.Context) {
		// 设置协议为 http
		c.Set("protocol", "http")
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Add RPC route
	engine.POST("/rpc/payment", func(c *gin.Context) {
		// 设置协议为 jsonrpc
		c.Set("protocol", "jsonrpc")
		c.JSON(http.StatusOK, gin.H{"result": "success"})
	})
	
	// Test HTTP route without API key (should pass)
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Test RPC route without API key (should fail)
	req = httptest.NewRequest("POST", "/rpc/payment", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	// Test RPC route with API key (should pass)
	req = httptest.NewRequest("POST", "/rpc/payment", nil)
	req.Header.Set("X-API-Key", "test-key")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayAuth_Execute_RPCRoute_PatternMatching(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	middleware := auth.Execute()
	
	// Create test context
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware)
	
	// Add routes without explicit protocol setting (rely on pattern matching)
	engine.GET("/grpc/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "grpc success"})
	})
	
	engine.POST("/rpc/payment", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"result": "rpc success"})
	})
	
	engine.GET("/v1/products", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"products": []string{}})
	})
	
	engine.GET("/api/orders", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"orders": []string{}})
	})
	
	// Test gRPC pattern without API key (should fail)
	req := httptest.NewRequest("GET", "/grpc/users", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	// Test JSON-RPC pattern without API key (should fail)
	req = httptest.NewRequest("POST", "/rpc/payment", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	// Test v1 pattern without API key (should fail)
	req = httptest.NewRequest("GET", "/v1/products", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	// Test regular API pattern without API key (should pass)
	req = httptest.NewRequest("GET", "/api/orders", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGatewayAuth_ShouldSkipAuth(t *testing.T) {
	config := AuthConfig{
		Enabled:   true,
		SkipPaths: []string{"/health", "/debug/*"},
	}
	
	auth := NewGatewayAuth(config)
	
	// Test path matching
	assert.True(t, auth.ShouldSkipAuth("/health"))
	assert.True(t, auth.ShouldSkipAuth("/debug/test"))
	assert.True(t, auth.ShouldSkipAuth("/debug/sub/path"))
	assert.False(t, auth.ShouldSkipAuth("/api/users"))
	assert.False(t, auth.ShouldSkipAuth("/protected"))
}

func TestGatewayAuth_IsRPCRoute(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	auth := NewGatewayAuth(config)
	
	// 测试 RPC 路由检测
	testCases := []struct {
		path     string
		method   string
		protocol string
		expected bool
	}{
		// 明确的协议设置
		{"/rpc/payment", "POST", "jsonrpc", true},
		{"/grpc/users", "GET", "grpc", true},
		{"/api/users", "GET", "http", false},
		{"/health", "GET", "http", false},
		
		// 路径模式匹配
		{"/grpc/users", "GET", "", true},
		{"/api/grpc/service", "POST", "", true},
		{"/v1/products", "GET", "", true},
		{"/v2/orders", "GET", "", true},
		{"/rpc/payment", "POST", "", true},
		{"/api/rpc/service", "POST", "", true},
		{"/jsonrpc/endpoint", "POST", "", true},
		{"/api/jsonrpc/call", "POST", "", true},
		
		// 非 RPC 路由
		{"/api/users", "GET", "", false},
		{"/api/orders", "POST", "", false},
		{"/health", "GET", "", false},
		{"/ready", "GET", "", false},
		{"/info", "GET", "", false},
		
		// JSON-RPC 必须是 POST
		{"/rpc/payment", "GET", "", false},
		{"/api/rpc/service", "GET", "", false},
	}
	
	for _, tc := range testCases {
		result := auth.IsRPCRoutePublic(tc.path, tc.method, tc.protocol)
		assert.Equal(t, tc.expected, result, 
			"Failed for path: %s, method: %s, protocol: %s", 
			tc.path, tc.method, tc.protocol)
	}
}
