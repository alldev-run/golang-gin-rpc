package gateway

import (
	"time"
)

// Config holds gateway configuration
type Config struct {
	// Server configuration
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" json:"idle_timeout"`

	// CORS configuration
	CORS CORSConfig `yaml:"cors" json:"cors"`

	// Rate limiting
	RateLimit RateLimitConfig `yaml:"rate_limit" json:"rate_limit"`

	// Service discovery
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`

	// Load balancing
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer" json:"load_balancer"`

	// Routes
	Routes []RouteConfig `yaml:"routes" json:"routes"`
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
}

// DefaultConfig returns default gateway configuration
func DefaultConfig() *Config {
	return &Config{
		Host:         "0.0.0.0",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
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
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy: "round_robin",
		},
		Routes: []RouteConfig{},
	}
}
