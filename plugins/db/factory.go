// Package db provides a unified database client factory and configuration management
// for MySQL, Redis, PostgreSQL, ClickHouse, and Elasticsearch.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/alldev-run/golang-gin-rpc/plugins/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/postgres"
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
	Type     Type            `yaml:"type" json:"type"`
	MySQL    mysql.Config    `yaml:"mysql" json:"mysql"`
	Redis    redis.Config    `yaml:"redis" json:"redis"`
	PG       postgres.Config `yaml:"postgres" json:"postgres"`
	Memcache memcache.Config `yaml:"memcache" json:"memcache"`

	// 重型驱动的配置由对应驱动子包解析（见各子包 register.go）：
	// CH 携带 clickhouse.Config，ES 携带 elasticsearch.Config，MongoDB 携带 mongodb.Config。
	CH      any `yaml:"clickhouse,omitempty" json:"clickhouse,omitempty"`
	ES      any `yaml:"elasticsearch,omitempty" json:"elasticsearch,omitempty"`
	MongoDB any `yaml:"mongodb,omitempty" json:"mongodb,omitempty"`
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
	mu      sync.RWMutex
	sf      singleflight.Group
}

// NewFactory creates a new database factory.
func NewFactory() *Factory {
	return &Factory{
		clients: make(map[Type]Client),
	}
}

// Create creates a new database client from config.
// 驱动经由注册表分发：未链接的驱动会得到明确错误（import 对应驱动子包即可）。
func (f *Factory) Create(cfg Config) (Client, error) {
	if cfg.Type == "" {
		return nil, errors.New("database type is required")
	}
	creator, ok := lookupDriver(cfg.Type)
	if !ok {
		return nil, fmt.Errorf("database driver %q not linked: import the corresponding plugins/db/<driver> package", cfg.Type)
	}

	key := Type(createKey(string(cfg.Type), cfg))
	v, err, _ := f.sf.Do(string(key), func() (interface{}, error) {
		f.mu.RLock()
		if client, exists := f.clients[key]; exists {
			f.mu.RUnlock()
			return client, nil
		}
		f.mu.RUnlock()

		client, err := creator(cfg)
		if err != nil {
			return nil, err
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		if existing, exists := f.clients[key]; exists {
			_ = client.Close()
			return existing, nil
		}
		f.clients[key] = client
		if _, exists := f.clients[cfg.Type]; !exists {
			f.clients[cfg.Type] = client
		}
		return client, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}

// createMySQL creates a MySQL client.
func (f *Factory) createMySQL(cfg mysql.Config) (Client, error) {
	key := Type(createKey("mysql", cfg))

	v, err, _ := f.sf.Do(string(key), func() (interface{}, error) {
		f.mu.RLock()
		if client, exists := f.clients[key]; exists {
			f.mu.RUnlock()
			return client, nil
		}
		f.mu.RUnlock()

		client, err := mysql.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &mysqlAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		if existing, exists := f.clients[key]; exists {
			_ = adapter.Close()
			return existing, nil
		}
		f.clients[key] = adapter
		if _, exists := f.clients[TypeMySQL]; !exists {
			f.clients[TypeMySQL] = adapter
		}
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}

// createRedis creates a Redis client.
func (f *Factory) createRedis(cfg redis.Config) (Client, error) {
	v, err, _ := f.sf.Do(string(TypeRedis), func() (interface{}, error) {
		f.mu.RLock()
		if client, exists := f.clients[TypeRedis]; exists {
			f.mu.RUnlock()
			return client, nil
		}
		f.mu.RUnlock()

		client, err := redis.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &redisAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.clients[TypeRedis] = adapter
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}

// createPostgres creates a PostgreSQL client.
func (f *Factory) createPostgres(cfg postgres.Config) (Client, error) {
	v, err, _ := f.sf.Do(string(TypePostgres), func() (interface{}, error) {
		f.mu.RLock()
		if client, exists := f.clients[TypePostgres]; exists {
			f.mu.RUnlock()
			return client, nil
		}
		f.mu.RUnlock()

		client, err := postgres.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &postgresAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.clients[TypePostgres] = adapter
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}

// createMemcache creates a Memcached client.
func (f *Factory) createMemcache(cfg memcache.Config) (Client, error) {
	v, err, _ := f.sf.Do(string(TypeMemcache), func() (interface{}, error) {
		f.mu.RLock()
		if client, exists := f.clients[TypeMemcache]; exists {
			f.mu.RUnlock()
			return client, nil
		}
		f.mu.RUnlock()

		client, err := memcache.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &memcacheAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.clients[TypeMemcache] = adapter
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
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

// GetMySQL returns the MySQL client from the factory.
func (f *Factory) GetMySQL() (*mysql.Client, error) {
	f.mu.RLock()
	client, exists := f.clients[TypeMySQL]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("MySQL client not found")
	}

	adapter, ok := client.(*mysqlAdapter)
	if !ok {
		return nil, fmt.Errorf("invalid MySQL client type")
	}

	return adapter.client, nil
}

// GetMySQLSQLClient returns the MySQL client as SQLClient interface.
func (f *Factory) GetMySQLSQLClient() (SQLClient, error) {
	f.mu.RLock()
	client, exists := f.clients[TypeMySQL]
	f.mu.RUnlock()

	if exists {
		if sqlClient, ok := client.(SQLClient); ok {
			return sqlClient, nil
		}
	}
	return nil, fmt.Errorf("MySQL SQL client not found")
}

// GetRedis returns the Redis client from the factory.
func (f *Factory) GetRedis() (*redis.Client, error) {
	f.mu.RLock()
	client, exists := f.clients[TypeRedis]
	f.mu.RUnlock()

	if exists {
		if adapter, ok := client.(*redisAdapter); ok {
			return adapter.client, nil
		}
	}
	return nil, fmt.Errorf("Redis client not found")
}

// GetPostgres returns the PostgreSQL client from the factory.
func (f *Factory) GetPostgres() (*postgres.Client, error) {
	f.mu.RLock()
	client, exists := f.clients[TypePostgres]
	f.mu.RUnlock()

	if exists {
		if adapter, ok := client.(*postgresAdapter); ok {
			return adapter.client, nil
		}
	}
	return nil, fmt.Errorf("PostgreSQL client not found")
}

// GetClient returns a client by type.
func (f *Factory) GetClient(dbType Type) (Client, error) {
	f.mu.RLock()
	client, exists := f.clients[dbType]
	f.mu.RUnlock()

	if exists {
		return client, nil
	}
	return nil, fmt.Errorf("client for type %s not found", dbType)
}

// ==================== Scheme B: Create with Config ====================

// createKey generates a unique key based on config content
func createKey(prefix string, cfg interface{}) string {
	return fmt.Sprintf("%s:%v", prefix, cfg)
}

// GetMySQLWithConfig creates or returns a MySQL client with specific config
func (f *Factory) GetMySQLWithConfig(cfg mysql.Config) (Client, error) {
	return f.createMySQL(cfg)
}

// GetRedisWithConfig creates or returns a Redis client with specific config
func (f *Factory) GetRedisWithConfig(cfg redis.Config) (Client, error) {
	key := createKey("redis", cfg)

	v, err, _ := f.sf.Do(key, func() (interface{}, error) {
		client, err := redis.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &redisAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.clients[Type(key)] = adapter
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}

// GetPostgresWithConfig creates or returns a PostgreSQL client with specific config
func (f *Factory) GetPostgresWithConfig(cfg postgres.Config) (Client, error) {
	key := createKey("postgres", cfg)

	v, err, _ := f.sf.Do(key, func() (interface{}, error) {
		client, err := postgres.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &postgresAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.clients[Type(key)] = adapter
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}

// GetMemcacheWithConfig creates or returns a Memcached client with specific config
func (f *Factory) GetMemcacheWithConfig(cfg memcache.Config) (Client, error) {
	key := createKey("memcache", cfg)

	v, err, _ := f.sf.Do(key, func() (interface{}, error) {
		client, err := memcache.New(cfg)
		if err != nil {
			return nil, err
		}
		adapter := &memcacheAdapter{client: client}

		f.mu.Lock()
		defer f.mu.Unlock()
		f.clients[Type(key)] = adapter
		return adapter, nil
	})

	if err != nil {
		return nil, err
	}
	return v.(Client), nil
}
