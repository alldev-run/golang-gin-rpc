// Package postgres provides a PostgreSQL database client with connection pooling,
// configuration management, and common query operations.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Config holds PostgreSQL connection configuration.
type Config struct {
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	Database        string        `yaml:"database" json:"database"`
	Username        string        `yaml:"username" json:"username"`
	Password        string        `yaml:"password" json:"password"`
	SSLMode         string        `yaml:"ssl_mode" json:"ssl_mode"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
}

// DefaultConfig returns default PostgreSQL configuration.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            5432,
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
	}
}

// Client wraps sql.DB with additional functionality.
type Client struct {
	db     *sql.DB
	config Config
}

// New creates a new PostgreSQL client from config.
func New(config Config) (*Client, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.Username,
		config.Password,
		config.Database,
		config.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &Client{
		db:     db,
		config: config,
	}, nil
}

// DB returns the underlying sql.DB instance.
func (c *Client) DB() *sql.DB {
	return c.db
}

// Close closes the database connection.
func (c *Client) Close() error {
	return c.db.Close()
}

// Ping checks the database connection health.
func (c *Client) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Stats returns database connection statistics.
func (c *Client) Stats() sql.DBStats {
	return c.db.Stats()
}

// Query executes a query that returns multiple rows.
func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning rows (INSERT, UPDATE, DELETE).
func (c *Client) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}

// Begin starts a new transaction.
func (c *Client) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, opts)
}

// Transaction executes a function within a transaction.
func (c *Client) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.Begin(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Prepare creates a prepared statement.
func (c *Client) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	return c.db.PrepareContext(ctx, query)
}

// CopyFrom performs a bulk copy operation using COPY FROM.
// This is more efficient than multiple INSERT statements.
func (c *Client) CopyFrom(ctx context.Context, tableName string, columns []string, rows [][]any) error {
	// For bulk operations, use COPY protocol
	// This is a placeholder - actual implementation would use pgx.CopyFrom
	// or similar for maximum efficiency
	return fmt.Errorf("bulk copy not implemented in base client")
}
