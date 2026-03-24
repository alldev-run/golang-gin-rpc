package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/cache"
	"github.com/alldev-run/golang-gin-rpc/pkg/errors"
	"github.com/alldev-run/golang-gin-rpc/pkg/health"
	"github.com/alldev-run/golang-gin-rpc/pkg/metrics"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/pool"

	"github.com/gin-gonic/gin"
)

// MockCache for integration testing
type MockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]interface{}),
	}
}

func (m *MockCache) Get(ctx context.Context, key string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, errors.ErrResourceNotFound
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.data[key] = value
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.data, key)
	return nil
}

func (m *MockCache) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	return m.Get(ctx, key)
}

func (m *MockCache) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	return m.Set(ctx, key, value, baseTTL)
}

func (m *MockCache) GetStats() cache.CacheStats {
	return cache.CacheStats{
		Hits:       10,
		Misses:     5,
		Sets:       8,
		Deletes:    2,
		Errors:     1,
		LastAccess: time.Now(),
	}
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockCache) Clear(ctx context.Context) error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *MockCache) Close() error {
	return nil
}

// TestCacheIntegration tests cache functionality
func TestCacheIntegration(t *testing.T) {
	// Create a mock cache
	mockCache := NewMockCache()
	
	// Test cache operations
	ctx := context.Background()
	
	// Set operation
	err := mockCache.Set(ctx, "test_key", "test_value", time.Hour)
	if err != nil {
		t.Errorf("Cache Set() error = %v", err)
	}
	
	// Get operation
	value, err := mockCache.Get(ctx, "test_key")
	if err != nil {
		t.Errorf("Cache Get() error = %v", err)
	}
	
	if value != "test_value" {
		t.Errorf("Cache Get() = %v, want %v", value, "test_value")
	}
	
	// Delete operation
	err = mockCache.Delete(ctx, "test_key")
	if err != nil {
		t.Errorf("Cache Delete() error = %v", err)
	}
	
	// Verify deletion
	value, err = mockCache.Get(ctx, "test_key")
	if err == nil {
		t.Error("Cache Get() should return error for deleted key")
	}
}

// TestErrorsIntegration tests error handling
func TestErrorsIntegration(t *testing.T) {
	// Test error creation
	err := errors.New(errors.ErrCodeValidationFailed, "Validation failed")
	
	if err.Code != errors.ErrCodeValidationFailed {
		t.Errorf("Error code = %v, want %v", err.Code, errors.ErrCodeValidationFailed)
	}
	
	if err.Message != "Validation failed" {
		t.Errorf("Error message = %v, want %v", err.Message, "Validation failed")
	}
	
	// Test error wrapping
	originalErr := errors.New(errors.ErrCodeInternalServer, "Internal error")
	wrappedErr := errors.Wrap(originalErr, errors.ErrCodeInternalServer, "Wrapper error")
	
	if wrappedErr.Cause != originalErr {
		t.Error("Wrapped error should contain original error as cause")
	}
}

// TestHealthIntegration tests health checking
func TestHealthIntegration(t *testing.T) {
	// Create health manager
	hm := health.NewHealthManager()
	
	// Create a custom health checker
	checker := health.NewCustomHealthChecker("test_checker", func(ctx context.Context) *health.CheckResult {
		return &health.CheckResult{
			Name:      "test_checker",
			Status:    health.StatusHealthy,
			Message:   "Test checker is healthy",
			Timestamp: time.Now(),
		}
	})
	
	// Register checker
	config := health.DefaultHealthCheckConfig()
	config.Enabled = true
	hm.RegisterChecker(checker, config)
	
	// Perform health check
	ctx := context.Background()
	report := hm.CheckHealth(ctx)
	
	if report.Status != health.StatusHealthy {
		t.Errorf("Health status = %v, want %v", report.Status, health.StatusHealthy)
	}
}

// TestMetricsIntegration tests metrics collection
func TestMetricsIntegration(t *testing.T) {
	// Create metrics collector
	collector := metrics.NewMetricsCollector()
	
	// Test recording metrics
	collector.RecordHTTPRequest("GET", "/api/test", "200", 100*time.Millisecond)
	collector.RecordDBQuery("mysql", "select", 50*time.Millisecond)
	collector.RecordCacheOperation("redis", "get", "hit")
	collector.UpdateActiveConnections("http", 10.0)
	
	// Test that metrics don't panic
	// Note: We can't easily test the actual metric values without accessing the internal registry
	// but we can verify the methods don't panic
}

// TestDBPoolIntegration tests database pool configuration
func TestDBPoolIntegration(t *testing.T) {
	// Test pool configurations
	defaultConfig := pool.DefaultPoolConfig()
	productionConfig := pool.ProductionPoolConfig()
	developmentConfig := pool.DevelopmentPoolConfig()
	
	// Verify default config
	if defaultConfig.MaxOpenConns != 25 {
		t.Errorf("Default MaxOpenConns = %v, want %v", defaultConfig.MaxOpenConns, 25)
	}
	
	// Verify production config
	if productionConfig.MaxOpenConns != 50 {
		t.Errorf("Production MaxOpenConns = %v, want %v", productionConfig.MaxOpenConns, 50)
	}
	
	// Verify development config
	if developmentConfig.MaxOpenConns != 10 {
		t.Errorf("Development MaxOpenConns = %v, want %v", developmentConfig.MaxOpenConns, 10)
	}
}

// TestGinIntegration tests Gin framework integration
func TestGinIntegration(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	
	// Create a simple Gin router
	router := gin.New()
	
	// Add a simple route
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "test"})
	})
	
	// Test that router is created successfully
	if router == nil {
		t.Error("Gin router should not be nil")
	}
}

// TestConcurrentIntegration tests concurrent operations
func TestConcurrentIntegration(t *testing.T) {
	mockCache := NewMockCache()
	
	var wg sync.WaitGroup
	numGoroutines := 10
	
	// Test concurrent cache operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			ctx := context.Background()
			key := fmt.Sprintf("key_%d", id)
			value := fmt.Sprintf("value_%d", id)
			
			// Set value
			err := mockCache.Set(ctx, key, value, time.Hour)
			if err != nil {
				t.Errorf("Goroutine %d: Set error = %v", id, err)
			}
			
			// Get value
			retrievedValue, err := mockCache.Get(ctx, key)
			if err != nil {
				t.Errorf("Goroutine %d: Get error = %v", id, err)
			}
			
			if retrievedValue != value {
				t.Errorf("Goroutine %d: got %v, want %v", id, retrievedValue, value)
			}
		}(i)
	}
	
	wg.Wait()
}
