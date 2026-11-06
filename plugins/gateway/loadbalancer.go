package gateway

// LoadBalancerFactory creates load balancers using pkg/loadbalancer
type LoadBalancerFactory struct{}

// NewLoadBalancerFactory creates a new factory
func NewLoadBalancerFactory() *LoadBalancerFactory {
	return &LoadBalancerFactory{}
}

// Create creates a load balancer based on strategy using pkg/loadbalancer
func (f *LoadBalancerFactory) Create(strategy string) LoadBalancer {
	return NewLoadBalancerAdapter(strategy)
}

// Legacy types for backward compatibility
// These are deprecated and will be removed in future versions

// WeightedTarget represents a target with weight (deprecated)
// Use loadbalancer.Target instead
type WeightedTarget struct {
	Address string
	Weight  int
}

// ConnectionTarget represents a target with connection count (deprecated)
// Use loadbalancer.Target with ConnectionCount field instead
type ConnectionTarget struct {
	Address     string
	Connections int32
}

// Legacy constructors for backward compatibility
// These are deprecated and will be removed in future versions

// NewRoundRobinLoadBalancer creates a new round-robin load balancer (deprecated)
// Use loadbalancer.NewLoadBalancerFactory().Create(loadbalancer.StrategyRoundRobin) instead
func NewRoundRobinLoadBalancer() LoadBalancer {
	return NewLoadBalancerAdapter("round_robin")
}

// NewRandomLoadBalancer creates a new random load balancer (deprecated)
// Use loadbalancer.NewLoadBalancerFactory().Create(loadbalancer.StrategyRandom) instead
func NewRandomLoadBalancer() LoadBalancer {
	return NewLoadBalancerAdapter("random")
}

// NewWeightedLoadBalancer creates a new weighted load balancer (deprecated)
// Use loadbalancer.NewLoadBalancerFactory().Create(loadbalancer.StrategyWeighted) instead
func NewWeightedLoadBalancer() LoadBalancer {
	return NewLoadBalancerAdapter("weighted")
}

// NewLeastConnectionsLoadBalancer creates a new least connections load balancer (deprecated)
// Use loadbalancer.NewLoadBalancerFactory().Create(loadbalancer.StrategyLeastConnections) instead
func NewLeastConnectionsLoadBalancer() LoadBalancer {
	return NewLoadBalancerAdapter("least_connections")
}
