
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/discovery"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger.Init(logger.Config{
		Level:   "info",
		Env:     "dev",
		LogPath: "./logs/discovery_example.log",
	})

	logger.Info("Starting Service Discovery Example")

	// Create discovery manager
	config := discovery.DefaultManagerConfig()
	config.Enabled = true
	config.RegistryType = "consul"
	config.RegistryAddress = "localhost:8500"
	config.ServiceName = "example-service"
	config.ServiceAddress = "localhost"
	config.ServicePort = 8080

	manager, err := discovery.NewServiceDiscoveryManager(config)
	if err != nil {
		log.Fatalf("Failed to create discovery manager: %v", err)
	}
	defer manager.Stop()

	// Start discovery manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start discovery manager: %v", err)
	}

	logger.Info("Discovery manager started successfully")

	// Create service selector
	selector := discovery.NewServiceSelector(manager, discovery.StrategyRoundRobin)

	// Register some example services
	go func() {
		time.Sleep(2 * time.Second) // Wait for discovery to start
		registerExampleServices(manager)
	}()

	// Start service watcher
	go func() {
		time.Sleep(3 * time.Second) // Wait for services to be registered
		watchServices(selector, manager)
	}()

	// Demo service discovery
	go func() {
		time.Sleep(5 * time.Second) // Wait for services to be available
		demoServiceDiscovery(selector, manager)
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	logger.Info("Shutting down service discovery example...")
}

func registerExampleServices(manager *discovery.ServiceDiscoveryManager) {
	logger.Info("Registering example services")

	// Register user service
	userService := &discovery.ServiceInstance{
		ID:      "user-service-1",
		Name:    "user-service",
		Address: "localhost",
		Port:    8081,
		Payload: map[string]string{
			"version": "1.0.0",
			"region":  "us-west-1",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.Register(ctx, userService); err != nil {
		logger.Errorf("Failed to register user service", zap.Error(err))
	} else {
		logger.Info("User service registered successfully")
	}

	// Register order service
	orderService := &discovery.ServiceInstance{
		ID:      "order-service-1",
		Name:    "order-service",
		Address: "localhost",
		Port:    8082,
		Payload: map[string]string{
			"version": "1.1.0",
			"region":  "us-west-2",
		},
	}

	if err := manager.Register(ctx, orderService); err != nil {
		logger.Errorf("Failed to register order service", zap.Error(err))
	} else {
		logger.Info("Order service registered successfully")
	}

	// Register payment service
	paymentService := &discovery.ServiceInstance{
		ID:      "payment-service-1",
		Name:    "payment-service",
		Address: "localhost",
		Port:    8083,
		Payload: map[string]string{
			"version": "2.0.0",
			"region":  "us-east-1",
		},
	}

	if err := manager.Register(ctx, paymentService); err != nil {
		logger.Errorf("Failed to register payment service", zap.Error(err))
	} else {
		logger.Info("Payment service registered successfully")
	}
}

func watchServices(selector *discovery.ServiceSelector, manager *discovery.ServiceDiscoveryManager) {
	logger.Info("Starting service watcher")

	ctx := context.Background()

	// Watch user service
	watcher := discovery.NewServiceWatcher(manager, "user-service")
	defer watcher.Stop()

	watchCh := watcher.Watch(ctx)

	for {
		select {
		case instances := <-watchCh:
			logger.Info("User service instances changed",
				zap.Int("count", len(instances)))
			for _, instance := range instances {
				logger.Info("User service instance",
					zap.String("id", instance.ID),
					zap.String("address", fmt.Sprintf("%s:%d", instance.Address, instance.Port)))
			}
		case <-time.After(30 * time.Second):
			logger.Info("Watch timeout, continuing...")
		}
	}
}

func demoServiceDiscovery(selector *discovery.ServiceSelector, manager *discovery.ServiceDiscoveryManager) {
	logger.Info("Demoing service discovery")

	ctx := context.Background()

	// Demo 1: Get user service instances
	logger.Info("=== Demo 1: Getting user service instances ===")
	instances, err := manager.GetService(ctx, "user-service")
	if err != nil {
		logger.Errorf("Failed to get user service", zap.Error(err))
	} else {
		logger.Info("User service instances found", zap.Int("count", len(instances)))
		for _, instance := range instances {
			logger.Info("Instance",
				zap.String("id", instance.ID),
				zap.String("address", fmt.Sprintf("%s:%d", instance.Address, instance.Port)),
				zap.Strings("payload_keys", getPayloadKeys(instance.Payload)))
		}
	}

	// Demo 2: Load balancing with different strategies
	logger.Info("=== Demo 2: Load balancing strategies ===")
	strategies := []discovery.LoadBalancerStrategy{
		discovery.StrategyRoundRobin,
		discovery.StrategyRandom,
		discovery.StrategyLeastConn,
		discovery.StrategyIPHash,
	}

	for _, strategy := range strategies {
		logger.Info("Testing strategy", zap.String("strategy", string(strategy)))
		selector.SetServiceStrategy("user-service", strategy)

		for i := 0; i < 5; i++ {
			instance, tracker, err := selector.SelectInstance(ctx, "user-service", "192.168.1.100")
			if err != nil {
				logger.Errorf("Failed to select instance", zap.Error(err))
				continue
			}

			logger.Info("Selected instance",
				zap.String("strategy", string(strategy)),
				zap.String("instance_id", instance.ID),
				zap.String("address", fmt.Sprintf("%s:%d", instance.Address, instance.Port)))

			// Simulate connection
			time.Sleep(100 * time.Millisecond)
			tracker.Close()
		}
	}

	// Demo 3: Service registry info
	logger.Info("=== Demo 3: Service registry information ===")
	registry := selector.GetRegistry()

	logger.Info("All registered services", zap.Strings("services", registry.ListServices()))

	for _, serviceName := range registry.ListServices() {
		info := registry.GetServiceInfo(serviceName)
		logger.Info("Service info",
			zap.String("service_name", serviceName),
			zap.Any("info", info))
	}

	// Demo 4: Health check simulation
	logger.Info("=== Demo 4: Health check simulation ===")
	// Since there's no dedicated health checker for discovery, we'll simulate one
	// In a real implementation, you would create a custom health checker
	logger.Info("Health check simulation - checking registered services")

	// Get all registered services for health simulation
	services, err := manager.GetAllServices(ctx)
	if err != nil {
		logger.Errorf("Failed to get services for health check", zap.Error(err))
	} else {
		logger.Info("Health status", zap.Any("services", services))
	}
}

func getPayloadKeys(payload map[string]string) []string {
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	return keys
}

// Example of creating a custom service with discovery integration
type DiscoveryService struct {
	name     string
	manager  *discovery.ServiceDiscoveryManager
	selector *discovery.ServiceSelector
	instance *discovery.ServiceInstance
	port     int
}

func NewDiscoveryService(name string, port int, manager *discovery.ServiceDiscoveryManager) *DiscoveryService {
	return &DiscoveryService{
		name:    name,
		manager: manager,
		port:    port,
	}
}

func (s *DiscoveryService) Start() error {
	// Register self with discovery
	s.instance = &discovery.ServiceInstance{
		ID:      fmt.Sprintf("%s-%d", s.name, time.Now().Unix()),
		Name:    s.name,
		Address: "localhost",
		Port:    8080 + s.port, // Avoid port conflicts
		Payload: map[string]string{
			"version":      "1.0.0",
			"started_at":   time.Now().Format(time.RFC3339),
			"service_type": "custom",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.manager.Register(ctx, s.instance); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Create service selector
	s.selector = discovery.NewServiceSelector(s.manager, discovery.StrategyRoundRobin)

	logger.Info("Custom service started and registered",
		zap.String("name", s.name),
		zap.String("id", s.instance.ID),
		zap.String("address", fmt.Sprintf("%s:%d", s.instance.Address, s.instance.Port)))

	return nil
}

func (s *DiscoveryService) Stop() error {
	if s.instance != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.manager.Deregister(ctx, s.instance); err != nil {
			return fmt.Errorf("failed to deregister service: %w", err)
		}

		logger.Info("Custom service stopped and deregistered",
			zap.String("name", s.name),
			zap.String("id", s.instance.ID))
	}

	return nil
}

func (s *DiscoveryService) CallOtherService(serviceName string) error {
	ctx := context.Background()

	instance, tracker, err := s.selector.SelectInstance(ctx, serviceName, "127.0.0.1")
	if err != nil {
		return fmt.Errorf("failed to select instance: %w", err)
	}
	defer tracker.Close()

	logger.Info("Calling other service",
		zap.String("service", serviceName),
		zap.String("instance", instance.ID),
		zap.String("address", fmt.Sprintf("%s:%d", instance.Address, instance.Port)))

	// In a real implementation, you would make an actual RPC/HTTP call here
	return nil
}
