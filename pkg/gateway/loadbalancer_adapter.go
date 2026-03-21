package gateway

import (
	"context"
	"errors"
	"strings"
	"sync"

	"alldev-gin-rpc/pkg/loadbalancer"
	"alldev-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

// LoadBalancerAdapter adapts pkg/loadbalancer to gateway.LoadBalancer interface
type LoadBalancerAdapter struct {
	lb       loadbalancer.LoadBalancer
	targets  []string
	mu       sync.RWMutex
}

// NewLoadBalancerAdapter creates a new adapter
func NewLoadBalancerAdapter(strategy string) LoadBalancer {
	opts := []loadbalancer.Option{
		loadbalancer.WithLogger(&gatewayLoggerAdapter{}),
	}
	
	var lbStrategy loadbalancer.Strategy
	switch strings.ToLower(strategy) {
	case "round_robin":
		lbStrategy = loadbalancer.StrategyRoundRobin
	case "random":
		lbStrategy = loadbalancer.StrategyRandom
	case "weighted":
		lbStrategy = loadbalancer.StrategyWeighted
	case "least_connections":
		lbStrategy = loadbalancer.StrategyLeastConnections
	default:
		lbStrategy = loadbalancer.StrategyRoundRobin
	}
	
	factory := loadbalancer.NewLoadBalancerFactory(opts...)
	lb, err := factory.Create(lbStrategy)
	if err != nil {
		// Fallback to round robin
		lb, _ = factory.Create(loadbalancer.StrategyRoundRobin)
	}
	
	return &LoadBalancerAdapter{lb: lb}
}

// Select selects a target using the adapted load balancer
func (a *LoadBalancerAdapter) Select(targets []string) (string, error) {
	a.mu.RLock()
	currentTargets := a.targets
	a.mu.RUnlock()
	
	// If targets provided, update them
	if len(targets) > 0 {
		a.UpdateTargets(targets)
		currentTargets = targets
	}
	
	if len(currentTargets) == 0 {
		return "", ErrLoadBalancerFailed
	}
	
	// Convert string targets to loadbalancer.Target
	lbTargets := make([]*loadbalancer.Target, len(currentTargets))
	for i, target := range currentTargets {
		lbTargets[i] = loadbalancer.NewTarget(target)
	}
	
	// Update targets in the load balancer
	if err := a.lb.UpdateTargets(lbTargets); err != nil {
		return "", ErrLoadBalancerFailed
	}
	
	// Select target
	selected, err := a.lb.Select(context.Background(), nil)
	if err != nil {
		if errors.Is(err, loadbalancer.ErrNoTargetsAvailable) {
			return "", ErrLoadBalancerFailed
		}
		return "", err
	}
	
	return selected.Address, nil
}

// UpdateTargets updates the target list
func (a *LoadBalancerAdapter) UpdateTargets(targets []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if len(targets) == 0 {
		return
	}
	
	// Store targets for future use
	a.targets = make([]string, len(targets))
	copy(a.targets, targets)
	
	// Convert string targets to loadbalancer.Target
	lbTargets := make([]*loadbalancer.Target, len(targets))
	for i, target := range targets {
		lbTargets[i] = loadbalancer.NewTarget(target)
	}
	
	_ = a.lb.UpdateTargets(lbTargets)
}

// Close closes the load balancer
func (a *LoadBalancerAdapter) Close() error {
	return a.lb.Close()
}

// gatewayLoggerAdapter adapts gateway logger to loadbalancer.Logger
type gatewayLoggerAdapter struct{}

func (l *gatewayLoggerAdapter) Debug(msg string, fields ...loadbalancer.Field) {
	logger.Debug(msg, convertFields(fields)...)
}

func (l *gatewayLoggerAdapter) Info(msg string, fields ...loadbalancer.Field) {
	logger.Info(msg, convertFields(fields)...)
}

func (l *gatewayLoggerAdapter) Warn(msg string, fields ...loadbalancer.Field) {
	logger.Warn(msg, convertFields(fields)...)
}

func (l *gatewayLoggerAdapter) Error(msg string, fields ...loadbalancer.Field) {
	logger.Errorf(msg, convertFields(fields)...)
}

func convertFields(lbFields []loadbalancer.Field) []zap.Field {
	fields := make([]zap.Field, len(lbFields))
	for i, field := range lbFields {
		fields[i] = zap.String(field.Key, toString(field.Value))
	}
	return fields
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
