package loadbalancer

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLoadBalancerFailed = errors.New("load balancer failed")
	ErrNoTargetsAvailable = errors.New("no targets available")
	ErrTargetNotFound     = errors.New("target not found")
)

// Target represents a load balancer target with metadata
type Target struct {
	// Address is the target address (e.g., "http://server:8080")
	Address string
	
	// Weight for weighted load balancing (default: 1)
	Weight int
	
	// Healthy indicates if the target is healthy (default: true)
	Healthy bool
	
	// ConnectionCount for least connections load balancing
	ConnectionCount int32
	
	// Metadata contains additional target information
	Metadata map[string]any
	
	// LastUpdated timestamp
	LastUpdated time.Time
}

// NewTarget creates a new target with default values
func NewTarget(address string) *Target {
	return &Target{
		Address:    address,
		Weight:     1,
		Healthy:    true,
		Metadata:   make(map[string]any),
		LastUpdated: time.Now(),
	}
}

// SetWeight sets the target weight
func (t *Target) SetWeight(weight int) {
	t.Weight = weight
	t.LastUpdated = time.Now()
}

// SetHealthy sets the target health status
func (t *Target) SetHealthy(healthy bool) {
	t.Healthy = healthy
	t.LastUpdated = time.Now()
}

// SetMetadata sets metadata key-value pair
func (t *Target) SetMetadata(key string, value any) {
	if t.Metadata == nil {
		t.Metadata = make(map[string]any)
	}
	t.Metadata[key] = value
	t.LastUpdated = time.Now()
}

// LoadBalancer interface for different load balancing strategies
type LoadBalancer interface {
	// Select selects a target based on the load balancing strategy
	Select(ctx context.Context, targets []*Target) (*Target, error)
	
	// UpdateTargets updates the target list
	UpdateTargets(targets []*Target) error
	
	// GetTargets returns the current target list
	GetTargets() []*Target
	
	// Close gracefully closes the load balancer
	Close() error
}

// Strategy represents load balancing strategy type
type Strategy string

const (
	StrategyRoundRobin     Strategy = "round_robin"
	StrategyRandom         Strategy = "random"
	StrategyWeighted       Strategy = "weighted"
	StrategyLeastConnections Strategy = "least_connections"
)

// Options configures load balancer behavior
type Options struct {
	// Strategy is the load balancing strategy
	Strategy Strategy
	
	// EnableMetrics enables metrics collection
	EnableMetrics bool
	
	// EnableHealthCheck enables health status checking
	EnableHealthCheck bool
	
	// UpdateInterval for background updates
	UpdateInterval time.Duration
	
	// Logger for logging
	Logger Logger
}

// Logger interface for load balancer logging
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// Field represents a log field
type Field struct {
	Key   string
	Value any
}

// DefaultOptions returns default load balancer options
func DefaultOptions() *Options {
	return &Options{
		Strategy:          StrategyRoundRobin,
		EnableMetrics:      false,
		EnableHealthCheck:  true,
		UpdateInterval:    30 * time.Second,
		Logger:            &NoopLogger{},
	}
}

// NoopLogger is a no-operation logger
type NoopLogger struct{}

func (l *NoopLogger) Debug(msg string, fields ...Field) {}
func (l *NoopLogger) Info(msg string, fields ...Field)  {}
func (l *NoopLogger) Warn(msg string, fields ...Field)  {}
func (l *NoopLogger) Error(msg string, fields ...Field) {}
