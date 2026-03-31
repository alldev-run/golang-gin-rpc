package middleware

import (
	"net/http"
	"strings"

	"github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
	"github.com/alldev-run/golang-gin-rpc/pkg/rbac"
	"github.com/alldev-run/golang-gin-rpc/pkg/response"

	"github.com/gin-gonic/gin"
)


// JWT creates a JWT authentication middleware
func JWT(config AuthConfig) gin.HandlerFunc {
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization:Bearer "
	}

	return func(c *gin.Context) {
		// Check if request should be skipped
		if shouldSkipAuth(c, config) {
			c.Next()
			return
		}

		// Extract token
		token, err := extractToken(c, config.TokenLookup)
		if err != nil {
			response.Error(c, "Authentication required: "+err.Error(), nil)
			c.Abort()
			return
		}

		// Validate token
		claims, err := jwtx.ValidateAccessToken(token)
		if err != nil {
			response.Error(c, "Invalid token: "+err.Error(), nil)
			c.Abort()
			return
		}

		// Set user information in context
		userID := claims.UserID
		if config.KeyFunc != nil {
			// Convert claims to interface{} for KeyFunc
			claimsInterface := interface{}(claims)
			if id := config.KeyFunc(&claimsInterface); id != "" {
				userID = id
			}
		}

		c.Set("user_id", userID)
		c.Set("username", claims.Username)
		c.Set("device_id", claims.DeviceID)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequirePermission creates a middleware that requires one RBAC permission.
func RequirePermission(policy *rbac.Policy, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if policy == nil {
			response.Error(c, "RBAC policy is not configured", nil)
			c.Abort()
			return
		}

		claims, exists := c.Get("claims")
		if !exists {
			response.Error(c, "Authentication required", nil)
			c.Abort()
			return
		}

		userClaims, ok := claims.(*jwtx.Claims)
		if !ok {
			response.Error(c, "Invalid claims", nil)
			c.Abort()
			return
		}

		if hasDirectPermission(userClaims, permission) {
			c.Next()
			return
		}

		if !policy.HasPermission(getUserRoles(userClaims), permission) {
			response.Error(c, "Insufficient permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyPermission creates a middleware that requires any of the given permissions.
func RequireAnyPermission(policy *rbac.Policy, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if policy == nil {
			response.Error(c, "RBAC policy is not configured", nil)
			c.Abort()
			return
		}

		claims, exists := c.Get("claims")
		if !exists {
			response.Error(c, "Authentication required", nil)
			c.Abort()
			return
		}

		userClaims, ok := claims.(*jwtx.Claims)
		if !ok {
			response.Error(c, "Invalid claims", nil)
			c.Abort()
			return
		}

		if hasAnyDirectPermission(userClaims, permissions) {
			c.Next()
			return
		}

		if !policy.HasAnyPermission(getUserRoles(userClaims), permissions) {
			response.Error(c, "Insufficient permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// JWTOptional creates an optional JWT authentication middleware
// It doesn't abort the request if token is invalid, but sets user info if valid
func JWTOptional(config AuthConfig) gin.HandlerFunc {
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization:Bearer "
	}

	return func(c *gin.Context) {
		// Check if request should be skipped
		if shouldSkipAuth(c, config) {
			c.Next()
			return
		}

		// Extract token
		token, err := extractToken(c, config.TokenLookup)
		if err != nil {
			c.Next()
			return
		}

		// Validate token
		claims, err := jwtx.ValidateAccessToken(token)
		if err != nil {
			c.Next()
			return
		}

		// Set user information in context
		userID := claims.UserID
		if config.KeyFunc != nil {
			// Convert claims to interface{} for KeyFunc
			claimsInterface := interface{}(claims)
			if id := config.KeyFunc(&claimsInterface); id != "" {
				userID = id
			}
		}

		c.Set("user_id", userID)
		c.Set("username", claims.Username)
		c.Set("device_id", claims.DeviceID)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireAuth creates a simple authentication middleware that requires user_id in context
// This should be used after JWT middleware
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			response.Error(c, "Authentication required", nil)
			c.Abort()
			return
		}

		if userID == "" {
			response.Error(c, "Invalid user", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole creates a middleware that requires specific user roles
// This requires claims to have a "roles" field in the payload
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			response.Error(c, "Authentication required", nil)
			c.Abort()
			return
		}

		userClaims, ok := claims.(*jwtx.Claims)
		if !ok {
			response.Error(c, "Invalid claims", nil)
			c.Abort()
			return
		}

		// Check if user has required role
		userRoles := getUserRoles(userClaims)
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			response.Error(c, "Insufficient permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// shouldSkipAuth checks if the request should skip authentication
func shouldSkipAuth(c *gin.Context, config AuthConfig) bool {
	// Check custom skipper function - convert string-based skipper to gin.Context-based
	if config.Skipper != nil {
		// For now, we'll just check the path since Skipper expects string, not gin.Context
		if config.Skipper(c.Request.URL.Path) {
			return true
		}
	}

	// Check skip paths
	for _, path := range config.SkipPaths {
		if strings.HasPrefix(c.Request.URL.Path, path) {
			return true
		}
	}

	return false
}

// extractToken extracts the token from the request based on the lookup scheme
func extractToken(c *gin.Context, lookup string) (string, error) {
	parts := strings.Split(lookup, ":")
	if len(parts) != 3 {
		return "", gin.Error{
			Type: gin.ErrorTypePublic,
			Err:  http.ErrNotSupported,
		}
	}

	source := parts[0]
	name := parts[1]
	prefix := parts[2]

	switch source {
	case "header":
		header := c.GetHeader(name)
		if header == "" {
			return "", gin.Error{
				Type: gin.ErrorTypePublic,
				Err:  http.ErrMissingFile,
			}
		}
		if prefix != "" {
			if !strings.HasPrefix(header, prefix) {
				return "", gin.Error{
					Type: gin.ErrorTypePublic,
					Err:  http.ErrMissingFile,
				}
			}
			return strings.TrimPrefix(header, prefix), nil
		}
		return header, nil

	case "query":
		query := c.Query(name)
		if query == "" {
			return "", gin.Error{
				Type: gin.ErrorTypePublic,
				Err:  http.ErrMissingFile,
			}
		}
		return query, nil

	case "cookie":
		cookie, err := c.Cookie(name)
		if err != nil {
			return "", err
		}
		return cookie, nil

	default:
		return "", gin.Error{
			Type: gin.ErrorTypePublic,
			Err:  http.ErrNotSupported,
		}
	}
}

// getUserRoles extracts roles from claims
// This assumes roles are stored in claims.Payload under "roles" key
func getUserRoles(claims *jwtx.Claims) []string {
	if claims.Payload == nil {
		return []string{}
	}

	rolesStr, exists := claims.Payload["roles"]
	if !exists {
		return []string{}
	}

	// Split roles by comma if they're stored as a string
	roles := strings.Split(rolesStr, ",")
	for i, role := range roles {
		roles[i] = strings.TrimSpace(role)
	}
	return roles
}

func getUserPermissions(claims *jwtx.Claims) []string {
	if claims.Payload == nil {
		return []string{}
	}

	permissionsStr, exists := claims.Payload["permissions"]
	if !exists {
		return []string{}
	}

	permissions := strings.Split(permissionsStr, ",")
	for i, permission := range permissions {
		permissions[i] = strings.TrimSpace(permission)
	}
	return permissions
}

func hasDirectPermission(claims *jwtx.Claims, permission string) bool {
	for _, perm := range getUserPermissions(claims) {
		if perm == permission {
			return true
		}
	}
	return false
}

func hasAnyDirectPermission(claims *jwtx.Claims, permissions []string) bool {
	userPerms := getUserPermissions(claims)
	for _, required := range permissions {
		for _, existing := range userPerms {
			if required == existing {
				return true
			}
		}
	}
	return false
}

// GetUserID gets the user ID from the context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	return userID.(string), true
}

// GetUsername gets the username from the context
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	return username.(string), true
}

// GetClaims gets the JWT claims from the context
func GetClaims(c *gin.Context) (*jwtx.Claims, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	return claims.(*jwtx.Claims), true
}
