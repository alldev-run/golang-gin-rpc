package loadbalancer

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// WeightedLoadBalancer implements weighted load balancing
type WeightedLoadBalancer struct {
	*BaseLoadBalancer
	rand *RandomGenerator
}

// NewWeightedLoadBalancer creates a new weighted load balancer
func NewWeightedLoadBalancer(opts *Options) *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		BaseLoadBalancer: NewBaseLoadBalancer(opts),
		rand:            NewRandomGenerator(),
	}
}

// Select selects a target based on weight
func (lb *WeightedLoadBalancer) Select(ctx context.Context, targets []*Target) (*Target, error) {
	targets = lb.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoTargetsAvailable
	}
	
	// Calculate total weight
	totalWeight := 0
	for _, target := range targets {
		if target.Weight > 0 {
			totalWeight += target.Weight
		}
	}
	
	if totalWeight == 0 {
		// If no valid weights, fall back to random
		index := lb.rand.Intn(len(targets))
		selected := targets[index]
		
		lb.opts.Logger.Debug("Weighted fallback to random", 
			Field{Key: "target", Value: selected.Address})
		
		return selected, nil
	}
	
	// Select based on weight
	random := lb.rand.Intn(totalWeight)
	currentWeight := 0
	
	for _, target := range targets {
		if target.Weight <= 0 {
			continue
		}
		currentWeight += target.Weight
		if random < currentWeight {
			lb.opts.Logger.Debug("Weighted selected target", 
				Field{Key: "target", Value: target.Address},
				Field{Key: "weight", Value: target.Weight})
			return target, nil
		}
	}
	
	// Fallback to first target
	selected := targets[0]
	lb.opts.Logger.Debug("Weighted fallback to first target", 
		Field{Key: "target", Value: selected.Address})
	
	return selected, nil
}

// LeastConnectionsLoadBalancer implements least connections load balancing
type LeastConnectionsLoadBalancer struct {
	*BaseLoadBalancer
}

// NewLeastConnectionsLoadBalancer creates a new least connections load balancer
func NewLeastConnectionsLoadBalancer(opts *Options) *LeastConnectionsLoadBalancer {
	return &LeastConnectionsLoadBalancer{
		BaseLoadBalancer: NewBaseLoadBalancer(opts),
	}
}

// Select selects a target with least connections
func (lb *LeastConnectionsLoadBalancer) Select(ctx context.Context, targets []*Target) (*Target, error) {
	targets = lb.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoTargetsAvailable
	}
	
	// Find target with least connections
	selected := targets[0]
	minConnections := atomic.LoadInt32(&selected.ConnectionCount)
	
	for _, target := range targets[1:] {
		connections := atomic.LoadInt32(&target.ConnectionCount)
		if connections < minConnections {
			selected = target
			minConnections = connections
		}
	}
	
	// Increment connection count
	atomic.AddInt32(&selected.ConnectionCount, 1)
	
	lb.opts.Logger.Debug("LeastConnections selected target", 
		Field{Key: "target", Value: selected.Address},
		Field{Key: "connections", Value: minConnections + 1})
	
	return selected, nil
}

// ReleaseConnection releases a connection for a target
func (lb *LeastConnectionsLoadBalancer) ReleaseConnection(targetAddress string) error {
	targets := lb.GetTargets()
	
	for _, target := range targets {
		if target.Address == targetAddress {
			atomic.AddInt32(&target.ConnectionCount, -1)
			
			lb.opts.Logger.Debug("Released connection", 
				Field{Key: "target", Value: targetAddress})
			
			return nil
		}
	}
	
	return ErrTargetNotFound
}

// GetConnectionCount returns connection count for a target
func (lb *LeastConnectionsLoadBalancer) GetConnectionCount(targetAddress string) int32 {
	targets := lb.GetTargets()
	
	for _, target := range targets {
		if target.Address == targetAddress {
			return atomic.LoadInt32(&target.ConnectionCount)
		}
	}
	
	return 0
}

// RandomGenerator provides thread-safe random number generation
type RandomGenerator struct {
	mu   sync.Mutex
	rand *rand.Rand
}

// NewRandomGenerator creates a new thread-safe random generator
func NewRandomGenerator() *RandomGenerator {
	return &RandomGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Intn returns a random integer in [0, n)
func (r *RandomGenerator) Intn(n int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Intn(n)
}

// Int63 returns a random int64
func (r *RandomGenerator) Int63() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Int63()
}
