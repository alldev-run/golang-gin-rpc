package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthConfig holds configuration for authentication middleware
type AuthConfig struct {
	// SkipPaths are paths that skip authentication
	SkipPaths []string `yaml:"skip_paths" json:"skip_paths"`
	
	// Skipper is a function to skip authentication for specific requests
	Skipper func(string) bool `yaml:"-" json:"-"`
	
	// Required for protected routes
	Required bool `yaml:"required" json:"required"`
	
	// TokenHeader is the header name for token
	TokenHeader string `yaml:"token_header" json:"token_header"`
	
	// TokenQuery is the query parameter name for token
	TokenQuery string `yaml:"token_query" json:"token_query"`
	
	// CookieName is the cookie name for token
	CookieName string `yaml:"cookie_name" json:"cookie_name"`
	
	// TokenLookup is how to look for the token (default: "header:Authorization:Bearer ")
	TokenLookup string `yaml:"token_lookup" json:"token_lookup"`
	
	// KeyFunc is a function to extract user ID from claims (default: claims.UserID)
	KeyFunc func(*interface{}) string `yaml:"-" json:"-"`
	
	// Enabled indicates if auth middleware is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// CORSConfig holds configuration for CORS middleware
type CORSConfig struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`
	
	// AllowedMethods is a list of methods the client is allowed to use for cross-domain requests
	AllowedMethods []string `yaml:"allowed_methods" json:"allowed_methods"`
	
	// AllowedHeaders is a list of headers the client is allowed to use for cross-domain requests
	AllowedHeaders []string `yaml:"allowed_headers" json:"allowed_headers"`
	
	// ExposedHeaders is a list of headers which are safe to expose to the API of a CORS API specification
	ExposedHeaders []string `yaml:"exposed_headers" json:"exposed_headers"`
	
	// AllowCredentials indicates whether cookies can be sent
	AllowCredentials bool `yaml:"allow_credentials" json:"allow_credentials"`
	
	// MaxAge indicates how long the results of a preflight request can be cached
	MaxAge int `yaml:"max_age" json:"max_age"`
	
	// OptionsPassthrough passes through the OPTIONS request to the next handler
	OptionsPassthrough bool `yaml:"options_passthrough" json:"options_passthrough"`
	
	// Enabled indicates if CORS middleware is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// RateLimiterConfig holds configuration for rate limiting middleware
type RateLimiterConfig struct {
	// RequestsPerMinute is the maximum number of requests allowed per minute
	RequestsPerMinute int `yaml:"requests_per_minute" json:"requests_per_minute"`
	
	// BurstSize is the maximum number of requests allowed in a short burst
	BurstSize int `yaml:"burst_size" json:"burst_size"`
	
	// SkipPaths are paths that skip rate limiting
	SkipPaths []string `yaml:"skip_paths" json:"skip_paths"`
	
	// KeyGenerator function for generating rate limit keys
	KeyGenerator func(string) string `yaml:"-" json:"-"`
	
	// SkipSuccessful skips counting successful requests (2xx status codes)
	SkipSuccessful bool `yaml:"skip_successful" json:"skip_successful"`
	
	// Message to return when rate limited
	Message string `yaml:"message" json:"message"`
	
	// Enabled indicates if rate limiting middleware is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// RecoveryConfig holds configuration for recovery middleware
type RecoveryConfig struct {
	// StackSize is the stack size to be printed
	StackSize int `yaml:"stack_size" json:"stack_size"`
	
	// Logger is the logger to use for logging panics
	Logger func(c *gin.Context, err interface{}) `yaml:"-" json:"-"`
	
	// LogAllRequests logs all requests, not just panics
	LogAllRequests bool `yaml:"log_all_requests" json:"log_all_requests"`
	
	// RequestBodyLimit limits the size of request body to log
	RequestBodyLimit int64 `yaml:"request_body_limit" json:"request_body_limit"`
	
	// DisableStackAll disables printing stack trace for all errors
	DisableStackAll bool `yaml:"disable_stack_all" json:"disable_stack_all"`
	
	// DisablePrintStack disables printing stack trace
	DisablePrintStack bool `yaml:"disable_print_stack" json:"disable_print_stack"`
	
	// Enabled indicates if recovery middleware is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// LoggingConfig holds configuration for logging middleware
type LoggingConfig struct {
	// SkipPaths are paths that skip logging
	SkipPaths []string `yaml:"skip_paths" json:"skip_paths"`
	
	// SkipLogger is a function to skip logging for specific requests
	SkipLogger func(string) bool `yaml:"-" json:"-"`
	
	// Output is the output destination
	Output string `yaml:"output" json:"output"`
	
	// Format is the log format (json, text)
	Format string `yaml:"format" json:"format"`
	
	// UTC indicates if timestamps should be in UTC
	UTC bool `yaml:"utc" json:"utc"`
	
	// SkipBody indicates if request body should be skipped
	SkipBody bool `yaml:"skip_body" json:"skip_body"`
	
	// MaxBodySize is the maximum body size to log
	MaxBodySize int `yaml:"max_body_size" json:"max_body_size"`
	
	// Enabled indicates if logging middleware is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	// XSSProtection enables XSS protection
	XSSProtection bool `yaml:"xss_protection" json:"xss_protection"`
	
	// ContentTypeNosniff enables content type nosniff
	ContentTypeNosniff bool `yaml:"content_type_nosniff" json:"content_type_nosniff"`
	
	// XFrameOptions sets X-Frame-Options header
	XFrameOptions string `yaml:"x_frame_options" json:"x_frame_options"`
	
	// HSTSMaxAge sets HSTS max age
	HSTSMaxAge int `yaml:"hsts_max_age" json:"hsts_max_age"`
	
	// HSTSIncludeSubdomains includes subdomains in HSTS
	HSTSIncludeSubdomains bool `yaml:"hsts_include_subdomains" json:"hsts_include_subdomains"`
	
	// ContentSecurityPolicy sets CSP header
	ContentSecurityPolicy string `yaml:"content_security_policy" json:"content_security_policy"`
	
	// ReferrerPolicy sets Referrer-Policy header
	ReferrerPolicy string `yaml:"referrer_policy" json:"referrer_policy"`
	
	// Enabled indicates if security middleware is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// Config holds all middleware configuration
type Config struct {
	// Auth configuration
	Auth AuthConfig `yaml:"auth" json:"auth"`
	
	// CORS configuration
	CORS CORSConfig `yaml:"cors" json:"cors"`
	
	// RateLimiter configuration
	RateLimiter RateLimiterConfig `yaml:"rate_limiter" json:"rate_limiter"`
	
	// Recovery configuration
	Recovery RecoveryConfig `yaml:"recovery" json:"recovery"`
	
	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`
	
	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`
	
	// IPFilter configuration
	IPFilter IPFilterConfig `yaml:"ip_filter" json:"ip_filter"`
}

// DefaultConfig returns default middleware configuration
func DefaultConfig() Config {
	return Config{
		Auth:        DefaultAuthConfig(),
		CORS:        DefaultCORSConfig(),
		RateLimiter: DefaultRateLimiterConfig(),
		Recovery:    DefaultRecoveryConfig(),
		Logging:     DefaultLoggingConfig(),
		Security:    DefaultSecurityConfig(),
		IPFilter:    DefaultIPFilterConfig(),
	}
}

// DefaultAuthConfig returns default auth configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		SkipPaths:   []string{"/health", "/metrics", "/ping"},
		Required:    true,
		TokenHeader: "Authorization",
		TokenQuery:  "token",
		CookieName:  "auth_token",
		Enabled:     true,
	}
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           86400,
		Enabled:          true,
	}
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerMinute: 100,
		BurstSize:         10,
		SkipPaths:         []string{"/health", "/metrics"},
		Enabled:           true,
	}
}

// DefaultRecoveryConfig returns default recovery configuration
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		StackSize:          4 * 1024,
		DisableStackAll:    false,
		DisablePrintStack:  false,
		Enabled:            true,
	}
}

// DefaultLoggingConfig returns default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		SkipPaths:    []string{"/health", "/metrics"},
		Output:       "stdout",
		Format:       "json",
		UTC:          true,
		SkipBody:     false,
		MaxBodySize:  1024 * 1024, // 1MB
		Enabled:      true,
	}
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		XSSProtection:         true,
		ContentTypeNosniff:    true,
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000, // 1 year
		HSTSIncludeSubdomains: false,
		ContentSecurityPolicy: "default-src 'self'",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		Enabled:               true,
	}
}

// APIConfig returns API-friendly middleware configuration
func APIConfig() Config {
	return Config{
		Auth: AuthConfig{
			SkipPaths:   []string{"/health", "/docs", "/openapi.json"},
			Required:    true,
			TokenHeader: "X-API-Key",
			TokenQuery:  "api_key",
			Enabled:     true,
		},
		CORS: CORSConfig{
			AllowedOrigins:   []string{"https://api.example.com"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
			AllowedHeaders:   []string{"Content-Type", "X-API-Key"},
			AllowCredentials: false,
			MaxAge:           3600,
			Enabled:          true,
		},
		RateLimiter: RateLimiterConfig{
			RequestsPerMinute: 1000,
			BurstSize:         100,
			SkipPaths:         []string{"/health"},
			Enabled:           true,
		},
		Recovery:    DefaultRecoveryConfig(),
		Logging: LoggingConfig{
			SkipPaths:    []string{"/health"},
			Output:       "stdout",
			Format:       "json",
			UTC:          true,
			SkipBody:     true,
			MaxBodySize:  1024,
			Enabled:      true,
		},
		Security: DefaultSecurityConfig(),
		IPFilter: DefaultIPFilterConfig(),
	}
}

// WebConfig returns web-friendly middleware configuration
func WebConfig() Config {
	return Config{
		Auth: AuthConfig{
			SkipPaths:   []string{"/", "/login", "/register", "/static"},
			Required:    false,
			TokenHeader: "Authorization",
			CookieName:  "session_token",
			Enabled:     true,
		},
		CORS: CORSConfig{
			AllowedOrigins:   []string{"https://example.com"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
			AllowedHeaders:   []string{"Content-Type"},
			AllowCredentials: true,
			MaxAge:           86400,
			Enabled:          true,
		},
		RateLimiter: RateLimiterConfig{
			RequestsPerMinute: 60,
			BurstSize:         10,
			SkipPaths:         []string{"/", "/static"},
			Enabled:           true,
		},
		Recovery: DefaultRecoveryConfig(),
		Logging: LoggingConfig{
			SkipPaths:    []string{"/static"},
			Output:       "stdout",
			Format:       "text",
			UTC:          false,
			SkipBody:     false,
			MaxBodySize:  2048,
			Enabled:      true,
		},
		Security: SecurityConfig{
			XSSProtection:         true,
			ContentTypeNosniff:    true,
			XFrameOptions:         "SAMEORIGIN",
			HSTSMaxAge:            0, // Disabled for web
			ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'",
			ReferrerPolicy:        "strict-origin-when-cross-origin",
			Enabled:               true,
		},
		IPFilter: DefaultIPFilterConfig(),
	}
}

// Validate validates the middleware configuration
func (c Config) Validate() error {
	if err := c.Auth.Validate(); err != nil {
		return err
	}
	if err := c.CORS.Validate(); err != nil {
		return err
	}
	if err := c.RateLimiter.Validate(); err != nil {
		return err
	}
	if err := c.Recovery.Validate(); err != nil {
		return err
	}
	if err := c.Logging.Validate(); err != nil {
		return err
	}
	if err := c.Security.Validate(); err != nil {
		return err
	}
	if err := c.IPFilter.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate validates the auth configuration
func (c AuthConfig) Validate() error {
	if len(c.SkipPaths) == 0 {
		c.SkipPaths = []string{"/health", "/metrics"}
	}
	if c.TokenHeader == "" {
		c.TokenHeader = "Authorization"
	}
	if c.TokenQuery == "" {
		c.TokenQuery = "token"
	}
	if c.CookieName == "" {
		c.CookieName = "auth_token"
	}
	return nil
}

// Validate validates the CORS configuration
func (c CORSConfig) Validate() error {
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	if len(c.AllowedMethods) == 0 {
		c.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(c.AllowedHeaders) == 0 {
		c.AllowedHeaders = []string{"*"}
	}
	if c.MaxAge == 0 {
		c.MaxAge = 86400
	}
	return nil
}

// Validate validates the rate limiter configuration
func (c RateLimiterConfig) Validate() error {
	if c.RequestsPerMinute == 0 {
		c.RequestsPerMinute = 100
	}
	if c.BurstSize == 0 {
		c.BurstSize = 10
	}
	if len(c.SkipPaths) == 0 {
		c.SkipPaths = []string{"/health", "/metrics"}
	}
	return nil
}

// Validate validates the recovery configuration
func (c RecoveryConfig) Validate() error {
	if c.StackSize == 0 {
		c.StackSize = 4 * 1024
	}
	return nil
}

// Validate validates the logging configuration
func (c LoggingConfig) Validate() error {
	if len(c.SkipPaths) == 0 {
		c.SkipPaths = []string{"/health", "/metrics"}
	}
	if c.Output == "" {
		c.Output = "stdout"
	}
	if c.Format == "" {
		c.Format = "json"
	}
	if c.MaxBodySize == 0 {
		c.MaxBodySize = 1024 * 1024
	}
	return nil
}

// Validate validates the security configuration
func (c SecurityConfig) Validate() error {
	if c.XFrameOptions == "" {
		c.XFrameOptions = "DENY"
	}
	if c.HSTSMaxAge == 0 && c.HSTSIncludeSubdomains {
		c.HSTSMaxAge = 31536000
	}
	if c.ContentSecurityPolicy == "" {
		c.ContentSecurityPolicy = "default-src 'self'"
	}
	if c.ReferrerPolicy == "" {
		c.ReferrerPolicy = "strict-origin-when-cross-origin"
	}
	return nil
}

// Validate validates the IP filter configuration
func (c IPFilterConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.Mode != IPFilterModeBlacklist && c.Mode != IPFilterModeWhitelist {
		c.Mode = IPFilterModeBlacklist
	}

	if c.BlockStatusCode == 0 {
		c.BlockStatusCode = http.StatusForbidden
	}

	if c.BlockMessage == "" {
		c.BlockMessage = "Access denied"
	}

	if len(c.SkipPaths) == 0 {
		c.SkipPaths = []string{"/health", "/metrics", "/ping"}
	}

	return nil
}
