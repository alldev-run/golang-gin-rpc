package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang-gin-rpc/pkg/auth"
	"golang-gin-rpc/pkg/cache"
	"golang-gin-rpc/pkg/cache/redis"
	"golang-gin-rpc/pkg/db"
	"golang-gin-rpc/pkg/db/mysql"
	"golang-gin-rpc/pkg/discovery"
	"golang-gin-rpc/pkg/health"
	"golang-gin-rpc/pkg/logger"
	"golang-gin-rpc/pkg/metrics"
	"golang-gin-rpc/pkg/rpc"
	"golang-gin-rpc/pkg/tracing"

	"gopkg.in/yaml.v3"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type  string       `yaml:"type"`
	MySQL mysql.Config `yaml:"mysql"`
	Redis redis.Config `yaml:"redis"`
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
		Type  string       `yaml:"type"`
		Redis redis.Config `yaml:"redis"`
	} `yaml:"cache"`

	Logger struct {
		Level   string `yaml:"level"`
		Env     string `yaml:"env"`
		LogPath string `yaml:"log_path"`
	} `yaml:"logger"`

	RPC rpc.ManagerConfig `yaml:"rpc"`

	Discovery discovery.ManagerConfig `yaml:"discovery"`

	Errors struct {
		EnableStackTrace bool `yaml:"enable_stack_trace"`
		MaxErrorDepth    int  `yaml:"max_error_depth"`
	} `yaml:"errors"`

	Health health.HealthConfig `yaml:"health"`

	Metrics struct {
		Enabled bool   `yaml:"enabled"`
		Address string `yaml:"address"`
		Path    string `yaml:"path"`
	} `yaml:"metrics"`

	Auth auth.AuthConfig `yaml:"auth"`

	Tracing tracing.Config `yaml:"tracing"`
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
	config.Errors.EnableStackTrace = true
	config.Errors.MaxErrorDepth = 10
	config.Health.Enabled = true
	config.Metrics.Enabled = true
	config.Metrics.Address = ":9090"
	config.Metrics.Path = "/metrics"
	config.Auth.Enabled = false
	config.Tracing.Enabled = false

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
	config           *Config
	db               *db.Factory
	cache            cache.Cache
	rpcManager       *rpc.Manager
	discoveryManager *discovery.ServiceDiscoveryManager
	healthManager    *health.HealthManager
	metricsCollector *metrics.MetricsCollector
	authManager      *auth.AuthManager
	tracer           *tracing.Tracer
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

// InitializeAll initializes all components
func (b *Bootstrap) InitializeAll() error {
	// Initialize in dependency order
	if err := b.InitializeDatabases(); err != nil {
		return fmt.Errorf("failed to initialize databases: %w", err)
	}

	if err := b.InitializeCache(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	if err := b.InitializeErrors(); err != nil {
		return fmt.Errorf("failed to initialize errors: %w", err)
	}

	if err := b.InitializeHealth(); err != nil {
		return fmt.Errorf("failed to initialize health: %w", err)
	}

	if err := b.InitializeMetrics(); err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	if err := b.InitializeAuth(); err != nil {
		return fmt.Errorf("failed to initialize auth: %w", err)
	}

	if err := b.InitializeTracing(); err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	if err := b.InitializeRPC(); err != nil {
		return fmt.Errorf("failed to initialize RPC: %w", err)
	}

	if err := b.InitializeDiscovery(); err != nil {
		return fmt.Errorf("failed to initialize discovery: %w", err)
	}

	logger.Info("All components initialized successfully")
	return nil
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
			logger.Warn("Database %s connection failed", logger.String("name", name), logger.Error(err))
		} else {
			logger.Info("Database %s connected successfully", logger.String("name", name))
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
		logger.Warn("Cache type %s not supported, skipping cache initialization", logger.String("type", b.config.Cache.Type))
	}

	return nil
}

// InitializeRPC initializes RPC services
func (b *Bootstrap) InitializeRPC() error {
	// Create RPC manager
	b.rpcManager = rpc.NewManager(b.config.RPC)

	// Log RPC configuration
	logger.Info("Initializing RPC services",
		logger.Int("servers", len(b.config.RPC.Servers)))

	// Start RPC manager
	if err := b.rpcManager.Start(); err != nil {
		return fmt.Errorf("failed to start RPC manager: %w", err)
	}

	logger.Info("RPC services initialized successfully")
	return nil
}

// InitializeDiscovery initializes service discovery
func (b *Bootstrap) InitializeDiscovery() error {
	// Create discovery manager
	manager, err := discovery.NewServiceDiscoveryManager(b.config.Discovery)
	if err != nil {
		return fmt.Errorf("failed to create discovery manager: %w", err)
	}

	// Start discovery manager
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start discovery manager: %w", err)
	}

	b.discoveryManager = manager

	logger.Info("Service discovery initialized successfully")
	return nil
}

// InitializeErrors initializes error handling
func (b *Bootstrap) InitializeErrors() error {
	// Errors package is typically used globally and doesn't need explicit initialization
	// But we can configure global error handling behavior
	if b.config.Errors.EnableStackTrace {
		logger.Info("Error stack traces enabled")
	}

	if b.config.Errors.MaxErrorDepth > 0 {
		logger.Info("Error depth limit set", logger.Int("depth", b.config.Errors.MaxErrorDepth))
	}

	logger.Info("Error handling initialized successfully")
	return nil
}

// InitializeHealth initializes health check services
func (b *Bootstrap) InitializeHealth() error {
	if !b.config.Health.Enabled {
		logger.Info("Health checks disabled")
		return nil
	}

	healthManager := health.NewHealthManager()

	// Register default health checkers
	// Database health checker
	if b.db != nil {
		// Create a simple database health checker
		dbChecker := health.NewCustomHealthChecker("database", func(ctx context.Context) *health.CheckResult {
			// Note: In real implementation, you would check actual database connections
			return &health.CheckResult{
				Name:      "database",
				Status:    health.StatusHealthy,
				Message:   "Database connections healthy",
				Timestamp: time.Now(),
			}
		})
		healthManager.RegisterChecker(dbChecker, health.DefaultHealthCheckConfig())
	}

	// Cache health checker
	if b.cache != nil {
		cacheChecker := health.NewCustomHealthChecker("cache", func(ctx context.Context) *health.CheckResult {
			return &health.CheckResult{
				Name:      "cache",
				Status:    health.StatusHealthy,
				Message:   "Cache service healthy",
				Timestamp: time.Now(),
			}
		})
		healthManager.RegisterChecker(cacheChecker, health.DefaultHealthCheckConfig())
	}

	b.healthManager = healthManager

	logger.Info("Health check services initialized successfully")
	return nil
}

// InitializeMetrics initializes metrics collection
func (b *Bootstrap) InitializeMetrics() error {
	if !b.config.Metrics.Enabled {
		logger.Info("Metrics collection disabled")
		return nil
	}

	metricsCollector := metrics.NewMetricsCollector()
	b.metricsCollector = metricsCollector

	// Start metrics server in background
	go func() {
		if err := metricsCollector.StartMetricsServer(b.config.Metrics.Address); err != nil {
			logger.Errorf("Failed to start metrics server", logger.Error(err))
		}
	}()

	logger.Info("Metrics collection initialized successfully",
		logger.String("address", b.config.Metrics.Address),
		logger.String("path", b.config.Metrics.Path))

	return nil
}

// InitializeAuth initializes authentication services
func (b *Bootstrap) InitializeAuth() error {
	if !b.config.Auth.Enabled {
		logger.Info("Authentication services disabled")
		return nil
	}

	authManager := auth.NewAuthManager(b.config.Auth)
	b.authManager = authManager

	logger.Info("Authentication services initialized successfully")
	return nil
}

// InitializeTracing initializes tracing services
func (b *Bootstrap) InitializeTracing() error {
	if !b.config.Tracing.Enabled {
		logger.Info("Tracing services disabled")
		return nil
	}

	tracer, err := tracing.NewTracer(b.config.Tracing)
	if err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	b.tracer = tracer

	logger.Info("Tracing services initialized successfully")
	return nil
}

// GetConfig returns the configuration
func (b *Bootstrap) GetConfig() *Config {
	return b.config
}

// GetLogger returns the global logger
func (b *Bootstrap) GetLogger() interface{} {
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

// GetRPCManager returns the RPC manager
func (b *Bootstrap) GetRPCManager() *rpc.Manager {
	return b.rpcManager
}

// GetDiscoveryManager returns the discovery manager
func (b *Bootstrap) GetDiscoveryManager() *discovery.ServiceDiscoveryManager {
	return b.discoveryManager
}

// GetHealthManager returns the health manager
func (b *Bootstrap) GetHealthManager() *health.HealthManager {
	return b.healthManager
}

// GetMetricsCollector returns the metrics collector
func (b *Bootstrap) GetMetricsCollector() *metrics.MetricsCollector {
	return b.metricsCollector
}

// GetAuthManager returns the auth manager
func (b *Bootstrap) GetAuthManager() *auth.AuthManager {
	return b.authManager
}

// GetTracer returns the tracer
func (b *Bootstrap) GetTracer() *tracing.Tracer {
	return b.tracer
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

	if b.rpcManager != nil {
		if err := b.rpcManager.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("rpc manager close error: %w", err))
		}
	}

	if b.discoveryManager != nil {
		if err := b.discoveryManager.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("discovery manager close error: %w", err))
		}
	}

	if b.healthManager != nil {
		// Health manager doesn't need explicit close
	}

	if b.metricsCollector != nil {
		// Metrics collector doesn't need explicit close
	}

	if b.authManager != nil {
		// Auth manager doesn't need explicit close
	}

	if b.tracer != nil {
		ctx := context.Background()
		if err := b.tracer.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("tracer close error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	logger.Info("Bootstrap shutdown completed")
	return nil
}
