package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alldev-run/golang-gin-rpc/bootstrap/pluginconfig"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/kernel"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/pkg/version"
	"github.com/alldev-run/golang-gin-rpc/plugins/actor"
	"github.com/alldev-run/golang-gin-rpc/plugins/auth"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache"
	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/postgres"
	"github.com/alldev-run/golang-gin-rpc/plugins/discovery"
	"github.com/alldev-run/golang-gin-rpc/plugins/gateway"
	"github.com/alldev-run/golang-gin-rpc/plugins/health"
	"github.com/alldev-run/golang-gin-rpc/plugins/metrics"
	"github.com/alldev-run/golang-gin-rpc/plugins/rpc"
	"github.com/alldev-run/golang-gin-rpc/plugins/tracing"
	"github.com/alldev-run/golang-gin-rpc/plugins/upload"
	"github.com/alldev-run/golang-gin-rpc/plugins/websocket"
)

// LoadConfig loads configuration from file using pkg/config
func LoadConfig(configPath string) (*config.GlobalConfig, error) {
	_, cfg, err := loadConfigWithLoader(configPath)
	return cfg, err
}

// loadConfigWithLoader 返回 loader 与解析后的配置；loader 保留原始 yaml 段，
// 供插件式组件按需反序列化自己的配置。
func loadConfigWithLoader(configPath string) (*config.Loader, *config.GlobalConfig, error) {
	loader := config.NewLoader()

	// Set defaults first
	loader.Set(config.DefaultConfig())

	// Load from file if exists
	if configPath != "" {
		if err := loader.Load(configPath); err != nil {
			return nil, nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	return loader, loader.Get(), nil
}

// Bootstrap handles application initialization
type Bootstrap struct {
	config           *config.GlobalConfig
	loader           *config.Loader
	db               *db.Factory
	cache            cache.Cache
	rpcManager       *rpc.Manager
	websocketServer  *websocket.Server
	websocketManager *websocket.Manager
	actorSystem      *actor.System
	discoveryManager *discovery.ServiceDiscoveryManager
	healthManager    *health.HealthManager
	metricsCollector *metrics.MetricsCollector
	authManager      *auth.AuthManager
	tracer           *tracing.Tracer
	gateway          *gateway.Gateway
	uploader         *upload.Uploader

	serviceMu        sync.RWMutex
	serviceFactories map[string]ServiceFactory
	managedServices  map[string]ManagedService
	serviceOrder     []string
	serviceDeps      map[string][]string

	pluginMu  sync.RWMutex
	pluginApp *kernel.App
	// pluginConfig holds the built-in plugin sections (redis, rpc, discovery, ...).
	pluginConfig *pluginconfig.Config
	registry     *kernel.Registry
	stoppedSet   map[string]bool
	// skippedComponents 记录因自身配置未启用而主动跳过初始化的内置组件。
	skippedComponents map[string]bool

	depMu        sync.RWMutex
	dependencies map[string]interface{}

	serviceDBConfigLoaded bool
}

// pluginStarted 返回内置组件插件是否已随框架启动。
func (b *Bootstrap) pluginStarted(name string) bool {
	b.pluginMu.RLock()
	defer b.pluginMu.RUnlock()
	return b.pluginApp != nil && b.pluginApp.Started(name)
}

// markComponentSkipped 记录某内置组件因自身配置未启用而主动跳过初始化
// （例如 security.jwt.enabled=false 时的 auth 组件）。该组件虽然 Start 成功，
// 但不会向依赖容器注入依赖，依赖覆盖校验应对其豁免。
func (b *Bootstrap) markComponentSkipped(name string) {
	b.pluginMu.Lock()
	defer b.pluginMu.Unlock()
	if b.skippedComponents == nil {
		b.skippedComponents = make(map[string]bool)
	}
	b.skippedComponents[name] = true
}

// componentSkipped 返回组件是否被标记为主动跳过。
func (b *Bootstrap) componentSkipped(name string) bool {
	b.pluginMu.RLock()
	defer b.pluginMu.RUnlock()
	return b.skippedComponents[name]
}

// pluginStopped 返回内置组件插件是否已被插件机制关闭
// （用于 Close 时跳过重复的资源释放）。
func (b *Bootstrap) pluginStopped(name string) bool {
	b.pluginMu.RLock()
	defer b.pluginMu.RUnlock()
	return b.stoppedSet[name]
}

// NewBootstrap creates a new bootstrap instance
func NewBootstrap(configPath string) (*Bootstrap, error) {
	// Load configuration
	loader, cfg, err := loadConfigWithLoader(configPath)
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

	logger.Info("Framework version", logger.String("version", version.GetVersion()))
	// logger.Info("Configuration loaded successfully")

	pluginCfg, err := pluginconfig.Load(loader)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin config: %w", err)
	}

	boot := &Bootstrap{
		config:           cfg,
		pluginConfig:     pluginCfg,
		loader:           loader,
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

	if err := b.InitializeUpload(); err != nil {
		return fmt.Errorf("failed to initialize upload: %w", err)
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

func buildDBConfig(dbConfig config.DBConfig, poolCfg config.DBPoolConfig) (db.Config, error) {
	switch dbConfig.Driver {
	case "mysql":
		cfg := mysql.DefaultConfig()
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
		// Set SQL logging configuration
		if dbConfig.LogEnabled {
			cfg.LogEnabled = true
			if dbConfig.LogLevel != "" {
				cfg.LogLevel = dbConfig.LogLevel
			}
			if dbConfig.SlowQueryThreshold > 0 {
				cfg.SlowQueryThreshold = time.Duration(dbConfig.SlowQueryThreshold)
			}
		}
		return db.Config{Type: db.TypeMySQL, MySQL: cfg}, nil
	case "postgres", "postgresql":
		cfg := postgres.DefaultConfig()
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
		// Set SQL logging configuration
		if dbConfig.LogEnabled {
			cfg.LogEnabled = true
			if dbConfig.LogLevel != "" {
				cfg.LogLevel = dbConfig.LogLevel
			}
			if dbConfig.SlowQueryThreshold > 0 {
				cfg.SlowQueryThreshold = time.Duration(dbConfig.SlowQueryThreshold)
			}
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

// rpcTimeoutOrDefault returns the configured RPC timeout in seconds, falling
// back to 30s when unset.
func rpcTimeoutOrDefault(seconds int) int {
	if seconds > 0 {
		return seconds
	}
	return int((30 * time.Second).Seconds())
}

// rpcMaxMsgSizeOrDefault returns the configured max message size in bytes,
// falling back to 4MB when unset.
func rpcMaxMsgSizeOrDefault(maxMsgSize int) int {
	if maxMsgSize > 0 {
		return maxMsgSize
	}
	return 4 * 1024 * 1024
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

// GetPluginConfig returns the built-in plugin configuration sections.
func (b *Bootstrap) GetPluginConfig() *pluginconfig.Config {
	return b.pluginConfig
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

// GetWebSocketManager returns the websocket connection manager wired by the
// websocket service factory (nil until the service has started).
func (b *Bootstrap) GetWebSocketManager() *websocket.Manager {
	return b.websocketManager
}

// GetActorSystem returns the actor system (nil until the actor plugin starts).
func (b *Bootstrap) GetActorSystem() *actor.System {
	return b.actorSystem
}

// GetDependency returns a dependency by key.
func (b *Bootstrap) GetDependency(key string) (interface{}, bool) {
	b.depMu.RLock()
	defer b.depMu.RUnlock()
	v, ok := b.dependencies[key]
	return v, ok
}

// SetDependency 注入或覆盖依赖容器中的对象，供应用侧注册自定义依赖，
// 或测试时构造容器内容。
func (b *Bootstrap) SetDependency(key string, value interface{}) {
	b.setDependency(key, value)
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

// GetUploader returns the upload uploader
func (b *Bootstrap) GetUploader() *upload.Uploader {
	return b.uploader
}

// shutdownTimeoutOrDefault returns the configured graceful shutdown timeout
// (server.shutdown_timeout), falling back to 30s when unset.
func (b *Bootstrap) shutdownTimeoutOrDefault() time.Duration {
	if b.config != nil && b.config.Server.ShutdownTimeout > 0 {
		return b.config.Server.ShutdownTimeout
	}
	return 30 * time.Second
}

// Close closes all resources
func (b *Bootstrap) Close() error {
	var errors []error

	// 组件级资源（cache/rpc/discovery/tracer）优先交给插件应用逆序停止；
	// 已由插件停止的组件跳过下面的逐项关闭，避免重复 Close。
	if b.pluginStarted("cache") || b.pluginStarted("rpc") || b.pluginStarted("discovery") || b.pluginStarted("tracing") {
		ctx, cancel := context.WithTimeout(context.Background(), b.shutdownTimeoutOrDefault())
		if err := b.stopPluginApp(ctx); err != nil {
			errors = append(errors, fmt.Errorf("plugin app stop error: %w", err))
		}
		cancel()
	}

	if b.cache != nil && !b.pluginStopped("cache") {
		if err := b.cache.Close(); err != nil {
			errors = append(errors, fmt.Errorf("cache close error: %w", err))
		}
	}

	if b.rpcManager != nil && !b.pluginStopped("rpc") {
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

	if b.discoveryManager != nil && !b.pluginStopped("discovery") {
		if err := b.discoveryManager.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("discovery manager close error: %w", err))
		}
	}

	if b.tracer != nil && !b.pluginStopped("tracing") {
		ctx := context.Background()
		if err := b.shutdownTracer(ctx); err != nil {
			errors = append(errors, fmt.Errorf("tracer close error: %w", err))
		}
	}

	if b.gateway != nil {
		if err := b.gateway.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("gateway close error: %w", err))
		}
	}

	if b.actorSystem != nil && !b.pluginStopped("actor") {
		if err := b.actorSystem.Shutdown(); err != nil {
			errors = append(errors, fmt.Errorf("actor system shutdown error: %w", err))
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

	// Mark that service database config has been loaded
	b.serviceDBConfigLoaded = true

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

// GetPostgresClient returns the PostgreSQL client from the database factory
func (b *Bootstrap) GetPostgresClient() (*postgres.Client, error) {
	if b.db == nil {
		return nil, fmt.Errorf("database factory not initialized")
	}
	return b.db.GetPostgres()
}
