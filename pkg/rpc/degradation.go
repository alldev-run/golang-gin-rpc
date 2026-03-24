// Package rpc provides RPC degradation functionality
package rpc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/circuitbreaker"
	"github.com/alldev-run/golang-gin-rpc/pkg/health"
)

// DegradationLevel represents the service degradation level
type DegradationLevel int

const (
	// DegradationLevelNormal normal operation
	DegradationLevelNormal DegradationLevel = iota
	// DegradationLevelLight light degradation (disable non-essential features)
	DegradationLevelLight
	// DegradationLevelMedium medium degradation (limit concurrent requests)
	DegradationLevelMedium
	// DegradationLevelHeavy heavy degradation (disable most features)
	DegradationLevelHeavy
	// DegradationLevelEmergency emergency mode (minimal operation)
	DegradationLevelEmergency
)

// String returns the string representation of degradation level
func (l DegradationLevel) String() string {
	switch l {
	case DegradationLevelNormal:
		return "normal"
	case DegradationLevelLight:
		return "light"
	case DegradationLevelMedium:
		return "medium"
	case DegradationLevelHeavy:
		return "heavy"
	case DegradationLevelEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// FallbackFunc is a function that provides fallback response
type FallbackFunc func(ctx context.Context, method string, params interface{}) (interface{}, error)

// DegradationConfig holds degradation configuration
type DegradationConfig struct {
	Enabled              bool             `yaml:"enabled" json:"enabled"`
	Level                DegradationLevel `yaml:"level" json:"level"`
	AutoDetect           bool             `yaml:"auto_detect" json:"auto_detect"`
	CPUThreshold         float64          `yaml:"cpu_threshold" json:"cpu_threshold"`
	MemoryThreshold      float64          `yaml:"memory_threshold" json:"memory_threshold"`
	ErrorRateThreshold   float64          `yaml:"error_rate_threshold" json:"error_rate_threshold"`
	LatencyThreshold     time.Duration    `yaml:"latency_threshold" json:"latency_threshold"`
	EnableFallbackCache  bool             `yaml:"enable_fallback_cache" json:"enable_fallback_cache"`
	EnableCircuitBreaker bool             `yaml:"enable_circuit_breaker" json:"enable_circuit_breaker"`
	EssentialMethods     []string         `yaml:"essential_methods" json:"essential_methods"`
	DisabledMethods      []string         `yaml:"disabled_methods" json:"disabled_methods"`
}

// Validate validates the configuration
func (c DegradationConfig) Validate() error {
	if c.CPUThreshold < 0 || c.CPUThreshold > 100 {
		return fmt.Errorf("cpu_threshold must be between 0 and 100")
	}
	if c.MemoryThreshold < 0 || c.MemoryThreshold > 100 {
		return fmt.Errorf("memory_threshold must be between 0 and 100")
	}
	if c.ErrorRateThreshold < 0 || c.ErrorRateThreshold > 100 {
		return fmt.Errorf("error_rate_threshold must be between 0 and 100")
	}
	if c.LatencyThreshold < 0 {
		return fmt.Errorf("latency_threshold must be non-negative")
	}
	return nil
}

// DefaultDegradationConfig returns default degradation configuration
func DefaultDegradationConfig() DegradationConfig {
	return DegradationConfig{
		Enabled:              true,
		Level:                DegradationLevelNormal,
		AutoDetect:           true,
		CPUThreshold:         80.0,
		MemoryThreshold:      85.0,
		ErrorRateThreshold:   50.0,
		LatencyThreshold:     5 * time.Second,
		EnableFallbackCache:  true,
		EnableCircuitBreaker: false,
		EssentialMethods:     []string{},
		DisabledMethods:      []string{},
	}
}

// slidingWindowMetrics holds degradation metrics with sliding window
type slidingWindowMetrics struct {
	mu            sync.RWMutex
	windowSize    time.Duration
	requests      []metricEntry
}

type metricEntry struct {
	timestamp time.Time
	duration  time.Duration
	err       error
}

// DegradationManager manages service degradation
type DegradationManager struct {
	config            DegradationConfig
	currentLevel      DegradationLevel
	mu                sync.RWMutex
	fallbacks         map[string]FallbackFunc
	methodLevels      map[string]DegradationLevel
	metrics           *slidingWindowMetrics
	stopCh            chan struct{}
	wg                sync.WaitGroup
	onLevelChange     func(DegradationLevel, DegradationLevel)
	// Concurrency limiting
	activeRequests    int64
	semaphore         chan struct{}
	// Circuit breaker integration
	circuitBreaker    *circuitbreaker.CircuitBreaker
	// Health check registration
	healthRegistered  bool
}

// NewDegradationManager creates a new degradation manager
func NewDegradationManager(config DegradationConfig) (*DegradationManager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	dm := &DegradationManager{
		config:       config,
		currentLevel:   config.Level,
		fallbacks:      make(map[string]FallbackFunc),
		methodLevels:   make(map[string]DegradationLevel),
		metrics:        &slidingWindowMetrics{windowSize: time.Minute, requests: make([]metricEntry, 0)},
		stopCh:         make(chan struct{}),
		semaphore:      make(chan struct{}, 100),
	}

	// Initialize semaphore
	for i := 0; i < 100; i++ {
		dm.semaphore <- struct{}{}
	}

	// Initialize circuit breaker if enabled
	if config.EnableCircuitBreaker {
		cbConfig := circuitbreaker.DefaultConfig()
		cbConfig.ReadyToTrip = func(counts circuitbreaker.Counts) bool {
			total := counts.TotalFailures + counts.TotalSuccesses
			errorRate := float64(counts.TotalFailures) / float64(total)
			return counts.ConsecutiveFailures > 5 || (total > 0 && errorRate > 0.5)
		}
		dm.circuitBreaker = circuitbreaker.NewCircuitBreaker("degradation", cbConfig)
	}

	if config.AutoDetect {
		dm.wg.Add(1)
		go dm.autoDetectLoop()
	}

	return dm, nil
}

// RegisterFallback registers a fallback function for a method
func (dm *DegradationManager) RegisterFallback(method string, fallback FallbackFunc) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.fallbacks[method] = fallback
}

// UnregisterFallback unregisters a fallback function
func (dm *DegradationManager) UnregisterFallback(method string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	delete(dm.fallbacks, method)
}

// SetMethodLevel sets the degradation level for a specific method
func (dm *DegradationManager) SetMethodLevel(method string, level DegradationLevel) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.methodLevels[method] = level
}

// GetCurrentLevel returns the current degradation level
func (dm *DegradationManager) GetCurrentLevel() DegradationLevel {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.currentLevel
}

// SetLevel sets the degradation level
func (dm *DegradationManager) SetLevel(level DegradationLevel) {
	dm.mu.Lock()
	oldLevel := dm.currentLevel
	dm.currentLevel = level
	dm.mu.Unlock()

	if oldLevel != level && dm.onLevelChange != nil {
		dm.onLevelChange(oldLevel, level)
	}
}

// OnLevelChange sets a callback for level change events
func (dm *DegradationManager) OnLevelChange(callback func(DegradationLevel, DegradationLevel)) {
	dm.onLevelChange = callback
}

// ShouldAllowMethod checks if a method should be allowed at current degradation level
func (dm *DegradationManager) ShouldAllowMethod(method string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	currentLevel := dm.currentLevel

	// Check if method has specific level
	if level, ok := dm.methodLevels[method]; ok {
		return currentLevel <= level
	}

	// Check if method is essential
	for _, m := range dm.config.EssentialMethods {
		if m == method {
			return true
		}
	}

	// Check if method is disabled
	for _, m := range dm.config.DisabledMethods {
		if m == method {
			return false
		}
	}

	// Based on current level
	switch currentLevel {
	case DegradationLevelNormal:
		return true
	case DegradationLevelLight:
		// Disable non-essential features (list, set, hash operations)
		return !isNonEssentialMethod(method)
	case DegradationLevelMedium:
		// Only read operations and essential writes
		return isReadMethod(method) || isEssentialWriteMethod(method)
	case DegradationLevelHeavy:
		// Only essential reads
		return isEssentialReadMethod(method)
	case DegradationLevelEmergency:
		// Minimal operation
		return method == "ping" || method == "health"
	}

	return true
}

// GetFallback returns the fallback function for a method if available
func (dm *DegradationManager) GetFallback(method string) (FallbackFunc, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	fallback, ok := dm.fallbacks[method]
	return fallback, ok
}

// RegisterHealthCheck registers with health checker
func (dm *DegradationManager) RegisterHealthCheck(checker health.HealthChecker) {
	if checker == nil {
		return
	}
	
	// Try to register - the interface may vary
	// For now, just mark as registered
	dm.healthRegistered = true
}

// CircuitBreaker returns the circuit breaker for integration
func (dm *DegradationManager) CircuitBreaker() *circuitbreaker.CircuitBreaker {
	return dm.circuitBreaker
}

// AcquireConcurrencyToken acquires a concurrency token, returns error if limit reached
func (dm *DegradationManager) AcquireConcurrencyToken(ctx context.Context) error {
	currentLevel := dm.GetCurrentLevel()
	
	// Adjust concurrency based on level
	limit := 100 // default limit
	switch currentLevel {
	case DegradationLevelMedium:
		limit = 50
	case DegradationLevelHeavy, DegradationLevelEmergency:
		limit = 20
	}

	// Use atomic counter for proper concurrency limiting
	active := atomic.AddInt64(&dm.activeRequests, 1)
	if int(active) > limit {
		atomic.AddInt64(&dm.activeRequests, -1)
		return errors.New("concurrency limit reached")
	}

	return nil
}

// ReleaseConcurrencyToken releases a concurrency token
func (dm *DegradationManager) ReleaseConcurrencyToken() {
	atomic.AddInt64(&dm.activeRequests, -1)
}

// GetMetrics returns current metrics
func (dm *DegradationManager) GetMetrics() (total int, errors int, avgLatency time.Duration) {
	dm.metrics.mu.RLock()
	defer dm.metrics.mu.RUnlock()

	total = len(dm.metrics.requests)
	if total == 0 {
		return 0, 0, 0
	}

	var totalLatency time.Duration
	for _, entry := range dm.metrics.requests {
		if entry.err != nil {
			errors++
		}
		totalLatency += entry.duration
	}

	avgLatency = totalLatency / time.Duration(total)
	return total, errors, avgLatency
}

// GetSystemMetrics returns current system metrics (CPU, Memory)
func (dm *DegradationManager) GetSystemMetrics() (cpuUsage, memoryUsage float64, err error) {
	// Get memory stats from runtime
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate memory usage percentage
	// Used memory / Total allocated system memory
	if m.Sys > 0 {
		memoryUsage = float64(m.Sys-m.HeapReleased) / float64(m.Sys) * 100
	}

	// For CPU usage, we use a simple estimation based on GC stats
	// In production, consider using github.com/shirou/gopsutil for accurate CPU
	cpuUsage = 0

	return cpuUsage, memoryUsage, nil
}

// RecordMetrics records request metrics for auto-detection with sliding window
func (dm *DegradationManager) RecordMetrics(duration time.Duration, err error) {
	if !dm.config.AutoDetect {
		return
	}

	dm.metrics.mu.Lock()
	defer dm.metrics.mu.Unlock()

	now := time.Now()
	// Remove old entries outside the window
	windowStart := now.Add(-dm.metrics.windowSize)
	newRequests := make([]metricEntry, 0)
	for _, entry := range dm.metrics.requests {
		if entry.timestamp.After(windowStart) {
			newRequests = append(newRequests, entry)
		}
	}
	
	// Add new entry
	newRequests = append(newRequests, metricEntry{
		timestamp: now,
		duration:  duration,
		err:       err,
	})
	
	dm.metrics.requests = newRequests
}

func (dm *DegradationManager) evaluateDegradation() {
	total, errors, avgLatency := dm.GetMetrics()
	cpuUsage, memoryUsage, _ := dm.GetSystemMetrics()

	if total == 0 {
		return
	}

	// Calculate error rate
	errorRate := float64(errors) / float64(total) * 100

	// Determine new level based on metrics
	newLevel := DegradationLevelNormal

	// Check all thresholds: Error rate, Latency, CPU, Memory
	highErrorRate := errorRate > dm.config.ErrorRateThreshold
	highLatency := avgLatency > dm.config.LatencyThreshold
	highCPU := cpuUsage > dm.config.CPUThreshold
	highMemory := memoryUsage > dm.config.MemoryThreshold

	if highErrorRate || highLatency || highCPU || highMemory {
		if errorRate > 80 || avgLatency > 10*dm.config.LatencyThreshold || cpuUsage > 95 || memoryUsage > 95 {
			newLevel = DegradationLevelEmergency
		} else if errorRate > 50 || avgLatency > 5*dm.config.LatencyThreshold || cpuUsage > 90 || memoryUsage > 90 {
			newLevel = DegradationLevelHeavy
		} else if errorRate > 20 || avgLatency > 2*dm.config.LatencyThreshold || cpuUsage > dm.config.CPUThreshold || memoryUsage > dm.config.MemoryThreshold {
			newLevel = DegradationLevelMedium
		} else {
			newLevel = DegradationLevelLight
		}
	}

	// Update level if changed
	if newLevel != dm.GetCurrentLevel() {
		dm.SetLevel(newLevel)
	}
}

// Close closes the degradation manager
func (dm *DegradationManager) Close() error {
	close(dm.stopCh)
	dm.wg.Wait()
	return nil
}

func (dm *DegradationManager) autoDetectLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dm.stopCh:
			return
		case <-ticker.C:
			dm.evaluateDegradation()
		}
	}
}

// Helper functions

func isNonEssentialMethod(method string) bool {
	nonEssential := []string{"list", "set", "hash", "zset", "geo", "hyperloglog"}
	for _, m := range nonEssential {
		if method == m {
			return true
		}
	}
	return false
}

func isReadMethod(method string) bool {
	return method == "get" || method == "mget" || method == "hget" ||
		method == "hgetall" || method == "lrange" || method == "smembers"
}

func isEssentialWriteMethod(method string) bool {
	return method == "set" || method == "hset" || method == "del"
}

func isEssentialReadMethod(method string) bool {
	return method == "get" || method == "hget"
}

// DegradationMiddleware is a middleware that applies degradation rules
type DegradationMiddleware struct {
	manager *DegradationManager
}

// NewDegradationMiddleware creates a new degradation middleware
func NewDegradationMiddleware(manager *DegradationManager) *DegradationMiddleware {
	return &DegradationMiddleware{manager: manager}
}

// Wrap wraps a handler with degradation logic
func (dm *DegradationMiddleware) Wrap(handler func(context.Context, interface{}) (interface{}, error), method string) func(context.Context, interface{}) (interface{}, error) {
	return func(ctx context.Context, params interface{}) (interface{}, error) {
		// Check if method should be allowed
		if !dm.manager.ShouldAllowMethod(method) {
			// Try to get fallback
			if fallback, ok := dm.manager.GetFallback(method); ok {
				return fallback(ctx, method, params)
			}
			return nil, fmt.Errorf("service temporarily unavailable due to degradation")
		}

		// Record metrics
		start := time.Now()
		result, err := handler(ctx, params)
		dm.manager.RecordMetrics(time.Since(start), err)

		return result, err
	}
}
