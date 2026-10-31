package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/alldev-run/golang-gin-rpc/pkg/response"

	"github.com/gin-gonic/gin"
)

// APIKeyConfig holds configuration for API key authentication
type APIKeyConfig struct {
	// APIKeys is a map of valid API keys (key -> description/user)
	APIKeys map[string]string
	
	// HeaderName is the header name for API key (default: X-API-Key)
	HeaderName string
	
	// QueryName is the query parameter name for API key (default: api_key)
	QueryName string
	
	// SkipPaths are paths that skip authentication
	SkipPaths []string
	
	// Skipper is a function to skip authentication for specific requests
	Skipper func(c *gin.Context) bool
	
	// Enabled indicates if API key middleware is enabled
	Enabled bool
}

// DefaultAPIKeyConfig returns default API key configuration
func DefaultAPIKeyConfig() APIKeyConfig {
	return APIKeyConfig{
		APIKeys:    make(map[string]string),
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		SkipPaths:  []string{"/health", "/metrics", "/docs"},
		Enabled:    false,
	}
}

// APIKey creates an API key authentication middleware
func APIKey(config APIKeyConfig) gin.HandlerFunc {
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	if config.HeaderName == "" {
		config.HeaderName = "X-API-Key"
	}
	if config.QueryName == "" {
		config.QueryName = "api_key"
	}

	return func(c *gin.Context) {
		// Check if request should be skipped
		if shouldSkipAPIKeyAuth(c, config) {
			c.Next()
			return
		}

		// Extract API key
		key, err := extractAPIKey(c, config)
		if err != nil {
			response.Error(c, "API key required: "+err.Error(), nil)
			c.Abort()
			return
		}

		// Validate API key
		if !validateAPIKey(key, config.APIKeys) {
			response.Error(c, "Invalid API key", nil)
			c.Abort()
			return
		}

		// Set API key info in context
		userInfo := config.APIKeys[key]
		c.Set("api_key", key)
		c.Set("api_user", userInfo)

		c.Next()
	}
}

// APIKeyOptional creates an optional API key authentication middleware
// It doesn't abort the request if API key is invalid, but sets user info if valid
func APIKeyOptional(config APIKeyConfig) gin.HandlerFunc {
	if !config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	if config.HeaderName == "" {
		config.HeaderName = "X-API-Key"
	}
	if config.QueryName == "" {
		config.QueryName = "api_key"
	}

	return func(c *gin.Context) {
		// Check if request should be skipped
		if shouldSkipAPIKeyAuth(c, config) {
			c.Next()
			return
		}

		// Extract API key
		key, err := extractAPIKey(c, config)
		if err != nil {
			c.Next()
			return
		}

		// Validate API key
		if !validateAPIKey(key, config.APIKeys) {
			c.Next()
			return
		}

		// Set API key info in context
		userInfo := config.APIKeys[key]
		c.Set("api_key", key)
		c.Set("api_user", userInfo)

		c.Next()
	}
}

// shouldSkipAPIKeyAuth checks if the request should skip API key authentication
func shouldSkipAPIKeyAuth(c *gin.Context, config APIKeyConfig) bool {
	// Check custom skipper function
	if config.Skipper != nil && config.Skipper(c) {
		return true
	}

	// Check skip paths
	for _, path := range config.SkipPaths {
		if strings.HasPrefix(c.Request.URL.Path, path) {
			return true
		}
	}

	return false
}

// extractAPIKey extracts the API key from the request
func extractAPIKey(c *gin.Context, config APIKeyConfig) (string, error) {
	// Try header first
	headerKey := c.GetHeader(config.HeaderName)
	if headerKey != "" {
		return headerKey, nil
	}

	// Try query parameter
	queryKey := c.Query(config.QueryName)
	if queryKey != "" {
		return queryKey, nil
	}

	return "", http.ErrMissingFile
}

// validateAPIKey validates the API key against the list of valid keys
func validateAPIKey(key string, validKeys map[string]string) bool {
	for validKey := range validKeys {
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			return true
		}
	}
	return false
}

// GetAPIKey gets the API key from the context
func GetAPIKey(c *gin.Context) (string, bool) {
	apiKey, exists := c.Get("api_key")
	if !exists {
		return "", false
	}
	return apiKey.(string), true
}

// GetAPIUser gets the API user info from the context
func GetAPIUser(c *gin.Context) (string, bool) {
	apiUser, exists := c.Get("api_user")
	if !exists {
		return "", false
	}
	return apiUser.(string), true
}

// RequireAPIKey creates a middleware that requires a valid API key
// This should be used after APIKey middleware
func RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, exists := GetAPIKey(c)
		if !exists {
			response.Error(c, "API key required", nil)
			c.Abort()
			return
		}

		if apiKey == "" {
			response.Error(c, "Invalid API key", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// AddAPIKey adds an API key to the configuration
func (config *APIKeyConfig) AddAPIKey(key, description string) {
	if config.APIKeys == nil {
		config.APIKeys = make(map[string]string)
	}
	config.APIKeys[key] = description
}

// RemoveAPIKey removes an API key from the configuration
func (config *APIKeyConfig) RemoveAPIKey(key string) {
	if config.APIKeys != nil {
		delete(config.APIKeys, key)
	}
}

// HasAPIKey checks if an API key exists in the configuration
func (config *APIKeyConfig) HasAPIKey(key string) bool {
	if config.APIKeys == nil {
		return false
	}
	_, exists := config.APIKeys[key]
	return exists
}
