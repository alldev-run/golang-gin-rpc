package bootstrap

import (
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/auth"
	"github.com/alldev-run/golang-gin-rpc/plugins/auth/jwtx"
)

// InitializeAuth initializes authentication services
func (b *Bootstrap) InitializeAuth() error {
	if !b.config.Security.JWT.Enabled {
		logger.Info("Authentication services disabled")
		return nil
	}

	authManager := auth.NewAuthManager(auth.AuthConfig{
		Enabled: b.config.Security.JWT.Enabled,
		JWT: jwtx.Config{
			Secret:          b.config.Security.JWT.Secret,
			AccessTokenTTL:  b.config.Security.JWT.Expiration,
			RefreshTokenTTL: b.config.Security.JWT.Expiration * 7,
		},
	})
	b.authManager = authManager
	b.setDependency(DepAuthManager, authManager)

	logger.Info("Authentication services initialized successfully")
	return nil
}
