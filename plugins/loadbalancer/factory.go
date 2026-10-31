package loadbalancer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancerFactory creates load balancers
type LoadBalancerFactory struct {
	opts *Options
}

// NewLoadBalancerFactory creates a new factory with default options
func NewLoadBalancerFactory(opts ...Option) *LoadBalancerFactory {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return &LoadBalancerFactory{opts: options}
}

// Create creates a load balancer based on strategy
func (f *LoadBalancerFactory) Create(strategy Strategy) (LoadBalancer, error) {
	switch strategy {
	case StrategyRoundRobin:
		return NewRoundRobinLoadBalancer(f.opts), nil
	case StrategyRandom:
		return NewRandomLoadBalancer(f.opts), nil
	case StrategyWeighted:
		return NewWeightedLoadBalancer(f.opts), nil
	case StrategyLeastConnections:
		return NewLeastConnectionsLoadBalancer(f.opts), nil
	default:
		return nil, errors.New("unknown load balancer strategy: " + string(strategy))
	}
}

// Option configures load balancer options
type Option func(*Options)

// WithStrategy sets the load balancing strategy
func WithStrategy(strategy Strategy) Option {
	return func(opts *Options) {
		opts.Strategy = strategy
	}
}

// WithMetrics enables metrics collection
func WithMetrics(enable bool) Option {
	return func(opts *Options) {
		opts.EnableMetrics = enable
	}
}

// WithHealthCheck enables health checking
func WithHealthCheck(enable bool) Option {
	return func(opts *Options) {
		opts.EnableHealthCheck = enable
	}
}

// WithUpdateInterval sets the update interval
func WithUpdateInterval(interval time.Duration) Option {
	return func(opts *Options) {
		opts.UpdateInterval = interval
	}
}

// WithLogger sets the logger
func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

// BaseLoadBalancer provides common functionality for all load balancers
type BaseLoadBalancer struct {
	targets []*Target
	mu      sync.RWMutex
	opts    *Options
	closed  bool
}

// NewBaseLoadBalancer creates a new base load balancer
func NewBaseLoadBalancer(opts *Options) *BaseLoadBalancer {
	return &BaseLoadBalancer{
		targets: make([]*Target, 0),
		opts:    opts,
	}
}

// UpdateTargets updates the target list
func (lb *BaseLoadBalancer) UpdateTargets(targets []*Target) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	if lb.closed {
		return ErrLoadBalancerFailed
	}
	
	// Create a copy of targets
	lb.targets = make([]*Target, len(targets))
	copy(lb.targets, targets)
	
	lb.opts.Logger.Debug("Load balancer targets updated", Field{Key: "count", Value: len(targets)})
	return nil
}

// GetTargets returns the current target list
func (lb *BaseLoadBalancer) GetTargets() []*Target {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	// Return a copy of targets
	targets := make([]*Target, len(lb.targets))
	copy(targets, lb.targets)
	return targets
}

// Close gracefully closes the load balancer
func (lb *BaseLoadBalancer) Close() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.closed = true
	lb.targets = nil
	return nil
}

// GetHealthyTargets returns only healthy targets
func (lb *BaseLoadBalancer) GetHealthyTargets() []*Target {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	if !lb.opts.EnableHealthCheck {
		// If health check is disabled, return all targets
		targets := make([]*Target, len(lb.targets))
		copy(targets, lb.targets)
		return targets
	}
	
	var healthyTargets []*Target
	for _, target := range lb.targets {
		if target.Healthy {
			healthyTargets = append(healthyTargets, target)
		}
	}
	return healthyTargets
}

// RoundRobinLoadBalancer implements round-robin load balancing
type RoundRobinLoadBalancer struct {
	*BaseLoadBalancer
	current uint64
}

// NewRoundRobinLoadBalancer creates a new round-robin load balancer
func NewRoundRobinLoadBalancer(opts *Options) *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{
		BaseLoadBalancer: NewBaseLoadBalancer(opts),
	}
}

// Select selects a target using round-robin
func (lb *RoundRobinLoadBalancer) Select(ctx context.Context, targets []*Target) (*Target, error) {
	targets = lb.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoTargetsAvailable
	}
	
	// Use atomic counter for thread safety
	index := atomic.AddUint64(&lb.current, 1) - 1
	selected := targets[index%uint64(len(targets))]
	
	lb.opts.Logger.Debug("RoundRobin selected target", 
		Field{Key: "target", Value: selected.Address},
		Field{Key: "index", Value: index})
	
	return selected, nil
}

// RandomLoadBalancer implements random load balancing
type RandomLoadBalancer struct {
	*BaseLoadBalancer
	rand *RandomGenerator
}

// NewRandomLoadBalancer creates a new random load balancer
func NewRandomLoadBalancer(opts *Options) *RandomLoadBalancer {
	return &RandomLoadBalancer{
		BaseLoadBalancer: NewBaseLoadBalancer(opts),
		rand:            NewRandomGenerator(),
	}
}

// Select selects a target randomly
func (lb *RandomLoadBalancer) Select(ctx context.Context, targets []*Target) (*Target, error) {
	targets = lb.GetHealthyTargets()
	if len(targets) == 0 {
		return nil, ErrNoTargetsAvailable
	}
	
	index := lb.rand.Intn(len(targets))
	selected := targets[index]
	
	lb.opts.Logger.Debug("Random selected target", 
		Field{Key: "target", Value: selected.Address},
		Field{Key: "index", Value: index})
	
	return selected, nil
}
