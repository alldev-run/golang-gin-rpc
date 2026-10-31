// Package clickhouse provides a ClickHouse database client optimized for
// analytical queries and bulk inserts.
package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config holds ClickHouse connection configuration.
type Config struct {
	Hosts           []string      `yaml:"hosts" json:"hosts"`
	Database        string        `yaml:"database" json:"database"`
	Username        string        `yaml:"username" json:"username"`
	Password        string        `yaml:"password" json:"password"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	DialTimeout     time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
}

// DefaultConfig returns default ClickHouse configuration.
func DefaultConfig() Config {
	return Config{
		Hosts:           []string{"localhost:9000"},
		Database:        "default",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		DialTimeout:     5 * time.Second,
	}
}

// Client wraps ClickHouse connection with additional functionality.
type Client struct {
	conn   driver.Conn
	config Config
}

// New creates a new ClickHouse client from config.
func New(config Config) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: config.Hosts,
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		MaxOpenConns:    config.MaxOpenConns,
		MaxIdleConns:    config.MaxIdleConns,
		ConnMaxLifetime: config.ConnMaxLifetime,
		DialTimeout:     config.DialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	return &Client{
		conn:   conn,
		config: config,
	}, nil
}

// Conn returns the underlying ClickHouse connection.
func (c *Client) Conn() driver.Conn {
	return c.conn
}

// Close closes the ClickHouse connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Ping checks the ClickHouse connection health.
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// Query executes a query that returns multiple rows.
func (c *Client) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

// QueryRow executes a query that returns a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return c.conn.QueryRow(ctx, query, args...)
}

// Exec executes a query without returning rows.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	return c.conn.Exec(ctx, query, args...)
}

// AsyncInsert performs an asynchronous insert for better performance.
func (c *Client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return c.conn.AsyncInsert(ctx, query, wait, args...)
}

// BatchCreate creates a new batch for bulk inserts.
func (c *Client) BatchCreate(ctx context.Context, query string) (driver.Batch, error) {
	return c.conn.PrepareBatch(ctx, query)
}

// InsertBatch performs a batch insert for improved performance.
func (c *Client) InsertBatch(ctx context.Context, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}

	// Build column list
	colList := ""
	for i, col := range columns {
		if i > 0 {
			colList += ", "
		}
		colList += col
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES", table, colList)
	batch, err := c.conn.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create batch: %w", err)
	}

	// Append rows
	for _, row := range rows {
		if err := batch.Append(row...); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}

	// Send batch
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}
