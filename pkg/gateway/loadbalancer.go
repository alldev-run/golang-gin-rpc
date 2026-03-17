package gateway

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"alldev-gin-rpc/pkg/logger"
)

// LoadBalancerFactory creates load balancers
type LoadBalancerFactory struct{}

// NewLoadBalancerFactory creates a new factory
func NewLoadBalancerFactory() *LoadBalancerFactory {
	return &LoadBalancerFactory{}
}

// Create creates a load balancer based on strategy
func (f *LoadBalancerFactory) Create(strategy string) LoadBalancer {
	switch strategy {
	case "round_robin":
		return NewRoundRobinLoadBalancer()
	case "random":
		return NewRandomLoadBalancer()
	case "weighted":
		return NewWeightedLoadBalancer()
	case "least_connections":
		return NewLeastConnectionsLoadBalancer()
	default:
		logger.Warn("Unknown load balancer strategy, using round_robin", logger.String("strategy", strategy))
		return NewRoundRobinLoadBalancer()
	}
}

// RoundRobinLoadBalancer implements round-robin load balancing
type RoundRobinLoadBalancer struct {
	targets []string
	current uint64
	mu      sync.RWMutex
}

// NewRoundRobinLoadBalancer creates a new round-robin load balancer
func NewRoundRobinLoadBalancer() *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{}
}

// Select selects a target using round-robin
func (lb *RoundRobinLoadBalancer) Select(targets []string) (string, error) {
	lb.mu.RLock()
	currentTargets := lb.targets
	lb.mu.RUnlock()

	if len(currentTargets) == 0 {
		return "", ErrLoadBalancerFailed
	}

	// Use atomic counter for thread safety
	index := atomic.AddUint64(&lb.current, 1) - 1
	selected := currentTargets[index%uint64(len(currentTargets))]

	logger.Debug("RoundRobin selected target", logger.String("target", selected))
	return selected, nil
}

// UpdateTargets updates the target list
func (lb *RoundRobinLoadBalancer) UpdateTargets(targets []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.targets = make([]string, len(targets))
	copy(lb.targets, targets)
	
	logger.Debug("RoundRobin updated targets", logger.Any("targets", targets))
}

// RandomLoadBalancer implements random load balancing
type RandomLoadBalancer struct {
	targets []string
	mu      sync.RWMutex
	rand    *rand.Rand
}

// NewRandomLoadBalancer creates a new random load balancer
func NewRandomLoadBalancer() *RandomLoadBalancer {
	return &RandomLoadBalancer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select selects a target randomly
func (lb *RandomLoadBalancer) Select(targets []string) (string, error) {
	lb.mu.RLock()
	currentTargets := lb.targets
	lb.mu.RUnlock()

	if len(currentTargets) == 0 {
		return "", ErrLoadBalancerFailed
	}

	index := lb.rand.Intn(len(currentTargets))
	selected := currentTargets[index]

	logger.Debug("Random selected target", logger.String("target", selected))
	return selected, nil
}

// UpdateTargets updates the target list
func (lb *RandomLoadBalancer) UpdateTargets(targets []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.targets = make([]string, len(targets))
	copy(lb.targets, targets)
	
	logger.Debug("Random updated targets", logger.Any("targets", targets))
}

// WeightedLoadBalancer implements weighted load balancing
type WeightedLoadBalancer struct {
	targets []WeightedTarget
	mu      sync.RWMutex
	rand    *rand.Rand
}

// WeightedTarget represents a target with weight
type WeightedTarget struct {
	Address string
	Weight  int
}

// NewWeightedLoadBalancer creates a new weighted load balancer
func NewWeightedLoadBalancer() *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select selects a target based on weight
func (lb *WeightedLoadBalancer) Select(targets []string) (string, error) {
	lb.mu.RLock()
	currentTargets := lb.targets
	lb.mu.RUnlock()

	if len(currentTargets) == 0 {
		return "", ErrLoadBalancerFailed
	}

	// Calculate total weight
	totalWeight := 0
	for _, target := range currentTargets {
		totalWeight += target.Weight
	}

	if totalWeight == 0 {
		// If no weights, fall back to random
		index := lb.rand.Intn(len(currentTargets))
		return currentTargets[index].Address, nil
	}

	// Select based on weight
	random := lb.rand.Intn(totalWeight)
	currentWeight := 0

	for _, target := range currentTargets {
		currentWeight += target.Weight
		if random < currentWeight {
			logger.Debug("Weighted selected target", logger.String("target", target.Address), logger.Int("weight", target.Weight))
			return target.Address, nil
		}
	}

	// Fallback to first target
	selected := currentTargets[0].Address
	logger.Debug("Weighted fallback selected target", logger.String("target", selected))
	return selected, nil
}

// UpdateTargets updates the weighted target list
func (lb *WeightedLoadBalancer) UpdateTargets(targets []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	// Convert string targets to weighted targets (default weight = 1)
	lb.targets = make([]WeightedTarget, len(targets))
	for i, target := range targets {
		lb.targets[i] = WeightedTarget{
			Address: target,
			Weight:  1,
		}
	}
	
	logger.Debug("Weighted updated targets", logger.Any("targets", targets))
}

// SetWeights sets weights for targets
func (lb *WeightedLoadBalancer) SetWeights(weightedTargets []WeightedTarget) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.targets = make([]WeightedTarget, len(weightedTargets))
	copy(lb.targets, weightedTargets)
	
	logger.Debug("Weighted set weights", logger.Any("weights", weightedTargets))
}

// LeastConnectionsLoadBalancer implements least connections load balancing
type LeastConnectionsLoadBalancer struct {
	targets []ConnectionTarget
	mu      sync.RWMutex
}

// ConnectionTarget represents a target with connection count
type ConnectionTarget struct {
	Address     string
	Connections int32
}

// NewLeastConnectionsLoadBalancer creates a new least connections load balancer
func NewLeastConnectionsLoadBalancer() *LeastConnectionsLoadBalancer {
	return &LeastConnectionsLoadBalancer{}
}

// Select selects a target with least connections
func (lb *LeastConnectionsLoadBalancer) Select(targets []string) (string, error) {
	lb.mu.RLock()
	currentTargets := lb.targets
	lb.mu.RUnlock()

	if len(currentTargets) == 0 {
		return "", ErrLoadBalancerFailed
	}

	// Find target with least connections
	selected := currentTargets[0]
	minConnections := selected.Connections

	for _, target := range currentTargets {
		if target.Connections < minConnections {
			selected = target
			minConnections = target.Connections
		}
	}

	// Increment connection count
	atomic.AddInt32(&selected.Connections, 1)

	logger.Debug("LeastConnections selected target", logger.String("target", selected.Address), logger.Int("connections", int(selected.Connections)))
	return selected.Address, nil
}

// UpdateTargets updates the target list
func (lb *LeastConnectionsLoadBalancer) UpdateTargets(targets []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.targets = make([]ConnectionTarget, len(targets))
	for i, target := range targets {
		lb.targets[i] = ConnectionTarget{
			Address:     target,
			Connections: 0,
		}
	}
	
	logger.Debug("LeastConnections updated targets", logger.Any("targets", targets))
}

// ReleaseConnection releases a connection for a target
func (lb *LeastConnectionsLoadBalancer) ReleaseConnection(target string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	for i, t := range lb.targets {
		if t.Address == target {
			atomic.AddInt32(&lb.targets[i].Connections, -1)
			break
		}
	}
}

// GetConnectionCount returns connection count for a target
func (lb *LeastConnectionsLoadBalancer) GetConnectionCount(target string) int32 {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	for _, t := range lb.targets {
		if t.Address == target {
			return t.Connections
		}
	}
	return 0
}
