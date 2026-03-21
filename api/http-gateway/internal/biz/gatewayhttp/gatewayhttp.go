package gatewayhttp

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"alldev-gin-rpc/pkg/gateway"
)

type Service struct {
	gw *gateway.Gateway
}

func New(gwCfg *gateway.Config) (*Service, error) {
	if gwCfg == nil {
		gwCfg = gateway.DefaultConfig()
	}

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
