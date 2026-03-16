package gateway

import (
	"context"
	"sync"
	"time"

	"golang-gin-rpc/pkg/discovery"
	"golang-gin-rpc/pkg/logger"
)

// Gateway represents the HTTP gateway
type Gateway struct {
	config     *Config
	server     *Server
	router     *Router
	discovery  *ServiceDiscovery
	balancer   LoadBalancer
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
	
	return &Gateway{
		config: config,
		server: &Server{
			host:         config.Host,
			port:         config.Port,
			readTimeout:  config.ReadTimeout,
			writeTimeout: config.WriteTimeout,
			idleTimeout:  config.IdleTimeout,
		},
		router: &Router{
			routes: make(map[string]*Route),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Initialize initializes the gateway
func (g *Gateway) Initialize() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Initialize service discovery
	if err := g.initDiscovery(); err != nil {
		logger.Errorf("Failed to initialize service discovery: %v", logger.Error(err))
		return err
	}

	// Initialize load balancer
	if err := g.initLoadBalancer(); err != nil {
		logger.Errorf("Failed to initialize load balancer: %v", logger.Error(err))
		return err
	}

	// Initialize routes
	if err := g.initRoutes(); err != nil {
		logger.Errorf("Failed to initialize routes: %v", logger.Error(err))
		return err
	}

	logger.Info("Gateway initialized successfully")
	return nil
}

// Start starts the gateway server
func (g *Gateway) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.started {
		return ErrGatewayAlreadyStarted
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

	g.cancel()
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
		route := &Route{
			config:   routeConfig,
			targets:  []string{},
			timeout:  routeConfig.Timeout,
			retries:  routeConfig.Retries,
		}
		
		// Get service endpoints
		if g.discovery != nil {
			endpoints, err := g.discovery.GetServiceEndpoints(routeConfig.Service)
			if err != nil {
				logger.Errorf("Failed to get endpoints for service %s: %v", 
					routeConfig.Service, logger.Error(err))
				// Continue with empty targets, will be refreshed later
			} else {
				route.targets = endpoints
			}
		}
		
		key := g.routeKey(routeConfig.Path, routeConfig.Method)
		g.router.routes[key] = route
	}

	logger.Infof("Initialized %d routes", len(g.config.Routes))
	return nil
}

// routeKey generates a unique key for a route
func (g *Gateway) routeKey(path, method string) string {
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
			g.checkHealth()
		}
	}
}

// refreshServices refreshes service endpoints
func (g *Gateway) refreshServices() {
	if g.discovery == nil {
		return
	}
	
	// Refresh routes with updated service endpoints
	g.router.mu.Lock()
	defer g.router.mu.Unlock()
	
	for _, route := range g.router.routes {
		endpoints, err := g.discovery.GetServiceEndpoints(route.config.Service)
		if err != nil {
			logger.Errorf("Failed to refresh endpoints for service %s: %v", 
				route.config.Service, logger.Error(err))
			continue
		}
		
		// Update route targets
		route.targets = endpoints
		
		// Update load balancer targets
		if g.balancer != nil {
			g.balancer.UpdateTargets(endpoints)
		}
	}
}

// checkHealth checks health of service endpoints
func (g *Gateway) checkHealth() {
	// Implementation for health checking
}
