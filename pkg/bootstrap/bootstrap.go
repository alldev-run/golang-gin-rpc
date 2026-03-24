package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	internalbootstrap "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/db"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
)

type Bootstrap = internalbootstrap.Bootstrap

type FrameworkOptions = internalbootstrap.FrameworkOptions

type APIGatewayServiceOptions = internalbootstrap.APIGatewayServiceOptions

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

// RegisterAPIGatewayServiceFactory registers API gateway service factory via package helper.
func RegisterAPIGatewayServiceFactory(boot *Bootstrap, options APIGatewayServiceOptions) error {
	if boot == nil {
		return fmt.Errorf("bootstrap instance is nil")
	}
	return boot.RegisterAPIGatewayServiceFactory(options)
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
	return boot.UpdateDatabaseConfig(dbConfigs)
}

// GetDatabaseFactory returns the database factory instance
func GetDatabaseFactory(boot *Bootstrap) *db.Factory {
	if boot == nil {
		return nil
	}
	return boot.GetDatabaseFactory()
}

// GetMySQLClient returns the MySQL client from the database factory
func GetMySQLClient(boot *Bootstrap) (*mysql.Client, error) {
	if boot == nil {
		return nil, fmt.Errorf("bootstrap instance is nil")
	}
	return boot.GetMySQLClient()
}

// GetMySQLSQLClient returns the MySQL client as SQLClient interface
func GetMySQLSQLClient(boot *Bootstrap) (db.SQLClient, error) {
	if boot == nil {
		return nil, fmt.Errorf("bootstrap instance is nil")
	}
	return boot.GetMySQLSQLClient()
}

// GetRedisClient returns the Redis client from the database factory
func GetRedisClient(boot *Bootstrap) (*redis.Client, error) {
	if boot == nil {
		return nil, fmt.Errorf("bootstrap instance is nil")
	}
	return boot.GetRedisClient()
}

// GetPostgresClient returns the PostgreSQL client from the database factory
func GetPostgresClient(boot *Bootstrap) (*postgres.Client, error) {
	if boot == nil {
		return nil, fmt.Errorf("bootstrap instance is nil")
	}
	return boot.GetPostgresClient()
}
