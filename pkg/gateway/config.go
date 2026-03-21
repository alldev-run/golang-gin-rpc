package gateway

import (
	"time"
	"alldev-gin-rpc/pkg/tracing"
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
	}
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level" json:"level"`
	
	// Log format (json, console)
	Format string `yaml:"format" json:"format"`
}
