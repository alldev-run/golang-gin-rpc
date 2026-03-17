package tracing

import "time"

// Config holds tracing configuration
type Config struct {
	// Type specifies the tracing backend (zipkin, jaeger, otlp, etc.)
	Type string `yaml:"type" json:"type"`
	
	// ServiceName is the name of the service
	ServiceName string `yaml:"service_name" json:"service_name"`
	
	// ServiceVersion is the version of the service
	ServiceVersion string `yaml:"service_version" json:"service_version"`
	
	// Environment is the deployment environment
	Environment string `yaml:"environment" json:"environment"`
	
	// Enabled indicates if tracing is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Host is the tracing server host
	Host string `yaml:"host" json:"host"`
	
	// Port is the tracing server port
	Port int `yaml:"port" json:"port"`
	
	// Endpoint is the tracing server endpoint
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	
	// Username for authentication (optional)
	Username string `yaml:"username" json:"username"`
	
	// Password for authentication (optional)
	Password string `yaml:"password" json:"password"`
	
	// SampleRate is the sampling rate (0.0 to 1.0)
	SampleRate float64 `yaml:"sample_rate" json:"sample_rate"`
	
	// BatchTimeout is the batch timeout for span export
	BatchTimeout time.Duration `yaml:"batch_timeout" json:"batch_timeout"`
	
	// MaxExportBatchSize is the maximum batch size for span export
	MaxExportBatchSize int `yaml:"max_export_batch_size" json:"max_export_batch_size"`
	
	// Additional options for the tracer
	Options map[string]interface{} `yaml:"options" json:"options"`
}

// DefaultConfig returns default tracing configuration
func DefaultConfig() Config {
	return Config{
		Type:               "zipkin",
		ServiceName:        "alldev-gin-rpc",
		ServiceVersion:     "1.0.0",
		Environment:        "development",
		Enabled:            false,
		Host:               "localhost",
		Port:               9411,
		Endpoint:           "/api/v2/spans",
		SampleRate:         1.0, // Sample 100% of traces in development
		BatchTimeout:       5 * time.Second,
		MaxExportBatchSize: 512,
		Options:            make(map[string]interface{}),
	}
}

// ProductionConfig returns production-friendly tracing configuration
func ProductionConfig(serviceName string) Config {
	return Config{
		Type:               "zipkin",
		ServiceName:        serviceName,
		ServiceVersion:     "1.0.0",
		Environment:        "production",
		Enabled:            true,
		Host:               "zipkin.internal",
		Port:               9411,
		Endpoint:           "/api/v2/spans",
		SampleRate:         0.1, // Sample 10% of traces in production
		BatchTimeout:       10 * time.Second,
		MaxExportBatchSize: 1024,
		Options:            make(map[string]interface{}),
	}
}

// DevelopmentConfig returns development-friendly tracing configuration
func DevelopmentConfig(serviceName string) Config {
	return Config{
		Type:               "zipkin",
		ServiceName:        serviceName,
		ServiceVersion:     "1.0.0",
		Environment:        "development",
		Enabled:            false, // Disabled by default in development
		Host:               "localhost",
		Port:               9411,
		Endpoint:           "/api/v2/spans",
		SampleRate:         1.0, // Sample 100% of traces when enabled
		BatchTimeout:       5 * time.Second,
		MaxExportBatchSize: 512,
		Options:            make(map[string]interface{}),
	}
}

