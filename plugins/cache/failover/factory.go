// Package failover provides failover cache functionality with configurable storage.
package failover

import (
	"fmt"
	"time"
)

// Factory provides factory methods for creating failover cache instances.
type Factory struct{}

// NewFactory creates a new failover cache factory.
func NewFactory() *Factory {
	return &Factory{}
}

// CreateFileFailover creates a failover cache with file storage as primary.
func (f *Factory) CreateFileFailover(directory, fileSuffix string) (*FailoverCache, error) {
	config := FileStorageConfig{
		Directory:       directory,
		FileSuffix:      fileSuffix,
		MaxSize:         100 * 1024 * 1024, // 100MB
		MaxFiles:        10000,
		CleanupInterval: 10 * time.Minute,
	}

	primary, err := NewFileStorage(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storage: %w", err)
	}

	secondary := NewMemoryStorage()

	failoverConfig := Config{
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	return NewFailoverCache(primary, secondary, nil, failoverConfig), nil
}

// CreateMemoryFileFailover creates a failover cache with memory storage as primary, file as secondary.
func (f *Factory) CreateMemoryFileFailover(directory, fileSuffix string) (*FailoverCache, error) {
	primary := NewMemoryStorage()

	config := FileStorageConfig{
		Directory:       directory,
		FileSuffix:      fileSuffix,
		MaxSize:         100 * 1024 * 1024, // 100MB
		MaxFiles:        10000,
		CleanupInterval: 10 * time.Minute,
	}

	secondary, err := NewFileStorage(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storage: %w", err)
	}

	failoverConfig := Config{
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	return NewFailoverCache(primary, secondary, nil, failoverConfig), nil
}

// CreateCustomFailover creates a failover cache with custom storage configuration.
func (f *Factory) CreateCustomFailover(primaryType, secondaryType, tertiaryType StorageType, configs map[string]interface{}) (*FailoverCache, error) {
	var primary, secondary, tertiary Storage
	var err error

	// Create primary storage
	if primaryType != "" {
		if config, exists := configs["primary"]; exists {
			primary, err = CreateStorage(primaryType, config)
		} else {
			primary, err = CreateStorage(primaryType, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create primary storage: %w", err)
		}
	}

	// Create secondary storage
	if secondaryType != "" {
		if config, exists := configs["secondary"]; exists {
			secondary, err = CreateStorage(secondaryType, config)
		} else {
			secondary, err = CreateStorage(secondaryType, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create secondary storage: %w", err)
		}
	}

	// Create tertiary storage
	if tertiaryType != "" {
		if config, exists := configs["tertiary"]; exists {
			tertiary, err = CreateStorage(tertiaryType, config)
		} else {
			tertiary, err = CreateStorage(tertiaryType, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create tertiary storage: %w", err)
		}
	}

	failoverConfig := Config{
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	return NewFailoverCache(primary, secondary, tertiary, failoverConfig), nil
}

// CreateFromConfig creates a failover cache from configuration.
func (f *Factory) CreateFromConfig(config Config) (*FailoverCache, error) {
	return NewFailoverCacheFromConfig(config)
}

// Convenience functions for common configurations

// NewFileFailover creates a file-based failover cache with default settings.
func NewFileFailover(directory, fileSuffix string) (*FailoverCache, error) {
	factory := NewFactory()
	return factory.CreateFileFailover(directory, fileSuffix)
}

// NewMemoryFileFailover creates a memory-primary, file-secondary failover cache.
func NewMemoryFileFailover(directory, fileSuffix string) (*FailoverCache, error) {
	factory := NewFactory()
	return factory.CreateMemoryFileFailover(directory, fileSuffix)
}

// NewDefaultFailover creates a default failover cache (memory primary, file secondary).
func NewDefaultFailover() (*FailoverCache, error) {
	return NewMemoryFileFailover("/tmp/failover_cache", ".failover")
}

// NewCustomFileFailover creates a file failover with custom parameters.
func NewCustomFileFailover(directory, fileSuffix string, maxSize int64, maxFiles int, cleanupInterval time.Duration) (*FailoverCache, error) {
	config := FileStorageConfig{
		Directory:       directory,
		FileSuffix:      fileSuffix,
		MaxSize:         maxSize,
		MaxFiles:        maxFiles,
		CleanupInterval: cleanupInterval,
	}

	primary, err := NewFileStorage(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storage: %w", err)
	}

	secondary := NewMemoryStorage()

	failoverConfig := Config{
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	return NewFailoverCache(primary, secondary, nil, failoverConfig), nil
}
