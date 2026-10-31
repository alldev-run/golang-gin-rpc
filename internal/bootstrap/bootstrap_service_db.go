package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	mysqlpkg "github.com/alldev-run/golang-gin-rpc/plugins/db/mysql"
	pg "github.com/alldev-run/golang-gin-rpc/plugins/db/postgres"
	cacheredis "github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
)

// initializeServiceDatabases initializes databases from service-level configuration format.
//
// 该逻辑服务于外部项目以"服务级"简配（mysql_primary / redis_cache …）接入框架的场景：
// 按候选路径查找 service database 配置文件 → 解析为 map → 逐项按类型/角色分发创建。
func (b *Bootstrap) initializeServiceDatabases(factory *db.Factory) error {
	serviceDBConfigPaths := []string{
		"database.yml",
		"database.yaml",
		"configs/database.yml",
		"configs/database.yaml",
		"database.service.yml",
		"database.service.yaml",
	}

	var serviceDBData []byte
	var serviceDBConfigPath string

	for _, path := range serviceDBConfigPaths {
		if _, err := os.Stat(path); err == nil {
			serviceDBConfigPath = path
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read service database config from %s: %w", path, err)
			}
			serviceDBData = data
			logger.Info("Found service database config", logger.String("path", path))
			break
		}
	}

	if serviceDBData == nil {
		return fmt.Errorf("no service database configuration file found")
	}

	var serviceDBConfigs map[string]interface{}
	if err := yaml.Unmarshal(serviceDBData, &serviceDBConfigs); err != nil {
		return fmt.Errorf("failed to parse service database config from %s: %w", serviceDBConfigPath, err)
	}

	logger.Info("Service database configs found", logger.Int("count", len(serviceDBConfigs)))
	for key := range serviceDBConfigs {
		logger.Info("Processing service config", logger.String("key", key))
	}

	for key, cfg := range serviceDBConfigs {
		if err := b.processServiceDatabaseConfig(key, cfg, factory); err != nil {
			return fmt.Errorf("failed to process service database config %s: %w", key, err)
		}
	}

	return nil
}

// processServiceDatabaseConfig processes a single service database configuration.
//
// key 形如 mysql_primary / mysql_replica / redis_cache；按 "_" 拆分得到 dbType 与 role，
// 再根据 type 字段分发到具体创建函数。
func (b *Bootstrap) processServiceDatabaseConfig(key string, config interface{}, factory *db.Factory) error {
	parts := strings.Split(key, "_")
	if len(parts) < 2 {
		return fmt.Errorf("invalid database config key format: %s", key)
	}

	dbType := parts[0]
	role := parts[1]

	if dbType == "redis" && role == "cache" {
		role = "primary"
	}

	configMap, ok := config.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid database config format for %s", key)
	}

	typeField, ok := configMap["type"].(string)
	if !ok || typeField != dbType {
		return fmt.Errorf("database type mismatch for %s: expected %s", key, dbType)
	}

	dbSpecificConfig, ok := configMap[dbType].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing %s configuration in %s", dbType, key)
	}

	switch dbType {
	case "mysql":
		return b.createMySQLFromServiceConfig(role, dbSpecificConfig, factory)
	case "postgres", "postgresql":
		return b.createPostgresFromServiceConfig(role, dbSpecificConfig, factory)
	case "redis":
		return b.createRedisFromServiceConfig(role, dbSpecificConfig, factory)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// createMySQLFromServiceConfig creates MySQL client from service configuration.
func (b *Bootstrap) createMySQLFromServiceConfig(role string, config map[string]interface{}, factory *db.Factory) error {
	mysqlConfig := mysqlpkg.DefaultConfig()

	if host, ok := config["host"].(string); ok {
		mysqlConfig.Host = host
	}
	if port, ok := config["port"].(int); ok {
		mysqlConfig.Port = port
	} else if portFloat, ok := config["port"].(float64); ok {
		mysqlConfig.Port = int(portFloat)
	}
	if database, ok := config["database"].(string); ok {
		mysqlConfig.Database = database
	}
	if username, ok := config["username"].(string); ok {
		mysqlConfig.Username = username
	}
	if password, ok := config["password"].(string); ok {
		mysqlConfig.Password = password
	}
	if charset, ok := config["charset"].(string); ok {
		mysqlConfig.Charset = charset
	}

	if maxOpenConns, ok := config["max_open_conns"].(int); ok {
		mysqlConfig.MaxOpenConns = maxOpenConns
	} else if maxOpenConnsFloat, ok := config["max_open_conns"].(float64); ok {
		mysqlConfig.MaxOpenConns = int(maxOpenConnsFloat)
	}
	if maxIdleConns, ok := config["max_idle_conns"].(int); ok {
		mysqlConfig.MaxIdleConns = maxIdleConns
	} else if maxIdleConnsFloat, ok := config["max_idle_conns"].(float64); ok {
		mysqlConfig.MaxIdleConns = int(maxIdleConnsFloat)
	}
	if connMaxLifetimeStr, ok := config["conn_max_lifetime"].(string); ok {
		if duration, err := time.ParseDuration(connMaxLifetimeStr); err == nil {
			mysqlConfig.ConnMaxLifetime = duration
		}
	}
	if connMaxIdleTimeStr, ok := config["conn_max_idle_time"].(string); ok {
		if duration, err := time.ParseDuration(connMaxIdleTimeStr); err == nil {
			mysqlConfig.ConnMaxIdleTime = duration
		}
	}

	if logEnabled, ok := config["log_enabled"].(bool); ok {
		mysqlConfig.LogEnabled = logEnabled
	}
	if logLevel, ok := config["log_level"].(string); ok {
		mysqlConfig.LogLevel = logLevel
	}
	if slowQueryThresholdStr, ok := config["slow_query_threshold"].(string); ok {
		if duration, err := time.ParseDuration(slowQueryThresholdStr); err == nil {
			mysqlConfig.SlowQueryThreshold = duration
		}
	}

	dbConfig := db.Config{
		Type:  db.TypeMySQL,
		MySQL: mysqlConfig,
	}

	client, err := factory.Create(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create MySQL client for role %s: %w", role, err)
	}

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		logger.Errorf("MySQL connection failed for role %s", logger.String("role", role), logger.Error(err))
	} else {
		logger.Info("MySQL connected successfully", logger.String("role", role))
	}

	return nil
}

// createPostgresFromServiceConfig creates PostgreSQL client from service configuration.
func (b *Bootstrap) createPostgresFromServiceConfig(role string, config map[string]interface{}, factory *db.Factory) error {
	postgresConfig := pg.DefaultConfig()

	if host, ok := config["host"].(string); ok {
		postgresConfig.Host = host
	}
	if port, ok := config["port"].(int); ok {
		postgresConfig.Port = port
	} else if portFloat, ok := config["port"].(float64); ok {
		postgresConfig.Port = int(portFloat)
	}
	if database, ok := config["database"].(string); ok {
		postgresConfig.Database = database
	}
	if username, ok := config["username"].(string); ok {
		postgresConfig.Username = username
	}
	if password, ok := config["password"].(string); ok {
		postgresConfig.Password = password
	}
	if sslMode, ok := config["ssl_mode"].(string); ok {
		postgresConfig.SSLMode = sslMode
	}

	if maxOpenConns, ok := config["max_open_conns"].(int); ok {
		postgresConfig.MaxOpenConns = maxOpenConns
	} else if maxOpenConnsFloat, ok := config["max_open_conns"].(float64); ok {
		postgresConfig.MaxOpenConns = int(maxOpenConnsFloat)
	}
	if maxIdleConns, ok := config["max_idle_conns"].(int); ok {
		postgresConfig.MaxIdleConns = maxIdleConns
	} else if maxIdleConnsFloat, ok := config["max_idle_conns"].(float64); ok {
		postgresConfig.MaxIdleConns = int(maxIdleConnsFloat)
	}
	if connMaxLifetimeStr, ok := config["conn_max_lifetime"].(string); ok {
		if duration, err := time.ParseDuration(connMaxLifetimeStr); err == nil {
			postgresConfig.ConnMaxLifetime = duration
		}
	}
	if connMaxIdleTimeStr, ok := config["conn_max_idle_time"].(string); ok {
		if duration, err := time.ParseDuration(connMaxIdleTimeStr); err == nil {
			_ = duration
		}
	}

	if logEnabled, ok := config["log_enabled"].(bool); ok {
		postgresConfig.LogEnabled = logEnabled
	}
	if logLevel, ok := config["log_level"].(string); ok {
		postgresConfig.LogLevel = logLevel
	}
	if slowQueryThresholdStr, ok := config["slow_query_threshold"].(string); ok {
		if duration, err := time.ParseDuration(slowQueryThresholdStr); err == nil {
			postgresConfig.SlowQueryThreshold = duration
		}
	}

	dbConfig := db.Config{
		Type: db.TypePostgres,
		PG:   postgresConfig,
	}

	client, err := factory.Create(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL client for role %s: %w", role, err)
	}

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		logger.Errorf("PostgreSQL connection failed for role %s", logger.String("role", role), logger.Error(err))
	} else {
		logger.Info("PostgreSQL connected successfully", logger.String("role", role))
	}

	return nil
}

// createRedisFromServiceConfig creates Redis client from service configuration.
func (b *Bootstrap) createRedisFromServiceConfig(role string, config map[string]interface{}, factory *db.Factory) error {
	redisConfig := cacheredis.DefaultConfig()

	if host, ok := config["host"].(string); ok {
		redisConfig.Host = host
	}
	if port, ok := config["port"].(int); ok {
		redisConfig.Port = port
	} else if portFloat, ok := config["port"].(float64); ok {
		redisConfig.Port = int(portFloat)
	}
	if password, ok := config["password"].(string); ok {
		redisConfig.Password = password
	}
	if database, ok := config["database"].(int); ok {
		redisConfig.Database = database
	} else if databaseFloat, ok := config["database"].(float64); ok {
		redisConfig.Database = int(databaseFloat)
	}
	if mode, ok := config["mode"].(string); ok {
		redisConfig.Mode = redis.Mode(mode)
	}

	if poolSize, ok := config["pool_size"].(int); ok {
		redisConfig.PoolSize = poolSize
	} else if poolSizeFloat, ok := config["pool_size"].(float64); ok {
		redisConfig.PoolSize = int(poolSizeFloat)
	}
	if minIdleConns, ok := config["min_idle_conns"].(int); ok {
		redisConfig.MinIdleConns = minIdleConns
	} else if minIdleConnsFloat, ok := config["min_idle_conns"].(float64); ok {
		redisConfig.MinIdleConns = int(minIdleConnsFloat)
	}
	if maxRetries, ok := config["max_retries"].(int); ok {
		redisConfig.MaxRetries = maxRetries
	} else if maxRetriesFloat, ok := config["max_retries"].(float64); ok {
		redisConfig.MaxRetries = int(maxRetriesFloat)
	}

	if dialTimeoutStr, ok := config["dial_timeout"].(string); ok {
		if duration, err := time.ParseDuration(dialTimeoutStr); err == nil {
			redisConfig.DialTimeout = duration
		}
	}
	if readTimeoutStr, ok := config["read_timeout"].(string); ok {
		if duration, err := time.ParseDuration(readTimeoutStr); err == nil {
			redisConfig.ReadTimeout = duration
		}
	}
	if writeTimeoutStr, ok := config["write_timeout"].(string); ok {
		if duration, err := time.ParseDuration(writeTimeoutStr); err == nil {
			redisConfig.WriteTimeout = duration
		}
	}
	if poolTimeoutStr, ok := config["pool_timeout"].(string); ok {
		if duration, err := time.ParseDuration(poolTimeoutStr); err == nil {
			redisConfig.PoolTimeout = duration
		}
	}

	dbConfig := db.Config{
		Type:  db.TypeRedis,
		Redis: redisConfig,
	}

	client, err := factory.Create(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create Redis client for role %s: %w", role, err)
	}

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		logger.Errorf("Redis connection failed for role %s", logger.String("role", role), logger.Error(err))
	} else {
		logger.Info("Redis connected successfully", logger.String("role", role))
	}

	return nil
}
