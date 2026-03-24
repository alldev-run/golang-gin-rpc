package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ExampleUsage demonstrates how to use the new service logger functionality
func ExampleUsage() {
	// Initialize global logger (optional, for fallback)
	Init(ProductionConfig())

	// Example 1: Simple service logger with default configuration
	apiLogger := GetServiceLogger("api-gateway")
	apiLogger.Info("API Gateway started", 
		zap.String("version", "1.0.0"),
		zap.Int("port", 8080))

	// Example 2: Custom service logger configuration
	rpcConfig := DefaultServiceLoggerConfig("rpc")
	rpcConfig.BaseDir = "/var/log/myapp"
	rpcConfig.EnableDateFolder = true
	rpcConfig.SeparateByLevel = true // Separate files for different log levels
	rpcConfig.InheritGlobalConfig = false
	rpcConfig.OverrideConfig = ProductionConfig()

	rpcLogger := GetServiceLoggerInstance("rpc", rpcConfig)
	rpcLogger.Info("RPC server started",
		zap.String("address", ":9090"))
	rpcLogger.Error("RPC connection failed",
		zap.Error(context.DeadlineExceeded))

	// Example 3: Service logger wrapper with additional fields
	wsLogger := NewServiceLogger("websocket")
	wsLoggerWithFields := wsLogger.With(
		zap.String("component", "connection-manager"),
		zap.Int("connection_id", 12345),
	)
	
	wsLoggerWithFields.Info("WebSocket connection established")
	wsLoggerWithFields.Warn("Connection idle timeout")

	// Example 4: Multiple services with different configurations
	services := []string{"api-gateway", "rpc", "websocket", "auth-service", "metrics"}
	
	for _, serviceName := range services {
		config := DefaultServiceLoggerConfig(serviceName)
		
		// Custom configuration per service
		switch serviceName {
		case "api-gateway":
			config.SeparateByLevel = true // More detailed logging for API
		case "rpc":
			config.BaseDir = "/var/log/rpc"
		case "websocket":
			config.EnableDateFolder = false // Single directory for WebSocket logs
		}
		
		serviceLogger := GetServiceLoggerInstance(serviceName, config)
		serviceLogger.Info("Service started",
			zap.String("service", serviceName),
			zap.Time("start_time", time.Now()))
	}

	// Example 5: Cleanup old logs (run this periodically)
	// Clean up logs older than 30 days
	err := CleanupOldLogs("/var/log/myapp", 30)
	if err != nil {
		// Handle error
		GetServiceLogger("cleanup").Error("Failed to cleanup old logs", zap.Error(err))
	}

	// Example 6: Get current log path for monitoring
	logPath := GetServiceLogPath("api-gateway", DefaultServiceLoggerConfig("api-gateway"))
	_ = logPath // Use for monitoring, health checks, etc.
}

// ExampleServiceIntegration shows how to integrate service logger in actual services
type ExampleService struct {
	logger *zap.Logger
}

func NewExampleService(serviceName string) *ExampleService {
	config := DefaultServiceLoggerConfig(serviceName)
	config.EnableDateFolder = true
	config.SeparateByLevel = false
	
	return &ExampleService{
		logger: GetServiceLoggerInstance(serviceName, config),
	}
}

func (s *ExampleService) Start() error {
	s.logger.Info("Service starting",
		zap.String("service", "example-service"),
		zap.Time("start_time", time.Now()))
	
	// Service startup logic here
	
	s.logger.Info("Service started successfully")
	return nil
}

func (s *ExampleService) Stop() error {
	s.logger.Info("Service stopping")
	
	// Service shutdown logic here
	
	s.logger.Info("Service stopped")
	return nil
}

func (s *ExampleService) ProcessRequest(requestID string) error {
	requestLogger := s.logger.With(
		zap.String("request_id", requestID),
		zap.String("method", "process"),
	)
	
	requestLogger.Info("Processing request started")
	
	// Simulate processing
	time.Sleep(100 * time.Millisecond)
	
	requestLogger.Info("Processing request completed")
	return nil
}

// ExampleConfiguration shows how to configure service logging in config files
/*
Example YAML configuration:

logger:
  level: "info"
  env: "production"
  output: "file"
  format: "json"
  log_path: "/var/log/app/app.log"
  max_size: 500
  max_backups: 30
  max_age: 90
  compress: true
  enable_caller: false
  enable_stacktrace: false

# Service-specific logging can be configured in application code
# or extended to support YAML configuration like:

services:
  api-gateway:
    base_dir: "/var/log/api-gateway"
    enable_date_folder: true
    separate_by_level: true
    inherit_global_config: true
    
  rpc:
    base_dir: "/var/log/rpc"
    enable_date_folder: true
    separate_by_level: false
    inherit_global_config: true
    
  websocket:
    base_dir: "/var/log/websocket"
    enable_date_folder: false
    separate_by_level: false
    inherit_global_config: false
    override_config:
      level: "debug"
      max_size: 100
*/
