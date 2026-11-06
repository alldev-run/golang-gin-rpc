// Package bootstrap 是框架的全量便捷组装入口。它经 internal/bootstrap 链接全部
// 内置插件；只需要内核时请直接使用 pkg/kernel。
//
// 两条组装路径：
//  1. 全量便捷入口（本包）：NewBootstrap + StartFramework 经组合根装配 10 个
//     内置插件与托管服务，二进制包含全部内置依赖；
//  2. 最小组装：直接使用 pkg/kernel（依赖闭包不含任何插件），应用按需 import
//     插件模块并 kernel.Register——未引入的插件及其重型 SDK 不进二进制。
//
// 插件对象的类型化访问用 Dependency[T]（应用自行 import 插件包做断言）。
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	internalbootstrap "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

type Bootstrap = internalbootstrap.Bootstrap

type FrameworkOptions = internalbootstrap.FrameworkOptions

type APIGatewayServiceOptions = internalbootstrap.APIGatewayServiceOptions

type WebSocketServiceOptions = internalbootstrap.WebSocketServiceOptions

// Dependency keys accepted by Dependency and Bootstrap.GetDependency.
const (
	DepDBFactory          = internalbootstrap.DepDBFactory
	DepCache              = internalbootstrap.DepCache
	DepRPCManager         = internalbootstrap.DepRPCManager
	DepDiscoveryManager   = internalbootstrap.DepDiscoveryManager
	DepErrorsInitialized  = internalbootstrap.DepErrorsInitialized
	DepHealthManager      = internalbootstrap.DepHealthManager
	DepMetricsCollector   = internalbootstrap.DepMetricsCollector
	DepAuthManager        = internalbootstrap.DepAuthManager
	DepTracer             = internalbootstrap.DepTracer
	DepUploader           = internalbootstrap.DepUploader
	DepActorSystem        = internalbootstrap.DepActorSystem
	DepGateway            = internalbootstrap.DepGateway
	DepGatewayHTTPService = internalbootstrap.DepGatewayHTTPService
	DepWebSocketServer    = internalbootstrap.DepWebSocketServer
	DepWebSocketManager   = internalbootstrap.DepWebSocketManager
)

func NewBootstrap(configPath string) (*Bootstrap, error) {
	return internalbootstrap.NewBootstrap(configPath)
}

func NewBootstrapWithDefaults() (*Bootstrap, error) {
	return internalbootstrap.NewBootstrap("")
}

func New(configPath string) (*Bootstrap, error) {
	return internalbootstrap.NewBootstrap(configPath)
}

func DefaultFrameworkOptions() FrameworkOptions {
	return internalbootstrap.DefaultFrameworkOptions()
}

func StartFramework(ctx context.Context, boot *Bootstrap, options FrameworkOptions) error {
	if boot == nil {
		return nil
	}
	return boot.StartFramework(ctx, options)
}

// StopFramework stops managed services and the plugin app in reverse start order.
func StopFramework(ctx context.Context, boot *Bootstrap, services ...string) error {
	if boot == nil {
		return nil
	}
	return boot.StopFramework(ctx, services...)
}

// RegisterAPIGatewayServiceFactory registers API gateway service factory via package helper.
func RegisterAPIGatewayServiceFactory(boot *Bootstrap, options APIGatewayServiceOptions) error {
	if boot == nil {
		return fmt.Errorf("bootstrap instance is nil")
	}
	return boot.RegisterAPIGatewayServiceFactory(options)
}

// RegisterWebSocketServiceFactory registers WebSocket service factory via package helper.
func RegisterWebSocketServiceFactory(boot *Bootstrap, options WebSocketServiceOptions) error {
	if boot == nil {
		return fmt.Errorf("bootstrap instance is nil")
	}
	return boot.RegisterWebSocketServiceFactory(options)
}

// LoadDatabaseConfig loads database configuration from a YAML file and updates the bootstrap instance
func LoadDatabaseConfig(boot *Bootstrap, dbConfigPath string) error {
	if boot == nil {
		return fmt.Errorf("bootstrap instance is nil")
	}

	data, err := os.ReadFile(dbConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read database config file: %w", err)
	}

	// Parse raw database config to handle nested structure
	var rawDBConfigs map[string]interface{}
	if err := yaml.Unmarshal(data, &rawDBConfigs); err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}

	// Convert to framework format
	dbConfigs := make(map[string]config.DBConfig)
	for name, rawConfig := range rawDBConfigs {
		if configMap, ok := rawConfig.(map[string]interface{}); ok {
			dbConfig := config.DBConfig{
				Enabled: true,
			}

			// Handle nested mysql structure (project format)
			if mysqlConfig, exists := configMap["mysql"]; exists {
				if mysqlMap, ok := mysqlConfig.(map[string]interface{}); ok {
					if host, ok := mysqlMap["host"].(string); ok {
						dbConfig.Host = host
					}
					if port, ok := mysqlMap["port"]; ok {
						switch v := port.(type) {
						case int:
							dbConfig.Port = v
						case float64:
							dbConfig.Port = int(v)
						case string:
							if portInt, err := strconv.Atoi(v); err == nil {
								dbConfig.Port = portInt
							}
						}
					}
					if database, ok := mysqlMap["database"].(string); ok {
						dbConfig.Database = database
					}
					if username, ok := mysqlMap["username"].(string); ok {
						dbConfig.Username = username
					}
					if password, ok := mysqlMap["password"].(string); ok {
						dbConfig.Password = password
					}
					if charset, ok := mysqlMap["charset"].(string); ok {
						dbConfig.SSLMode = charset // Use charset as ssl_mode for MySQL
					}
					// SQL logging configuration
					if logEnabled, ok := mysqlMap["log_enabled"].(bool); ok {
						dbConfig.LogEnabled = logEnabled
					}
					if logLevel, ok := mysqlMap["log_level"].(string); ok {
						dbConfig.LogLevel = logLevel
					}
					if slowQueryThresholdStr, ok := mysqlMap["slow_query_threshold"].(string); ok {
						if duration, err := time.ParseDuration(slowQueryThresholdStr); err == nil {
							dbConfig.SlowQueryThreshold = duration
						}
					}
					// Debug log to verify config parsing
					logger.Debug("Parsed MySQL config",
						logger.String("name", name),
						logger.Bool("log_enabled", dbConfig.LogEnabled),
						logger.String("log_level", dbConfig.LogLevel),
						logger.String("slow_query_threshold", dbConfig.SlowQueryThreshold.String()))
				}
			} else {
				// Handle flat format (framework standard)
				if host, ok := configMap["host"].(string); ok {
					dbConfig.Host = host
				}
				if port, ok := configMap["port"]; ok {
					switch v := port.(type) {
					case int:
						dbConfig.Port = v
					case float64:
						dbConfig.Port = int(v)
					case string:
						if portInt, err := strconv.Atoi(v); err == nil {
							dbConfig.Port = portInt
						}
					}
				}
				if database, ok := configMap["database"].(string); ok {
					dbConfig.Database = database
				}
				if username, ok := configMap["username"].(string); ok {
					dbConfig.Username = username
				}
				if password, ok := configMap["password"].(string); ok {
					dbConfig.Password = password
				}
			}

			// Set driver based on name or explicit type
			if dbType, exists := configMap["type"]; exists {
				if typeStr, ok := dbType.(string); ok {
					dbConfig.Driver = typeStr
				}
			} else {
				if strings.Contains(name, "mysql") {
					dbConfig.Driver = "mysql"
				} else if strings.Contains(name, "postgres") || strings.Contains(name, "pg") {
					dbConfig.Driver = "postgres"
				}
			}

			dbConfigs[name] = dbConfig
		}
	}

	// Update bootstrap config with database settings
	if err := boot.UpdateDatabaseConfig(dbConfigs); err != nil {
		return err
	}
	logger.Info("Service database config loaded via LoadDatabaseConfig")
	return nil
}

// Dependency 按依赖键取回插件对象并断言为具体类型。本包不 import 任何插件
// 模块（链接期裁剪：应用自行 import 需要的插件包后在此做类型断言），例如：
//
//	factory, err := bootstrap.Dependency[*db.Factory](boot, bootstrap.DepDBFactory)
//	system, err := bootstrap.Dependency[*actor.System](boot, bootstrap.DepActorSystem)
//
// 依赖键使用本包导出的 Dep* 常量（值与历史字符串键保持一致）。
func Dependency[T any](boot *Bootstrap, key string) (T, error) {
	var zero T
	if boot == nil {
		return zero, fmt.Errorf("bootstrap instance is nil")
	}
	v, ok := boot.GetDependency(key)
	if !ok {
		return zero, fmt.Errorf("dependency not found: %s", key)
	}
	typed, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("dependency %s is %T, not %T", key, v, zero)
	}
	return typed, nil
}
