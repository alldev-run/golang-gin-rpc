// Package poolcb integrates connection pooling with circuit breaker pattern.
// It provides a resilient database client that automatically handles failures
// and prevents cascading failures across the system.
package poolcb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/circuitbreaker"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/pool"
)

// Config holds the combined pool and circuit breaker configuration.
type Config struct {
	PoolConfig    pool.Config
	BreakerConfig circuitbreaker.Config
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		PoolConfig:    pool.DefaultConfig(),
		BreakerConfig: circuitbreaker.DefaultConfig(),
	}
}

// PoolWithBreaker wraps a connection pool with circuit breaker protection.
type PoolWithBreaker struct {
	pool    *pool.Pool
	breaker *circuitbreaker.CircuitBreaker
}

// New creates a new pool with circuit breaker.
func New(config Config, factory *db.Factory) *PoolWithBreaker {
	if factory == nil {
		factory = db.NewFactory()
	}

	return &PoolWithBreaker{
		pool:    pool.New(config.PoolConfig, factory),
		breaker: circuitbreaker.New(config.BreakerConfig),
	}
}

// Close shuts down the pool and circuit breaker.
func (p *PoolWithBreaker) Close() error {
	return p.pool.Close()
}

// Register adds a new connection to the pool with circuit breaker protection.
func (p *PoolWithBreaker) Register(name string, cfg db.Config) error {
	return p.pool.Register(name, cfg)
}

// Unregister removes a connection from the pool.
func (p *PoolWithBreaker) Unregister(name string) error {
	return p.pool.Unregister(name)
}

// Acquire gets a client from the pool with circuit breaker protection.
func (p *PoolWithBreaker) Acquire(ctx context.Context, name string) (db.Client, error) {
	var client db.Client

	err := p.breaker.Execute(ctx, func() error {
		var err error
		client, err = p.pool.Acquire(ctx, name)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	return &protectedClient{
		client:  client,
		breaker: p.breaker,
	}, nil
}

// protectedClient wraps a db.Client with circuit breaker protection.
type protectedClient struct {
	client  db.Client
	breaker *circuitbreaker.CircuitBreaker
}

// Ping checks connection health with circuit breaker protection.
func (c *protectedClient) Ping(ctx context.Context) error {
	return c.breaker.Execute(ctx, func() error {
		return c.client.Ping(ctx)
	})
}

// Close closes the client.
func (c *protectedClient) Close() error {
	return c.client.Close()
}

// underlyingSQL returns the wrapped client as db.SQLClient.
func (c *protectedClient) underlyingSQL() (db.SQLClient, bool) {
	sqlClient, ok := c.client.(db.SQLClient)
	return sqlClient, ok
}

// DB exposes the underlying *sql.DB without breaker protection.
func (c *protectedClient) DB() *sql.DB {
	if sqlClient, ok := c.underlyingSQL(); ok {
		return sqlClient.DB()
	}
	return nil
}

// Query executes a query with circuit breaker protection.
func (c *protectedClient) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	sqlClient, ok := c.underlyingSQL()
	if !ok {
		return nil, fmt.Errorf("client is not SQLClient")
	}

	var rows *sql.Rows
	err := c.breaker.Execute(ctx, func() error {
		var err error
		rows, err = sqlClient.Query(ctx, query, args...)
		return err
	})
	return rows, err
}

// QueryRow executes a single-row query.
func (c *protectedClient) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	sqlClient, ok := c.underlyingSQL()
	if !ok {
		return nil
	}
	// The error is only observable at Scan time, so the breaker cannot wrap it.
	return sqlClient.QueryRow(ctx, query, args...)
}

// Exec executes a command with circuit breaker protection.
func (c *protectedClient) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	sqlClient, ok := c.underlyingSQL()
	if !ok {
		return nil, fmt.Errorf("client is not SQLClient")
	}

	var result sql.Result
	err := c.breaker.Execute(ctx, func() error {
		var err error
		result, err = sqlClient.Exec(ctx, query, args...)
		return err
	})
	return result, err
}

// Begin starts a transaction on the wrapped client.
func (c *protectedClient) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	sqlClient, ok := c.underlyingSQL()
	if !ok {
		return nil, fmt.Errorf("client is not SQLClient")
	}
	return sqlClient.Begin(ctx, opts)
}

// Transaction executes a function within a transaction with circuit breaker protection.
func (c *protectedClient) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	sqlClient, ok := c.underlyingSQL()
	if !ok {
		return fmt.Errorf("client is not SQLClient")
	}
	return c.breaker.Execute(ctx, func() error {
		return sqlClient.Transaction(ctx, fn)
	})
}

// GetPoolStats returns pool statistics.
func (p *PoolWithBreaker) GetPoolStats() map[string]pool.ClientStats {
	return p.pool.GetStats()
}

// GetBreakerStats returns circuit breaker statistics.
func (p *PoolWithBreaker) GetBreakerStats() circuitbreaker.Stats {
	return p.breaker.GetStats()
}

// ForceOpen manually opens the circuit breaker.
func (p *PoolWithBreaker) ForceOpen() {
	p.breaker.ForceOpen()
}

// ForceClosed manually closes the circuit breaker.
func (p *PoolWithBreaker) ForceClosed() {
	p.breaker.ForceClosed()
}

// GetState returns current circuit breaker state.
func (p *PoolWithBreaker) GetState() circuitbreaker.State {
	return p.breaker.State()
}

// ==================== SQL Client Support ====================

// SQLPoolWithBreaker extends PoolWithBreaker with SQL operations.
type SQLPoolWithBreaker struct {
	*PoolWithBreaker
}

// NewSQL creates a new SQL pool with circuit breaker.
func NewSQL(config Config, factory *db.Factory) *SQLPoolWithBreaker {
	return &SQLPoolWithBreaker{
		PoolWithBreaker: New(config, factory),
	}
}

// Query executes a query with circuit breaker protection.
func (p *SQLPoolWithBreaker) Query(ctx context.Context, name string, query string, args ...any) (*sql.Rows, error) {
	client, err := p.Acquire(ctx, name)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sqlClient, ok := client.(db.SQLClient)
	if !ok {
		return nil, fmt.Errorf("client is not SQLClient")
	}

	var rows *sql.Rows
	err = p.breaker.Execute(ctx, func() error {
		var err error
		rows, err = sqlClient.Query(ctx, query, args...)
		return err
	})

	return rows, err
}

// QueryRow executes a query row with circuit breaker protection.
func (p *SQLPoolWithBreaker) QueryRow(ctx context.Context, name string, query string, args ...any) *sql.Row {
	client, err := p.Acquire(ctx, name)
	if err != nil {
		return nil
	}
	defer client.Close()

	sqlClient, ok := client.(db.SQLClient)
	if !ok {
		return nil
	}

	// Note: Can't wrap in circuit breaker without knowing result
	return sqlClient.QueryRow(ctx, query, args...)
}

// Exec executes a command with circuit breaker protection.
func (p *SQLPoolWithBreaker) Exec(ctx context.Context, name string, query string, args ...any) (sql.Result, error) {
	client, err := p.Acquire(ctx, name)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sqlClient, ok := client.(db.SQLClient)
	if !ok {
		return nil, fmt.Errorf("client is not SQLClient")
	}

	var result sql.Result
	err = p.breaker.Execute(ctx, func() error {
		var err error
		result, err = sqlClient.Exec(ctx, query, args...)
		return err
	})

	return result, err
}

// Transaction executes a function within a transaction with circuit breaker protection.
func (p *SQLPoolWithBreaker) Transaction(ctx context.Context, name string, fn func(*sql.Tx) error) error {
	client, err := p.Acquire(ctx, name)
	if err != nil {
		return err
	}
	defer client.Close()

	sqlClient, ok := client.(db.SQLClient)
	if !ok {
		return fmt.Errorf("client is not SQLClient")
	}

	return p.breaker.Execute(ctx, func() error {
		return sqlClient.Transaction(ctx, fn)
	})
}
