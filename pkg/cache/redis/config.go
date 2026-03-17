package redis

import "time"

// Config holds Redis connection configuration
type Config struct {
	// Host is the Redis server host
	Host string `yaml:"host" json:"host"`
	
	// Port is the Redis server port
	Port int `yaml:"port" json:"port"`
	
	// Password for authentication (optional)
	Password string `yaml:"password" json:"password"`
	
	// Database is the Redis database number
	Database int `yaml:"database" json:"database"`
	
	// KeyPrefix is the prefix for all keys
	KeyPrefix string `yaml:"key_prefix" json:"key_prefix"`
	
	// Timeout for connection and operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// MaxRetries for failed operations
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	
	// PoolSize is the maximum number of connections
	PoolSize int `yaml:"pool_size" json:"pool_size"`
	
	// MinIdleConns is the minimum number of idle connections
	MinIdleConns int `yaml:"min_idle_conns" json:"min_idle_conns"`
	
	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int `yaml:"max_idle_conns" json:"max_idle_conns"`
	
	// ConnMaxIdleTime is the maximum idle time for connections
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
	
	// ConnMaxLifetime is the maximum lifetime for connections
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	
	// DialTimeout for establishing new connections
	DialTimeout time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	
	// ReadTimeout for read operations
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout"`
	
	// WriteTimeout for write operations
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	
	// PoolTimeout for getting connection from pool
	PoolTimeout time.Duration `yaml:"pool_timeout" json:"pool_timeout"`
	
	// IdleTimeout for idle connections
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	
	// IdleCheckFrequency for checking idle connections
	IdleCheckFrequency time.Duration `yaml:"idle_check_frequency" json:"idle_check_frequency"`
	
	// Enabled indicates if Redis is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns default Redis configuration
func DefaultConfig() Config {
	return Config{
		Host:               "localhost",
		Port:               6379,
		Password:           "",
		Database:           0,
		KeyPrefix:          "",
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		PoolSize:           10,
		MinIdleConns:       2,
		MaxIdleConns:       5,
		ConnMaxIdleTime:    30 * time.Minute,
		ConnMaxLifetime:    time.Hour,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4 * time.Second,
		IdleTimeout:        5 * time.Minute,
		IdleCheckFrequency: time.Minute,
		Enabled:            true,
	}
}

// ClusterConfig returns configuration for Redis Cluster
func ClusterConfig() Config {
	return Config{
		Host:               "localhost",
		Port:               6379,
		Password:           "",
		Database:           0,
		KeyPrefix:          "",
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		PoolSize:           20,
		MinIdleConns:       5,
		MaxIdleConns:       10,
		ConnMaxIdleTime:    30 * time.Minute,
		ConnMaxLifetime:    time.Hour,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4 * time.Second,
		IdleTimeout:        5 * time.Minute,
		IdleCheckFrequency: time.Minute,
		Enabled:            true,
	}
}

// SentinelConfig returns configuration for Redis Sentinel
func SentinelConfig() Config {
	return Config{
		Host:               "localhost",
		Port:               26379, // Sentinel default port
		Password:           "",
		Database:           0,
		KeyPrefix:          "",
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		PoolSize:           10,
		MinIdleConns:       2,
		MaxIdleConns:       5,
		ConnMaxIdleTime:    30 * time.Minute,
		ConnMaxLifetime:    time.Hour,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4 * time.Second,
		IdleTimeout:        5 * time.Minute,
		IdleCheckFrequency: time.Minute,
		Enabled:            true,
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Host == "" {
		c.Host = "localhost"
	}
	if c.Port == 0 {
		c.Port = 6379
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
	if c.MinIdleConns == 0 {
		c.MinIdleConns = 2
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 30 * time.Minute
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = time.Hour
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
	if c.PoolTimeout == 0 {
		c.PoolTimeout = 4 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 5 * time.Minute
	}
	if c.IdleCheckFrequency == 0 {
		c.IdleCheckFrequency = time.Minute
	}
	return nil
}
