package auth

import (
	"golang-gin-rpc/pkg/auth/jwtx"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// Enabled indicates if authentication is enabled
	Enabled bool `yaml:"enabled"`
	// JWT holds JWT configuration
	JWT jwtx.Config `yaml:"jwt"`
}

// AuthManager manages authentication services
type AuthManager struct {
	config AuthConfig
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(config AuthConfig) *AuthManager {
	if config.Enabled {
		jwtx.Init(config.JWT)
	}
	return &AuthManager{
		config: config,
	}
}

// IsEnabled returns whether authentication is enabled
func (am *AuthManager) IsEnabled() bool {
	return am.config.Enabled
}
