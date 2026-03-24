// Package db provides a unified database client factory and configuration management
// for MySQL, Redis, PostgreSQL, ClickHouse, and Elasticsearch.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/clickhouse"
	"github.com/alldev-run/golang-gin-rpc/pkg/search/elasticsearch"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mongodb"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
)

// Type represents the database type.
type Type string

const (
	TypeMySQL      Type = "mysql"
	TypeRedis      Type = "redis"
	TypePostgres   Type = "postgres"
	TypeClickHouse Type = "clickhouse"
	TypeES         Type = "elasticsearch"
	TypeMemcache   Type = "memcache"
	TypeMongoDB    Type = "mongodb"
)

// Config holds configuration for all database types.
type Config struct {
	Type     Type                 `yaml:"type" json:"type"`
	MySQL    mysql.Config         `yaml:"mysql" json:"mysql"`
	Redis    redis.Config         `yaml:"redis" json:"redis"`
	PG       postgres.Config      `yaml:"postgres" json:"postgres"`
	CH       clickhouse.Config    `yaml:"clickhouse" json:"clickhouse"`
	ES       elasticsearch.Config `yaml:"elasticsearch" json:"elasticsearch"`
	Memcache memcache.Config      `yaml:"memcache" json:"memcache"`
	MongoDB  mongodb.Config       `yaml:"mongodb" json:"mongodb"`
}

// Client is the unified database client interface.
type Client interface {
	// Ping checks the connection health.
	Ping(ctx context.Context) error
	// Close closes the connection.
	Close() error
}

// SQLClient extends Client for SQL databases.
type SQLClient interface {
	Client
	DB() *sql.DB
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Transaction(ctx context.Context, fn func(*sql.Tx) error) error
}

// Factory creates database clients based on configuration.
type Factory struct {
	clients map[Type]Client
}

// NewFactory creates a new database factory.
func NewFactory() *Factory {
	return &Factory{
		clients: make(map[Type]Client),
	}
}

// Create creates a new database client from config.
func (f *Factory) Create(cfg Config) (Client, error) {
	switch cfg.Type {
	case TypeMySQL:
		return f.createMySQL(cfg.MySQL)
	case TypeRedis:
		return f.createRedis(cfg.Redis)
	case TypePostgres:
		return f.createPostgres(cfg.PG)
	case TypeClickHouse:
		return f.createClickHouse(cfg.CH)
	case TypeES:
		return f.createES(cfg.ES)
	case TypeMemcache:
		return f.createMemcache(cfg.Memcache)
	case TypeMongoDB:
		return f.createMongoDB(cfg.MongoDB)
	case "":
		return nil, errors.New("database type is required")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

// createMySQL creates a MySQL client.
func (f *Factory) createMySQL(cfg mysql.Config) (Client, error) {
	client, err := mysql.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql client: %w", err)
	}
	adapter := &mysqlAdapter{client: client}
	// Store the client in the factory for later retrieval
	f.clients[TypeMySQL] = adapter
	return adapter, nil
}

// createRedis creates a Redis client.
func (f *Factory) createRedis(cfg redis.Config) (Client, error) {
	client, err := redis.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}
	return &redisAdapter{client: client}, nil
}

// createPostgres creates a PostgreSQL client.
func (f *Factory) createPostgres(cfg postgres.Config) (Client, error) {
	client, err := postgres.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres client: %w", err)
	}
	return &postgresAdapter{client: client}, nil
}

// createClickHouse creates a ClickHouse client.
func (f *Factory) createClickHouse(cfg clickhouse.Config) (Client, error) {
	client, err := clickhouse.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse client: %w", err)
	}
	return &clickhouseAdapter{client: client}, nil
}

// createES creates an Elasticsearch client.
func (f *Factory) createES(cfg elasticsearch.Config) (Client, error) {
	client, err := elasticsearch.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}
	return &esAdapter{client: client}, nil
}

// createMemcache creates a Memcached client.
func (f *Factory) createMemcache(cfg memcache.Config) (Client, error) {
	client, err := memcache.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create memcache client: %w", err)
	}
	return &memcacheAdapter{client: client}, nil
}

// createMongoDB creates a MongoDB client.
func (f *Factory) createMongoDB(cfg mongodb.Config) (Client, error) {
	client, err := mongodb.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongodb client: %w", err)
	}
	return &mongodbAdapter{client: client}, nil
}

// ==================== Adapters ====================

// mysqlAdapter adapts mysql.Client to db.Client.
type mysqlAdapter struct {
	client *mysql.Client
}

func (a *mysqlAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

func (a *mysqlAdapter) Close() error {
	return a.client.Close()
}

func (a *mysqlAdapter) DB() *sql.DB {
	return a.client.DB()
}

func (a *mysqlAdapter) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return a.client.Query(ctx, query, args...)
}

func (a *mysqlAdapter) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return a.client.QueryRow(ctx, query, args...)
}

func (a *mysqlAdapter) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return a.client.Exec(ctx, query, args...)
}

func (a *mysqlAdapter) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return a.client.Begin(ctx, opts)
}

func (a *mysqlAdapter) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	return a.client.Transaction(ctx, fn)
}

// redisAdapter adapts redis.Client to db.Client.
type redisAdapter struct {
	client *redis.Client
}

func (a *redisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

func (a *redisAdapter) Close() error {
	return a.client.Close()
}

// postgresAdapter adapts postgres.Client to db.Client.
type postgresAdapter struct {
	client *postgres.Client
}

func (a *postgresAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

func (a *postgresAdapter) Close() error {
	return a.client.Close()
}

func (a *postgresAdapter) DB() *sql.DB {
	return a.client.DB()
}

func (a *postgresAdapter) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return a.client.Query(ctx, query, args...)
}

func (a *postgresAdapter) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return a.client.QueryRow(ctx, query, args...)
}

func (a *postgresAdapter) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return a.client.Exec(ctx, query, args...)
}

func (a *postgresAdapter) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return a.client.Begin(ctx, opts)
}

func (a *postgresAdapter) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	return a.client.Transaction(ctx, fn)
}

// clickhouseAdapter adapts clickhouse.Client to db.Client.
type clickhouseAdapter struct {
	client *clickhouse.Client
}

func (a *clickhouseAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

func (a *clickhouseAdapter) Close() error {
	return a.client.Close()
}

// esAdapter adapts elasticsearch.Client to db.Client.
type esAdapter struct {
	client *elasticsearch.Client
}

func (a *esAdapter) Ping(ctx context.Context) error {
	_, err := a.client.Info(ctx)
	return err
}

func (a *esAdapter) Close() error {
	// ES client doesn't need explicit close
	return nil
}

// memcacheAdapter adapts memcache.Client to db.Client.
type memcacheAdapter struct {
	client *memcache.Client
}

func (a *memcacheAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

func (a *memcacheAdapter) Close() error {
	return a.client.Close()
}

// mongodbAdapter adapts mongodb.Client to db.Client.
type mongodbAdapter struct {
	client *mongodb.Client
}

func (a *mongodbAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx)
}

func (a *mongodbAdapter) Close() error {
	return a.client.Close(nil)
}

// GetMySQL returns the MySQL client from the factory.
func (f *Factory) GetMySQL() (*mysql.Client, error) {
	if client, exists := f.clients[TypeMySQL]; exists {
		if adapter, ok := client.(*mysqlAdapter); ok {
			return adapter.client, nil
		}
	}
	return nil, fmt.Errorf("MySQL client not found")
}

// GetMySQLSQLClient returns the MySQL client as SQLClient interface.
func (f *Factory) GetMySQLSQLClient() (SQLClient, error) {
	if client, exists := f.clients[TypeMySQL]; exists {
		if sqlClient, ok := client.(SQLClient); ok {
			return sqlClient, nil
		}
	}
	return nil, fmt.Errorf("MySQL SQL client not found")
}

// GetRedis returns the Redis client from the factory.
func (f *Factory) GetRedis() (*redis.Client, error) {
	if client, exists := f.clients[TypeRedis]; exists {
		if adapter, ok := client.(*redisAdapter); ok {
			return adapter.client, nil
		}
	}
	return nil, fmt.Errorf("Redis client not found")
}

// GetPostgres returns the PostgreSQL client from the factory.
func (f *Factory) GetPostgres() (*postgres.Client, error) {
	if client, exists := f.clients[TypePostgres]; exists {
		if adapter, ok := client.(*postgresAdapter); ok {
			return adapter.client, nil
		}
	}
	return nil, fmt.Errorf("PostgreSQL client not found")
}

// GetClient returns a client by type.
func (f *Factory) GetClient(dbType Type) (Client, error) {
	if client, exists := f.clients[dbType]; exists {
		return client, nil
	}
	return nil, fmt.Errorf("client for type %s not found", dbType)
}
