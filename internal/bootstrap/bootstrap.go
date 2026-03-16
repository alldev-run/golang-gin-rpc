package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"golang-gin-rpc/pkg/db"
	"golang-gin-rpc/pkg/cache"
	"golang-gin-rpc/pkg/logger"
	"golang-gin-rpc/pkg/db/mysql"
	"golang-gin-rpc/pkg/cache/redis"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type string                 `yaml:"type"`
	MySQL mysql.Config           `yaml:"mysql"`
	Redis redis.Config           `yaml:"redis"`
	// Add other database types as needed
}

// Config holds all application configuration
type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
		Mode string `yaml:"mode"`
	} `yaml:"server"`
	
	Database map[string]DatabaseConfig `yaml:"database"`
	
	Cache struct {
		Type string        `yaml:"type"`
		Redis redis.Config `yaml:"redis"`
	} `yaml:"cache"`
	
	Logger struct {
		Level   string `yaml:"level"`
		Env     string `yaml:"env"`
		LogPath string `yaml:"log_path"`
	} `yaml:"logger"`
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}
	
	// Set defaults
	config.Server.Host = "localhost"
	config.Server.Port = "8080"
	config.Server.Mode = "debug"
	config.Logger.Level = "info"
	config.Logger.Env = "dev"
	config.Logger.LogPath = "./logs/app.log"
	
	// Load config file if exists
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}
	
	return config, nil
}

// Bootstrap handles application initialization
type Bootstrap struct {
	config *Config
	db     *db.Factory
	cache  cache.Cache
}

// NewBootstrap creates a new bootstrap instance
func NewBootstrap(configPath string) (*Bootstrap, error) {
	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Initialize logger
	logger.Init(logger.Config{
		Level:   config.Logger.Level,
		Env:     config.Logger.Env,
		LogPath: config.Logger.LogPath,
	})
	
	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(config.Logger.LogPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	logger.Info("Configuration loaded successfully")
	
	return &Bootstrap{
		config: config,
	}, nil
}

// InitializeDatabases initializes all database connections
func (b *Bootstrap) InitializeDatabases() error {
	factory := db.NewFactory()
	
	for name, dbConfig := range b.config.Database {
		config := db.Config{
			Type: db.Type(dbConfig.Type),
		}
		
		switch dbConfig.Type {
		case "mysql":
			config.MySQL = dbConfig.MySQL
		case "redis":
			config.Redis = dbConfig.Redis
		// Add other database types
		}
		
		client, err := factory.Create(config)
		if err != nil {
			return fmt.Errorf("failed to create database client %s: %w", name, err)
		}
		
		// Test connection
		ctx := context.Background()
		if err := client.Ping(ctx); err != nil {
			logger.Warn("Database %s connection failed", zap.String("name", name), zap.Error(err))
		} else {
			logger.Info("Database %s connected successfully", zap.String("name", name))
		}
	}
	
	b.db = factory
	logger.Info("Database initialization completed")
	return nil
}

// InitializeCache initializes cache
func (b *Bootstrap) InitializeCache() error {
	switch b.config.Cache.Type {
	case "redis":
		cacheInstance, err := cache.NewRedisCache(b.config.Cache.Redis)
		if err != nil {
			return fmt.Errorf("failed to create redis cache: %w", err)
		}
		b.cache = cacheInstance
		logger.Info("Redis cache initialized")
	default:
		logger.Warn("Cache type %s not supported, skipping cache initialization", zap.String("type", b.config.Cache.Type))
	}
	
	return nil
}

// GetConfig returns the configuration
func (b *Bootstrap) GetConfig() *Config {
	return b.config
}

// GetLogger returns the global logger
func (b *Bootstrap) GetLogger() *zap.Logger {
	return logger.L()
}

// GetDatabase returns the database factory
func (b *Bootstrap) GetDatabase() *db.Factory {
	return b.db
}

// GetCache returns the cache instance
func (b *Bootstrap) GetCache() cache.Cache {
	return b.cache
}

// Close closes all resources
func (b *Bootstrap) Close() error {
	var errors []error
	
	if b.cache != nil {
		if err := b.cache.Close(); err != nil {
			errors = append(errors, fmt.Errorf("cache close error: %w", err))
		}
	}
	
	if b.db != nil {
	// Database factory doesn't need explicit close
	// Clients are managed individually
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}
	
	logger.Info("Bootstrap shutdown completed")
	return nil
}
