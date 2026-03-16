package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang-gin-rpc/pkg/cache"
)

// HealthStatus represents the health status
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnknown  HealthStatus = "unknown"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	Name         string                 `json:"name"`
	Status       HealthStatus           `json:"status"`
	Message      string                 `json:"message,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Timestamp    time.Time              `json:"timestamp"`
	LastChecked  time.Time              `json:"last_checked"`
}

// HealthReport represents the overall health report
type HealthReport struct {
	Status    HealthStatus            `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Version   string                  `json:"version,omitempty"`
	Checks    map[string]*CheckResult `json:"checks"`
	Summary   Summary                 `json:"summary"`
}

// Summary provides a summary of health checks
type Summary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded int `json:"degraded"`
	Unknown  int `json:"unknown"`
}

// HealthChecker defines the interface for health checkers
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) *CheckResult
}

// HealthCheckConfig holds configuration for health checks
type HealthCheckConfig struct {
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`
	Interval        time.Duration `yaml:"interval" json:"interval"`
	FailureThreshold int          `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold int          `json:"success_threshold" json:"success_threshold"`
	Enabled         bool          `yaml:"enabled" json:"enabled"`
}

// DefaultHealthCheckConfig returns default configuration
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Timeout:          5 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
	}
}

// HealthManager manages health checks
type HealthManager struct {
	checkers map[string]HealthChecker
	configs  map[string]HealthCheckConfig
	results  map[string]*CheckResult
	mu       sync.RWMutex
	
	// Background checker
	ticker  *time.Ticker
	stopCh  chan struct{}
	running bool
}

// NewHealthManager creates a new health manager
func NewHealthManager() *HealthManager {
	return &HealthManager{
		checkers: make(map[string]HealthChecker),
		configs:  make(map[string]HealthCheckConfig),
		results:  make(map[string]*CheckResult),
		stopCh:   make(chan struct{}),
	}
}

// RegisterChecker registers a health checker
func (hm *HealthManager) RegisterChecker(checker HealthChecker, config HealthCheckConfig) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	hm.checkers[checker.Name()] = checker
	hm.configs[checker.Name()] = config
}

// UnregisterChecker unregisters a health checker
func (hm *HealthManager) UnregisterChecker(name string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	delete(hm.checkers, name)
	delete(hm.configs, name)
	delete(hm.results, name)
}

// Start starts the health check background routine
func (hm *HealthManager) Start() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	if hm.running {
		return
	}
	
	hm.running = true
	hm.ticker = time.NewTicker(30 * time.Second) // Default interval
	
	go hm.runChecks()
}

// Stop stops the health check background routine
func (hm *HealthManager) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	
	if !hm.running {
		return
	}
	
	hm.running = false
	close(hm.stopCh)
	
	if hm.ticker != nil {
		hm.ticker.Stop()
	}
}

// CheckHealth performs a health check for all registered checkers
func (hm *HealthManager) CheckHealth(ctx context.Context) *HealthReport {
	hm.mu.RLock()
	checkers := make(map[string]HealthChecker)
	configs := make(map[string]HealthCheckConfig)
	
	for name, checker := range hm.checkers {
		checkers[name] = checker
		configs[name] = hm.configs[name]
	}
	hm.mu.RUnlock()
	
	results := make(map[string]*CheckResult)
	
	for name, checker := range checkers {
		config := configs[name]
		if !config.Enabled {
			continue
		}
		
		checkCtx, cancel := context.WithTimeout(ctx, config.Timeout)
		result := checker.Check(checkCtx)
		cancel()
		
		results[name] = result
	}
	
	// Update cached results
	hm.mu.Lock()
	for name, result := range results {
		hm.results[name] = result
	}
	hm.mu.Unlock()
	
	return hm.generateReport(results)
}

// CheckHealthByName checks health for a specific checker
func (hm *HealthManager) CheckHealthByName(ctx context.Context, name string) *CheckResult {
	hm.mu.RLock()
	checker, exists := hm.checkers[name]
	config := hm.configs[name]
	hm.mu.RUnlock()
	
	if !exists {
		return &CheckResult{
			Name:      name,
			Status:    StatusUnknown,
			Message:   "Checker not found",
			Timestamp: time.Now(),
		}
	}
	
	if !config.Enabled {
		return &CheckResult{
			Name:      name,
			Status:    StatusUnknown,
			Message:   "Checker disabled",
			Timestamp: time.Now(),
		}
	}
	
	checkCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	result := checker.Check(checkCtx)
	cancel()
	
	// Update cached result
	hm.mu.Lock()
	hm.results[name] = result
	hm.mu.Unlock()
	
	return result
}

// GetCachedResults returns the last cached health check results
func (hm *HealthManager) GetCachedResults() *HealthReport {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	
	return hm.generateReport(hm.results)
}

// runChecks runs health checks in the background
func (hm *HealthManager) runChecks() {
	for {
		select {
		case <-hm.stopCh:
			return
		case <-hm.ticker.C:
			hm.CheckHealth(context.Background())
		}
	}
}

// generateReport generates a health report from results
func (hm *HealthManager) generateReport(results map[string]*CheckResult) *HealthReport {
	report := &HealthReport{
		Timestamp: time.Now(),
		Checks:    results,
		Summary:   Summary{},
	}
	
	// Calculate summary and overall status
	for _, result := range results {
		report.Summary.Total++
		
		switch result.Status {
		case StatusHealthy:
			report.Summary.Healthy++
		case StatusUnhealthy:
			report.Summary.Unhealthy++
		case StatusDegraded:
			report.Summary.Degraded++
		default:
			report.Summary.Unknown++
		}
	}
	
	// Determine overall status
	if report.Summary.Unhealthy > 0 {
		report.Status = StatusUnhealthy
	} else if report.Summary.Degraded > 0 {
		report.Status = StatusDegraded
	} else if report.Summary.Healthy == report.Summary.Total {
		report.Status = StatusHealthy
	} else {
		report.Status = StatusUnknown
	}
	
	return report
}

// HTTPHandler returns an HTTP handler for health checks
func (hm *HealthManager) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Check if specific checker is requested
		if name := r.URL.Query().Get("check"); name != "" {
			result := hm.CheckHealthByName(ctx, name)
			
			w.Header().Set("Content-Type", "application/json")
			if result.Status == StatusHealthy {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			
			json.NewEncoder(w).Encode(result)
			return
		}
		
		// Full health check
		report := hm.CheckHealth(ctx)
		
		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else if report.Status == StatusDegraded {
			w.WriteHeader(http.StatusOK) // Still 200 for degraded
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		
		json.NewEncoder(w).Encode(report)
	}
}

// DatabaseHealthChecker checks database health
type DatabaseHealthChecker struct {
	name string
	db   Pinger
}

// Pinger interface for database ping
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(name string, db Pinger) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		name: name,
		db:   db,
	}
}

// Name returns the checker name
func (dhc *DatabaseHealthChecker) Name() string {
	return dhc.name
}

// Check performs the health check
func (dhc *DatabaseHealthChecker) Check(ctx context.Context) *CheckResult {
	start := time.Now()
	
	result := &CheckResult{
		Name:      dhc.name,
		Timestamp: start,
	}
	
	defer func() {
		result.Duration = time.Since(start)
		result.LastChecked = time.Now()
	}()
	
	if dhc.db == nil {
		result.Status = StatusUnhealthy
		result.Message = "Database connection is nil"
		return result
	}
	
	if err := dhc.db.Ping(ctx); err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Database ping failed: %v", err)
		return result
	}
	
	result.Status = StatusHealthy
	result.Message = "Database is healthy"
	return result
}

// CacheHealthChecker checks cache health
type CacheHealthChecker struct {
	name  string
	cache cache.Cache
	key   string // Test key for health check
}

// NewCacheHealthChecker creates a new cache health checker
func NewCacheHealthChecker(name string, cache cache.Cache) *CacheHealthChecker {
	return &CacheHealthChecker{
		name:  name,
		cache: cache,
		key:   "health_check_" + name,
	}
}

// Name returns the checker name
func (chc *CacheHealthChecker) Name() string {
	return chc.name
}

// Check performs the health check
func (chc *CacheHealthChecker) Check(ctx context.Context) *CheckResult {
	start := time.Now()
	
	result := &CheckResult{
		Name:      chc.name,
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	defer func() {
		result.Duration = time.Since(start)
		result.LastChecked = time.Now()
	}()
	
	if chc.cache == nil {
		result.Status = StatusUnhealthy
		result.Message = "Cache is nil"
		return result
	}
	
	// Test set operation
	testValue := fmt.Sprintf("health_check_%d", time.Now().Unix())
	if err := chc.cache.Set(ctx, chc.key, testValue, time.Minute); err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Cache set failed: %v", err)
		return result
	}
	
	// Test get operation
	value, err := chc.cache.Get(ctx, chc.key)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Cache get failed: %v", err)
		return result
	}
	
	if value != testValue {
		result.Status = StatusUnhealthy
		result.Message = "Cache value mismatch"
		return result
	}
	
	// Test delete operation
	if err := chc.cache.Delete(ctx, chc.key); err != nil {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("Cache delete failed: %v", err)
		return result
	}
	
	// Get cache stats if available
	if statsChecker, ok := chc.cache.(interface{ GetStats() cache.CacheStats }); ok {
		stats := statsChecker.GetStats()
		result.Details["hits"] = stats.Hits
		result.Details["misses"] = stats.Misses
		result.Details["sets"] = stats.Sets
		result.Details["deletes"] = stats.Deletes
		result.Details["errors"] = stats.Errors
	}
	
	result.Status = StatusHealthy
	result.Message = "Cache is healthy"
	return result
}

// HTTPHealthChecker checks HTTP endpoint health
type HTTPHealthChecker struct {
	name string
	url  string
	client *http.Client
}

// NewHTTPHealthChecker creates a new HTTP health checker
func NewHTTPHealthChecker(name, url string) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Name returns the checker name
func (hhc *HTTPHealthChecker) Name() string {
	return hhc.name
}

// Check performs the health check
func (hhc *HTTPHealthChecker) Check(ctx context.Context) *CheckResult {
	start := time.Now()
	
	result := &CheckResult{
		Name:      hhc.name,
		Timestamp: start,
		Details:   make(map[string]interface{}),
	}
	
	defer func() {
		result.Duration = time.Since(start)
		result.LastChecked = time.Now()
	}()
	
	req, err := http.NewRequestWithContext(ctx, "GET", hhc.url, nil)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("Failed to create request: %v", err)
		return result
	}
	
	resp, err := hhc.client.Do(req)
	if err != nil {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("HTTP request failed: %v", err)
		return result
	}
	defer resp.Body.Close()
	
	result.Details["status_code"] = resp.StatusCode
	result.Details["response_time"] = time.Since(start).Seconds()
	
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = StatusHealthy
		result.Message = "HTTP endpoint is healthy"
	} else if resp.StatusCode >= 500 {
		result.Status = StatusUnhealthy
		result.Message = fmt.Sprintf("HTTP endpoint returned error: %d", resp.StatusCode)
	} else {
		result.Status = StatusDegraded
		result.Message = fmt.Sprintf("HTTP endpoint returned warning: %d", resp.StatusCode)
	}
	
	return result
}

// CustomHealthChecker allows custom health check logic
type CustomHealthChecker struct {
	name     string
	checkFunc func(ctx context.Context) *CheckResult
}

// NewCustomHealthChecker creates a new custom health checker
func NewCustomHealthChecker(name string, checkFunc func(ctx context.Context) *CheckResult) *CustomHealthChecker {
	return &CustomHealthChecker{
		name:     name,
		checkFunc: checkFunc,
	}
}

// Name returns the checker name
func (chc *CustomHealthChecker) Name() string {
	return chc.name
}

// Check performs the health check
func (chc *CustomHealthChecker) Check(ctx context.Context) *CheckResult {
	if chc.checkFunc == nil {
		return &CheckResult{
			Name:      chc.name,
			Status:    StatusUnhealthy,
			Message:   "Check function is nil",
			Timestamp: time.Now(),
		}
	}
	
	return chc.checkFunc(ctx)
}

// Global health manager instance
var GlobalHealthManager *HealthManager
var once sync.Once

// GetGlobalHealthManager returns the global health manager
func GetGlobalHealthManager() *HealthManager {
	once.Do(func() {
		GlobalHealthManager = NewHealthManager()
	})
	return GlobalHealthManager
}
