package pool

import (
	"testing"
	"time"
)

// TestDefaultPoolConfig tests the default pool configuration
func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()
	
	if config.MaxOpenConns != 25 {
		t.Errorf("DefaultPoolConfig() MaxOpenConns = %v, want %v", config.MaxOpenConns, 25)
	}
	
	if config.MaxIdleConns != 10 {
		t.Errorf("DefaultPoolConfig() MaxIdleConns = %v, want %v", config.MaxIdleConns, 10)
	}
	
	if config.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("DefaultPoolConfig() ConnMaxLifetime = %v, want %v", config.ConnMaxLifetime, 30*time.Minute)
	}
	
	if config.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("DefaultPoolConfig() ConnMaxIdleTime = %v, want %v", config.ConnMaxIdleTime, 5*time.Minute)
	}
	
	if config.MinOpenConns != 5 {
		t.Errorf("DefaultPoolConfig() MinOpenConns = %v, want %v", config.MinOpenConns, 5)
	}
	
	if config.ConnectTimeout != 10*time.Second {
		t.Errorf("DefaultPoolConfig() ConnectTimeout = %v, want %v", config.ConnectTimeout, 10*time.Second)
	}
	
	if config.QueryTimeout != 30*time.Second {
		t.Errorf("DefaultPoolConfig() QueryTimeout = %v, want %v", config.QueryTimeout, 30*time.Second)
	}
	
	if config.HealthCheckPeriod != 1*time.Minute {
		t.Errorf("DefaultPoolConfig() HealthCheckPeriod = %v, want %v", config.HealthCheckPeriod, 1*time.Minute)
	}
	
	if config.HealthCheckTimeout != 5*time.Second {
		t.Errorf("DefaultPoolConfig() HealthCheckTimeout = %v, want %v", config.HealthCheckTimeout, 5*time.Second)
	}
	
	if config.HealthCheckQuery != "SELECT 1" {
		t.Errorf("DefaultPoolConfig() HealthCheckQuery = %v, want %v", config.HealthCheckQuery, "SELECT 1")
	}
	
	if config.MaxRetries != 3 {
		t.Errorf("DefaultPoolConfig() MaxRetries = %v, want %v", config.MaxRetries, 3)
	}
	
	if config.RetryDelay != 1*time.Second {
		t.Errorf("DefaultPoolConfig() RetryDelay = %v, want %v", config.RetryDelay, 1*time.Second)
	}
	
	if config.EnableMetrics != true {
		t.Errorf("DefaultPoolConfig() EnableMetrics = %v, want %v", config.EnableMetrics, true)
	}
	
	if config.MetricsInterval != 30*time.Second {
		t.Errorf("DefaultPoolConfig() MetricsInterval = %v, want %v", config.MetricsInterval, 30*time.Second)
	}
}

// TestProductionPoolConfig tests the production pool configuration
func TestProductionPoolConfig(t *testing.T) {
	config := ProductionPoolConfig()
	
	if config.MaxOpenConns != 50 {
		t.Errorf("ProductionPoolConfig() MaxOpenConns = %v, want %v", config.MaxOpenConns, 50)
	}
	
	if config.MaxIdleConns != 25 {
		t.Errorf("ProductionPoolConfig() MaxIdleConns = %v, want %v", config.MaxIdleConns, 25)
	}
	
	if config.ConnMaxLifetime != 60*time.Minute {
		t.Errorf("ProductionPoolConfig() ConnMaxLifetime = %v, want %v", config.ConnMaxLifetime, 60*time.Minute)
	}
	
	if config.MinOpenConns != 10 {
		t.Errorf("ProductionPoolConfig() MinOpenConns = %v, want %v", config.MinOpenConns, 10)
	}
	
	if config.MaxRetries != 5 {
		t.Errorf("ProductionPoolConfig() MaxRetries = %v, want %v", config.MaxRetries, 5)
	}
}

// TestDevelopmentPoolConfig tests the development pool configuration
func TestDevelopmentPoolConfig(t *testing.T) {
	config := DevelopmentPoolConfig()
	
	if config.MaxOpenConns != 10 {
		t.Errorf("DevelopmentPoolConfig() MaxOpenConns = %v, want %v", config.MaxOpenConns, 10)
	}
	
	if config.MaxIdleConns != 5 {
		t.Errorf("DevelopmentPoolConfig() MaxIdleConns = %v, want %v", config.MaxIdleConns, 5)
	}
	
	if config.ConnMaxLifetime != 10*time.Minute {
		t.Errorf("DevelopmentPoolConfig() ConnMaxLifetime = %v, want %v", config.ConnMaxLifetime, 10*time.Minute)
	}
	
	if config.QueryTimeout != 60*time.Second {
		t.Errorf("DevelopmentPoolConfig() QueryTimeout = %v, want %v", config.QueryTimeout, 60*time.Second)
	}
	
	if config.MaxRetries != 2 {
		t.Errorf("DevelopmentPoolConfig() MaxRetries = %v, want %v", config.MaxRetries, 2)
	}
}

// TestNewEnhancedPool tests the creation of enhanced pool
func TestNewEnhancedPool(t *testing.T) {
	config := DefaultPoolConfig()
	
	// Test with nil database (should fail)
	_, err := NewEnhancedPool(nil, config)
	if err == nil {
		t.Error("NewEnhancedPool() should return error for nil database")
	}
}
