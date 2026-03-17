package discovery

import "time"

// RegistryType represents the type of service registry
type RegistryType string

const (
	RegistryTypeConsul RegistryType = "consul"
	RegistryTypeEtcd   RegistryType = "etcd"
	RegistryTypeZk     RegistryType = "zookeeper"
	RegistryTypeStatic RegistryType = "static"
)

// Config holds service discovery configuration
type Config struct {
	// Type is the registry type (consul, etcd, zookeeper, static)
	Type RegistryType `yaml:"type" json:"type"`
	
	// Address is the registry server address
	Address string `yaml:"address" json:"address"`
	
	// Namespace for services
	Namespace string `yaml:"namespace" json:"namespace"`
	
	// Timeout for operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// Username for authentication (optional)
	Username string `yaml:"username" json:"username"`
	
	// Password for authentication (optional)
	Password string `yaml:"password" json:"password"`
	
	// Token for authentication (optional)
	Token string `yaml:"token" json:"token"`
	
	// HealthCheckInterval for service health checks
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	
	// DeregisterCriticalServiceAfter for auto-deregistration
	DeregisterCriticalServiceAfter time.Duration `yaml:"deregister_critical_service_after" json:"deregister_critical_service_after"`
	
	// Options additional configuration options
	Options map[string]interface{} `yaml:"options" json:"options"`
	
	// Enabled indicates if service discovery is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns default service discovery configuration
func DefaultConfig() Config {
	return Config{
		Type:                          RegistryTypeConsul,
		Address:                       "localhost:8500",
		Namespace:                     "default",
		Timeout:                       5 * time.Second,
		Username:                      "",
		Password:                      "",
		Token:                         "",
		HealthCheckInterval:           30 * time.Second,
		DeregisterCriticalServiceAfter: 24 * time.Hour,
		Options:                       make(map[string]interface{}),
		Enabled:                       true,
	}
}

// ConsulConfig returns Consul-specific configuration
func ConsulConfig(address string) Config {
	return Config{
		Type:                          RegistryTypeConsul,
		Address:                       address,
		Namespace:                     "default",
		Timeout:                       5 * time.Second,
		Username:                      "",
		Password:                      "",
		Token:                         "",
		HealthCheckInterval:           30 * time.Second,
		DeregisterCriticalServiceAfter: 24 * time.Hour,
		Options: map[string]interface{}{
			"datacenter": "dc1",
			"scheme":     "http",
		},
		Enabled: true,
	}
}

// EtcdConfig returns etcd-specific configuration
func EtcdConfig(address string) Config {
	return Config{
		Type:                          RegistryTypeEtcd,
		Address:                       address,
		Namespace:                     "default",
		Timeout:                       5 * time.Second,
		Username:                      "",
		Password:                      "",
		Token:                         "",
		HealthCheckInterval:           30 * time.Second,
		DeregisterCriticalServiceAfter: 24 * time.Hour,
		Options: map[string]interface{}{
			"dial_timeout":   5 * time.Second,
			"operation_timeout": 5 * time.Second,
		},
		Enabled: true,
	}
}

// ZookeeperConfig returns Zookeeper-specific configuration
func ZookeeperConfig(address string) Config {
	return Config{
		Type:                          RegistryTypeZk,
		Address:                       address,
		Namespace:                     "default",
		Timeout:                       5 * time.Second,
		Username:                      "",
		Password:                      "",
		Token:                         "",
		HealthCheckInterval:           30 * time.Second,
		DeregisterCriticalServiceAfter: 24 * time.Hour,
		Options: map[string]interface{}{
			"session_timeout": 30 * time.Second,
			"base_path":       "/services",
		},
		Enabled: true,
	}
}

// StaticConfig returns static configuration (no registry)
func StaticConfig() Config {
	return Config{
		Type:    RegistryTypeStatic,
		Address: "",
		Namespace: "default",
		Timeout: 5 * time.Second,
		Options: make(map[string]interface{}),
		Enabled: true,
	}
}

// DevelopmentConfig returns development-friendly configuration
func DevelopmentConfig() Config {
	return Config{
		Type:                          RegistryTypeConsul,
		Address:                       "localhost:8500",
		Namespace:                     "development",
		Timeout:                       10 * time.Second,
		Username:                      "",
		Password:                      "",
		Token:                         "",
		HealthCheckInterval:           10 * time.Second,
		DeregisterCriticalServiceAfter: time.Hour,
		Options: map[string]interface{}{
			"datacenter": "dc1",
			"scheme":     "http",
		},
		Enabled: true,
	}
}

// ProductionConfig returns production-friendly configuration
func ProductionConfig(address string) Config {
	return Config{
		Type:                          RegistryTypeConsul,
		Address:                       address,
		Namespace:                     "production",
		Timeout:                       3 * time.Second,
		Username:                      "",
		Password:                      "",
		Token:                         "",
		HealthCheckInterval:           15 * time.Second,
		DeregisterCriticalServiceAfter: 12 * time.Hour,
		Options: map[string]interface{}{
			"datacenter": "prod-dc1",
			"scheme":     "https",
		},
		Enabled: true,
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Type == "" {
		c.Type = RegistryTypeConsul
	}
	if c.Address == "" && c.Type != RegistryTypeStatic {
		switch c.Type {
		case RegistryTypeConsul:
			c.Address = "localhost:8500"
		case RegistryTypeEtcd:
			c.Address = "localhost:2379"
		case RegistryTypeZk:
			c.Address = "localhost:2181"
		}
	}
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.HealthCheckInterval == 0 {
		c.HealthCheckInterval = 30 * time.Second
	}
	if c.DeregisterCriticalServiceAfter == 0 {
		c.DeregisterCriticalServiceAfter = 24 * time.Hour
	}
	if c.Options == nil {
		c.Options = make(map[string]interface{})
	}
	return nil
}
