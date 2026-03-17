package redis

import (
	"fmt"
	"time"
)

// Mode defines the Redis operation mode
type Mode string

const (
	// ModeSingle single instance mode
	ModeSingle Mode = "single"
	// ModeCluster cluster mode
	ModeCluster Mode = "cluster"
	// ModeSentinel sentinel mode for HA
	ModeSentinel Mode = "sentinel"
	// ModeMasterSlave master-slave mode for read/write splitting
	ModeMasterSlave Mode = "master_slave"
	// ModeMulti multi-instance mode for business sharding
	ModeMulti Mode = "multi"
)

// NodeConfig defines a single Redis node configuration
type NodeConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Password string `yaml:"password" json:"password"`
	Database int    `yaml:"database" json:"database"`
	// Weight for read splitting (master-slave mode)
	Weight int `yaml:"weight" json:"weight"`
	// IsMaster indicates if this is master node (master-slave mode)
	IsMaster bool `yaml:"is_master" json:"is_master"`
}

// Config holds Redis connection configuration
type Config struct {
	// Mode is the Redis operation mode: single, cluster, sentinel, master_slave, multi
	Mode Mode `yaml:"mode" json:"mode"`

	// Nodes is the list of Redis nodes (for cluster, sentinel, master_slave, multi modes)
	Nodes []NodeConfig `yaml:"nodes" json:"nodes"`

	// Legacy single instance config (for backward compatibility)
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

	// PoolSize is the maximum number of connections per node
	PoolSize int `yaml:"pool_size" json:"pool_size"`

	// MinIdleConns is the minimum number of idle connections per node
	MinIdleConns int `yaml:"min_idle_conns" json:"min_idle_conns"`

	// MaxIdleConns is the maximum number of idle connections per node
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

	// Sentinel specific config
	Sentinel struct {
		// MasterName is the name of the master to use
		MasterName string `yaml:"master_name" json:"master_name"`
		// SentinelAddrs is the list of sentinel addresses
		SentinelAddrs []string `yaml:"sentinel_addrs" json:"sentinel_addrs"`
		// SentinelPassword is the password for sentinel
		SentinelPassword string `yaml:"sentinel_password" json:"sentinel_password"`
	} `yaml:"sentinel" json:"sentinel"`

	// Cluster specific config
	Cluster struct {
		// MaxRedirects is the maximum number of redirects to follow
		MaxRedirects int `yaml:"max_redirects" json:"max_redirects"`
		// ReadOnly allows routing read-only commands to slave nodes
		ReadOnly bool `yaml:"read_only" json:"read_only"`
		// RouteByLatency routes commands to the node with the lowest latency
		RouteByLatency bool `yaml:"route_by_latency" json:"route_by_latency"`
		// RouteRandomly routes commands randomly to nodes
		RouteRandomly bool `yaml:"route_randomly" json:"route_randomly"`
	} `yaml:"cluster" json:"cluster"`

	// Multi specific config for business sharding
	Multi struct {
		// ShardingStrategy is the strategy for sharding: hash, prefix, range
		ShardingStrategy string `yaml:"sharding_strategy" json:"sharding_strategy"`
		// KeyPrefixRoutes maps key prefixes to node indices
		KeyPrefixRoutes map[string]int `yaml:"key_prefix_routes" json:"key_prefix_routes"`
		// DefaultNode is the default node index for keys not matching any prefix
		DefaultNode int `yaml:"default_node" json:"default_node"`
	} `yaml:"multi" json:"multi"`
}

// DefaultConfig returns default Redis configuration
func DefaultConfig() Config {
	return Config{
		Mode:               ModeSingle,
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
		Cluster: struct {
			MaxRedirects   int  `yaml:"max_redirects" json:"max_redirects"`
			ReadOnly       bool `yaml:"read_only" json:"read_only"`
			RouteByLatency bool `yaml:"route_by_latency" json:"route_by_latency"`
			RouteRandomly  bool `yaml:"route_randomly" json:"route_randomly"`
		}{
			MaxRedirects:   3,
			ReadOnly:       false,
			RouteByLatency: false,
			RouteRandomly:  false,
		},
	}
}

// ClusterConfig returns configuration for Redis Cluster
func ClusterConfig() Config {
	return Config{
		Mode:               ModeCluster,
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
		Cluster: struct {
			MaxRedirects   int  `yaml:"max_redirects" json:"max_redirects"`
			ReadOnly       bool `yaml:"read_only" json:"read_only"`
			RouteByLatency bool `yaml:"route_by_latency" json:"route_by_latency"`
			RouteRandomly  bool `yaml:"route_randomly" json:"route_randomly"`
		}{
			MaxRedirects:   3,
			ReadOnly:       true,
			RouteByLatency: false,
			RouteRandomly:  false,
		},
	}
}

// SentinelConfig returns configuration for Redis Sentinel
func SentinelConfig() Config {
	return Config{
		Mode:               ModeSentinel,
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
		Sentinel: struct {
			MasterName       string   `yaml:"master_name" json:"master_name"`
			SentinelAddrs    []string `yaml:"sentinel_addrs" json:"sentinel_addrs"`
			SentinelPassword string   `yaml:"sentinel_password" json:"sentinel_password"`
		}{
			MasterName:       "mymaster",
			SentinelAddrs:    []string{"localhost:26379"},
			SentinelPassword: "",
		},
	}
}

// MultiConfig returns configuration for Redis Multi-Instance sharding mode
func MultiConfig() Config {
	return Config{
		Mode:               ModeMulti,
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
		Nodes: []NodeConfig{
			{Host: "localhost", Port: 6379, Password: "", Database: 0}, // Instance 1 - default
			{Host: "localhost", Port: 6380, Password: "", Database: 0}, // Instance 2 - users
			{Host: "localhost", Port: 6381, Password: "", Database: 0}, // Instance 3 - orders
		},
		Multi: struct {
			ShardingStrategy string         `yaml:"sharding_strategy" json:"sharding_strategy"`
			KeyPrefixRoutes  map[string]int `yaml:"key_prefix_routes" json:"key_prefix_routes"`
			DefaultNode      int            `yaml:"default_node" json:"default_node"`
		}{
			ShardingStrategy: "prefix", // hash, prefix, range
			KeyPrefixRoutes: map[string]int{
				"user:":  1, // user data -> instance 1
				"order:": 2, // order data -> instance 2
			},
			DefaultNode: 0, // default -> instance 0
		},
	}
}
func MasterSlaveConfig() Config {
	return Config{
		Mode:               ModeMasterSlave,
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
		Nodes: []NodeConfig{
			{Host: "localhost", Port: 6379, IsMaster: true, Weight: 0},  // Master for writes
			{Host: "localhost", Port: 6380, IsMaster: false, Weight: 1}, // Slave 1 for reads
			{Host: "localhost", Port: 6381, IsMaster: false, Weight: 1}, // Slave 2 for reads
		},
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Mode == "" {
		c.Mode = ModeSingle
	}

	// Validate based on mode
	switch c.Mode {
	case ModeSingle:
		if c.Host == "" {
			c.Host = "localhost"
		}
		if c.Port == 0 {
			c.Port = 6379
		}
	case ModeCluster, ModeMasterSlave, ModeMulti:
		if len(c.Nodes) == 0 {
			// Fallback to single config if no nodes
			c.Nodes = []NodeConfig{
				{Host: c.Host, Port: c.Port, Password: c.Password, Database: c.Database},
			}
		}
		// Set defaults for nodes
		for i := range c.Nodes {
			if c.Nodes[i].Host == "" {
				c.Nodes[i].Host = "localhost"
			}
			if c.Nodes[i].Port == 0 {
				c.Nodes[i].Port = 6379
			}
		}
		// Multi mode specific defaults
		if c.Mode == ModeMulti {
			if c.Multi.ShardingStrategy == "" {
				c.Multi.ShardingStrategy = "hash"
			}
			if c.Multi.DefaultNode < 0 || c.Multi.DefaultNode >= len(c.Nodes) {
				c.Multi.DefaultNode = 0
			}
		}
	case ModeSentinel:
		if c.Sentinel.MasterName == "" {
			c.Sentinel.MasterName = "mymaster"
		}
		if len(c.Sentinel.SentinelAddrs) == 0 {
			c.Sentinel.SentinelAddrs = []string{fmt.Sprintf("%s:%d", c.Host, c.Port)}
		}
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
