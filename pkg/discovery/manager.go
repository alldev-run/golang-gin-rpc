// Package discovery provides enhanced service discovery functionality
package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"alldev-gin-rpc/pkg/logger"
)

// ManagerConfig holds service discovery manager configuration
type ManagerConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	RegistryType     string        `yaml:"registry_type" json:"registry_type"`
	RegistryAddress  string        `yaml:"registry_address" json:"registry_address"`
	Timeout          time.Duration `yaml:"timeout" json:"timeout"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	AutoRegister     bool          `yaml:"auto_register" json:"auto_register"`
	ServiceName      string        `yaml:"service_name" json:"service_name"`
	ServiceAddress   string        `yaml:"service_address" json:"service_address"`
	ServicePort      int           `yaml:"service_port" json:"service_port"`
	ServiceTags      []string      `yaml:"service_tags" json:"service_tags"`
}

// DefaultManagerConfig returns default discovery manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Enabled:             false,
		RegistryType:        "consul",
		RegistryAddress:     "localhost:8500",
		Timeout:             30 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		AutoRegister:        true,
		ServiceName:         "alldev-gin-rpc",
		ServiceAddress:      "localhost",
		ServicePort:         8080,
		ServiceTags:         []string{"go", "rpc", "api"},
	}
}

// ServiceDiscoveryManager manages service discovery operations
type ServiceDiscoveryManager struct {
	config     ManagerConfig
	discovery Discovery
	instances map[string]*ServiceInstance
	mu        sync.RWMutex
	started   bool
	stopCh    chan struct{}
}

// NewServiceDiscoveryManager creates a new service discovery manager
func NewServiceDiscoveryManager(config ManagerConfig) (*ServiceDiscoveryManager, error) {
	if !config.Enabled {
		return &ServiceDiscoveryManager{
			config:     config,
			instances: make(map[string]*ServiceInstance),
			stopCh:    make(chan struct{}),
		}, nil
	}

	// Create discovery client
	discoveryConfig := Config{
		Type:    RegistryType(config.RegistryType),
		Address: config.RegistryAddress,
		Timeout: config.Timeout,
	}

	discovery, err := NewDiscovery(discoveryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	return &ServiceDiscoveryManager{
		config:     config,
		discovery: discovery,
		instances: make(map[string]*ServiceInstance),
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts the service discovery manager
func (m *ServiceDiscoveryManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("service discovery manager already started")
	}

	if !m.config.Enabled {
		logger.Info("Service discovery is disabled")
		m.started = true
		return nil
	}

	logger.Info("Starting service discovery manager",
		zap.String("registry_type", m.config.RegistryType),
		zap.String("registry_address", m.config.RegistryAddress))

	// Auto register this service if enabled
	if m.config.AutoRegister {
		if err := m.registerSelf(); err != nil {
			return fmt.Errorf("failed to register self: %w", err)
		}
	}

	// Start health check routine
	go m.healthCheckLoop()

	m.started = true
	logger.Info("Service discovery manager started successfully")
	return nil
}

// Stop stops the service discovery manager
func (m *ServiceDiscoveryManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	logger.Info("Stopping service discovery manager")

	// Stop health check routine
	close(m.stopCh)

	// Deregister self if auto registered
	if m.config.AutoRegister {
		if err := m.deregisterSelf(); err != nil {
			logger.Errorf("Failed to deregister self", zap.Error(err))
		}
	}

	m.started = false
	logger.Info("Service discovery manager stopped")
	return nil
}

// Register registers a service instance
func (m *ServiceDiscoveryManager) Register(ctx context.Context, instance *ServiceInstance) error {
	if !m.config.Enabled {
		return fmt.Errorf("service discovery is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.discovery.Register(ctx, instance); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	m.instances[instance.ID] = instance
	logger.Info("Service registered",
		zap.String("id", instance.ID),
		zap.String("name", instance.Name),
		zap.String("address", fmt.Sprintf("%s:%d", instance.Address, instance.Port)))

	return nil
}

// Deregister deregisters a service instance
func (m *ServiceDiscoveryManager) Deregister(ctx context.Context, instance *ServiceInstance) error {
	if !m.config.Enabled {
		return fmt.Errorf("service discovery is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.discovery.Deregister(ctx, instance); err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	delete(m.instances, instance.ID)
	logger.Info("Service deregistered",
		zap.String("id", instance.ID),
		zap.String("name", instance.Name))

	return nil
}

// GetService retrieves service instances by name
func (m *ServiceDiscoveryManager) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("service discovery is disabled")
	}

	instances, err := m.discovery.GetService(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	logger.Debug("Retrieved service instances",
		zap.String("service_name", serviceName),
		zap.Int("instance_count", len(instances)))

	return instances, nil
}

// GetAllServices retrieves all registered services
func (m *ServiceDiscoveryManager) GetAllServices(ctx context.Context) (map[string][]*ServiceInstance, error) {
	if !m.config.Enabled {
		return make(map[string][]*ServiceInstance), nil
	}

	services := make(map[string][]*ServiceInstance)
	
	m.mu.RLock()
	for _, instance := range m.instances {
		services[instance.Name] = append(services[instance.Name], instance)
	}
	m.mu.RUnlock()

	return services, nil
}

// GetRegisteredInstances returns locally registered instances
func (m *ServiceDiscoveryManager) GetRegisteredInstances() map[string]*ServiceInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instances := make(map[string]*ServiceInstance)
	for id, instance := range m.instances {
		instances[id] = instance
	}
	return instances
}

// IsStarted returns true if the manager is started
func (m *ServiceDiscoveryManager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// GetConfig returns the manager configuration
func (m *ServiceDiscoveryManager) GetConfig() ManagerConfig {
	return m.config
}

// registerSelf registers the current service instance
func (m *ServiceDiscoveryManager) registerSelf() error {
	instance := &ServiceInstance{
		ID:   fmt.Sprintf("%s-%d", m.config.ServiceName, time.Now().Unix()),
		Name: m.config.ServiceName,
		Address: m.config.ServiceAddress,
		Port:   m.config.ServicePort,
		Payload: map[string]string{
			"version":     "1.0.0",
			"registered_at": time.Now().Format(time.RFC3339),
		},
	}

	// Add tags to payload
	if len(m.config.ServiceTags) > 0 {
		tags := ""
		for i, tag := range m.config.ServiceTags {
			if i > 0 {
				tags += ","
			}
			tags += tag
		}
		instance.Payload["tags"] = tags
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	return m.Register(ctx, instance)
}

// deregisterSelf deregisters the current service instance
func (m *ServiceDiscoveryManager) deregisterSelf() error {
	m.mu.RLock()
	var instance *ServiceInstance
	for _, inst := range m.instances {
		if inst.Name == m.config.ServiceName {
			instance = inst
			break
		}
	}
	m.mu.RUnlock()

	if instance == nil {
		return nil // Not registered
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	return m.Deregister(ctx, instance)
}

// healthCheckLoop runs periodic health checks
func (m *ServiceDiscoveryManager) healthCheckLoop() {
	if m.config.HealthCheckInterval <= 0 {
		return
	}

	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck performs health check on registered instances
func (m *ServiceDiscoveryManager) performHealthCheck() {
	m.mu.RLock()
	instances := make([]*ServiceInstance, 0, len(m.instances))
	for _, instance := range m.instances {
		instances = append(instances, instance)
	}
	m.mu.RUnlock()

	for _, instance := range instances {
		// Simple health check - in a real implementation, you would check the service health
		logger.Debug("Health check", 
			zap.String("service", instance.Name),
			zap.String("address", fmt.Sprintf("%s:%d", instance.Address, instance.Port)))
	}
}

// ServiceWatcher watches for service changes
type ServiceWatcher struct {
	manager   *ServiceDiscoveryManager
	serviceName string
	watchCh   chan []*ServiceInstance
	stopCh    chan struct{}
}

// NewServiceWatcher creates a new service watcher
func NewServiceWatcher(manager *ServiceDiscoveryManager, serviceName string) *ServiceWatcher {
	return &ServiceWatcher{
		manager:     manager,
		serviceName: serviceName,
		watchCh:     make(chan []*ServiceInstance, 10),
		stopCh:      make(chan struct{}),
	}
}

// Watch starts watching for service changes
func (w *ServiceWatcher) Watch(ctx context.Context) <-chan []*ServiceInstance {
	go w.watchLoop(ctx)
	return w.watchCh
}

// Stop stops the watcher
func (w *ServiceWatcher) Stop() {
	close(w.stopCh)
}

// watchLoop runs the watch loop
func (w *ServiceWatcher) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastInstances []*ServiceInstance

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			instances, err := w.manager.GetService(ctx, w.serviceName)
			if err != nil {
				logger.Errorf("Failed to get service during watch", 
					zap.String("service", w.serviceName),
					zap.Error(err))
				continue
			}

			// Check if instances have changed
			if !w.instancesEqual(lastInstances, instances) {
				w.watchCh <- instances
				lastInstances = instances
			}
		}
	}
}

// instancesEqual checks if two slices of instances are equal
func (w *ServiceWatcher) instancesEqual(a, b []*ServiceInstance) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]*ServiceInstance)
	for _, inst := range a {
		aMap[inst.ID] = inst
	}

	bMap := make(map[string]*ServiceInstance)
	for _, inst := range b {
		bMap[inst.ID] = inst
	}

	for id, aInst := range aMap {
		bInst, exists := bMap[id]
		if !exists || aInst.Address != bInst.Address || aInst.Port != bInst.Port {
			return false
		}
	}

	return true
}
