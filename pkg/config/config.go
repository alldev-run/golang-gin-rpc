// Package config provides unified configuration management for all modules
package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Loader handles configuration loading from multiple sources
type Loader struct {
	mu       sync.RWMutex
	config   *GlobalConfig
	watchers []func(*GlobalConfig)
}

// GlobalConfig holds all module configurations
type GlobalConfig struct {
	// Application basic config
	App AppConfig `yaml:"app" json:"app"`
	
	// Server configs
	Server ServerConfig `yaml:"server" json:"server"`
	
	// Database configs
	Database DatabaseConfig `yaml:"database" json:"database"`
	
	// Redis cache configs  
	Redis RedisConfig `yaml:"redis" json:"redis"`
	
	// RPC configs
	RPC RPCConfig `yaml:"rpc" json:"rpc"`
	
	// Service discovery
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`
	
	// Message queue
	Messaging MessagingConfig `yaml:"messaging" json:"messaging"`
	
	// Observability
	Observability ObservabilityConfig `yaml:"observability" json:"observability"`
	
	// Security
	Security SecurityConfig `yaml:"security" json:"security"`
}

// AppConfig application basic configuration
type AppConfig struct {
	Name        string            `yaml:"name" json:"name"`
	Version     string            `yaml:"version" json:"version"`
	Environment string            `yaml:"environment" json:"environment"`
	Debug       bool              `yaml:"debug" json:"debug"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
}

// ServerConfig server configuration
type ServerConfig struct {
	HTTP HTTPConfig `yaml:"http" json:"http"`
	GRPC GRPCConfig `yaml:"grpc" json:"grpc"`
}

// HTTPConfig HTTP server configuration
type HTTPConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	MaxHeaderBytes  int           `yaml:"max_header_bytes" json:"max_header_bytes"`
}

// GRPCConfig gRPC server configuration
type GRPCConfig struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Host        string        `yaml:"host" json:"host"`
	Port        int           `yaml:"port" json:"port"`
	MaxConnAge  time.Duration `yaml:"max_conn_age" json:"max_conn_age"`
	Keepalive   KeepaliveConfig `yaml:"keepalive" json:"keepalive"`
}

// KeepaliveConfig keepalive settings
type KeepaliveConfig struct {
	Enabled               bool          `yaml:"enabled" json:"enabled"`
	MaxConnectionIdle     time.Duration `yaml:"max_connection_idle" json:"max_connection_idle"`
	MaxConnectionAge      time.Duration `yaml:"max_connection_age" json:"max_connection_age"`
	MaxConnectionAgeGrace time.Duration `yaml:"max_connection_age_grace" json:"max_connection_age_grace"`
	Time                  time.Duration `yaml:"time" json:"time"`
	Timeout               time.Duration `yaml:"timeout" json:"timeout"`
}

// DatabaseConfig database configuration
type DatabaseConfig struct {
	Primary DBConfig            `yaml:"primary" json:"primary"`
	Replica DBConfig            `yaml:"replica" json:"replica"`
	Pool    DBPoolConfig        `yaml:"pool" json:"pool"`
}

// DBConfig single database configuration
type DBConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Driver   string `yaml:"driver" json:"driver"`
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	SSLMode  string `yaml:"ssl_mode" json:"ssl_mode"`
}

// DBPoolConfig database pool configuration
type DBPoolConfig struct {
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

// RedisConfig Redis configuration
type RedisConfig struct {
	Mode           string           `yaml:"mode" json:"mode"`
	Host           string           `yaml:"host" json:"host"`
	Port           int              `yaml:"port" json:"port"`
	Password       string           `yaml:"password" json:"password"`
	Database       int              `yaml:"database" json:"database"`
	PoolSize       int              `yaml:"pool_size" json:"pool_size"`
	MinIdleConns   int              `yaml:"min_idle_conns" json:"min_idle_conns"`
	Nodes          []RedisNode      `yaml:"nodes" json:"nodes"`
}

// RedisNode Redis cluster/sentinel node
type RedisNode struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Password string `yaml:"password" json:"password"`
	Database int    `yaml:"database" json:"database"`
	IsMaster bool   `yaml:"is_master" json:"is_master"`
}

// RPCConfig RPC configuration
type RPCConfig struct {
	Enabled     bool            `yaml:"enabled" json:"enabled"`
	Protocol    string          `yaml:"protocol" json:"protocol"`
	Degradation DegradationCfg  `yaml:"degradation" json:"degradation"`
}

// DegradationCfg RPC degradation configuration
type DegradationCfg struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	Level               string        `yaml:"level" json:"level"`
	AutoDetect          bool          `yaml:"auto_detect" json:"auto_detect"`
	CPUThreshold        float64       `yaml:"cpu_threshold" json:"cpu_threshold"`
	MemoryThreshold     float64       `yaml:"memory_threshold" json:"memory_threshold"`
	ErrorRateThreshold  float64       `yaml:"error_rate_threshold" json:"error_rate_threshold"`
	LatencyThreshold    time.Duration `yaml:"latency_threshold" json:"latency_threshold"`
	EnableCircuitBreaker bool       `yaml:"enable_circuit_breaker" json:"enable_circuit_breaker"`
}

// DiscoveryConfig service discovery configuration
type DiscoveryConfig struct {
	Enabled  bool              `yaml:"enabled" json:"enabled"`
	Type     string            `yaml:"type" json:"type"`
	Address  string            `yaml:"address" json:"address"`
	Timeout  time.Duration     `yaml:"timeout" json:"timeout"`
	Config   map[string]string `yaml:"config" json:"config"`
}

// MessagingConfig message queue configuration
type MessagingConfig struct {
	Enabled bool       `yaml:"enabled" json:"enabled"`
	Type    string     `yaml:"type" json:"type"`
	Broker BrokerConfig `yaml:"broker" json:"broker"`
}

// BrokerConfig message broker configuration
type BrokerConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

// ObservabilityConfig observability configuration
type ObservabilityConfig struct {
	Metrics  MetricsConfig  `yaml:"metrics" json:"metrics"`
	Tracing  TracingConfig  `yaml:"tracing" json:"tracing"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
	Alerting AlertingConfig `yaml:"alerting" json:"alerting"`
}

// MetricsConfig metrics configuration
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Type       string `yaml:"type" json:"type"`
	Endpoint   string `yaml:"endpoint" json:"endpoint"`
	Namespace  string `yaml:"namespace" json:"namespace"`
}

// TracingConfig tracing configuration
type TracingConfig struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Type        string        `yaml:"type" json:"type"`
	Endpoint    string        `yaml:"endpoint" json:"endpoint"`
	SampleRate  float64       `yaml:"sample_rate" json:"sample_rate"`
	BatchSize   int           `yaml:"batch_size" json:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
}

// LoggingConfig logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`
	Format     string `yaml:"format" json:"format"`
	Output     string `yaml:"output" json:"output"`
	FilePath   string `yaml:"file_path" json:"file_path"`
	MaxSize    int    `yaml:"max_size" json:"max_size"`
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `yaml:"max_age" json:"max_age"`
}

// AlertingConfig alerting configuration
type AlertingConfig struct {
	Enabled  bool            `yaml:"enabled" json:"enabled"`
	Channels []AlertChannel  `yaml:"channels" json:"channels"`
}

// AlertChannel alert channel configuration
type AlertChannel struct {
	Type   string            `yaml:"type" json:"type"`
	Name   string            `yaml:"name" json:"name"`
	Config map[string]string `yaml:"config" json:"config"`
}

// SecurityConfig security configuration
type SecurityConfig struct {
	JWT      JWTConfig      `yaml:"jwt" json:"jwt"`
	APIKey   APIKeyConfig   `yaml:"api_key" json:"api_key"`
	CORS     CORSConfig     `yaml:"cors" json:"cors"`
	RateLimit RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
}

// JWTConfig JWT configuration
type JWTConfig struct {
	Enabled    bool          `yaml:"enabled" json:"enabled"`
	Secret     string        `yaml:"secret" json:"secret"`
	Algorithm  string        `yaml:"algorithm" json:"algorithm"`
	Expiration time.Duration `yaml:"expiration" json:"expiration"`
}

// APIKeyConfig API key configuration
type APIKeyConfig struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	Keys       []string `yaml:"keys" json:"keys"`
	HeaderName string   `yaml:"header_name" json:"header_name"`
}

// CORSConfig CORS configuration
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled" json:"enabled"`
	AllowOrigins     []string `yaml:"allow_origins" json:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods" json:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers" json:"allow_headers"`
	AllowCredentials bool     `yaml:"allow_credentials" json:"allow_credentials"`
}

// RateLimitConfig rate limiting configuration
type RateLimitConfig struct {
	Enabled bool          `yaml:"enabled" json:"enabled"`
	Type    string        `yaml:"type" json:"type"`
	Limit   int           `yaml:"limit" json:"limit"`
	Window  time.Duration `yaml:"window" json:"window"`
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		config:   &GlobalConfig{},
		watchers: make([]func(*GlobalConfig), 0),
	}
}

// Load loads configuration from file
func (l *Loader) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := yaml.Unmarshal(data, l.config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply environment variable overrides
	l.applyEnvOverrides(l.config)

	// Validate configuration
	if err := l.validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

// Get returns the current configuration
func (l *Loader) Get() *GlobalConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// Set sets the configuration
func (l *Loader) Set(config *GlobalConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = config
	l.notifyWatchers()
}

// Watch registers a callback for config changes
func (l *Loader) Watch(callback func(*GlobalConfig)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.watchers = append(l.watchers, callback)
}

// notifyWatchers notifies all watchers of config changes
func (l *Loader) notifyWatchers() {
	for _, watcher := range l.watchers {
		go watcher(l.config)
	}
}

// applyEnvOverrides applies environment variable overrides to config
func (l *Loader) applyEnvOverrides(config interface{}) {
	v := reflect.ValueOf(config)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	
	v = v.Elem()
	t := v.Type()
	
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		
		// Skip unexported fields
		if !field.CanSet() {
			continue
		}
		
		envTag := fieldType.Tag.Get("env")
		if envTag != "" {
			if value := os.Getenv(envTag); value != "" {
				l.setFieldValue(field, value)
			}
		}
		
		// Recursively process nested structs
		if field.Kind() == reflect.Struct {
			l.applyEnvOverrides(field.Addr().Interface())
		}
	}
}

// setFieldValue sets a field value from string
func (l *Loader) setFieldValue(field reflect.Value, value string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Handle time.Duration which is int64
		if field.Type().String() == "time.Duration" {
			if duration, err := time.ParseDuration(value); err == nil {
				field.Set(reflect.ValueOf(duration))
			}
		} else if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			field.SetInt(intVal)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintVal, err := strconv.ParseUint(value, 10, 64); err == nil {
			field.SetUint(uintVal)
		}
	case reflect.Float32, reflect.Float64:
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			field.SetFloat(floatVal)
		}
	case reflect.Bool:
		if boolVal, err := strconv.ParseBool(value); err == nil {
			field.SetBool(boolVal)
		}
	}
}

// validate validates the configuration
func (l *Loader) validate() error {
	config := l.config
	
	// Validate app config
	if config.App.Name == "" {
		return fmt.Errorf("app name is required")
	}
	
	// Validate server config
	if config.Server.HTTP.Enabled && config.Server.HTTP.Port == 0 {
		return fmt.Errorf("http port is required when enabled")
	}
	
	// Validate database config
	if config.Database.Primary.Enabled {
		if config.Database.Primary.Host == "" {
			return fmt.Errorf("database host is required")
		}
	}
	
	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *GlobalConfig {
	return &GlobalConfig{
		App: AppConfig{
			Name:        "golang-gin-rpc",
			Version:     "1.0.0",
			Environment: "development",
			Debug:       true,
			Labels:      map[string]string{},
		},
		Server: ServerConfig{
			HTTP: HTTPConfig{
				Enabled:        true,
				Host:           "0.0.0.0",
				Port:           8080,
				ReadTimeout:    30 * time.Second,
				WriteTimeout:   30 * time.Second,
				IdleTimeout:    120 * time.Second,
				MaxHeaderBytes: 1 << 20,
			},
			GRPC: GRPCConfig{
				Enabled: true,
				Host:    "0.0.0.0",
				Port:    9090,
				Keepalive: KeepaliveConfig{
					Enabled: true,
				},
			},
		},
		Database: DatabaseConfig{
			Primary: DBConfig{
				Enabled:  true,
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				Database: "app",
				SSLMode:  "disable",
			},
			Pool: DBPoolConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    10,
				ConnMaxLifetime: 5 * time.Minute,
				ConnMaxIdleTime: 10 * time.Minute,
			},
		},
		Redis: RedisConfig{
			Mode:         "single",
			Host:         "localhost",
			Port:         6379,
			Database:     0,
			PoolSize:     10,
			MinIdleConns: 2,
		},
		RPC: RPCConfig{
			Enabled:  true,
			Protocol: "grpc",
			Degradation: DegradationCfg{
				Enabled:              true,
				Level:                "normal",
				AutoDetect:           true,
				CPUThreshold:         80.0,
				MemoryThreshold:      85.0,
				ErrorRateThreshold:   50.0,
				LatencyThreshold:     5 * time.Second,
				EnableCircuitBreaker: true,
			},
		},
		Discovery: DiscoveryConfig{
			Enabled: false,
			Type:    "consul",
		},
		Messaging: MessagingConfig{
			Enabled: false,
			Type:    "kafka",
		},
		Observability: ObservabilityConfig{
			Metrics: MetricsConfig{
				Enabled:   true,
				Type:      "prometheus",
				Endpoint:  "/metrics",
				Namespace: "app",
			},
			Tracing: TracingConfig{
				Enabled:       false,
				Type:          "jaeger",
				SampleRate:    0.1,
				BatchSize:     100,
				FlushInterval: time.Second,
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			Alerting: AlertingConfig{
				Enabled: false,
			},
		},
		Security: SecurityConfig{
			JWT: JWTConfig{
				Enabled:    false,
				Algorithm:  "HS256",
				Expiration: 24 * time.Hour,
			},
			CORS: CORSConfig{
				Enabled:          true,
				AllowOrigins:     []string{"*"},
				AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowHeaders:     []string{"*"},
				AllowCredentials: false,
			},
			RateLimit: RateLimitConfig{
				Enabled: false,
				Type:    "token_bucket",
				Limit:   100,
				Window:  time.Minute,
			},
		},
	}
}

// MustLoad loads configuration or panics
func MustLoad(path string) *GlobalConfig {
	loader := NewLoader()
	if err := loader.Load(path); err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	return loader.Get()
}
