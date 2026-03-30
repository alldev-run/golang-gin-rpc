package gateway

import (
	"time"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
)

// Config holds gateway configuration
type Config struct {
	// Server configuration
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`

	// Service name for tracing and monitoring
	ServiceName string `yaml:"service_name" json:"service_name"`

	// CORS configuration
	CORS CORSConfig `yaml:"cors" json:"cors"`

	// Rate limiting
	RateLimit RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`

	// Service discovery
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`

	// Load balancing
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer" json:"load_balancer"`

	// Tracing configuration
	Tracing *tracing.Config `yaml:"tracing" json:"tracing"`

	// Protocol support
	Protocols ProtocolConfig `yaml:"protocols" json:"protocols"`

	// Routes
	Routes []RouteConfig `yaml:"routes" json:"routes"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Audit configuration
	Audit AuditConfig `yaml:"audit" json:"audit"`
}

// AuditConfig holds audit middleware configuration.
type AuditConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	SkipPaths     []string `yaml:"skip_paths" json:"skip_paths"`
	SensitiveKeys []string `yaml:"sensitive_keys" json:"sensitive_keys"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins" json:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods" json:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers" json:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers" json:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials" json:"allow_credentials"`
	MaxAge           int      `yaml:"max_age" json:"max_age"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled bool    `yaml:"enabled" json:"enabled"`
	Requests int    `yaml:"requests" json:"requests"`
	Window   string `yaml:"window" json:"window"` // duration string like "1m", "30s"
}

// DiscoveryConfig holds service discovery configuration
type DiscoveryConfig struct {
	Type      string            `yaml:"type" json:"type"` // "consul", "etcd", "static"
	Endpoints []string          `yaml:"endpoints" json:"endpoints"`
	Namespace string            `yaml:"namespace" json:"namespace"`
	Timeout   time.Duration     `yaml:"timeout" json:"timeout"`
	Options   map[string]string `yaml:"options" json:"options"`
	Enabled   bool              `yaml:"enabled" json:"enabled"` // 是否启用服务发现
}

// LoadBalancerConfig holds load balancer configuration
type LoadBalancerConfig struct {
	Strategy string `yaml:"strategy" json:"strategy"` // "round_robin", "random", "weighted", "least_connections"
}

// AuthConfig holds RPC authentication configuration
type AuthConfig struct {
	// Enabled indicates if RPC authentication is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Type indicates the RPC authentication type (apikey, jwt, oauth2)
	Type string `yaml:"type" json:"type"`
	
	// HeaderName is the header name for API key (default: X-API-Key)
	HeaderName string `yaml:"header_name" json:"header_name"`
	
	// QueryName is the query parameter name for API key (default: api_key)
	QueryName string `yaml:"query_name" json:"query_name"`
	
	// SkipPaths are RPC paths that skip authentication
	SkipPaths []string `yaml:"skip_paths" json:"skip_paths"`
	
	// SkipMethods are HTTP methods that skip RPC authentication
	SkipMethods []string `yaml:"skip_methods" json:"skip_methods"`
	
	// APIKeys is a map of valid RPC API keys (key -> description/user)
	APIKeys map[string]string `yaml:"api_keys" json:"api_keys"`
}

// SecurityConfig holds RPC security configuration
type SecurityConfig struct {
	// RPC authentication configuration
	Auth AuthConfig `yaml:"auth" json:"auth"`
	
	// TLS configuration for transport layer security
	TLS TLSConfig `yaml:"tls" json:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	// Enable TLS for the gateway
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// TLS certificate file path
	CertFile string `yaml:"cert_file" json:"cert_file"`
	
	// TLS key file path
	KeyFile string `yaml:"key_file" json:"key_file"`
	
	// CA certificate file path
	CAFile string `yaml:"ca_file" json:"ca_file"`
	
	// Server name for TLS verification
	ServerName string `yaml:"server_name" json:"server_name"`
	
	// Insecure connection (skip certificate verification)
	Insecure bool `yaml:"insecure" json:"insecure"`
	
	// Client certificate file path (for mutual TLS)
	ClientCertFile string `yaml:"client_cert_file" json:"client_cert_file"`
	
	// Client key file path (for mutual TLS)
	ClientKeyFile string `yaml:"client_key_file" json:"client_key_file"`
}

// RouteConfig holds route configuration
type RouteConfig struct {
	Path        string            `yaml:"path" json:"path"`
	Method      string            `yaml:"method" json:"method"`
	Service     string            `yaml:"service" json:"service"`
	Targets     []string          `yaml:"targets" json:"targets"`
	StripPrefix bool              `yaml:"strip_prefix" json:"strip_prefix"`
	Timeout     time.Duration     `yaml:"timeout" json:"timeout"`
	Retries     int               `yaml:"retries" json:"retries"`
	Headers     map[string]string `yaml:"headers" json:"headers"`
	Query       map[string]string `yaml:"query" json:"query"`
	Protocol    string            `yaml:"protocol" json:"protocol"` // "http", "grpc", "jsonrpc"
}

// ProtocolConfig holds protocol support configuration
type ProtocolConfig struct {
	// Enable HTTP/1.1 support
	HTTP bool `yaml:"http" json:"http"`
	
	// Enable HTTP/2 support
	HTTP2 bool `yaml:"http2" json:"http2"`
	
	// Enable gRPC proxy support
	GRPC bool `yaml:"grpc" json:"grpc"`
	
	// Enable JSON-RPC proxy support
	JSONRPC bool `yaml:"jsonrpc" json:"jsonrpc"`
	
	// gRPC configuration
	GRPCConfig GRPCConfig `yaml:"grpc_config" json:"grpc_config"`
	
	// JSON-RPC configuration
	JSONRPCConfig JSONRPCConfig `yaml:"jsonrpc_config" json:"jsonrpc_config"`
	
	// RPC security configuration
	Security SecurityConfig `yaml:"security" json:"security"`
}

// GRPCConfig holds gRPC specific configuration
type GRPCConfig struct {
	// Enable TLS for gRPC
	EnableTLS bool `yaml:"enable_tls" json:"enable_tls"`
	
	// TLS certificate file path
	CertFile string `yaml:"cert_file" json:"cert_file"`
	
	// TLS key file path
	KeyFile string `yaml:"key_file" json:"key_file"`
	
	// CA certificate file path
	CAFile string `yaml:"ca_file" json:"ca_file"`
	
	// Server name for TLS verification
	ServerName string `yaml:"server_name" json:"server_name"`
	
	// Insecure connection (skip certificate verification)
	Insecure bool `yaml:"insecure" json:"insecure"`
	
	// Connection timeout
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// JSONRPCConfig holds JSON-RPC specific configuration
type JSONRPCConfig struct {
	// JSON-RPC version (2.0 or 1.0)
	Version string `yaml:"version" json:"version"`
	
	// Enable batch requests
	EnableBatch bool `yaml:"enable_batch" json:"enable_batch"`
	
	// Request timeout
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// Custom headers for JSON-RPC requests
	Headers map[string]string `yaml:"headers" json:"headers"`
}

// DefaultConfig returns default gateway configuration
func DefaultConfig() *Config {
	return &Config{
		Host:         "0.0.0.0",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		ServiceName: "gateway",
		CORS: CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: false,
			MaxAge:           86400,
		},
		RateLimit: RateLimitConfig{
			Enabled: false,
			Requests: 100,
			Window: "1m",
		},
		Discovery: DiscoveryConfig{
			Type:      "static",
			Endpoints: []string{},
			Namespace: "default",
			Timeout:   5 * time.Second,
			Options:   make(map[string]string),
			Enabled:   false, // 默认关闭服务发现
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy: "round_robin",
		},
		Tracing: &tracing.Config{
			Type:        "jaeger",
			ServiceName: "gateway",
			Enabled:     false,
			Host:        "localhost",
			Port:        6831,
			SampleRate:  1.0,
		},
		Protocols: ProtocolConfig{
			HTTP:    true,
			HTTP2:   true,
			GRPC:    false,
			JSONRPC: false,
			GRPCConfig: GRPCConfig{
				EnableTLS: false,
				Timeout:   30 * time.Second,
			},
			JSONRPCConfig: JSONRPCConfig{
				Version:     "2.0",
				EnableBatch: false,
				Timeout:     30 * time.Second,
			},
			Security: SecurityConfig{
				Auth: AuthConfig{
					APIKeys:      make(map[string]string), // 空配置，避免写死
					HeaderName:   "X-API-Key",
					QueryName:    "api_key",
					SkipPaths:    []string{"/health", "/ready", "/info", "/debug/*"},
					SkipMethods:  []string{"OPTIONS"},
					Enabled:      false, // 默认禁用
					Type:         "apikey",
				},
				TLS: TLSConfig{
					Enabled:         false,
					Insecure:        false,
				},
			},
		},
		Routes: []RouteConfig{
			{
				Path:     "/api/*",
				Method:   "*",
				Protocol: "http",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Audit: AuditConfig{
			Enabled:       true,
			SkipPaths:     []string{"/health", "/ready", "/metrics"},
			SensitiveKeys: []string{"password", "token", "authorization", "api_key", "secret"},
		},
	}
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level" json:"level"`
	
	// Log format (json, console)
	Format string `yaml:"format" json:"format"`
	
	// HTTP request logging configuration
	HTTPLogging *HTTPLoggingConfig `yaml:"http_logging" json:"http_logging"`
	
	// Service logging configuration
	ServiceLogging *ServiceLoggingConfig `yaml:"service_logging" json:"service_logging"`
}

// ServiceLoggingConfig holds service-specific logging configuration
type ServiceLoggingConfig struct {
	// Enable service-specific logging
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Base directory for service logs
	BaseDir string `yaml:"base_dir" json:"base_dir"`
	
	// Enable date-based folders
	EnableDateFolders bool `yaml:"enable_date_folders" json:"enable_date_folders"`
	
	// Separate files by log level
	SeparateByLevel bool `yaml:"separate_by_level" json:"separate_by_level"`
	
	// Inherit global logger configuration
	InheritGlobalConfig bool `yaml:"inherit_global_config" json:"inherit_global_config"`
	
	// Override configuration
	OverrideConfig OverrideLoggerConfig `yaml:"override_config" json:"override_config"`
	
	// Component-specific configurations
	Components map[string]ComponentLoggingConfig `yaml:"components" json:"components"`
	
	// Cleanup configuration
	Cleanup CleanupConfig `yaml:"cleanup" json:"cleanup"`
}

// OverrideLoggerConfig holds logger configuration overrides
type OverrideLoggerConfig struct {
	Level               string `yaml:"level" json:"level"`
	Format              string `yaml:"format" json:"format"`
	MaxSize             int    `yaml:"max_size" json:"max_size"`
	MaxBackups          int    `yaml:"max_backups" json:"max_backups"`
	MaxAge              int    `yaml:"max_age" json:"max_age"`
	Compress            bool   `yaml:"compress" json:"compress"`
	EnableCaller        bool   `yaml:"enable_caller" json:"enable_caller"`
	EnableStacktrace    bool   `yaml:"enable_stacktrace" json:"enable_stacktrace"`
	TimeFormat          string `yaml:"time_format" json:"time_format"`
	Sampling            SamplingConfig `yaml:"sampling" json:"sampling"`
}

// ComponentLoggingConfig holds component-specific logging configuration
type ComponentLoggingConfig struct {
	BaseDir        string              `yaml:"base_dir" json:"base_dir"`
	SeparateByLevel bool                `yaml:"separate_by_level" json:"separate_by_level"`
	OverrideConfig OverrideLoggerConfig `yaml:"override_config" json:"override_config"`
}

// CleanupConfig holds log cleanup configuration
type CleanupConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	Schedule          string   `yaml:"schedule" json:"schedule"`
	MaxAgeDays        int      `yaml:"max_age_days" json:"max_age_days"`
	ExcludeComponents []string `yaml:"exclude_components" json:"exclude_components"`
	CompressOld       bool     `yaml:"compress_old" json:"compress_old"`
}

// SamplingConfig holds log sampling configuration
type SamplingConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Rate       float64 `yaml:"rate" json:"rate"`
	Tick       string `yaml:"tick" json:"tick"`
	Initial    int    `yaml:"initial" json:"initial"`
	Thereafter int    `yaml:"thereafter" json:"thereafter"`
}

// HTTPLoggingConfig holds HTTP request logging configuration
type HTTPLoggingConfig struct {
	// Enable HTTP request logging
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Log request body
	LogRequestBody bool `yaml:"log_request_body" json:"log_request_body"`
	
	// Log response body
	LogResponseBody bool `yaml:"log_response_body" json:"log_response_body"`
	
	// Maximum body size to log (in bytes)
	MaxBodySize int64 `yaml:"max_body_size" json:"max_body_size"`
	
	// Log request headers
	LogHeaders bool `yaml:"log_headers" json:"log_headers"`
	
	// Sensitive headers to mask
	SensitiveHeaders []string `yaml:"sensitive_headers" json:"sensitive_headers"`
	
	// Skip logging for these paths
	SkipPaths []string `yaml:"skip_paths" json:"skip_paths"`
	
	// Slow request threshold (duration string, e.g., "1s", "500ms")
	SlowRequestThreshold string `yaml:"slow_request_threshold" json:"slow_request_threshold"`
	
	// Enable request ID generation
	EnableRequestID bool `yaml:"enable_request_id" json:"enable_request_id"`
	
	// Custom request ID header name (default: X-Request-ID)
	RequestIDHeader string `yaml:"request_id_header" json:"request_id_header"`
	
	// Log level thresholds
	LogLevelThresholds LogLevelThresholds `yaml:"log_level_thresholds" json:"log_level_thresholds"`
}

// LogLevelThresholds defines status code to log level mapping
type LogLevelThresholds struct {
	// Status codes >= this level will be logged as ERROR
	ErrorThreshold int `yaml:"error_threshold" json:"error_threshold"`
	
	// Status codes >= this level will be logged as WARN
	WarnThreshold int `yaml:"warn_threshold" json:"warn_threshold"`
	
	// Status codes >= this level will be logged as INFO
	InfoThreshold int `yaml:"info_threshold" json:"info_threshold"`
}
