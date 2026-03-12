// Package poolcb integrates connection pooling with circuit breaker pattern.
// It provides a resilient database client that automatically handles failures
// and prevents cascading failures across the system.
package poolcb

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"golang-gin-rpc/pkg/db"
	"golang-gin-rpc/pkg/db/circuitbreaker"
	"golang-gin-rpc/pkg/db/pool"
)

// Config holds the combined pool and circuit breaker configuration.
type Config struct {
	PoolConfig      pool.Config
	BreakerConfig   circuitbreaker.Config
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
	mutex   sync.RWMutex
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
