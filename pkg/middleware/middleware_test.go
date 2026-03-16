package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthConfig(t *testing.T) {
	config := AuthConfig{
		SkipPaths:   []string{"/health", "/metrics"},
		TokenLookup: "header:Authorization:Bearer ",
	}

	if len(config.SkipPaths) != 2 {
		t.Errorf("Expected 2 skip paths, got %d", len(config.SkipPaths))
	}
	if config.TokenLookup != "header:Authorization:Bearer " {
		t.Errorf("Expected TokenLookup to be 'header:Authorization:Bearer ', got %s", config.TokenLookup)
	}
	if config.Skipper != nil {
		t.Error("Expected Skipper to be nil")
	}
	if config.KeyFunc != nil {
		t.Error("Expected KeyFunc to be nil")
	}
}

func TestJWT_Middleware_DefaultConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := AuthConfig{}
	middleware := JWT(config)
	
	if middleware == nil {
		t.Fatal("JWT middleware should not be nil")
	}
}

func TestJWT_Middleware_WithSkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := AuthConfig{
		SkipPaths: []string{"/health", "/metrics"},
	}
	
	router := gin.New()
	router.Use(JWT(config))
	
	// Add a test route
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	// Test skipped path
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 404 { // Route not found, but middleware should not block
		t.Errorf("Expected status 404 for skipped path, got %d", w.Code)
	}
	
	// Test non-skipped path without token
	req, _ = http.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 401 { // Should be blocked by auth middleware
		t.Errorf("Expected status 401 for protected path, got %d", w.Code)
	}
}

func TestJWT_Middleware_WithSkipper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := AuthConfig{
		Skipper: func(c *gin.Context) bool {
			return c.Request.Header.Get("X-Skip-Auth") == "true"
		},
	}
	
	router := gin.New()
	router.Use(JWT(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	// Test with skipper header
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Skip-Auth", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 { // Should be skipped
		t.Errorf("Expected status 200 for skipped request, got %d", w.Code)
	}
	
	// Test without skipper header
	req, _ = http.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 401 { // Should be blocked
		t.Errorf("Expected status 401 for blocked request, got %d", w.Code)
	}
}

func TestCORS_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := DefaultCORSConfig()
	router := gin.New()
	router.Use(CORS(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	// Test preflight request
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 204 {
		t.Errorf("Expected status 204 for preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected Access-Control-Allow-Origin header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
	
	// Test actual request
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200 for actual request, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("Expected Access-Control-Allow-Origin header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRecovery_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})
	
	req, _ := http.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 500 {
		t.Errorf("Expected status 500 for panic, got %d", w.Code)
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := RateLimiterConfig{
		RequestsPerMinute: 60, // 1 request per second
		BurstSize:         5,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
	
	router := gin.New()
	router.Use(RateLimiter(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	// Test within rate limit
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("Request %d should succeed, got status %d", i+1, w.Code)
		}
	}
	
	// Test exceeding rate limit
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Errorf("Request should be rate limited, got status %d", w.Code)
	}
}

func TestRequestID_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		c.JSON(200, gin.H{"request_id": requestID})
	})
	
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header")
	}
	
	// Test response body contains request ID
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if response["request_id"] == "" {
		t.Error("Expected request_id in response body")
	}
}

func TestMiddleware_Chain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	
	// Chain multiple middlewares
	router.Use(RequestID())
	router.Use(Recovery())
	router.Use(CORS(DefaultCORSConfig()))
	router.Use(RateLimiter(RateLimiterConfig{
		RequestsPerMinute: 1000,
		BurstSize:         100,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}))
	
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		c.JSON(200, gin.H{
			"message":    "success",
			"request_id": requestID,
			"client_ip":  c.ClientIP(),
		})
	})
	
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("Expected X-Request-ID header")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("Expected Access-Control-Allow-Origin header")
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}
	if response["message"] != "success" {
		t.Error("Expected success message")
	}
	if response["request_id"] == "" {
		t.Error("Expected request_id in response")
	}
}

func TestMiddleware_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Test JWT middleware with invalid token
	config := AuthConfig{
		TokenLookup: "header:Authorization:Bearer ",
	}
	
	router := gin.New()
	router.Use(JWT(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	// Test with invalid token
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != 401 {
		t.Errorf("Expected status 401 for invalid token, got %d", w.Code)
	}
}

func TestMiddleware_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := RateLimiterConfig{
		RequestsPerMinute: 6000, // 100 requests per second
		BurstSize:         200,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
	
	router := gin.New()
	router.Use(RequestID())
	router.Use(Recovery())
	router.Use(RateLimiter(config))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	// Test concurrent requests
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			done <- true
		}()
	}
	
	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
