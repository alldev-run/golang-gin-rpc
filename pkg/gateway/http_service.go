package gateway

import (
	"fmt"
	"net/http"
)

type Middleware func(http.Handler) http.Handler

type HTTPServiceOptions struct {
	BizHandler    http.Handler
	IsBusinessPath func(string) bool
	Middlewares   []Middleware
}

type HTTPService struct {
	gw      *Gateway
	handler http.Handler
}

func NewHTTPService(gwCfg *Config, bizHandler http.Handler, isBizPath func(string) bool) (*HTTPService, error) {
	return NewHTTPServiceWithOptions(gwCfg, HTTPServiceOptions{
		BizHandler:     bizHandler,
		IsBusinessPath: isBizPath,
	})
}

func NewHTTPServiceWithOptions(gwCfg *Config, opt HTTPServiceOptions) (*HTTPService, error) {
	if gwCfg == nil {
		gwCfg = DefaultConfig()
	}

	gw := NewGateway(gwCfg)
	if err := gw.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize gateway: %w", err)
	}
	if err := gw.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gateway: %w", err)
	}

	svc := &HTTPService{gw: gw}
	svc.handler = chainMiddlewares(svc.buildHandler(opt.BizHandler, opt.IsBusinessPath), opt.Middlewares)
	return svc, nil
}

func (s *HTTPService) Handler() http.Handler {
	return s.handler
}

func (s *HTTPService) Gateway() *Gateway {
	return s.gw
}

func (s *HTTPService) Close() error {
	if s.gw == nil {
		return nil
	}
	return s.gw.Stop()
}

func (s *HTTPService) buildHandler(bizHandler http.Handler, isBizPath func(string) bool) http.Handler {
	gatewayHandler := s.gw.Handler()
	if bizHandler == nil || isBizPath == nil {
		return gatewayHandler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isBizPath(r.URL.Path) {
			bizHandler.ServeHTTP(w, r)
			return
		}
		gatewayHandler.ServeHTTP(w, r)
	})
}

func chainMiddlewares(h http.Handler, mws []Middleware) http.Handler {
	if h == nil {
		h = http.NewServeMux()
	}
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] == nil {
			continue
		}
		h = mws[i](h)
	}
	return h
}
