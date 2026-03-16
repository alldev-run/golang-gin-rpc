package tracing

import (
	"context"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	"golang-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

// InitFromFile initializes tracing from a YAML configuration file
func InitFromFile(configPath string) error {
	config, err := LoadConfigFromFile(configPath)
	if err != nil {
		return err
	}
	return InitGlobalTracer(config)
}

// LoadConfigFromFile loads tracing configuration from a YAML file
func LoadConfigFromFile(configPath string) (Config, error) {
	config := DefaultConfig()
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}
	
	return config, nil
}

// InitWithDefaults initializes tracing with default configuration
func InitWithDefaults(serviceName string) error {
	config := DefaultConfig()
	config.ServiceName = serviceName
	return InitGlobalTracer(config)
}

// InitForProduction initializes tracing with production-friendly defaults
func InitForProduction(serviceName, zipkinURL string) error {
	config := Config{
		ServiceName:       serviceName,
		ServiceVersion:    "1.0.0",
		Environment:       "production",
		Enabled:           true,
		ZipkinURL:         zipkinURL,
		SampleRate:        0.1, // Sample 10% of traces in production
		BatchTimeout:      10 * time.Second,
		MaxExportBatchSize: 1024,
	}
	return InitGlobalTracer(config)
}

// InitForDevelopment initializes tracing with development-friendly defaults
func InitForDevelopment(serviceName string) error {
	config := Config{
		ServiceName:       serviceName,
		ServiceVersion:    "1.0.0",
		Environment:       "development",
		Enabled:           true,
		ZipkinURL:         "http://localhost:9411/api/v2/spans",
		SampleRate:        1.0, // Sample 100% of traces in development
		BatchTimeout:      5 * time.Second,
		MaxExportBatchSize: 512,
	}
	return InitGlobalTracer(config)
}
