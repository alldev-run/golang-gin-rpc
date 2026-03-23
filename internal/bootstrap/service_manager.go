package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/websocket"
)

const (
	ServiceAPIGateway = "api-gateway"
	ServiceRPC        = "rpc"
	ServiceWebSocket  = "websocket"
)

// ManagedService is the runtime contract for framework-managed services.
type ManagedService interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

// RegisterAPIGatewayServiceFactory registers a custom api-gateway service factory.
func (b *Bootstrap) RegisterAPIGatewayServiceFactory(options APIGatewayServiceOptions) error {
	name := options.Name
	if name == "" {
		name = ServiceAPIGateway
	}
	cfg := options.Config
	if cfg == nil {
		cfg = gateway.DefaultConfig()
	}

	return b.RegisterServiceFactory(name, func(boot *Bootstrap) (ManagedService, error) {
		service := &managedAPIGatewayService{
			name:    name,
			config:  cfg,
			httpOpt: options.HTTPOptions,
		}
		return &managedServiceAdapter{
			name: name,
			startFn: func(ctx context.Context) error {
				if err := service.Start(ctx); err != nil {
					return err
				}
				if service.httpService != nil {
					boot.gateway = service.httpService.Gateway()
					boot.setDependency("gateway", boot.gateway)
					boot.setDependency("gateway.http_service", service.httpService)
				}
				return nil
			},
			stopFn: service.Stop,
		}, nil
	})
}

// ServiceFactory builds a managed service using bootstrap-managed dependencies.
type ServiceFactory func(*Bootstrap) (ManagedService, error)

type managedServiceAdapter struct {
	name    string
	startFn func(context.Context) error
	stopFn  func(context.Context) error
}

type managedAPIGatewayService struct {
	name    string
	config  *gateway.Config
	httpOpt gateway.HTTPServiceOptions

	httpService *gateway.HTTPService
	server      *http.Server
}

func (s *managedServiceAdapter) Name() string {
	return s.name
}

func (s *managedServiceAdapter) Start(ctx context.Context) error {
	if s.startFn == nil {
		return nil
	}
	return s.startFn(ctx)
}

func (s *managedServiceAdapter) Stop(ctx context.Context) error {
	if s.stopFn == nil {
		return nil
	}
	return s.stopFn(ctx)
}

func (s *managedAPIGatewayService) Name() string {
	return s.name
}

func (s *managedAPIGatewayService) Start(ctx context.Context) error {
	_ = ctx
	if s.server != nil {
		return nil
	}
	svc, err := gateway.NewHTTPServiceWithOptions(s.config, s.httpOpt)
	if err != nil {
		return err
	}
	cfg := s.config
	if cfg == nil {
		cfg = gateway.DefaultConfig()
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      svc.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
	s.httpService = svc

	go func() {
		_ = s.server.ListenAndServe()
	}()
	return nil
}

func (s *managedAPIGatewayService) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	stopCtx := ctx
	if stopCtx == nil {
		var cancel context.CancelFunc
		stopCtx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	}
	err := s.server.Shutdown(stopCtx)
	if s.httpService != nil {
		closeErr := s.httpService.Close()
		if err == nil {
			err = closeErr
		}
	}
	s.server = nil
	s.httpService = nil
	return err
}

// WebSocketServiceOptions controls websocket service registration.
type WebSocketServiceOptions struct {
	Name    string
	Config  websocket.ServerConfig
	Path    string
	Handler websocket.Handler
}

// APIGatewayServiceOptions controls api-gateway service registration.
type APIGatewayServiceOptions struct {
	Name        string
	Config      *gateway.Config
	HTTPOptions gateway.HTTPServiceOptions
}

// RegisterDefaultServiceFactories registers built-in service factories.
func (b *Bootstrap) RegisterDefaultServiceFactories() error {
	if err := b.RegisterServiceFactory(ServiceAPIGateway, func(boot *Bootstrap) (ManagedService, error) {
		cfg := gateway.DefaultConfig()
		cfg.Host = boot.config.Server.HTTP.Host
		cfg.Port = boot.config.Server.HTTP.Port
		cfg.ReadTimeout = boot.config.Server.HTTP.ReadTimeout
		cfg.WriteTimeout = boot.config.Server.HTTP.WriteTimeout
		cfg.IdleTimeout = boot.config.Server.HTTP.IdleTimeout
		cfg.CORS.AllowedOrigins = boot.config.Security.CORS.AllowOrigins
		cfg.CORS.AllowedMethods = boot.config.Security.CORS.AllowMethods
		cfg.CORS.AllowedHeaders = boot.config.Security.CORS.AllowHeaders
		cfg.CORS.AllowCredentials = boot.config.Security.CORS.AllowCredentials
		cfg.RateLimit.Enabled = boot.config.Security.RateLimit.Enabled
		cfg.RateLimit.Requests = boot.config.Security.RateLimit.Limit
		cfg.RateLimit.Window = boot.config.Security.RateLimit.Window.String()
		cfg.Discovery.Type = boot.config.Discovery.Type
		if boot.config.Discovery.Address != "" {
			cfg.Discovery.Endpoints = []string{boot.config.Discovery.Address}
		}
		cfg.Discovery.Namespace = firstNonEmpty(boot.config.Discovery.Config["namespace"], "default")
		cfg.Discovery.Timeout = boot.config.Discovery.Timeout

		service := &managedAPIGatewayService{name: ServiceAPIGateway, config: cfg}
		return &managedServiceAdapter{
			name: ServiceAPIGateway,
			startFn: func(ctx context.Context) error {
				if err := service.Start(ctx); err != nil {
					return err
				}
				if service.httpService != nil {
					boot.gateway = service.httpService.Gateway()
					boot.setDependency("gateway", boot.gateway)
					boot.setDependency("gateway.http_service", service.httpService)
				}
				return nil
			},
			stopFn: service.Stop,
		}, nil
	}); err != nil {
		return err
	}

	if err := b.RegisterServiceFactory(ServiceRPC, func(boot *Bootstrap) (ManagedService, error) {
		return &managedServiceAdapter{
			name: ServiceRPC,
			startFn: func(ctx context.Context) error {
				_ = ctx
				if boot.rpcManager == nil {
					return boot.InitializeRPC()
				}
				if boot.rpcManager.IsStarted() {
					return nil
				}
				return boot.rpcManager.Start()
			},
			stopFn: func(ctx context.Context) error {
				_ = ctx
				if boot.rpcManager == nil || !boot.rpcManager.IsStarted() {
					return nil
				}
				return boot.rpcManager.Stop()
			},
		}, nil
	}); err != nil {
		return err
	}

	return nil
}

// RegisterWebSocketServiceFactory registers a websocket service factory.
func (b *Bootstrap) RegisterWebSocketServiceFactory(options WebSocketServiceOptions) error {
	name := options.Name
	if name == "" {
		name = ServiceWebSocket
	}

	cfg := options.Config
	if cfg.Addr == "" {
		cfg = websocket.DefaultServerConfig()
	}
	path := options.Path
	if path == "" {
		path = cfg.Path
	}
	if path == "" {
		path = "/ws"
	}
	handler := options.Handler
	if handler == nil {
		handler = func(ctx context.Context, conn *websocket.Conn) {
			for {
				msgType, payload, err := conn.Receive(ctx)
				if err != nil {
					return
				}
				switch msgType {
				case 2:
					_ = conn.SendBinary(ctx, payload)
				default:
					_ = conn.SendText(ctx, string(payload))
				}
			}
		}
	}

	return b.RegisterServiceFactory(name, func(boot *Bootstrap) (ManagedService, error) {
		return &managedServiceAdapter{
			name: name,
			startFn: func(ctx context.Context) error {
				_ = ctx
				if boot.websocketServer == nil {
					server := websocket.NewServer(cfg)
					server.Handle(path, handler)
					boot.websocketServer = server
					boot.setDependency("websocket.server", server)
				}
				return boot.websocketServer.Start()
			},
			stopFn: func(ctx context.Context) error {
				if boot.websocketServer == nil {
					return nil
				}
				if ctx == nil {
					stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					return boot.websocketServer.Stop(stopCtx)
				}
				return boot.websocketServer.Stop(ctx)
			},
		}, nil
	})
}

// RegisterServiceFactory registers a service factory for later runtime management.
func (b *Bootstrap) RegisterServiceFactory(name string, factory ServiceFactory) error {
	if name == "" {
		return fmt.Errorf("service name is required")
	}
	if factory == nil {
		return fmt.Errorf("service factory %s is nil", name)
	}

	b.serviceMu.Lock()
	defer b.serviceMu.Unlock()
	if _, exists := b.serviceFactories[name]; exists {
		return fmt.Errorf("service factory %s already registered", name)
	}
	b.serviceFactories[name] = factory
	b.serviceOrder = append(b.serviceOrder, name)
	return nil
}

// StartServices builds and starts managed services in registration order.
func (b *Bootstrap) StartServices(ctx context.Context, names ...string) error {
	serviceNames := b.resolveServiceNames(names...)
	for _, name := range serviceNames {
		svc, err := b.ensureManagedService(name)
		if err != nil {
			return err
		}
		if err := svc.Start(ctx); err != nil {
			return fmt.Errorf("failed to start service %s: %w", name, err)
		}
	}
	return nil
}

// StopServices stops managed services in reverse order.
func (b *Bootstrap) StopServices(ctx context.Context, names ...string) error {
	serviceNames := b.resolveServiceNames(names...)
	for i := len(serviceNames) - 1; i >= 0; i-- {
		name := serviceNames[i]
		svc, exists := b.getManagedService(name)
		if !exists {
			continue
		}
		if err := svc.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop service %s: %w", name, err)
		}
	}
	return nil
}

// ListServiceFactories returns registered service factory names in order.
func (b *Bootstrap) ListServiceFactories() []string {
	b.serviceMu.RLock()
	defer b.serviceMu.RUnlock()
	out := make([]string, len(b.serviceOrder))
	copy(out, b.serviceOrder)
	return out
}

func (b *Bootstrap) resolveServiceNames(names ...string) []string {
	if len(names) > 0 {
		out := make([]string, len(names))
		copy(out, names)
		return out
	}
	b.serviceMu.RLock()
	defer b.serviceMu.RUnlock()
	out := make([]string, len(b.serviceOrder))
	copy(out, b.serviceOrder)
	return out
}

func (b *Bootstrap) ensureManagedService(name string) (ManagedService, error) {
	if svc, exists := b.getManagedService(name); exists {
		return svc, nil
	}

	b.serviceMu.Lock()
	defer b.serviceMu.Unlock()
	if svc, exists := b.managedServices[name]; exists {
		return svc, nil
	}
	factory, exists := b.serviceFactories[name]
	if !exists {
		return nil, fmt.Errorf("service factory %s not found", name)
	}
	service, err := factory(b)
	if err != nil {
		return nil, fmt.Errorf("failed to build service %s: %w", name, err)
	}
	b.managedServices[name] = service
	return service, nil
}

func (b *Bootstrap) getManagedService(name string) (ManagedService, bool) {
	b.serviceMu.RLock()
	defer b.serviceMu.RUnlock()
	svc, exists := b.managedServices[name]
	return svc, exists
}

