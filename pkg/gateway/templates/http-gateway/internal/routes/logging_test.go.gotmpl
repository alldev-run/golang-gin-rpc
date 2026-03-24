package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	
	"github.com/alldev-run/golang-gin-rpc/pkg/middleware"
)

func TestAuthMiddleware(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a test router
	router := gin.New()
	
	// Add our auth middleware
	router.Use(authMiddleware())
	
	// Add a test endpoint
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected"})
	})
	
	// Test without API key
	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	// Test with invalid API key
	req, _ = http.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	
	// Test with valid API key
	req, _ = http.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "protected")
}

func TestFrameworkRequestLogger(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a test router with framework request logger
	router := gin.New()
	router.Use(middleware.RequestLogger(middleware.RequestLoggerConfig{
		LogRequestBody:  true,
		LogResponseBody: false,
		LogHeaders:      true,
		MaxBodySize:     1024,
		SkipPaths:       []string{"/health"},
	}))
	
	// Add test endpoints
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})
	
	router.POST("/api/users", handleUserList)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	
	// Test regular request logging
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test")
	
	// Test request with body logging
	userData := `{"name": "Test User", "email": "test@example.com"}`
	req, _ = http.NewRequest("POST", "/api/users", strings.NewReader(userData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Test skipped path logging
	req, _ = http.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserRoutesWithFrameworkLogging(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a test router with framework logging
	router := gin.New()
	router.Use(middleware.RequestLogger())
	
	// Add user routes without auth middleware for testing
	router.GET("/api/users", handleUserList)
	
	// Test user list endpoint
	req, _ := http.NewRequest("GET", "/api/users?page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Test with auth middleware
	authRouter := gin.New()
	authRouter.Use(middleware.RequestLogger())
	authRouter.Use(authMiddleware())
	authRouter.POST("/api/user", handleUserCreate)
	
	// Test user creation with auth
	userData := `{"name": "Test User", "email": "test@example.com", "age": 25}`
	req, _ = http.NewRequest("POST", "/api/user", strings.NewReader(userData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-api-key")
	w = httptest.NewRecorder()
	authRouter.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}
