package auth

import (
	"errors"
	"alldev-gin-rpc/pkg/auth/jwtx"
)

var ErrInvalidAuthConfig = errors.New("invalid auth config")

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// Enabled indicates if authentication is enabled
	Enabled bool `yaml:"enabled"`
	// JWT holds JWT configuration
	JWT jwtx.Config `yaml:"jwt"`
}

// AuthManager manages authentication services
type AuthManager struct {
	config     AuthConfig
	jwtManager *jwtx.Manager
	initErr    error
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(config AuthConfig) *AuthManager {
	manager := &AuthManager{config: config}
	if err := config.Validate(); err != nil {
		manager.initErr = err
		return manager
	}
	if config.Enabled {
		manager.jwtManager = jwtx.NewManager(config.JWT)
		jwtx.Init(config.JWT)
	}
	manager.config = config
	return manager
}

// IsEnabled returns whether authentication is enabled
func (am *AuthManager) IsEnabled() bool {
	return am.config.Enabled
}

func (c *AuthConfig) Validate() error {
	if c == nil {
		return nil
	}
	if !c.Enabled {
		return nil
	}
	if c.JWT.Secret == "" {
		return ErrInvalidAuthConfig
	}
	return nil
}

func (am *AuthManager) IsReady() bool {
	if am == nil {
		return false
	}
	return am.initErr == nil
}

func (am *AuthManager) ValidationError() error {
	if am == nil {
		return nil
	}
	return am.initErr
}

func (am *AuthManager) JWT() *jwtx.Manager {
	if am == nil {
		return nil
	}
	return am.jwtManager
}
