package gatewayhttp

import (
	"fmt"
	"net/http"

	"alldev-gin-rpc/api/http-gateway/internal/httpapi"
	"alldev-gin-rpc/pkg/gateway"
)

type Service struct {
	gw      *gateway.Gateway
	handler http.Handler
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

	svc := &Service{gw: gw}
	svc.handler = svc.buildHandler()
	return svc, nil
}

// Handler returns the HTTP handler for the gateway service.
// It includes:
// - pkg/gateway middlewares + proxy routes + /health /ready /info
// - business routes registered under api/http-gateway/internal/router
func (s *Service) Handler() http.Handler {
	return s.handler
}

func (s *Service) buildHandler() http.Handler {
	gatewayHandler := s.gw.Handler()
	bizHandler := httpapi.NewRouter(s.gw.GetConfig()).Handler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpapi.IsBusinessPath(r.URL.Path) {
			bizHandler.ServeHTTP(w, r)
			return
		}
		gatewayHandler.ServeHTTP(w, r)
	})
}

func (s *Service) Close() error {
	if s.gw == nil {
		return nil
	}
	return s.gw.Stop()
}
