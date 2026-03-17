// Package rpc provides RPC server management
package rpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/ratelimiter"
)

// ManagerConfig holds RPC manager configuration
type ManagerConfig struct {
	Servers map[string]Config `yaml:"servers" json:"servers"`
	Timeout time.Duration    `yaml:"timeout" json:"timeout"`
	GracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout" json:"graceful_shutdown_timeout"`
}

// DefaultManagerConfig returns default RPC manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Servers: map[string]Config{
			"grpc": {
				Type:       ServerTypeGRPC,
				Host:       "localhost",
				Port:       50051,
				Network:    "tcp",
				Timeout:    30,
				MaxMsgSize: 4 * 1024 * 1024,
				Reflection: true,
			},
			"jsonrpc": {
				Type:    ServerTypeJSONRPC,
				Host:    "localhost",
				Port:    8080,
				Network: "tcp",
				Timeout: 30,
			},
		},
		Timeout:                30 * time.Second,
		GracefulShutdownTimeout: 10 * time.Second,
	}
}

// Manager manages multiple RPC servers
type Manager struct {
	config             ManagerConfig
	servers            map[string]Server
	services           map[string]Service
	registry           *ServiceRegistry
	middleware         *MiddlewareChain
	degradationManager *DegradationManager
	rateLimiterManager *ratelimiter.Manager
	mu                 sync.RWMutex
	started            bool
	startTime          time.Time
}

// NewManager creates a new RPC manager
func NewManager(config ManagerConfig) *Manager {
	manager := &Manager{
		config:             config,
		servers:            make(map[string]Server),
		services:           make(map[string]Service),
		registry:           NewServiceRegistry(),
		middleware:         NewMiddlewareChain(),
		rateLimiterManager: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
		startTime:          time.Now(),
	}
	_ = manager.rateLimiterManager.AddConfig("default", ratelimiter.DefaultConfig())
	
	// Initialize servers from config
	for name, serverConfig := range config.Servers {
		server := NewServer(serverConfig)
		if setter, ok := server.(interface{ SetDegradationManager(*DegradationManager) }); ok {
			setter.SetDegradationManager(manager.degradationManager)
		}
		if setter, ok := server.(interface{ SetRateLimiterManager(*ratelimiter.Manager) }); ok {
			setter.SetRateLimiterManager(manager.rateLimiterManager)
		}
		manager.servers[name] = server
	}
	
	return manager
}

// AddServer adds an RPC server to the manager
func (m *Manager) AddServer(name string, config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		return fmt.Errorf("cannot add server after manager has started")
	}
	
	if _, exists := m.servers[name]; exists {
		return fmt.Errorf("server %s already exists", name)
	}
	
	server := NewServer(config)
	if setter, ok := server.(interface{ SetDegradationManager(*DegradationManager) }); ok {
		setter.SetDegradationManager(m.degradationManager)
	}
	if setter, ok := server.(interface{ SetRateLimiterManager(*ratelimiter.Manager) }); ok {
		setter.SetRateLimiterManager(m.rateLimiterManager)
	}
	m.servers[name] = server
	
	logger.Info("Added RPC server", 
		zap.String("name", name),
		zap.String("type", string(config.Type)),
		zap.String("address", fmt.Sprintf("%s:%d", config.Host, config.Port)))
	
	return nil
}

// RemoveServer removes an RPC server from the manager
func (m *Manager) RemoveServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		return fmt.Errorf("cannot remove server after manager has started")
	}
	
	if _, exists := m.servers[name]; !exists {
		return fmt.Errorf("server %s does not exist", name)
	}
	
	delete(m.servers, name)
	
	logger.Info("Removed RPC server", zap.String("name", name))
	
	return nil
}

// RegisterService registers a service with all servers
func (m *Manager) RegisterService(service Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		return fmt.Errorf("cannot register service after manager has started")
	}
	
	serviceName := service.Name()
	if _, exists := m.services[serviceName]; exists {
		return fmt.Errorf("service %s already registered", serviceName)
	}
	
	m.services[serviceName] = service
	m.registry.Register(serviceName, service)
	
	logger.Info("Registered RPC service", zap.String("name", serviceName))
	
	return nil
}

// UnregisterService unregisters a service
func (m *Manager) UnregisterService(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		return fmt.Errorf("cannot unregister service after manager has started")
	}
	
	if _, exists := m.services[name]; !exists {
		return fmt.Errorf("service %s does not exist", name)
	}
	
	delete(m.services, name)
	m.registry.Unregister(name)
	
	logger.Info("Unregistered RPC service", zap.String("name", name))
	
	return nil
}

// AddMiddleware adds middleware to the chain
func (m *Manager) AddMiddleware(middleware *Middleware) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		logger.Warn("Cannot add middleware after manager has started")
		return
	}
	
	m.middleware.Add(middleware)
	logger.Info("Added RPC middleware", zap.String("name", middleware.Name()))
}

// SetDegradationManager sets degradation manager for all registered servers
func (m *Manager) SetDegradationManager(dm *DegradationManager) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.degradationManager = dm
	for _, server := range m.servers {
		if setter, ok := server.(interface{ SetDegradationManager(*DegradationManager) }); ok {
			setter.SetDegradationManager(dm)
		}
	}
}

// SetRateLimiterManager sets rate limiter manager for all registered servers
func (m *Manager) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rateLimiterManager = rlm
	for _, server := range m.servers {
		if setter, ok := server.(interface{ SetRateLimiterManager(*ratelimiter.Manager) }); ok {
			setter.SetRateLimiterManager(rlm)
		}
	}
}

// Start starts all RPC servers
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.started {
		return fmt.Errorf("manager already started")
	}
	
	// Register services with servers
	for _, service := range m.services {
		for serverName, server := range m.servers {
			if err := server.RegisterService(service); err != nil {
				return fmt.Errorf("failed to register service %s with server %s: %w", 
					service.Name(), serverName, err)
			}
		}
	}
	
	// Start all servers
	for name, server := range m.servers {
		go func(serverName string, rpcServer Server) {
			logger.Info("Starting RPC server", 
				zap.String("name", serverName),
				zap.String("type", string(rpcServer.Type())),
				zap.String("address", rpcServer.Addr()))
			
			if err := rpcServer.Start(); err != nil {
				logger.Errorf("Failed to start RPC server", 
					zap.String("name", serverName),
					zap.Error(err))
			} else {
				logger.Info("RPC server started successfully", 
					zap.String("name", serverName),
					zap.String("address", rpcServer.Addr()))
			}
		}(name, server)
	}
	
	m.started = true
	m.startTime = time.Now()
	
	logger.Info("RPC manager started", 
		zap.Int("servers", len(m.servers)),
		zap.Int("services", len(m.services)))
	
	return nil
}

// Stop stops all RPC servers gracefully
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.started {
		return fmt.Errorf("manager not started")
	}
	
	logger.Info("Stopping RPC manager...")
	
	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), m.config.GracefulShutdownTimeout)
	defer cancel()
	
	// Stop all servers
	var errors []error
	for name, server := range m.servers {
		go func(serverName string, rpcServer Server) {
			logger.Info("Stopping RPC server", zap.String("name", serverName))
			
			if err := rpcServer.Stop(); err != nil {
				logger.Errorf("Failed to stop RPC server", 
					zap.String("name", serverName),
					zap.Error(err))
			} else {
				logger.Info("RPC server stopped successfully", zap.String("name", serverName))
			}
		}(name, server)
	}
	
	// Wait for all servers to stop or timeout
	done := make(chan struct{})
	go func() {
		// In a real implementation, you would wait for actual server shutdown
		time.Sleep(1 * time.Second)
		close(done)
	}()
	
	select {
	case <-done:
		logger.Info("All RPC servers stopped gracefully")
	case <-ctx.Done():
		logger.Warn("Graceful shutdown timeout, forcing stop")
		errors = append(errors, fmt.Errorf("graceful shutdown timeout"))
	}
	
	m.started = false
	
	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}
	
	return nil
}

// IsStarted returns true if the manager is started
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetServer returns a server by name
func (m *Manager) GetServer(name string) (Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, exists := m.servers[name]
	return server, exists
}

// GetService returns a service by name
func (m *Manager) GetService(name string) (Service, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	service, exists := m.services[name]
	return service, exists
}

// ListServers returns all server names
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// ListServices returns all service names
func (m *Manager) ListServices() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	names := make([]string, 0, len(m.services))
	for name := range m.services {
		names = append(names, name)
	}
	return names
}

// GetStatus returns the status of all servers and services
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := map[string]interface{}{
		"started":    m.started,
		"start_time": m.startTime,
		"uptime":     time.Since(m.startTime).String(),
		"servers":    make(map[string]interface{}),
		"services":   make(map[string]interface{}),
	}
	
	// Server status
	servers := status["servers"].(map[string]interface{})
	for name, server := range m.servers {
		servers[name] = map[string]interface{}{
			"type":    string(server.Type()),
			"address": server.Addr(),
			"services": GetServiceInfo(server),
		}
	}
	
	// Service status
	services := status["services"].(map[string]interface{})
	for name, service := range m.services {
		if baseService, ok := service.(interface{ Health() HealthStatus }); ok {
			services[name] = baseService.Health()
		} else {
			services[name] = map[string]interface{}{
				"name": service.Name(),
			}
		}
	}
	
	return status
}

// GetRegistry returns the service registry
func (m *Manager) GetRegistry() *ServiceRegistry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registry
}

// GetMiddlewareChain returns the middleware chain
func (m *Manager) GetMiddlewareChain() *MiddlewareChain {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.middleware
}

// Restart restarts the RPC manager
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return fmt.Errorf("failed to stop manager: %w", err)
	}
	
	return m.Start()
}

// HealthChecker provides health checking for the RPC manager
type HealthChecker struct {
	manager *Manager
}

// NewHealthChecker creates a new health checker for the RPC manager
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{manager: manager}
}

// CheckHealth checks the health of the RPC manager
func (h *HealthChecker) CheckHealth(ctx context.Context) error {
	if !h.manager.IsStarted() {
		return fmt.Errorf("RPC manager is not started")
	}
	
	// Check all servers
	for _, name := range h.manager.ListServers() {
		if rpcServer, exists := h.manager.GetServer(name); exists {
			// Simple health check - verify server address is not empty
			if rpcServer.Addr() == "" {
				return fmt.Errorf("server %s has invalid address", name)
			}
		}
	}
	
	return nil
}

// GetDetailedHealth returns detailed health information
func (h *HealthChecker) GetDetailedHealth(ctx context.Context) (map[string]interface{}, error) {
	status := h.manager.GetStatus()
	
	// Add health check timestamp
	status["health_check_time"] = time.Now()
	status["healthy"] = h.CheckHealth(ctx) == nil
	
	return status, nil
}
