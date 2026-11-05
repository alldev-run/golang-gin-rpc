package bootstrap

import (
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/gateway"
)

// InitializeGateway initializes the HTTP gateway
func (b *Bootstrap) InitializeGateway() error {
	gatewayConfig := gateway.DefaultConfig()
	gatewayConfig.Host = b.config.Server.HTTP.Host
	gatewayConfig.Port = b.config.Server.HTTP.Port
	gatewayConfig.ReadTimeout = b.config.Server.HTTP.ReadTimeout
	gatewayConfig.WriteTimeout = b.config.Server.HTTP.WriteTimeout
	gatewayConfig.IdleTimeout = b.config.Server.HTTP.IdleTimeout
	gatewayConfig.CORS.AllowedOrigins = b.config.Security.CORS.AllowOrigins
	gatewayConfig.CORS.AllowedMethods = b.config.Security.CORS.AllowMethods
	gatewayConfig.CORS.AllowedHeaders = b.config.Security.CORS.AllowHeaders
	gatewayConfig.CORS.AllowCredentials = b.config.Security.CORS.AllowCredentials
	gatewayConfig.RateLimit.Enabled = b.config.Security.RateLimit.Enabled
	gatewayConfig.RateLimit.Requests = b.config.Security.RateLimit.Limit
	gatewayConfig.RateLimit.Window = b.config.Security.RateLimit.Window.String()
	gatewayConfig.Discovery.Type = b.config.Discovery.Type
	if b.config.Discovery.Address != "" {
		gatewayConfig.Discovery.Endpoints = []string{b.config.Discovery.Address}
	}
	gatewayConfig.Discovery.Namespace = firstNonEmpty(b.config.Discovery.Config["namespace"], "default")
	gatewayConfig.Discovery.Timeout = b.config.Discovery.Timeout

	b.gateway = gateway.NewGateway(gatewayConfig)
	b.setDependency(DepGateway, b.gateway)

	// Initialize gateway
	if err := b.gateway.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}

	// Start gateway
	if err := b.gateway.Start(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	logger.Info("Gateway initialized successfully",
		logger.String("host", gatewayConfig.Host),
		logger.Int("port", gatewayConfig.Port))
	return nil
}
