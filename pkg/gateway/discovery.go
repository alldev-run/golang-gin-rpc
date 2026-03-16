package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang-gin-rpc/pkg/discovery"
	"golang-gin-rpc/pkg/logger"
)

// ServiceDiscovery integrates with existing discovery package
type ServiceDiscovery struct {
	discovery discovery.Discovery
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// ServiceInfo holds service information
type ServiceInfo struct {
	Name      string
	Instances []ServiceInstance
	LastSync  time.Time
}

// ServiceInstance represents a service instance
type ServiceInstance struct {
	ID       string
	Address  string
	Port     int
	Metadata map[string]string
	Healthy  bool
}

// NewServiceDiscovery creates a new service discovery instance using existing discovery package
func NewServiceDiscovery(config DiscoveryConfig) (*ServiceDiscovery, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create discovery instance using existing factory
	discoveryConfig := discovery.Config{
		Type:    config.Type,
		Addr:    config.Endpoints[0], // Use first endpoint for now
		Timeout: config.Timeout,
	}
	
	if config.Type == "static" || len(config.Endpoints) == 0 {
		// For static discovery, return a mock implementation
		return &ServiceDiscovery{
			ctx:    ctx,
			cancel: cancel,
		}, nil
	}
	
	disc, err := discovery.NewDiscovery(discoveryConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create discovery: %w", err)
	}
	
	return &ServiceDiscovery{
		discovery: disc,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Initialize initializes the service discovery
func (sd *ServiceDiscovery) Initialize() error {
	if sd.discovery != nil {
		logger.Info("Service discovery initialized",
			logger.String("type", "dynamic"))
	} else {
		logger.Info("Service discovery initialized",
			logger.String("type", "static"))
	}
	return nil
}

// GetServiceEndpoints returns endpoints for a service using existing discovery
func (sd *ServiceDiscovery) GetServiceEndpoints(serviceName string) ([]string, error) {
	if sd.discovery == nil {
		// Static discovery - return mock endpoints
		return sd.getStaticEndpoints(serviceName)
	}
	
	// Use existing discovery
	instances, err := sd.discovery.GetService(context.Background(), serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}
	
	endpoints := make([]string, 0, len(instances))
	for _, instance := range instances {
		endpoint := fmt.Sprintf("http://%s:%d", instance.Address, instance.Port)
		endpoints = append(endpoints, endpoint)
	}
	
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no instances found for service: %s", serviceName)
	}
	
	return endpoints, nil
}

// Start starts the service discovery
func (sd *ServiceDiscovery) Start() error {
	// Start background sync
	go sd.syncLoop()
	
	discoveryType := "static"
	if sd.discovery != nil {
		discoveryType = "dynamic"
	}
	
	logger.Info("Service discovery started",
		logger.String("type", discoveryType))
	
	return nil
}

// Stop stops the service discovery
func (sd *ServiceDiscovery) Stop() error {
	sd.cancel()
	
	if sd.discovery != nil {
		// Existing discovery doesn't have explicit close method
		// Just log the shutdown
		logger.Info("Dynamic service discovery stopped")
	} else {
		logger.Info("Static service discovery stopped")
	}
	
	return nil
}

// getStaticEndpoints returns static endpoints for testing/demo
func (sd *ServiceDiscovery) getStaticEndpoints(serviceName string) ([]string, error) {
	switch serviceName {
	case "user-service":
		return []string{"http://localhost:8001"}, nil
	case "order-service":
		return []string{"http://localhost:8002"}, nil
	case "health-service":
		return []string{"http://localhost:8003"}, nil
	default:
		return nil, fmt.Errorf("unknown static service: %s", serviceName)
	}
}


// syncLoop runs the background sync loop
func (sd *ServiceDiscovery) syncLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sd.ctx.Done():
			return
		case <-ticker.C:
			sd.syncServices()
		}
	}
}

// syncServices synchronizes services from discovery
func (sd *ServiceDiscovery) syncServices() {
	// With existing discovery package, services are fetched on-demand
	// No need to maintain local cache
	logger.Debug("Service discovery sync completed")
}

// RegisterService registers a service (for testing)
func (sd *ServiceDiscovery) RegisterService(name string, instances []ServiceInstance) error {
	if sd.discovery == nil {
		// Static discovery - just log
		logger.Info("Static registration for service", 
			logger.String("service", name),
			logger.Int("instances", len(instances)))
		return nil
	}
	
	// Register with existing discovery
	for _, instance := range instances {
		discInstance := &discovery.ServiceInstance{
			ID:      instance.ID,
			Name:    name,
			Address: instance.Address,
			Port:    instance.Port,
			Payload: instance.Metadata,
		}
		
		if err := sd.discovery.Register(context.Background(), discInstance); err != nil {
			return fmt.Errorf("failed to register instance %s: %w", instance.ID, err)
		}
	}
	
	logger.Info("Registered service", 
		logger.String("service", name),
		logger.Int("instances", len(instances)))
	return nil
}

