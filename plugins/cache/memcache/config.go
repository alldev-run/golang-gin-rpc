package memcache

import "time"

// Config holds Memcached connection configuration
type Config struct {
	// Hosts is the list of memcached servers
	Hosts []string `yaml:"hosts" json:"hosts"`
	
	// MaxIdleConns is the maximum number of idle connections per server
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`
	
	// Timeout for connection and operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// MaxRetries for failed operations
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	
	// DialTimeout for establishing new connections
	DialTimeout time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	
	// ReadTimeout for read operations
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout"`
	
	// WriteTimeout for write operations
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	
	// IdleTimeout for idle connections
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	
	// HashAlgorithm for key distribution
	HashAlgorithm string `yaml:"hash_algorithm" json:"hash_algorithm"`
	
	// KeyPrefix for all keys
	KeyPrefix string `yaml:"key_prefix" json:"key_prefix"`
	
	// CompressionEnabled indicates if compression is enabled
	CompressionEnabled bool `yaml:"compression_enabled" json:"compression_enabled"`
	
	// SerializationFormat for values
	SerializationFormat string `yaml:"serialization_format" json:"serialization_format"`
	
	// HealthCheckInterval for checking server health
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	
	// FailoverEnabled indicates if automatic failover is enabled
	FailoverEnabled bool `yaml:"failover_enabled" json:"failover_enabled"`
	
	// Enabled indicates if Memcached is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns default Memcached configuration
func DefaultConfig() Config {
	return Config{
		Hosts:              []string{"localhost:11211"},
		MaxIdleConns:       2,
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		IdleTimeout:        5 * time.Minute,
		HashAlgorithm:      "md5",
		KeyPrefix:          "",
		CompressionEnabled: false,
		SerializationFormat: "json",
		HealthCheckInterval: time.Minute,
		FailoverEnabled:    true,
		Enabled:            true,
	}
}

// ClusterConfig returns configuration for Memcached cluster
func ClusterConfig() Config {
	return Config{
		Hosts:              []string{
			"localhost:11211",
			"localhost:11212",
			"localhost:11213",
		},
		MaxIdleConns:       5,
		Timeout:            3 * time.Second,
		MaxRetries:         5,
		DialTimeout:        3 * time.Second,
		ReadTimeout:        2 * time.Second,
		WriteTimeout:       2 * time.Second,
		IdleTimeout:        3 * time.Minute,
		HashAlgorithm:      "md5",
		KeyPrefix:          "",
		CompressionEnabled: true,
		SerializationFormat: "json",
		HealthCheckInterval: 30 * time.Second,
		FailoverEnabled:    true,
		Enabled:            true,
	}
}

// HighPerformanceConfig returns configuration for high performance scenarios
func HighPerformanceConfig() Config {
	return Config{
		Hosts:              []string{"localhost:11211"},
		MaxIdleConns:       10,
		Timeout:            1 * time.Second,
		MaxRetries:         2,
		DialTimeout:        1 * time.Second,
		ReadTimeout:        500 * time.Millisecond,
		WriteTimeout:       500 * time.Millisecond,
		IdleTimeout:        2 * time.Minute,
		HashAlgorithm:      "crc32",
		KeyPrefix:          "",
		CompressionEnabled: true,
		SerializationFormat: "gob",
		HealthCheckInterval: 30 * time.Second,
		FailoverEnabled:    true,
		Enabled:            true,
	}
}

// DevelopmentConfig returns configuration for development
func DevelopmentConfig() Config {
	return Config{
		Hosts:              []string{"localhost:11211"},
		MaxIdleConns:       1,
		Timeout:            10 * time.Second,
		MaxRetries:         1,
		DialTimeout:        10 * time.Second,
		ReadTimeout:        5 * time.Second,
		WriteTimeout:       5 * time.Second,
		IdleTimeout:        10 * time.Minute,
		HashAlgorithm:      "md5",
		KeyPrefix:          "dev:",
		CompressionEnabled: false,
		SerializationFormat: "json",
		HealthCheckInterval: 5 * time.Minute,
		FailoverEnabled:    false,
		Enabled:            true,
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if len(c.Hosts) == 0 {
		c.Hosts = []string{"localhost:11211"}
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 2
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 5 * time.Minute
	}
	if c.HashAlgorithm == "" {
		c.HashAlgorithm = "md5"
	}
	if c.SerializationFormat == "" {
		c.SerializationFormat = "json"
	}
	if c.HealthCheckInterval == 0 {
		c.HealthCheckInterval = time.Minute
	}
	return nil
}
