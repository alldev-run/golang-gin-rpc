package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"
	"strings"

	"alldev-gin-rpc/pkg/health"
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/tracing"
)

// Gateway represents the HTTP gateway
type Gateway struct {
	config     *Config
	server     *Server
	router     *Router
	discovery  *ServiceDiscovery
	balancer   LoadBalancer
	health     *health.HealthManager
	tracer     *tracing.TracerProvider
	grpcProxy  *GRPCProxy
	jsonProxy  *JSONRPCProxy
	proxy      *Proxy           // HTTP proxy
	mu         sync.RWMutex
	started    bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Server represents the HTTP server
type Server struct {
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration
}

// Router handles HTTP routing
type Router struct {
	routes map[string]*Route
	mu     sync.RWMutex
}

// Route represents a single route
type Route struct {
	config    RouteConfig
	targets   []string
	healthyTargets []string
	lastHealthCheck time.Time
	balancer  LoadBalancer
	timeout   time.Duration
	retries   int
}

// LoadBalancer interface for different load balancing strategies
type LoadBalancer interface {
	Select(targets []string) (string, error)
	UpdateTargets(targets []string)
}

// NewGateway creates a new gateway instance
func NewGateway(config *Config) *Gateway {
	ctx, cancel := context.WithCancel(context.Background())
	router := &Router{
		routes: make(map[string]*Route),
	}
	balancer := NewLoadBalancerFactory().Create(config.LoadBalancer.Strategy)

	gateway := &Gateway{
		config: config,
		server: &Server{
			host:         config.Host,
			port:         config.Port,
			readTimeout:  config.ReadTimeout,
			writeTimeout: config.WriteTimeout,
			idleTimeout:  config.IdleTimeout,
		},
		router:   router,
		balancer: balancer,
		ctx:      ctx,
		cancel:   cancel,
	}

	for _, routeConfig := range config.Routes {
		routeConfig.Path = normalizeGinRoutePath(routeConfig.Path)
		route := &Route{
			config:  routeConfig,
			targets: append([]string(nil), routeConfig.Targets...),
			timeout: routeConfig.Timeout,
			retries: routeConfig.Retries,
		}
		key := gateway.routeKey(routeConfig.Path, routeConfig.Method)
		router.routes[key] = route
		if len(route.targets) > 0 && balancer != nil {
			balancer.UpdateTargets(route.targets)
		}
	}

	return gateway
}

// Initialize initializes the gateway
func (g *Gateway) Initialize() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Initialize service discovery
	if err := g.initDiscovery(); err != nil {
		logger.Errorf("Failed to initialize service discovery", logger.Error(err))
		return err
	}

	// Initialize load balancer
	if err := g.initLoadBalancer(); err != nil {
		logger.Errorf("Failed to initialize load balancer", logger.Error(err))
		return err
	}

	// Initialize routes
	if err := g.initRoutes(); err != nil {
		logger.Errorf("Failed to initialize routes", logger.Error(err))
		return err
	}

	g.initHealth()
	g.initTracing()

	logger.Info("Gateway initialized successfully")
	return nil
}

func (g *Gateway) initHealth() {
	hm := health.NewHealthManager()
	cfg := health.DefaultHealthCheckConfig()
	cfg.Enabled = true
	cfg.Timeout = 2 * time.Second
	hm.RegisterChecker(&upstreamHealthChecker{gw: g}, cfg)
	g.health = hm
}

func (g *Gateway) initTracing() {
	if g.config.Tracing != nil && g.config.Tracing.Enabled {
		// Initialize tracing with the provided configuration
		if err := tracing.InitGlobalTracer(*g.config.Tracing); err != nil {
			logger.Errorf("Failed to initialize tracing", logger.Error(err))
			return
		}
		g.tracer = tracing.GlobalTracer()
		logger.Info("Tracing initialized successfully",
			logger.String("type", g.config.Tracing.Type),
			logger.String("service", g.config.Tracing.ServiceName))
	}
	
	// Initialize protocol proxies
	g.grpcProxy = NewGRPCProxy(g)
	g.jsonProxy = NewJSONRPCProxy(g)
}

// Start starts the gateway server
func (g *Gateway) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.started {
		return ErrGatewayAlreadyStarted
	}

	// Start service discovery
	if g.discovery != nil {
		if err := g.discovery.Start(); err != nil {
			logger.Errorf("Failed to start service discovery", logger.Error(err))
		}
	}

	// Start health manager
	if g.health != nil {
		g.health.Start()
	}

	// Start background tasks
	go g.serviceDiscoveryLoop()
	go g.healthCheckLoop()

	g.started = true
	logger.Info("Gateway started successfully",
		logger.String("host", g.server.host),
		logger.Int("port", g.server.port))

	return nil
}

// Stop stops the gateway server
func (g *Gateway) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.started {
		return ErrGatewayNotStarted
	}

	logger.Info("Stopping gateway server...")

	// Cancel context to stop all background goroutines
	g.cancel()
	logger.Info("Gateway context cancelled")
	
	// Stop service discovery
	if g.discovery != nil {
		if err := g.discovery.Stop(); err != nil {
			logger.Errorf("Failed to stop service discovery", logger.Error(err))
		} else {
			logger.Info("Service discovery stopped")
		}
	}
	
	// Stop health manager
	if g.health != nil {
		g.health.Stop()
		logger.Info("Health manager stopped")
	}
	
	// Close protocol proxies
	if g.grpcProxy != nil {
		if err := g.grpcProxy.Close(); err != nil {
			logger.Errorf("Failed to close gRPC proxy", logger.Error(err))
		} else {
			logger.Info("gRPC proxy closed")
		}
	}
	
	if g.jsonProxy != nil {
		if err := g.jsonProxy.Close(); err != nil {
			logger.Errorf("Failed to close JSON-RPC proxy", logger.Error(err))
		} else {
			logger.Info("JSON-RPC proxy closed")
		}
	}
	
	// Close HTTP proxy
	if g.proxy != nil {
		if err := g.proxy.Close(); err != nil {
			logger.Errorf("Failed to close HTTP proxy", logger.Error(err))
		} else {
			logger.Info("HTTP proxy closed")
		}
	}
	
	g.started = false
	logger.Info("Gateway stopped successfully")

	return nil
}

// GetConfig returns the gateway configuration
func (g *Gateway) GetConfig() *Config {
	return g.config
}

// GetRouter returns the router instance
func (g *Gateway) GetRouter() *Router {
	return g.router
}

// GetDiscovery returns the service discovery instance
func (g *Gateway) GetDiscovery() *ServiceDiscovery {
	return g.discovery
}

// GetLoadBalancer returns the load balancer instance
func (g *Gateway) GetLoadBalancer() LoadBalancer {
	return g.balancer
}

// initDiscovery initializes the service discovery using existing discovery package
func (g *Gateway) initDiscovery() error {
	// Create service discovery using existing package
	sd, err := NewServiceDiscovery(g.config.Discovery)
	if err != nil {
		return fmt.Errorf("failed to create service discovery: %w", err)
	}
	
	// Store the discovery instance
	g.discovery = sd
	
	return nil
}

// initLoadBalancer initializes the load balancer
func (g *Gateway) initLoadBalancer() error {
	// This will be implemented based on the strategy
	// For now, return a mock implementation
	return nil
}

// initRoutes initializes the routes
func (g *Gateway) initRoutes() error {
	g.router.mu.Lock()
	defer g.router.mu.Unlock()

	for _, routeConfig := range g.config.Routes {
		routeConfig.Path = normalizeGinRoutePath(routeConfig.Path)
		route := &Route{
			config:   routeConfig,
			targets:  append([]string(nil), routeConfig.Targets...),
			timeout:  routeConfig.Timeout,
			retries:  routeConfig.Retries,
		}
		
		// Get service endpoints
		if g.discovery != nil && g.config.Discovery.Enabled {
			// 只有在服务发现启用时才尝试获取动态端点
			endpoints, err := g.discovery.GetServiceEndpoints(routeConfig.Service)
			if err != nil {
				logger.Debug("Service discovery failed during initialization", 
					logger.String("service", routeConfig.Service), 
					logger.String("error", err.Error()))
				// 使用配置中的静态 targets
				if len(routeConfig.Targets) > 0 {
					route.targets = append([]string(nil), routeConfig.Targets...)
					logger.Debug("Using static targets for service", 
						logger.String("service", routeConfig.Service),
						logger.Any("targets", routeConfig.Targets))
				}
			} else if len(endpoints) > 0 {
				route.targets = endpoints
				logger.Info("Using discovery endpoints for service", 
					logger.String("service", routeConfig.Service),
					logger.Any("endpoints", endpoints))
			} else {
				// 服务发现返回空结果，使用静态 targets
				if len(routeConfig.Targets) > 0 {
					route.targets = append([]string(nil), routeConfig.Targets...)
					logger.Debug("Discovery returned empty, using static targets for service", 
						logger.String("service", routeConfig.Service),
						logger.Any("targets", routeConfig.Targets))
				}
			}
		} else {
			// 服务发现被禁用或不可用，直接使用静态 targets
			if len(routeConfig.Targets) > 0 {
				route.targets = append([]string(nil), routeConfig.Targets...)
				logger.Debug("Service discovery disabled, using static targets for service", 
					logger.String("service", routeConfig.Service),
					logger.Any("targets", routeConfig.Targets))
			} else {
				logger.Warn("No targets available for service", 
					logger.String("service", routeConfig.Service))
			}
		}
		
		key := g.routeKey(routeConfig.Path, routeConfig.Method)
		g.router.routes[key] = route
	}

	logger.Info("Initialized routes", logger.Int("count", len(g.config.Routes)))
	return nil
}

// routeKey generates a unique key for a route
func (g *Gateway) routeKey(path, method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "*"
	}
	if method == "ANY" {
		method = "*"
	}
	return method + ":" + path
}

// serviceDiscoveryLoop runs service discovery in background
func (g *Gateway) serviceDiscoveryLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.refreshServices()
		}
	}
}

// healthCheckLoop runs health checks in background
func (g *Gateway) healthCheckLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if g.health != nil {
				g.health.CheckHealth(context.Background())
			}
		}
	}
}

// refreshServices refreshes service endpoints
func (g *Gateway) refreshServices() {
	if g.discovery == nil {
		return
	}
	
	// 检查服务发现是否被禁用
	if !g.config.Discovery.Enabled {
		return
	}
	
	// Refresh routes with updated service endpoints
	g.router.mu.Lock()
	defer g.router.mu.Unlock()
	
	for _, route := range g.router.routes {
		endpoints, err := g.discovery.GetServiceEndpoints(route.config.Service)
		if err != nil {
			// 减少日志噪音：只在第一次或错误类型变化时记录
			logger.Debug("Service discovery refresh failed", 
				logger.String("service", route.config.Service), 
				logger.String("error", err.Error()))
			
			// 使用配置中的静态 targets
			if len(route.config.Targets) > 0 {
				route.targets = append([]string(nil), route.config.Targets...)
				logger.Debug("Using static targets for service", 
					logger.String("service", route.config.Service),
					logger.Any("targets", route.config.Targets))
			}
			continue
		}
		
		// 如果服务发现返回空结果，使用静态 targets
		if len(endpoints) == 0 && len(route.config.Targets) > 0 {
			route.targets = append([]string(nil), route.config.Targets...)
			logger.Debug("Discovery returned empty, using static targets for service", 
				logger.String("service", route.config.Service),
				logger.Any("targets", route.config.Targets))
		} else {
			// Update route targets
			route.targets = endpoints
			logger.Debug("Updated service endpoints from discovery", 
				logger.String("service", route.config.Service),
				logger.Any("endpoints", endpoints))
		}
		route.healthyTargets = nil
		
		// Update load balancer targets
		if g.balancer != nil {
			g.balancer.UpdateTargets(route.targets)
		}
	}

	if g.health != nil {
		go g.health.CheckHealth(context.Background())
	}
}
