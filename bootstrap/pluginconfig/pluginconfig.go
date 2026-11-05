// Package pluginconfig holds the configuration sections of the built-in
// plugins wired by the full bootstrap (redis, rpc, discovery, messaging,
// security). They are read from the same YAML file as pkg/config.GlobalConfig
// and keep the same top-level keys, so the core config package does not need
// to know any plugin's configuration shape.
package pluginconfig

import (
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/kernel"
)

// Config groups the built-in plugin sections.
type Config struct {
	Redis     RedisConfig     `yaml:"redis" json:"redis"`
	RPC       RPCConfig       `yaml:"rpc" json:"rpc"`
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`
	Messaging MessagingConfig `yaml:"messaging" json:"messaging"`
	Security  SecurityConfig  `yaml:"security" json:"security"`
}

// Default returns the built-in defaults for every section.
func Default() *Config {
	return &Config{
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

// Load overlays the sections found in src onto Default(). A nil src, or a
// source without a given section, keeps the defaults for that section.
func Load(src kernel.ConfigSource) (*Config, error) {
	cfg := Default()
	if src == nil {
		return cfg, nil
	}
	sections := []struct {
		name string
		out  any
	}{
		{"redis", &cfg.Redis},
		{"rpc", &cfg.RPC},
		{"discovery", &cfg.Discovery},
		{"messaging", &cfg.Messaging},
		{"security", &cfg.Security},
	}
	for _, s := range sections {
		if _, err := src.UnmarshalSection(s.name, s.out); err != nil {
			return nil, fmt.Errorf("pluginconfig: %w", err)
		}
	}
	return cfg, nil
}

// RedisConfig Redis configuration
type RedisConfig struct {
	Mode         string      `yaml:"mode" json:"mode"`
	Host         string      `yaml:"host" json:"host"`
	Port         int         `yaml:"port" json:"port"`
	Password     string      `yaml:"password" json:"password"`
	Database     int         `yaml:"database" json:"database"`
	PoolSize     int         `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int         `yaml:"min_idle_conns" json:"min_idle_conns"`
	Nodes        []RedisNode `yaml:"nodes" json:"nodes"`
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
	Enabled     bool           `yaml:"enabled" json:"enabled"`
	Protocol    string         `yaml:"protocol" json:"protocol"`
	Degradation DegradationCfg `yaml:"degradation" json:"degradation"`
}

// DegradationCfg RPC degradation configuration
type DegradationCfg struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	Level                string        `yaml:"level" json:"level"`
	AutoDetect           bool          `yaml:"auto_detect" json:"auto_detect"`
	CPUThreshold         float64       `yaml:"cpu_threshold" json:"cpu_threshold"`
	MemoryThreshold      float64       `yaml:"memory_threshold" json:"memory_threshold"`
	ErrorRateThreshold   float64       `yaml:"error_rate_threshold" json:"error_rate_threshold"`
	LatencyThreshold     time.Duration `yaml:"latency_threshold" json:"latency_threshold"`
	EnableCircuitBreaker bool          `yaml:"enable_circuit_breaker" json:"enable_circuit_breaker"`
}

// DiscoveryConfig service discovery configuration
type DiscoveryConfig struct {
	Enabled bool              `yaml:"enabled" json:"enabled"`
	Type    string            `yaml:"type" json:"type"`
	Address string            `yaml:"address" json:"address"`
	Timeout time.Duration     `yaml:"timeout" json:"timeout"`
	Config  map[string]string `yaml:"config" json:"config"`
}

// MessagingConfig message queue configuration
type MessagingConfig struct {
	Enabled bool         `yaml:"enabled" json:"enabled"`
	Type    string       `yaml:"type" json:"type"`
	Broker  BrokerConfig `yaml:"broker" json:"broker"`
}

// BrokerConfig message broker configuration
type BrokerConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

// SecurityConfig security configuration
type SecurityConfig struct {
	JWT       JWTConfig       `yaml:"jwt" json:"jwt"`
	APIKey    APIKeyConfig    `yaml:"api_key" json:"api_key"`
	CORS      CORSConfig      `yaml:"cors" json:"cors"`
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
