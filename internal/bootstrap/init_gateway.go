package bootstrap

import (
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/gateway"
)

// resolveGatewayPort picks the gateway listen port: an explicit gateway.port
// from the YAML section wins; otherwise fall back to main-app port + 2
// (port order: main app 8080, JSON-RPC 8081, gateway next) so the gateway
// never competes with the gin server for the same listener.
func (b *Bootstrap) resolveGatewayPort() int {
	port := b.config.Server.HTTP.Port + 2
	var section struct {
		Port int `yaml:"port"`
	}
	if ok, err := b.loader.UnmarshalSection("gateway", &section); err == nil && ok && section.Port > 0 {
		port = section.Port
	}
	return port
}

// InitializeGateway initializes the HTTP gateway
func (b *Bootstrap) InitializeGateway() error {
	gatewayConfig := gateway.DefaultConfig()
	gatewayConfig.Host = b.config.Server.HTTP.Host
	gatewayConfig.Port = b.resolveGatewayPort()
	gatewayConfig.ReadTimeout = b.config.Server.HTTP.ReadTimeout
	gatewayConfig.WriteTimeout = b.config.Server.HTTP.WriteTimeout
	gatewayConfig.IdleTimeout = b.config.Server.HTTP.IdleTimeout
	gatewayConfig.CORS.AllowedOrigins = b.pluginConfig.Security.CORS.AllowOrigins
	gatewayConfig.CORS.AllowedMethods = b.pluginConfig.Security.CORS.AllowMethods
	gatewayConfig.CORS.AllowedHeaders = b.pluginConfig.Security.CORS.AllowHeaders
	gatewayConfig.CORS.AllowCredentials = b.pluginConfig.Security.CORS.AllowCredentials
	gatewayConfig.RateLimit.Enabled = b.pluginConfig.Security.RateLimit.Enabled
	gatewayConfig.RateLimit.Requests = b.pluginConfig.Security.RateLimit.Limit
	gatewayConfig.RateLimit.Window = b.pluginConfig.Security.RateLimit.Window.String()
	gatewayConfig.Discovery.Type = b.pluginConfig.Discovery.Type
	if b.pluginConfig.Discovery.Address != "" {
		gatewayConfig.Discovery.Endpoints = []string{b.pluginConfig.Discovery.Address}
	}
	gatewayConfig.Discovery.Namespace = firstNonEmpty(b.pluginConfig.Discovery.Config["namespace"], "default")
	gatewayConfig.Discovery.Timeout = b.pluginConfig.Discovery.Timeout

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
