// Package failover provides failover cache functionality with configurable storage.
package failover

import (
	"time"
)

// Config holds failover cache configuration.
type Config struct {
	// Storage configuration
	StorageType StorageType `yaml:"storage_type" json:"storage_type"`
	StoragePath string      `yaml:"storage_path" json:"storage_path"`
	FileSuffix  string      `yaml:"file_suffix" json:"file_suffix"`
	
	// Cache behavior
	MaxRetries      int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	
	// Fallback order
	FallbackOrder []string `yaml:"fallback_order" json:"fallback_order"`
}

// StorageType represents the type of storage backend.
type StorageType string

const (
	StorageTypeMemory StorageType = "memory"
	StorageTypeFile   StorageType = "file"
	StorageTypeRedis  StorageType = "redis"
	StorageTypeMemcache StorageType = "memcache"
)

// DefaultConfig returns default failover configuration.
func DefaultConfig() Config {
	return Config{
		StorageType:         StorageTypeFile,
		StoragePath:         "/tmp/failover_cache",
		FileSuffix:          ".failover",
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		FallbackOrder:       []string{"file", "memory"},
	}
}

// FileStorageConfig holds file-specific storage configuration.
type FileStorageConfig struct {
	Directory       string        `yaml:"directory" json:"directory"`
	FileSuffix      string        `yaml:"file_suffix" json:"file_suffix"`
	MaxSize         int64         `yaml:"max_size" json:"max_size"`
	MaxFiles        int           `yaml:"max_files" json:"max_files"`
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
}

// RedisStorageConfig holds Redis-specific storage configuration.
type RedisStorageConfig struct {
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	Password     string        `yaml:"password" json:"password"`
	Database     int           `yaml:"database" json:"database"`
	KeyPrefix    string        `yaml:"key_prefix" json:"key_prefix"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
}

// MemcacheStorageConfig holds Memcache-specific storage configuration.
type MemcacheStorageConfig struct {
	Hosts        []string      `yaml:"hosts" json:"hosts"`
	KeyPrefix    string        `yaml:"key_prefix" json:"key_prefix"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}
	if c.StorageType == "" {
		c.StorageType = StorageTypeFile
	}
	if c.StoragePath == "" {
		c.StoragePath = "/tmp/failover_cache"
	}
	if c.FileSuffix == "" {
		c.FileSuffix = ".failover"
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.RetryDelay <= 0 {
		c.RetryDelay = 1 * time.Second
	}
	if c.HealthCheckInterval <= 0 {
		c.HealthCheckInterval = 30 * time.Second
	}
	if len(c.FallbackOrder) == 0 {
		c.FallbackOrder = []string{"file", "memory"}
	}
	return nil
}
