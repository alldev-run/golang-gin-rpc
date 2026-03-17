package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)


// CORS creates a CORS middleware with the given configuration
func CORS(config CORSConfig) gin.HandlerFunc {
	// Set defaults
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	}
	if config.MaxAge == 0 {
		config.MaxAge = 86400
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Set CORS headers
		if len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if isAllowedOrigin(origin, config.AllowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ","))
		c.Header("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ","))
		
		if len(config.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ","))
		}

		if config.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Max-Age", string(config.MaxAge))

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			if config.OptionsPassthrough {
				c.Next()
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isAllowedOrigin checks if the origin is allowed
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowedOrigin := range allowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
		// Support wildcard subdomains
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// RestrictiveCORS creates a restrictive CORS configuration for production
func RestrictiveCORS(allowedOrigins []string) gin.HandlerFunc {
	config := CORSConfig{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders: []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		ExposedHeaders: []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge: 3600, // 1 hour
		OptionsPassthrough: false,
	}
	return CORS(config)
}

// DevelopmentCORS creates a permissive CORS configuration for development
func DevelopmentCORS() gin.HandlerFunc {
	config := CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{"*"},
		AllowCredentials: false,
		MaxAge: 86400,
		OptionsPassthrough: false,
	}
	return CORS(config)
}
