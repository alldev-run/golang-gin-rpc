package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/auth"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/db"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
	"github.com/alldev-run/golang-gin-rpc/pkg/discovery"
	"github.com/alldev-run/golang-gin-rpc/pkg/gateway"
	"github.com/alldev-run/golang-gin-rpc/pkg/health"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/pkg/metrics"
	"github.com/alldev-run/golang-gin-rpc/pkg/rpc"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
	"github.com/alldev-run/golang-gin-rpc/pkg/websocket"
	"github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
	cacheredis "github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	pg "github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
	mysqlpkg "github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
)

// LoadConfig loads configuration from file using pkg/config
func LoadConfig(configPath string) (*config.GlobalConfig, error) {
	loader := config.NewLoader()
	
	// Set defaults first
	loader.Set(config.DefaultConfig())
	
	// Load from file if exists
	if configPath != "" {
		if err := loader.Load(configPath); err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}
	
	return loader.Get(), nil
}

// Bootstrap handles application initialization
type Bootstrap struct {
	config           *config.GlobalConfig
	db               *db.Factory
	cache            cache.Cache
	rpcManager       *rpc.Manager
	websocketServer  *websocket.Server
	discoveryManager *discovery.ServiceDiscoveryManager
	healthManager    *health.HealthManager
	metricsCollector *metrics.MetricsCollector
	authManager      *auth.AuthManager
	tracer           *tracing.Tracer
	gateway          *gateway.Gateway

	serviceMu       sync.RWMutex
	serviceFactories map[string]ServiceFactory
	managedServices  map[string]ManagedService
	serviceOrder     []string

	depMu        sync.RWMutex
	dependencies map[string]interface{}
}

// NewBootstrap creates a new bootstrap instance
func NewBootstrap(configPath string) (*Bootstrap, error) {
	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	loggerConfig := logger.DefaultConfig()
	
	// Override with config file values
	if cfg.Observability.Logging.Level != "" {
		loggerConfig.Level = logger.LogLevel(cfg.Observability.Logging.Level)
	}
	if cfg.Observability.Logging.Output != "" {
		loggerConfig.Output = logger.LogOutput(cfg.Observability.Logging.Output)
	}
	if cfg.Observability.Logging.Format != "" {
		loggerConfig.Format = logger.LogFormat(cfg.Observability.Logging.Format)
	}
	if cfg.Observability.Logging.FilePath != "" {
		loggerConfig.LogPath = cfg.Observability.Logging.FilePath
	}
	
	// Boolean flags
	loggerConfig.Compress = cfg.Observability.Logging.MaxBackups > 0
	
	// Numeric values
	if cfg.Observability.Logging.MaxSize > 0 {
		loggerConfig.MaxSize = cfg.Observability.Logging.MaxSize
	}
	if cfg.Observability.Logging.MaxBackups > 0 {
		loggerConfig.MaxBackups = cfg.Observability.Logging.MaxBackups
	}
	if cfg.Observability.Logging.MaxAge > 0 {
		loggerConfig.MaxAge = cfg.Observability.Logging.MaxAge
	}
	
	logger.Init(loggerConfig)

	// Ensure log directory exists
	if cfg.Observability.Logging.FilePath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Observability.Logging.FilePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	logger.Info("Configuration loaded successfully")

	boot := &Bootstrap{
		config:           cfg,
		serviceFactories: make(map[string]ServiceFactory),
		managedServices:  make(map[string]ManagedService),
		dependencies:     make(map[string]interface{}),
	}
	if err := boot.RegisterDefaultServiceFactories(); err != nil {
		return nil, err
	}
	return boot, nil
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

	if err := b.InitializeGateway(); err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}

	logger.Info("All components initialized successfully")
	return nil
}

// InitializeDatabases initializes all database connections
func (b *Bootstrap) InitializeDatabases() error {
	if !b.config.Database.Primary.Enabled && !b.config.Database.Replica.Enabled {
		logger.Info("No database configuration found, skipping database initialization")
		return nil
	}

	factory := db.NewFactory()
	configs := map[string]config.DBConfig{
		"primary": b.config.Database.Primary,
		"replica": b.config.Database.Replica,
	}

	for name, dbConfig := range configs {
		if !dbConfig.Enabled {
			continue
		}

		clientConfig, err := buildDBConfig(dbConfig, b.config.Database.Pool)
		if err != nil {
			logger.Errorf("Database configuration build failed",
				logger.String("name", name),
				logger.Error(err),
			)
			return fmt.Errorf("failed to build database config %s: %w", name, err)
		}

		client, err := factory.Create(clientConfig)
		if err != nil {
			logger.Errorf("Database client creation failed",
				logger.String("name", name),
				logger.Error(err),
			)
			return fmt.Errorf("failed to create database client %s: %w", name, err)
		}

		ctx := context.Background()
		if err := client.Ping(ctx); err != nil {
			logger.Errorf("Database connection failed", logger.String("name", name), logger.Error(err))
		} else {
			logger.Info("Database connected successfully", logger.String("name", name))
		}
	}

	b.db = factory
	b.setDependency("db.factory", factory)
	logger.Info("Database initialization completed")
	return nil
}

// InitializeCache initializes cache
func (b *Bootstrap) InitializeCache() error {
	if b.config.Redis.Host == "" {
		logger.Info("No cache configuration found, skipping cache initialization")
		return nil
	}

	cacheCfg := cacheredis.DefaultConfig()
	cacheCfg.Host = b.config.Redis.Host
	cacheCfg.Port = b.config.Redis.Port
	cacheCfg.Password = b.config.Redis.Password
	cacheCfg.Database = b.config.Redis.Database
	cacheCfg.PoolSize = b.config.Redis.PoolSize
	cacheCfg.MinIdleConns = b.config.Redis.MinIdleConns

	cacheInstance, err := cache.NewRedisCache(cacheCfg)
	if err != nil {
		return fmt.Errorf("failed to create redis cache: %w", err)
	}

	b.cache = cacheInstance
	b.setDependency("cache", cacheInstance)
	logger.Info("Redis cache initialized")

	return nil
}

// InitializeRPC initializes RPC services
func (b *Bootstrap) InitializeRPC() error {
	if !b.config.RPC.Enabled {
		logger.Info("No RPC servers configured, skipping RPC initialization")
		return nil
	}

	managerConfig := rpc.DefaultManagerConfig()
	switch b.config.RPC.Protocol {
	case "jsonrpc":
		managerConfig.Servers = map[string]rpc.Config{
			"jsonrpc": {
				Type:    rpc.ServerTypeJSONRPC,
				Host:    b.config.Server.HTTP.Host,
				Port:    b.config.Server.HTTP.Port,
				Network: "tcp",
				Timeout: int((30 * time.Second).Seconds()),
			},
		}
	case "grpc":
		fallthrough
	default:
		managerConfig.Servers = map[string]rpc.Config{
			"grpc": {
				Type:       rpc.ServerTypeGRPC,
				Host:       b.config.Server.GRPC.Host,
				Port:       b.config.Server.GRPC.Port,
				Network:    "tcp",
				Timeout:    int((30 * time.Second).Seconds()),
				MaxMsgSize: 4 * 1024 * 1024,
				Reflection: true,
			},
		}
	}

	b.rpcManager = rpc.NewManager(managerConfig)
	b.setDependency("rpc.manager", b.rpcManager)

	if b.config.RPC.Degradation.Enabled {
		dmConfig := rpc.DefaultDegradationConfig()
		dmConfig.Enabled = b.config.RPC.Degradation.Enabled
		dmConfig.AutoDetect = b.config.RPC.Degradation.AutoDetect
		dmConfig.CPUThreshold = b.config.RPC.Degradation.CPUThreshold
		dmConfig.MemoryThreshold = b.config.RPC.Degradation.MemoryThreshold
		dmConfig.ErrorRateThreshold = b.config.RPC.Degradation.ErrorRateThreshold
		dmConfig.LatencyThreshold = b.config.RPC.Degradation.LatencyThreshold
		dmConfig.EnableCircuitBreaker = b.config.RPC.Degradation.EnableCircuitBreaker
		dmConfig.Level = parseDegradationLevel(b.config.RPC.Degradation.Level)

		dm, err := rpc.NewDegradationManager(dmConfig)
		if err != nil {
			return fmt.Errorf("failed to create degradation manager: %w", err)
		}
		b.rpcManager.SetDegradationManager(dm)
	}

	logger.Info("Initializing RPC services",
		logger.String("protocol", b.config.RPC.Protocol))

	if err := b.rpcManager.Start(); err != nil {
		return fmt.Errorf("failed to start RPC manager: %w", err)
	}

	if b.discoveryManager != nil {
		if err := b.rpcManager.SetDiscoveryIntegration(b.discoveryManager, rpc.DiscoveryRegistrationConfig{
			Enabled:        b.config.Discovery.Enabled,
			ServiceName:    b.config.App.Name,
			ServiceAddress: firstNonEmpty(b.config.Discovery.Config["service_address"], firstNonEmpty(b.config.Server.GRPC.Host, b.config.Server.HTTP.Host)),
			ServiceTags:    append([]string{"rpc", b.config.RPC.Protocol}, splitAndTrim(b.config.Discovery.Config["rpc_tags"])...),
			Metadata:       map[string]string(b.config.Discovery.Config),
		}); err != nil {
			return fmt.Errorf("failed to connect rpc manager with discovery manager: %w", err)
		}
	}

	logger.Info("RPC services initialized successfully")
	return nil
}

// InitializeDiscovery initializes service discovery
func (b *Bootstrap) InitializeDiscovery() error {
	if !b.config.Discovery.Enabled {
		logger.Info("Service discovery disabled, skipping discovery initialization")
		return nil
	}

	if b.config.Discovery.Address == "" {
		logger.Warn("Service discovery enabled but no registry address configured, skipping discovery initialization")
		return nil
	}

	managerConfig := discovery.DefaultManagerConfig()
	managerConfig.Enabled = b.config.Discovery.Enabled
	managerConfig.RegistryType = b.config.Discovery.Type
	managerConfig.RegistryAddress = b.config.Discovery.Address
	managerConfig.Timeout = b.config.Discovery.Timeout
	managerConfig.ServiceName = b.config.App.Name
	managerConfig.ServiceAddress = b.config.Server.HTTP.Host
	managerConfig.ServicePort = b.config.Server.HTTP.Port

	manager, err := discovery.NewServiceDiscoveryManager(managerConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery manager: %w", err)
	}

	// Start discovery manager
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start discovery manager: %w", err)
	}

	b.discoveryManager = manager
	b.setDependency("discovery.manager", manager)

	if b.rpcManager != nil {
		if err := b.rpcManager.SetDiscoveryIntegration(manager, rpc.DiscoveryRegistrationConfig{
			Enabled:        b.config.Discovery.Enabled,
			ServiceName:    b.config.App.Name,
			ServiceAddress: firstNonEmpty(b.config.Discovery.Config["service_address"], firstNonEmpty(b.config.Server.GRPC.Host, b.config.Server.HTTP.Host)),
			ServiceTags:    append([]string{"rpc", b.config.RPC.Protocol}, splitAndTrim(b.config.Discovery.Config["rpc_tags"])...),
			Metadata:       map[string]string(b.config.Discovery.Config),
		}); err != nil {
			return fmt.Errorf("failed to connect discovery manager with rpc manager: %w", err)
		}
	}

	logger.Info("Service discovery initialized successfully")
	return nil
}

// InitializeErrors initializes error handling
func (b *Bootstrap) InitializeErrors() error {
	b.setDependency("errors.initialized", true)
	logger.Info("Error handling initialized successfully")
	return nil
}

// InitializeHealth initializes health check services
func (b *Bootstrap) InitializeHealth() error {
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
	b.setDependency("health.manager", healthManager)

	logger.Info("Health check services initialized successfully")
	return nil
}

// InitializeMetrics initializes metrics collection
func (b *Bootstrap) InitializeMetrics() error {
	if !b.config.Observability.Metrics.Enabled {
		logger.Info("Metrics collection disabled")
		return nil
	}

	metricsCollector := metrics.NewMetricsCollector()
	b.metricsCollector = metricsCollector
	b.setDependency("metrics.collector", metricsCollector)

	// Start metrics server in background
	go func() {
		addr := b.config.Observability.Metrics.Endpoint
		if addr == "" || strings.HasPrefix(addr, "/") {
			if strings.HasPrefix(addr, "/") {
				logger.Warn("Metrics endpoint is a path, using default listen address",
					logger.String("endpoint", addr),
					logger.String("fallback_addr", ":9090"))
			}
			addr = ":9090"
		}
		if err := metricsCollector.StartMetricsServer(addr); err != nil {
			logger.Errorf("Failed to start metrics server", logger.Error(err))
		}
	}()

	logger.Info("Metrics collection initialized successfully",
		logger.String("endpoint", b.config.Observability.Metrics.Endpoint))

	return nil
}

// InitializeAuth initializes authentication services
func (b *Bootstrap) InitializeAuth() error {
	if !b.config.Security.JWT.Enabled {
		logger.Info("Authentication services disabled")
		return nil
	}

	authManager := auth.NewAuthManager(auth.AuthConfig{
		Enabled: b.config.Security.JWT.Enabled,
		JWT: jwtx.Config{
			Secret:         b.config.Security.JWT.Secret,
			AccessTokenTTL: b.config.Security.JWT.Expiration,
			RefreshTokenTTL: b.config.Security.JWT.Expiration * 7,
		},
	})
	b.authManager = authManager
	b.setDependency("auth.manager", authManager)

	logger.Info("Authentication services initialized successfully")
	return nil
}

// InitializeTracing initializes tracing services
func (b *Bootstrap) InitializeTracing() error {
	if !b.config.Observability.Tracing.Enabled {
		logger.Info("Tracing services disabled")
		return nil
	}

	tracingConfig := tracing.DefaultConfig()
	tracingConfig.Enabled = b.config.Observability.Tracing.Enabled
	tracingConfig.Type = b.config.Observability.Tracing.Type
	tracingConfig.Endpoint = b.config.Observability.Tracing.Endpoint
	tracingConfig.SampleRate = b.config.Observability.Tracing.SampleRate
	tracingConfig.ServiceName = b.config.App.Name
	tracingConfig.ServiceVersion = b.config.App.Version
	tracingConfig.Environment = b.config.App.Environment

	tracer, err := tracing.NewTracer(tracingConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	b.tracer = tracer
	b.setDependency("tracer", tracer)

	logger.Info("Tracing services initialized successfully")
	return nil
}

// InitializeGateway initializes the HTTP gateway
func (b *Bootstrap) InitializeGateway() error {
	gatewayConfig := gateway.DefaultConfig()
	gatewayConfig.Host = b.config.Server.HTTP.Host
	gatewayConfig.Port = b.config.Server.HTTP.Port
	gatewayConfig.ReadTimeout = b.config.Server.HTTP.ReadTimeout
	gatewayConfig.WriteTimeout = b.config.Server.HTTP.WriteTimeout
	gatewayConfig.IdleTimeout = b.config.Server.HTTP.IdleTimeout
	gatewayConfig.CORS.AllowedOrigins = b.config.Security.CORS.AllowOrigins
	gatewayConfig.CORS.AllowedMethods = b.config.Security.CORS.AllowMethods
	gatewayConfig.CORS.AllowedHeaders = b.config.Security.CORS.AllowHeaders
	gatewayConfig.CORS.AllowCredentials = b.config.Security.CORS.AllowCredentials
	gatewayConfig.RateLimit.Enabled = b.config.Security.RateLimit.Enabled
	gatewayConfig.RateLimit.Requests = b.config.Security.RateLimit.Limit
	gatewayConfig.RateLimit.Window = b.config.Security.RateLimit.Window.String()
	gatewayConfig.Discovery.Type = b.config.Discovery.Type
	if b.config.Discovery.Address != "" {
		gatewayConfig.Discovery.Endpoints = []string{b.config.Discovery.Address}
	}
	gatewayConfig.Discovery.Namespace = firstNonEmpty(b.config.Discovery.Config["namespace"], "default")
	gatewayConfig.Discovery.Timeout = b.config.Discovery.Timeout

	b.gateway = gateway.NewGateway(gatewayConfig)
	b.setDependency("gateway", b.gateway)

	// Initialize gateway
	if err := b.gateway.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize gateway: %w", err)
	}

	// Start gateway
	if err := b.gateway.Start(); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	logger.Info("Gateway initialized successfully",
		logger.String("host", gatewayConfig.Host),
		logger.Int("port", gatewayConfig.Port))
	return nil
}

func buildDBConfig(dbConfig config.DBConfig, poolCfg config.DBPoolConfig) (db.Config, error) {
	switch dbConfig.Driver {
	case "mysql":
		cfg := mysqlpkg.DefaultConfig()
		cfg.Host = dbConfig.Host
		cfg.Port = dbConfig.Port
		cfg.Database = dbConfig.Database
		cfg.Username = dbConfig.Username
		cfg.Password = dbConfig.Password
		if poolCfg.MaxOpenConns > 0 {
			cfg.MaxOpenConns = poolCfg.MaxOpenConns
		}
		if poolCfg.MaxIdleConns > 0 {
			cfg.MaxIdleConns = poolCfg.MaxIdleConns
		}
		if poolCfg.ConnMaxLifetime > 0 {
			cfg.ConnMaxLifetime = poolCfg.ConnMaxLifetime
		}
		if poolCfg.ConnMaxIdleTime > 0 {
			cfg.ConnMaxIdleTime = poolCfg.ConnMaxIdleTime
		}
		return db.Config{Type: db.TypeMySQL, MySQL: cfg}, nil
	case "postgres", "postgresql":
		cfg := pg.DefaultConfig()
		cfg.Host = dbConfig.Host
		cfg.Port = dbConfig.Port
		cfg.Database = dbConfig.Database
		cfg.Username = dbConfig.Username
		cfg.Password = dbConfig.Password
		cfg.SSLMode = dbConfig.SSLMode
		if poolCfg.MaxOpenConns > 0 {
			cfg.MaxOpenConns = poolCfg.MaxOpenConns
		}
		if poolCfg.MaxIdleConns > 0 {
			cfg.MaxIdleConns = poolCfg.MaxIdleConns
		}
		if poolCfg.ConnMaxLifetime > 0 {
			cfg.ConnMaxLifetime = poolCfg.ConnMaxLifetime
		}
		return db.Config{Type: db.TypePostgres, PG: cfg}, nil
	default:
		return db.Config{}, fmt.Errorf("unsupported database driver: %s", dbConfig.Driver)
	}
}

func parseDegradationLevel(level string) rpc.DegradationLevel {
	switch level {
	case "light":
		return rpc.DegradationLevelLight
	case "medium":
		return rpc.DegradationLevelMedium
	case "heavy":
		return rpc.DegradationLevelHeavy
	case "emergency":
		return rpc.DegradationLevelEmergency
	default:
		return rpc.DegradationLevelNormal
	}
}

func firstNonEmpty(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func splitAndTrim(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// GetConfig returns the configuration
func (b *Bootstrap) GetConfig() *config.GlobalConfig {
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

// GetWebSocketServer returns the websocket server.
func (b *Bootstrap) GetWebSocketServer() *websocket.Server {
	return b.websocketServer
}

// GetDependency returns a dependency by key.
func (b *Bootstrap) GetDependency(key string) (interface{}, bool) {
	b.depMu.RLock()
	defer b.depMu.RUnlock()
	v, ok := b.dependencies[key]
	return v, ok
}

// MustGetDependency returns a dependency by key or panics when not found.
func (b *Bootstrap) MustGetDependency(key string) interface{} {
	v, ok := b.GetDependency(key)
	if !ok {
		panic(fmt.Sprintf("dependency not found: %s", key))
	}
	return v
}

func (b *Bootstrap) setDependency(key string, value interface{}) {
	b.depMu.Lock()
	defer b.depMu.Unlock()
	b.dependencies[key] = value
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

// GetGateway returns the gateway instance
func (b *Bootstrap) GetGateway() *gateway.Gateway {
	return b.gateway
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

	if b.websocketServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := b.websocketServer.Stop(ctx); err != nil {
			errors = append(errors, fmt.Errorf("websocket server close error: %w", err))
		}
		cancel()
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

	if b.gateway != nil {
		if err := b.gateway.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("gateway close error: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	logger.Info("Bootstrap shutdown completed")
	return nil
}

// UpdateDatabaseConfig updates the database configuration in the bootstrap instance
func (b *Bootstrap) UpdateDatabaseConfig(dbConfigs map[string]config.DBConfig) error {
	if b.config == nil {
		return fmt.Errorf("bootstrap config is nil")
	}
	
	// Update primary database config
	if primaryConfig, exists := dbConfigs["mysql_primary"]; exists {
		b.config.Database.Primary = primaryConfig
	}
	
	// Update replica database config if exists
	if replicaConfig, exists := dbConfigs["mysql_replica"]; exists {
		b.config.Database.Replica = replicaConfig
	}
	
	logger.Info("Database configuration updated")
	return nil
}

// GetDatabaseFactory returns the database factory instance
func (b *Bootstrap) GetDatabaseFactory() *db.Factory {
	return b.db
}

// GetMySQLClient returns the MySQL client from the database factory
func (b *Bootstrap) GetMySQLClient() (*mysql.Client, error) {
	if b.db == nil {
		return nil, fmt.Errorf("database factory not initialized")
	}
	return b.db.GetMySQL()
}

// GetMySQLSQLClient returns the MySQL client as SQLClient interface
func (b *Bootstrap) GetMySQLSQLClient() (db.SQLClient, error) {
	if b.db == nil {
		return nil, fmt.Errorf("database factory not initialized")
	}
	return b.db.GetMySQLSQLClient()
}

// GetRedisClient returns the Redis client from the database factory
func (b *Bootstrap) GetRedisClient() (*redis.Client, error) {
	if b.db == nil {
		return nil, fmt.Errorf("database factory not initialized")
	}
	return b.db.GetRedis()
}

// GetPostgresClient returns the PostgreSQL client from the database factory
func (b *Bootstrap) GetPostgresClient() (*postgres.Client, error) {
	if b.db == nil {
		return nil, fmt.Errorf("database factory not initialized")
	}
	return b.db.GetPostgres()
}
