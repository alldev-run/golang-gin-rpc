package rpc

import (
	"context"
	"fmt"
)

// AuthConfig holds RPC authentication configuration
type AuthConfig struct {
	// APIKeys is a map of valid API keys (key -> description/user)
	APIKeys map[string]string
	
	// HeaderName is the header name for API key (default: X-API-Key)
	HeaderName string
	
	// QueryName is the query parameter name for API key (default: api_key)
	QueryName string
	
	// SkipMethods are RPC methods that skip authentication
	SkipMethods []string
	
	// Enabled indicates if RPC authentication is enabled
	Enabled bool
}

// DefaultAuthConfig returns default RPC authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		APIKeys:     make(map[string]string),
		HeaderName:  "X-API-Key",
		QueryName:   "api_key",
		SkipMethods: []string{"system.ping", "health.check", "service.stats"},
		Enabled:     false,
	}
}

// RPCAuth provides RPC authentication middleware
type RPCAuth struct {
	config AuthConfig
}

// NewRPCAuth creates a new RPC authentication middleware
func NewRPCAuth(config AuthConfig) *RPCAuth {
	return &RPCAuth{config: config}
}

// Name returns the middleware name
func (a *RPCAuth) Name() string {
	return "rpc_auth"
}

// Execute executes the RPC authentication middleware
func (a *RPCAuth) Execute(ctx context.Context, req interface{}) (interface{}, error) {
	if !a.config.Enabled {
		return req, nil
	}

	// Extract method name from request context if available
	methodName := extractRPCMethod(ctx)
	
	// Check if method should skip authentication
	if a.shouldSkipMethod(methodName) {
		return req, nil
	}

	// Extract API key from context (should be set by HTTP/gRPC transport layer)
	apiKey, err := extractAPIKeyFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("API key required: %w", err)
	}

	// Validate API key
	if !a.validateAPIKey(apiKey) {
		return nil, fmt.Errorf("invalid API key")
	}

	// Set authentication info in context
	ctx = setAuthInfo(ctx, apiKey, a.config.APIKeys[apiKey])

	return req, nil
}

// shouldSkipMethod checks if the method should skip authentication
func (a *RPCAuth) shouldSkipMethod(methodName string) bool {
	for _, skipMethod := range a.config.SkipMethods {
		if methodName == skipMethod {
			return true
		}
	}
	return false
}

// validateAPIKey validates the API key
func (a *RPCAuth) validateAPIKey(key string) bool {
	for validKey := range a.config.APIKeys {
		if key == validKey {
			return true
		}
	}
	return false
}

// AddAPIKey adds an API key to the configuration
func (a *RPCAuth) AddAPIKey(key, description string) {
	if a.config.APIKeys == nil {
		a.config.APIKeys = make(map[string]string)
	}
	a.config.APIKeys[key] = description
}

// RemoveAPIKey removes an API key from the configuration
func (a *RPCAuth) RemoveAPIKey(key string) {
	if a.config.APIKeys != nil {
		delete(a.config.APIKeys, key)
	}
}

// HasAPIKey checks if an API key exists in the configuration
func (a *RPCAuth) HasAPIKey(key string) bool {
	if a.config.APIKeys == nil {
		return false
	}
	_, exists := a.config.APIKeys[key]
	return exists
}

// ShouldSkipAuth checks if a method should skip authentication (public method)
func (a *RPCAuth) ShouldSkipAuth(methodName string) bool {
	return a.shouldSkipMethod(methodName)
}

// IsAuthenticated checks if the context has valid authentication
func (a *RPCAuth) IsAuthenticated(ctx context.Context) bool {
	apiKey, err := extractAPIKeyFromContext(ctx)
	if err != nil {
		return false
	}
	return a.validateAPIKey(apiKey)
}

// Context keys for authentication
type contextKey string

const (
	apiKeyContextKey    contextKey = "api_key"
	apiUserContextKey   contextKey = "api_user"
	methodContextKey    contextKey = "rpc_method"
)

// extractRPCMethod extracts RPC method name from context
func extractRPCMethod(ctx context.Context) string {
	if method, ok := ctx.Value(methodContextKey).(string); ok {
		return method
	}
	return ""
}

// extractAPIKeyFromContext extracts API key from context
func extractAPIKeyFromContext(ctx context.Context) (string, error) {
	// Try to get from context first
	if apiKey, ok := ctx.Value(apiKeyContextKey).(string); ok {
		return apiKey, nil
	}

	// For HTTP transport, check headers (this would be set by HTTP middleware)
	// For gRPC transport, check metadata (this would be set by gRPC interceptor)
	return "", fmt.Errorf("API key not found in context")
}

// setAuthInfo sets authentication info in context
func setAuthInfo(ctx context.Context, apiKey, apiUser string) context.Context {
	ctx = context.WithValue(ctx, apiKeyContextKey, apiKey)
	ctx = context.WithValue(ctx, apiUserContextKey, apiUser)
	return ctx
}

// GetAPIKeyFromContext gets the API key from context
func GetAPIKeyFromContext(ctx context.Context) (string, bool) {
	apiKey, ok := ctx.Value(apiKeyContextKey).(string)
	return apiKey, ok
}

// GetAPIUserFromContext gets the API user from context
func GetAPIUserFromContext(ctx context.Context) (string, bool) {
	apiUser, ok := ctx.Value(apiUserContextKey).(string)
	return apiUser, ok
}

// SetRPCMethodInContext sets the RPC method in context (for authentication)
func SetRPCMethodInContext(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, methodContextKey, method)
}

// SetAPIKeyInContext sets the API key in context (for transport layer)
func SetAPIKeyInContext(ctx context.Context, apiKey string) context.Context {
	return context.WithValue(ctx, apiKeyContextKey, apiKey)
}

// SetAPIUserInContext sets the API user in context (for transport layer)
func SetAPIUserInContext(ctx context.Context, apiUser string) context.Context {
	return context.WithValue(ctx, apiUserContextKey, apiUser)
}
