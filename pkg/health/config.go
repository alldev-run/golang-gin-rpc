package health

import "time"

// HealthConfig holds configuration for the health service
type HealthConfig struct {
	// Enabled indicates if the health service is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Host is the health service host
	Host string `yaml:"host" json:"host"`
	
	// Port is the health service port
	Port int `yaml:"port" json:"port"`
	
	// Path is the health check endpoint path
	Path string `yaml:"path" json:"path"`
	
	// Timeout for health check operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// CheckInterval for periodic health checks
	CheckInterval time.Duration `yaml:"check_interval" json:"check_interval"`
	
	// Detailed indicates if detailed health information should be returned
	Detailed bool `yaml:"detailed" json:"detailed"`
	
	// Version information to include in health response
	Version string `yaml:"version" json:"version"`
	
	// Build information to include in health response
	Build string `yaml:"build" json:"build"`
	
	// Description of the service
	Description string `yaml:"description" json:"description"`
}

// HealthCheckConfig holds configuration for individual health checks
type HealthCheckConfig struct {
	// Name of the health check
	Name string `yaml:"name" json:"name"`
	
	// Type of health check (http, tcp, redis, mysql, etc.)
	Type string `yaml:"type" json:"type"`
	
	// Target is the check target (URL, host:port, etc.)
	Target string `yaml:"target" json:"target"`
	
	// Timeout for this specific check
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// Interval for this specific check
	Interval time.Duration `yaml:"interval" json:"interval"`
	
	// FailureThreshold before marking as unhealthy
	FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold"`
	
	// SuccessThreshold before marking as healthy again
	SuccessThreshold int `yaml:"success_threshold" json:"success_threshold"`
	
	// Enabled indicates if this check is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Critical indicates if this check is critical for overall health
	Critical bool `yaml:"critical" json:"critical"`
	
	// Additional configuration options
	Options map[string]interface{} `yaml:"options" json:"options"`
}

// DefaultHealthConfig returns default health service configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:     true,
		Host:        "0.0.0.0",
		Port:        8081,
		Path:        "/health",
		Timeout:     5 * time.Second,
		CheckInterval: 30 * time.Second,
		Detailed:    true,
		Version:     "1.0.0",
		Build:       "",
		Description: "AllDev Gin RPC Service",
	}
}

// DefaultHealthCheckConfig returns default health check configuration
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Name:             "default",
		Type:             "http",
		Target:           "http://localhost:8080/health",
		Timeout:          5 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
		Critical:         true,
		Options:          make(map[string]interface{}),
	}
}

// HTTPCheckConfig returns configuration for HTTP health check
func HTTPCheckConfig(name, url string) HealthCheckConfig {
	return HealthCheckConfig{
		Name:             name,
		Type:             "http",
		Target:           url,
		Timeout:          5 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
		Critical:         true,
		Options: map[string]interface{}{
			"method":           "GET",
			"expected_status":  200,
			"follow_redirects": true,
		},
	}
}

// TCPCheckConfig returns configuration for TCP health check
func TCPCheckConfig(name, address string) HealthCheckConfig {
	return HealthCheckConfig{
		Name:             name,
		Type:             "tcp",
		Target:           address,
		Timeout:          3 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
		Critical:         true,
		Options:          make(map[string]interface{}),
	}
}

// RedisCheckConfig returns configuration for Redis health check
func RedisCheckConfig(name, address string) HealthCheckConfig {
	return HealthCheckConfig{
		Name:             name,
		Type:             "redis",
		Target:           address,
		Timeout:          3 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
		Critical:         false,
		Options: map[string]interface{}{
			"database": 0,
			"password": "",
		},
	}
}

// MySQLCheckConfig returns configuration for MySQL health check
func MySQLCheckConfig(name, dsn string) HealthCheckConfig {
	return HealthCheckConfig{
		Name:             name,
		Type:             "mysql",
		Target:           dsn,
		Timeout:          5 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
		Critical:         false,
		Options: map[string]interface{}{
			"max_open_conns": 1,
			"max_idle_conns": 1,
		},
	}
}

// CustomCheckConfig returns configuration for custom health check
func CustomCheckConfig(name string, checkFunc func() error) HealthCheckConfig {
	return HealthCheckConfig{
		Name:             name,
		Type:             "custom",
		Target:           "",
		Timeout:          5 * time.Second,
		Interval:         30 * time.Second,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Enabled:          true,
		Critical:         false,
		Options: map[string]interface{}{
			"check_func": checkFunc,
		},
	}
}

// Validate validates the health service configuration
func (c HealthConfig) Validate() error {
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Port == 0 {
		c.Port = 8081
	}
	if c.Path == "" {
		c.Path = "/health"
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.CheckInterval == 0 {
		c.CheckInterval = 30 * time.Second
	}
	return nil
}

// Validate validates the health check configuration
func (c HealthCheckConfig) Validate() error {
	if c.Name == "" {
		c.Name = "default"
	}
	if c.Type == "" {
		c.Type = "http"
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Second
	}
	if c.Interval == 0 {
		c.Interval = 30 * time.Second
	}
	if c.FailureThreshold == 0 {
		c.FailureThreshold = 3
	}
	if c.SuccessThreshold == 0 {
		c.SuccessThreshold = 2
	}
	if c.Options == nil {
		c.Options = make(map[string]interface{})
	}
	return nil
}
