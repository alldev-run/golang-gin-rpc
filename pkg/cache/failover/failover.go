// Package failover provides failover cache functionality with configurable storage.
package failover

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FailoverCache provides failover support between multiple storage backends.
type FailoverCache struct {
	primary           Storage
	secondary         Storage
	tertiary          Storage
	maxRetries        int
	retryDelay        time.Duration
	healthCheckInterval time.Duration
	mu                sync.RWMutex
	healthTicker      *time.Ticker
	healthStatus      map[string]bool
}

// NewFailoverCache creates a new failover cache with multiple storage backends.
func NewFailoverCache(primary, secondary, tertiary Storage, config Config) *FailoverCache {
	fc := &FailoverCache{
		primary:             primary,
		secondary:           secondary,
		tertiary:            tertiary,
		maxRetries:          config.MaxRetries,
		retryDelay:          config.RetryDelay,
		healthCheckInterval: config.HealthCheckInterval,
		healthStatus:        make(map[string]bool),
	}

	// Initialize health status
	fc.updateHealthStatus()

	// Start health check goroutine
	if config.HealthCheckInterval > 0 {
		fc.healthTicker = time.NewTicker(config.HealthCheckInterval)
		go fc.startHealthCheck()
	}

	return fc
}

// Get retrieves a value trying primary, then secondary, then tertiary.
func (fc *FailoverCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// Try primary first
	if fc.isHealthy("primary") && fc.primary != nil {
		if value, found := fc.primary.Get(ctx, key); found {
			return value, true
		}
	}

	// Try secondary
	if fc.isHealthy("secondary") && fc.secondary != nil {
		if value, found := fc.secondary.Get(ctx, key); found {
			// Restore to primary for future faster access
			if fc.isHealthy("primary") && fc.primary != nil {
				fc.primary.Set(ctx, key, value, 5*time.Minute)
			}
			return value, true
		}
	}

	// Try tertiary
	if fc.isHealthy("tertiary") && fc.tertiary != nil {
		if value, found := fc.tertiary.Get(ctx, key); found {
			// Restore to primary and secondary
			if fc.isHealthy("primary") && fc.primary != nil {
				fc.primary.Set(ctx, key, value, 5*time.Minute)
			}
			if fc.isHealthy("secondary") && fc.secondary != nil {
				fc.secondary.Set(ctx, key, value, 5*time.Minute)
			}
			return value, true
		}
	}

	return nil, false
}

// Set stores a value in all available healthy backends.
func (fc *FailoverCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var errors []error

	// Try to set in all healthy backends
	if fc.isHealthy("primary") && fc.primary != nil {
		if err := fc.primary.Set(ctx, key, value, ttl); err != nil {
			errors = append(errors, fmt.Errorf("primary storage error: %w", err))
		}
	}

	if fc.isHealthy("secondary") && fc.secondary != nil {
		if err := fc.secondary.Set(ctx, key, value, ttl); err != nil {
			errors = append(errors, fmt.Errorf("secondary storage error: %w", err))
		}
	}

	if fc.isHealthy("tertiary") && fc.tertiary != nil {
		if err := fc.tertiary.Set(ctx, key, value, ttl); err != nil {
			errors = append(errors, fmt.Errorf("tertiary storage error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failover set errors: %v", errors)
	}

	return nil
}

// Delete removes a value from all backends.
func (fc *FailoverCache) Delete(ctx context.Context, key string) error {
	var errors []error

	if fc.primary != nil {
		if err := fc.primary.Delete(ctx, key); err != nil {
			errors = append(errors, fmt.Errorf("primary storage error: %w", err))
		}
	}

	if fc.secondary != nil {
		if err := fc.secondary.Delete(ctx, key); err != nil {
			errors = append(errors, fmt.Errorf("secondary storage error: %w", err))
		}
	}

	if fc.tertiary != nil {
		if err := fc.tertiary.Delete(ctx, key); err != nil {
			errors = append(errors, fmt.Errorf("tertiary storage error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failover delete errors: %v", errors)
	}

	return nil
}

// Clear removes all values from all backends.
func (fc *FailoverCache) Clear(ctx context.Context) error {
	var errors []error

	if fc.primary != nil {
		if err := fc.primary.Clear(ctx); err != nil {
			errors = append(errors, fmt.Errorf("primary storage error: %w", err))
		}
	}

	if fc.secondary != nil {
		if err := fc.secondary.Clear(ctx); err != nil {
			errors = append(errors, fmt.Errorf("secondary storage error: %w", err))
		}
	}

	if fc.tertiary != nil {
		if err := fc.tertiary.Clear(ctx); err != nil {
			errors = append(errors, fmt.Errorf("tertiary storage error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failover clear errors: %v", errors)
	}

	return nil
}

// Keys returns keys from the primary healthy backend.
func (fc *FailoverCache) Keys(ctx context.Context) []string {
	if fc.isHealthy("primary") && fc.primary != nil {
		return fc.primary.Keys(ctx)
	}

	if fc.isHealthy("secondary") && fc.secondary != nil {
		return fc.secondary.Keys(ctx)
	}

	if fc.isHealthy("tertiary") && fc.tertiary != nil {
		return fc.tertiary.Keys(ctx)
	}

	return nil
}

// GetStorageStatus returns the health status of all storage backends.
func (fc *FailoverCache) GetStorageStatus() map[string]bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	status := make(map[string]bool)
	for name, healthy := range fc.healthStatus {
		status[name] = healthy
	}

	return status
}

// isHealthy checks if a storage backend is healthy.
func (fc *FailoverCache) isHealthy(storage string) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return fc.healthStatus[storage]
}

// updateHealthStatus updates the health status of all storage backends.
func (fc *FailoverCache) updateHealthStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Check primary
	if fc.primary != nil {
		fc.healthStatus["primary"] = fc.primary.HealthCheck(ctx) == nil
	} else {
		fc.healthStatus["primary"] = false
	}

	// Check secondary
	if fc.secondary != nil {
		fc.healthStatus["secondary"] = fc.secondary.HealthCheck(ctx) == nil
	} else {
		fc.healthStatus["secondary"] = false
	}

	// Check tertiary
	if fc.tertiary != nil {
		fc.healthStatus["tertiary"] = fc.tertiary.HealthCheck(ctx) == nil
	} else {
		fc.healthStatus["tertiary"] = false
	}
}

// startHealthCheck starts the health check goroutine.
func (fc *FailoverCache) startHealthCheck() {
	for range fc.healthTicker.C {
		fc.updateHealthStatus()
	}
}

// Close closes all storage backends and stops health checks.
func (fc *FailoverCache) Close() error {
	var errors []error

	if fc.healthTicker != nil {
		fc.healthTicker.Stop()
	}

	if fc.primary != nil {
		if err := fc.primary.Close(); err != nil {
			errors = append(errors, fmt.Errorf("primary storage close error: %w", err))
		}
	}

	if fc.secondary != nil {
		if err := fc.secondary.Close(); err != nil {
			errors = append(errors, fmt.Errorf("secondary storage close error: %w", err))
		}
	}

	if fc.tertiary != nil {
		if err := fc.tertiary.Close(); err != nil {
			errors = append(errors, fmt.Errorf("tertiary storage close error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failover close errors: %v", errors)
	}

	return nil
}

// CreateStorage creates a storage instance based on configuration.
func CreateStorage(storageType StorageType, config interface{}) (Storage, error) {
	switch storageType {
	case StorageTypeMemory:
		return NewMemoryStorage(), nil
		
	case StorageTypeFile:
		if fileConfig, ok := config.(FileStorageConfig); ok {
			return NewFileStorage(fileConfig)
		}
		return nil, fmt.Errorf("invalid file storage configuration")
		
	case StorageTypeRedis:
		if redisConfig, ok := config.(RedisStorageConfig); ok {
			return NewRedisStorage(redisConfig)
		}
		return nil, fmt.Errorf("invalid redis storage configuration")
		
	case StorageTypeMemcache:
		if memcacheConfig, ok := config.(MemcacheStorageConfig); ok {
			return NewMemcacheStorage(memcacheConfig)
		}
		return nil, fmt.Errorf("invalid memcache storage configuration")
		
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}
}

// NewFailoverCacheFromConfig creates a failover cache from configuration.
func NewFailoverCacheFromConfig(config Config) (*FailoverCache, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	var storages []Storage

	// Create storages based on fallback order
	for _, storageType := range config.FallbackOrder {
		var storage Storage
		var err error

		switch StorageType(storageType) {
		case StorageTypeMemory:
			storage = NewMemoryStorage()
			
		case StorageTypeFile:
			fileConfig := FileStorageConfig{
				Directory:       config.StoragePath,
				FileSuffix:      config.FileSuffix,
				MaxSize:         100 * 1024 * 1024, // 100MB default
				MaxFiles:        10000,
				CleanupInterval: 10 * time.Minute,
			}
			storage, err = NewFileStorage(fileConfig)
			
		case StorageTypeRedis:
			// Redis configuration would need to be passed in config
			// This is a placeholder
			err = fmt.Errorf("redis storage configuration not provided")
			
		case StorageTypeMemcache:
			// Memcache configuration would need to be passed in config
			// This is a placeholder
			err = fmt.Errorf("memcache storage configuration not provided")
		}

		if err == nil && storage != nil {
			storages = append(storages, storage)
		}
	}

	if len(storages) == 0 {
		return nil, fmt.Errorf("no valid storage backends could be created")
	}

	var primary, secondary, tertiary Storage
	primary = storages[0]
	if len(storages) > 1 {
		secondary = storages[1]
	}
	if len(storages) > 2 {
		tertiary = storages[2]
	}

	return NewFailoverCache(primary, secondary, tertiary, config), nil
}
