// Package discovery provides load balancing functionality
package discovery

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
)

// LoadBalancerStrategy represents load balancing strategy
type LoadBalancerStrategy string

const (
	StrategyRoundRobin     LoadBalancerStrategy = "round_robin"
	StrategyRandom         LoadBalancerStrategy = "random"
	StrategyWeightedRandom LoadBalancerStrategy = "weighted_random"
	StrategyLeastConn      LoadBalancerStrategy = "least_conn"
	StrategyIPHash         LoadBalancerStrategy = "ip_hash"
)

// LoadBalancer provides load balancing for service instances
type LoadBalancer struct {
	strategy    LoadBalancerStrategy
	instances   []*ServiceInstance
	currentIndex int
	mu          sync.RWMutex
	connections map[string]int // Track connections for least_conn strategy
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy LoadBalancerStrategy) *LoadBalancer {
	return &LoadBalancer{
		strategy:    strategy,
		instances:   make([]*ServiceInstance, 0),
		currentIndex: 0,
		connections: make(map[string]int),
	}
}

// UpdateInstances updates the service instances
func (lb *LoadBalancer) UpdateInstances(instances []*ServiceInstance) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.instances = instances
	
	// Reset connection tracking for new instances
	for _, inst := range instances {
		if _, exists := lb.connections[inst.ID]; !exists {
			lb.connections[inst.ID] = 0
		}
	}
	
	// Remove connection tracking for removed instances
	for id := range lb.connections {
		found := false
		for _, inst := range instances {
			if inst.ID == id {
				found = true
				break
			}
		}
		if !found {
			delete(lb.connections, id)
		}
	}
}

// NextInstance selects the next instance based on the load balancing strategy
func (lb *LoadBalancer) NextInstance(ctx context.Context, clientIP string) (*ServiceInstance, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.instances) == 0 {
		return nil, fmt.Errorf("no available instances")
	}

	switch lb.strategy {
	case StrategyRoundRobin:
		return lb.roundRobin(), nil
	case StrategyRandom:
		return lb.random(), nil
	case StrategyWeightedRandom:
		return lb.weightedRandom(), nil
	case StrategyLeastConn:
		return lb.leastConn(), nil
	case StrategyIPHash:
		return lb.ipHash(clientIP), nil
	default:
		return lb.roundRobin(), nil
	}
}

// roundRobin implements round-robin load balancing
func (lb *LoadBalancer) roundRobin() *ServiceInstance {
	instance := lb.instances[lb.currentIndex]
	lb.currentIndex = (lb.currentIndex + 1) % len(lb.instances)
	return instance
}

// random implements random load balancing
func (lb *LoadBalancer) random() *ServiceInstance {
	return lb.instances[rand.Intn(len(lb.instances))]
}

// weightedRandom implements weighted random load balancing
func (lb *LoadBalancer) weightedRandom() *ServiceInstance {
	// For now, use equal weights. In a real implementation, you would parse weights from metadata
	totalWeight := len(lb.instances)
	if totalWeight == 0 {
		return lb.instances[0]
	}

	target := rand.Intn(totalWeight)
	return lb.instances[target]
}

// leastConn implements least connections load balancing
func (lb *LoadBalancer) leastConn() *ServiceInstance {
	var selected *ServiceInstance
	minConnections := int(^uint(0) >> 1) // Max int

	for _, instance := range lb.instances {
		connections := lb.connections[instance.ID]
		if connections < minConnections {
			minConnections = connections
			selected = instance
		}
	}

	return selected
}

// ipHash implements IP hash load balancing
func (lb *LoadBalancer) ipHash(clientIP string) *ServiceInstance {
	if clientIP == "" {
		return lb.instances[0]
	}

	// Simple hash function
	hash := 0
	for _, c := range clientIP {
		hash = hash*31 + int(c)
	}

	if hash < 0 {
		hash = -hash
	}

	index := hash % len(lb.instances)
	return lb.instances[index]
}

// IncrementConnections increments the connection count for an instance
func (lb *LoadBalancer) IncrementConnections(instanceID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.connections[instanceID]++
}

// DecrementConnections decrements the connection count for an instance
func (lb *LoadBalancer) DecrementConnections(instanceID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	if lb.connections[instanceID] > 0 {
		lb.connections[instanceID]--
	}
}

// GetConnections returns the connection count for an instance
func (lb *LoadBalancer) GetConnections(instanceID string) int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.connections[instanceID]
}

// GetStrategy returns the current load balancing strategy
func (lb *LoadBalancer) GetStrategy() LoadBalancerStrategy {
	return lb.strategy
}

// SetStrategy changes the load balancing strategy
func (lb *LoadBalancer) SetStrategy(strategy LoadBalancerStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
	lb.currentIndex = 0
}

// GetInstances returns all instances
func (lb *LoadBalancer) GetInstances() []*ServiceInstance {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	instances := make([]*ServiceInstance, len(lb.instances))
	copy(instances, lb.instances)
	return instances
}

// ServiceRegistry manages multiple load balancers for different services
type ServiceRegistry struct {
	loadBalancers map[string]*LoadBalancer
	mu            sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		loadBalancers: make(map[string]*LoadBalancer),
	}
}

// RegisterService registers a service with a load balancer
func (sr *ServiceRegistry) RegisterService(serviceName string, strategy LoadBalancerStrategy) *LoadBalancer {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	lb := NewLoadBalancer(strategy)
	sr.loadBalancers[serviceName] = lb
	return lb
}

// GetLoadBalancer returns the load balancer for a service
func (sr *ServiceRegistry) GetLoadBalancer(serviceName string) (*LoadBalancer, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	lb, exists := sr.loadBalancers[serviceName]
	return lb, exists
}

// UpdateServiceInstances updates instances for a service
func (sr *ServiceRegistry) UpdateServiceInstances(serviceName string, instances []*ServiceInstance) {
	sr.mu.RLock()
	lb, exists := sr.loadBalancers[serviceName]
	sr.mu.RUnlock()

	if exists {
		lb.UpdateInstances(instances)
	}
}

// RemoveService removes a service from the registry
func (sr *ServiceRegistry) RemoveService(serviceName string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.loadBalancers, serviceName)
}

// ListServices returns all registered service names
func (sr *ServiceRegistry) ListServices() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	services := make([]string, 0, len(sr.loadBalancers))
	for serviceName := range sr.loadBalancers {
		services = append(services, serviceName)
	}
	sort.Strings(services)
	return services
}

// GetServiceInfo returns information about a service
func (sr *ServiceRegistry) GetServiceInfo(serviceName string) map[string]interface{} {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	lb, exists := sr.loadBalancers[serviceName]
	if !exists {
		return nil
	}

	instances := lb.GetInstances()
	connections := make(map[string]int)
	for _, instance := range instances {
		connections[instance.ID] = lb.GetConnections(instance.ID)
	}

	return map[string]interface{}{
		"service_name":  serviceName,
		"strategy":      string(lb.GetStrategy()),
		"instances":     len(instances),
		"connections":   connections,
		"load_balancer": lb,
	}
}

// GetAllServiceInfo returns information about all services
func (sr *ServiceRegistry) GetAllServiceInfo() map[string]map[string]interface{} {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	info := make(map[string]map[string]interface{})
	for serviceName := range sr.loadBalancers {
		info[serviceName] = sr.GetServiceInfo(serviceName)
	}
	return info
}

// ConnectionTracker tracks connections for load balancing
type ConnectionTracker struct {
	lb         *LoadBalancer
	instanceID string
}

// NewConnectionTracker creates a new connection tracker
func NewConnectionTracker(lb *LoadBalancer, instanceID string) *ConnectionTracker {
	lb.IncrementConnections(instanceID)
	return &ConnectionTracker{
		lb:         lb,
		instanceID: instanceID,
	}
}

// Close closes the connection tracker and decrements the connection count
func (ct *ConnectionTracker) Close() {
	ct.lb.DecrementConnections(ct.instanceID)
}

// ServiceSelector provides intelligent service selection
type ServiceSelector struct {
	registry   *ServiceRegistry
	manager    *ServiceDiscoveryManager
	defaultStrategy LoadBalancerStrategy
}

// NewServiceSelector creates a new service selector
func NewServiceSelector(manager *ServiceDiscoveryManager, defaultStrategy LoadBalancerStrategy) *ServiceSelector {
	return &ServiceSelector{
		registry:        NewServiceRegistry(),
		manager:         manager,
		defaultStrategy: defaultStrategy,
	}
}

// SelectInstance selects an instance for a service
func (ss *ServiceSelector) SelectInstance(ctx context.Context, serviceName, clientIP string) (*ServiceInstance, *ConnectionTracker, error) {
	// Get load balancer for the service
	lb, exists := ss.registry.GetLoadBalancer(serviceName)
	if !exists {
		// Create a new load balancer with default strategy
		lb = ss.registry.RegisterService(serviceName, ss.defaultStrategy)
		
		// Get initial instances
		instances, err := ss.manager.GetService(ctx, serviceName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get service instances: %w", err)
		}
		lb.UpdateInstances(instances)
	}

	// Select instance using load balancer
	instance, err := lb.NextInstance(ctx, clientIP)
	if err != nil {
		return nil, nil, err
	}

	// Create connection tracker
	tracker := NewConnectionTracker(lb, instance.ID)

	return instance, tracker, nil
}

// UpdateService updates the instances for a service
func (ss *ServiceSelector) UpdateService(ctx context.Context, serviceName string) error {
	instances, err := ss.manager.GetService(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to get service instances: %w", err)
	}

	ss.registry.UpdateServiceInstances(serviceName, instances)
	return nil
}

// SetServiceStrategy sets the load balancing strategy for a service
func (ss *ServiceSelector) SetServiceStrategy(serviceName string, strategy LoadBalancerStrategy) {
	lb, exists := ss.registry.GetLoadBalancer(serviceName)
	if !exists {
		lb = ss.registry.RegisterService(serviceName, strategy)
	} else {
		lb.SetStrategy(strategy)
	}
}

// GetRegistry returns the service registry
func (ss *ServiceSelector) GetRegistry() *ServiceRegistry {
	return ss.registry
}
