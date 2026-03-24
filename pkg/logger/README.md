# Logger Package - Service-Aware Logging with Date-Based Paths

This package provides structured logging capabilities with support for service-specific log files organized by date.

## Features

- **Service-Specific Logging**: Each service gets its own isolated log files
- **Date-Based Organization**: Logs are automatically organized in date folders (YYYY-MM-DD)
- **Level Separation**: Optional separate files for different log levels (debug, info, warn, error)
- **File Rotation**: Built-in log rotation with size limits and retention policies
- **Performance**: High-performance logging using zap and lumberjack
- **Flexible Configuration**: Support for different environments (dev, test, production)
- **Cleanup Utilities**: Automatic cleanup of old log directories

## Directory Structure

```
logs/
├── api-gateway/
│   ├── 2026-03-24/
│   │   ├── api-gateway.log          # Single file mode
│   │   ├── api-gateway.debug.log    # Separate level mode
│   │   ├── api-gateway.info.log
│   │   ├── api-gateway.warn.log
│   │   └── api-gateway.error.log
│   └── 2026-03-25/
├── rpc/
│   ├── 2026-03-24/
│   └── 2026-03-25/
└── websocket/
    ├── 2026-03-24/
    └── 2026-03-25/
```

## Quick Start

### Basic Usage

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/logger"

// Simple service logger with default configuration
apiLogger := logger.GetServiceLogger("api-gateway")
apiLogger.Info("API Gateway started", 
    zap.String("version", "1.0.0"),
    zap.Int("port", 8080))
```

### Custom Configuration

```go
config := logger.DefaultServiceLoggerConfig("rpc")
config.BaseDir = "/var/log/myapp"
config.EnableDateFolder = true
config.SeparateByLevel = true // Separate files for different levels
config.InheritGlobalConfig = false
config.OverrideConfig = logger.ProductionConfig()

rpcLogger := logger.GetServiceLoggerInstance("rpc", config)
```

### Service Logger Wrapper

```go
serviceLogger := logger.NewServiceLogger("websocket")
serviceLogger.With(
    zap.String("component", "connection-manager"),
    zap.Int("connection_id", 12345),
).Info("WebSocket connection established")
```

## Configuration Options

### ServiceLoggerConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ServiceName` | string | required | Name of the service |
| `BaseDir` | string | "./logs" | Base directory for all logs |
| `EnableDateFolder` | bool | true | Create date-based subdirectories |
| `SeparateByLevel` | bool | false | Create separate files for log levels |
| `InheritGlobalConfig` | bool | true | Inherit global logger settings |
| `OverrideConfig` | Config | DefaultConfig() | Override specific settings |

### Environment-Specific Defaults

#### Development
- Console output with colors
- Debug level logging
- Separate level files enabled
- Text format

#### Production
- File output only
- Info level logging
- Single file per service
- JSON format
- Sampling enabled (1%)

#### Test
- Console output
- Debug level logging
- No date folders
- No file rotation

## API Reference

### Core Functions

```go
// Get service logger with default configuration
func GetServiceLogger(serviceName string) *zap.Logger

// Get service logger with custom configuration
func GetServiceLoggerInstance(serviceName string, config ServiceLoggerConfig) *zap.Logger

// Create service logger wrapper
func NewServiceLogger(serviceName string) *ServiceLogger

// Create service logger wrapper with custom config
func NewServiceLoggerFromConfig(serviceName string, config ServiceLoggerConfig) *ServiceLogger
```

### Utility Functions

```go
// Get current log path for a service
func GetServiceLogPath(serviceName string, config ServiceLoggerConfig) string

// Clean up old log directories
func CleanupOldLogs(baseDir string, maxAgeDays int) error

// Default service logger configuration
func DefaultServiceLoggerConfig(serviceName string) ServiceLoggerConfig
```

### ServiceLogger Wrapper

```go
type ServiceLogger struct {
    serviceName string
    logger      *zap.Logger
}

func (sl *ServiceLogger) Logger() *zap.Logger
func (sl *ServiceLogger) With(fields ...zap.Field) *ServiceLogger
func (sl *ServiceLogger) Debug(msg string, fields ...zap.Field)
func (sl *ServiceLogger) Info(msg string, fields ...zap.Field)
func (sl *ServiceLogger) Warn(msg string, fields ...zap.Field)
func (sl *ServiceLogger) Error(msg string, fields ...zap.Field)
func (sl *ServiceLogger) Fatal(msg string, fields ...zap.Field)
func (sl *ServiceLogger) Panic(msg string, fields ...zap.Field)
```

## Integration Examples

### HTTP Service

```go
type HTTPService struct {
    logger *zap.Logger
}

func NewHTTPService() *HTTPService {
    config := logger.DefaultServiceLoggerConfig("http-service")
    config.SeparateByLevel = true // Detailed logging for HTTP
    
    return &HTTPService{
        logger: logger.GetServiceLoggerInstance("http-service", config),
    }
}

func (s *HTTPService) HandleRequest(r *http.Request) {
    requestLogger := s.logger.With(
        zap.String("method", r.Method),
        zap.String("path", r.URL.Path),
        zap.String("remote_addr", r.RemoteAddr),
    )
    
    requestLogger.Info("Handling request")
    // Handle request...
    requestLogger.Info("Request completed")
}
```

### RPC Service

```go
type RPCService struct {
    logger *zap.Logger
}

func NewRPCService() *RPCService {
    config := logger.DefaultServiceLoggerConfig("rpc")
    config.BaseDir = "/var/log/rpc"
    config.EnableDateFolder = true
    
    return &RPCService{
        logger: logger.GetServiceLoggerInstance("rpc", config),
    }
}
```

## Log Cleanup

Implement periodic cleanup to manage disk space:

```go
// Run cleanup daily
func startLogCleanup() {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Keep logs for 30 days
            err := logger.CleanupOldLogs("/var/log/myapp", 30)
            if err != nil {
                logger.L().Error("Log cleanup failed", zap.Error(err))
            }
        }
    }
}
```

## Performance Considerations

- Log files are automatically rotated based on size (default: 100MB)
- Old log files are compressed to save space
- Sampling can be enabled in production to reduce volume
- Service loggers are cached and reused
- File operations are buffered for performance

## Migration from Global Logger

Replace:
```go
logger.Info("Message", zap.String("key", "value"))
```

With:
```go
serviceLogger := logger.GetServiceLogger("service-name")
serviceLogger.Info("Message", zap.String("key", "value"))
```

## Testing

The package includes comprehensive tests covering:
- Service logger creation and configuration
- Date-based folder structure
- Level separation
- Multiple services
- Log cleanup functionality
- Performance benchmarks

Run tests:
```bash
go test ./pkg/logger/... -v
```

## Best Practices

1. **Use descriptive service names** (e.g., "api-gateway", "user-service")
2. **Enable level separation in development** for easier debugging
3. **Configure appropriate retention policies** based on storage capacity
4. **Use structured fields** instead of formatted strings
5. **Implement periodic cleanup** to manage disk space
6. **Monitor log file sizes** and adjust rotation settings as needed
7. **Use sampling in production** to reduce log volume while maintaining visibility
