package gatewayhttp

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"alldev-gin-rpc/pkg/config"
	"alldev-gin-rpc/pkg/gateway"
)

type Service struct {
	gw *gateway.Gateway
}

func NewFromConfig(cfg *config.GlobalConfig) (*Service, error) {
	gwCfg := gateway.DefaultConfig()
	gwCfg.Host = cfg.Server.HTTP.Host
	gwCfg.Port = cfg.Server.HTTP.Port
	gwCfg.ReadTimeout = cfg.Server.HTTP.ReadTimeout
	gwCfg.WriteTimeout = cfg.Server.HTTP.WriteTimeout
	gwCfg.IdleTimeout = cfg.Server.HTTP.IdleTimeout

	gwCfg.CORS.AllowedOrigins = cfg.Security.CORS.AllowOrigins
	gwCfg.CORS.AllowedMethods = cfg.Security.CORS.AllowMethods
	gwCfg.CORS.AllowedHeaders = cfg.Security.CORS.AllowHeaders
	gwCfg.CORS.AllowCredentials = cfg.Security.CORS.AllowCredentials

	gwCfg.RateLimit.Enabled = cfg.Security.RateLimit.Enabled
	gwCfg.RateLimit.Requests = cfg.Security.RateLimit.Limit
	gwCfg.RateLimit.Window = cfg.Security.RateLimit.Window.String()

	gwCfg.Discovery.Type = cfg.Discovery.Type
	if cfg.Discovery.Address != "" {
		gwCfg.Discovery.Endpoints = []string{cfg.Discovery.Address}
	}
	var discoveryNamespace string
	if cfg.Discovery.Config != nil {
		discoveryNamespace = cfg.Discovery.Config["namespace"]
	}
	gwCfg.Discovery.Namespace = firstNonEmpty(discoveryNamespace, "default")
	gwCfg.Discovery.Timeout = cfg.Discovery.Timeout

	gw := gateway.NewGateway(gwCfg)
	if err := gw.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize gateway: %w", err)
	}
	if err := gw.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gateway: %w", err)
	}

	return &Service{gw: gw}, nil
}

func (s *Service) Register(engine *gin.Engine) {
	s.gw.SetupRoutes(engine)
}

func (s *Service) Close() error {
	if s.gw == nil {
		return nil
	}
	return s.gw.Stop()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
