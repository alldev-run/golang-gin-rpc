package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/alldev-run/golang-gin-rpc/pkg/cors"
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

	cc := cors.Config{
		AllowedOrigins:   config.AllowedOrigins,
		AllowedMethods:   config.AllowedMethods,
		AllowedHeaders:   config.AllowedHeaders,
		ExposedHeaders:   config.ExposedHeaders,
		AllowCredentials: config.AllowCredentials,
		MaxAge:           config.MaxAge,
		OptionsPassthrough: config.OptionsPassthrough,
	}

	return func(c *gin.Context) {
		if handled := cors.Apply(c.Writer, c.Request, cc); handled {
			c.Abort()
			return
		}

		c.Next()
	}
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
