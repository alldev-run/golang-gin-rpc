package bootstrap

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/postgres"
)

// InitializeDatabases initializes all database connections
func (b *Bootstrap) InitializeDatabases() error {
	factory := db.NewFactory()

	// Create database logger if service logging is configured
	var databaseLogger *zap.Logger

	// Try logging.service_logging.components first (user's config format)
	if b.config.Logger.Logging.ServiceLogging.Enabled {
		if dbConfig, exists := b.config.Logger.Logging.ServiceLogging.Components["database"]; exists {
			serviceLoggerConfig := logger.ServiceLoggerConfig{
				ServiceName:         "database",
				BaseDir:             dbConfig.BaseDir,
				EnableDateFolder:    b.config.Logger.Logging.ServiceLogging.EnableDateFolders,
				SeparateByLevel:     dbConfig.SeparateByLevel,
				InheritGlobalConfig: b.config.Logger.Logging.ServiceLogging.InheritGlobalConfig,
			}

			// Apply override config
			if dbConfig.OverrideConfig.Level != "" {
				serviceLoggerConfig.OverrideConfig.Level = logger.LogLevel(dbConfig.OverrideConfig.Level)
			}
			if dbConfig.OverrideConfig.Format != "" {
				serviceLoggerConfig.OverrideConfig.Format = logger.LogFormat(dbConfig.OverrideConfig.Format)
			}
			if dbConfig.OverrideConfig.MaxSize > 0 {
				serviceLoggerConfig.OverrideConfig.MaxSize = dbConfig.OverrideConfig.MaxSize
			}
			if dbConfig.OverrideConfig.MaxBackups > 0 {
				serviceLoggerConfig.OverrideConfig.MaxBackups = dbConfig.OverrideConfig.MaxBackups
			}
			if dbConfig.OverrideConfig.MaxAge > 0 {
				serviceLoggerConfig.OverrideConfig.MaxAge = dbConfig.OverrideConfig.MaxAge
			}
			if dbConfig.OverrideConfig.Compress {
				serviceLoggerConfig.OverrideConfig.Compress = dbConfig.OverrideConfig.Compress
			}
			if dbConfig.OverrideConfig.EnableCaller {
				serviceLoggerConfig.OverrideConfig.EnableCaller = dbConfig.OverrideConfig.EnableCaller
			}
			if dbConfig.OverrideConfig.EnableStacktrace {
				serviceLoggerConfig.OverrideConfig.EnableStacktrace = dbConfig.OverrideConfig.EnableStacktrace
			}

			databaseLogger = logger.GetServiceLoggerInstance("database", serviceLoggerConfig)
			logger.Info("Database component logger created from logging.service_logging", logger.String("base_dir", dbConfig.BaseDir))
		}
	} else if b.config.Logger.ServiceLogging.Enabled {
		// Fallback to logger.service_logging.components (framework config format)
		if dbConfig, exists := b.config.Logger.ServiceLogging.Components["database"]; exists {
			serviceLoggerConfig := logger.ServiceLoggerConfig{
				ServiceName:         "database",
				BaseDir:             dbConfig.BaseDir,
				EnableDateFolder:    b.config.Logger.ServiceLogging.EnableDateFolders,
				SeparateByLevel:     dbConfig.SeparateByLevel,
				InheritGlobalConfig: b.config.Logger.ServiceLogging.InheritGlobalConfig,
			}

			// Apply override config
			if dbConfig.OverrideConfig.Level != "" {
				serviceLoggerConfig.OverrideConfig.Level = logger.LogLevel(dbConfig.OverrideConfig.Level)
			}
			if dbConfig.OverrideConfig.Format != "" {
				serviceLoggerConfig.OverrideConfig.Format = logger.LogFormat(dbConfig.OverrideConfig.Format)
			}
			if dbConfig.OverrideConfig.MaxSize > 0 {
				serviceLoggerConfig.OverrideConfig.MaxSize = dbConfig.OverrideConfig.MaxSize
			}
			if dbConfig.OverrideConfig.MaxBackups > 0 {
				serviceLoggerConfig.OverrideConfig.MaxBackups = dbConfig.OverrideConfig.MaxBackups
			}
			if dbConfig.OverrideConfig.MaxAge > 0 {
				serviceLoggerConfig.OverrideConfig.MaxAge = dbConfig.OverrideConfig.MaxAge
			}
			if dbConfig.OverrideConfig.Compress {
				serviceLoggerConfig.OverrideConfig.Compress = dbConfig.OverrideConfig.Compress
			}
			if dbConfig.OverrideConfig.EnableCaller {
				serviceLoggerConfig.OverrideConfig.EnableCaller = dbConfig.OverrideConfig.EnableCaller
			}
			if dbConfig.OverrideConfig.EnableStacktrace {
				serviceLoggerConfig.OverrideConfig.EnableStacktrace = dbConfig.OverrideConfig.EnableStacktrace
			}

			databaseLogger = logger.GetServiceLoggerInstance("database", serviceLoggerConfig)
			logger.Info("Database component logger created from logger.service_logging", logger.String("base_dir", dbConfig.BaseDir))
		}
	}

	// Check if database configuration has been loaded via LoadDatabaseConfig
	if b.serviceDBConfigLoaded {
		logger.Info("Using pre-loaded database configuration")
		configs := map[string]config.DBConfig{
			"primary": b.config.Database.Primary,
			"replica": b.config.Database.Replica,
		}

		for name, dbConfig := range configs {
			if !dbConfig.Enabled {
				continue
			}

			clientConfig, err := buildDBConfig(dbConfig, b.config.Database.Pool)
			if err != nil {
				logger.Errorf("Database configuration build failed",
					logger.String("name", name),
					logger.Error(err),
				)
				return fmt.Errorf("failed to build database config %s: %w", name, err)
			}

			client, err := factory.Create(clientConfig)
			if err != nil {
				logger.Errorf("Database client creation failed",
					logger.String("name", name),
					logger.Error(err),
				)
				return fmt.Errorf("failed to create database client %s: %w", name, err)
			}

			// Set database logger if configured
			if databaseLogger != nil {
				if mysqlClient, ok := client.(*mysql.Client); ok {
					mysqlClient.SetSQLLogger(databaseLogger)
				} else if postgresClient, ok := client.(*postgres.Client); ok {
					postgresClient.SetSQLLogger(databaseLogger)
				}
			}

			ctx := context.Background()
			if err := client.Ping(ctx); err != nil {
				logger.Errorf("Database connection failed", logger.String("name", name), logger.Error(err))
			} else {
				logger.Info("Database connected successfully", logger.String("name", name))
			}
		}

		b.db = factory
		b.setDependency(DepDBFactory, factory)
		// Set global factory for db helper functions
		db.SetGlobalFactory(factory)
		logger.Info("Database initialization completed")
		return nil
	}

	// First, try to load service-level database configurations
	if err := b.initializeServiceDatabases(factory); err != nil {
		logger.Warn("Failed to initialize service-level databases, falling back to framework config", logger.Error(err))

		// Fallback to framework database configuration
		if !b.config.Database.Primary.Enabled && !b.config.Database.Replica.Enabled {
			logger.Info("No database configuration found, skipping database initialization")
			return nil
		}

		configs := map[string]config.DBConfig{
			"primary": b.config.Database.Primary,
			"replica": b.config.Database.Replica,
		}

		for name, dbConfig := range configs {
			if !dbConfig.Enabled {
				continue
			}

			clientConfig, err := buildDBConfig(dbConfig, b.config.Database.Pool)
			if err != nil {
				logger.Errorf("Database configuration build failed",
					logger.String("name", name),
					logger.Error(err),
				)
				return fmt.Errorf("failed to build database config %s: %w", name, err)
			}

			client, err := factory.Create(clientConfig)
			if err != nil {
				logger.Errorf("Database client creation failed",
					logger.String("name", name),
					logger.Error(err),
				)
				return fmt.Errorf("failed to create database client %s: %w", name, err)
			}

			// Set database logger if configured
			if databaseLogger != nil {
				if mysqlClient, ok := client.(*mysql.Client); ok {
					mysqlClient.SetSQLLogger(databaseLogger)
				} else if postgresClient, ok := client.(*postgres.Client); ok {
					postgresClient.SetSQLLogger(databaseLogger)
				}
			}

			ctx := context.Background()
			if err := client.Ping(ctx); err != nil {
				logger.Errorf("Database connection failed", logger.String("name", name), logger.Error(err))
			} else {
				logger.Info("Database connected successfully", logger.String("name", name))
			}
		}
	}

	b.db = factory
	b.setDependency(DepDBFactory, factory)
	// Set global factory for db helper functions
	db.SetGlobalFactory(factory)
	logger.Info("Database initialization completed")
	return nil
}
