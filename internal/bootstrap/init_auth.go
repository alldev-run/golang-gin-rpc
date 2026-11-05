package bootstrap

import (
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/auth"
	"github.com/alldev-run/golang-gin-rpc/plugins/auth/jwtx"
)

// InitializeAuth initializes authentication services
func (b *Bootstrap) InitializeAuth() error {
	if !b.pluginConfig.Security.JWT.Enabled {
		logger.Info("Authentication services disabled")
		return nil
	}

	authManager := auth.NewAuthManager(auth.AuthConfig{
		Enabled: b.pluginConfig.Security.JWT.Enabled,
		JWT: jwtx.Config{
			Secret:          b.pluginConfig.Security.JWT.Secret,
			AccessTokenTTL:  b.pluginConfig.Security.JWT.Expiration,
			RefreshTokenTTL: b.pluginConfig.Security.JWT.Expiration * 7,
		},
	})
	b.authManager = authManager
	b.setDependency(DepAuthManager, authManager)

	logger.Info("Authentication services initialized successfully")
	return nil
}
