// Package orm provides a database-agnostic ORM layer with query builders.
package orm

import (
	"context"
	"database/sql"
	"time"
)

// DB defines the interface for database operations.
type DB interface {
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Ping(ctx context.Context) error
	Stats() sql.DBStats
	Close() error
}

// DBWrapper wraps sql.DB to implement the DB interface.
type DBWrapper struct {
	*sql.DB
}

// NewDBWrapper creates a new DB wrapper.
func NewDBWrapper(db *sql.DB) *DBWrapper {
	return &DBWrapper{DB: db}
}

// Query executes a query with context.
func (w *DBWrapper) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return w.DB.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row with context.
func (w *DBWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return w.DB.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning rows with context.
func (w *DBWrapper) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return w.DB.ExecContext(ctx, query, args...)
}

// Begin begins a transaction with context.
func (w *DBWrapper) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return w.DB.BeginTx(ctx, opts)
}

// Ping checks the database connection health.
func (w *DBWrapper) Ping(ctx context.Context) error {
	return w.DB.PingContext(ctx)
}

// Stats returns database connection statistics.
func (w *DBWrapper) Stats() sql.DBStats {
	return w.DB.Stats()
}

// Close closes the database connection.
func (w *DBWrapper) Close() error {
	return w.DB.Close()
}

// TxWrapper wraps sql.Tx to implement the DB interface for transactions.
type TxWrapper struct {
	*sql.Tx
}

// NewTxWrapper creates a new transaction wrapper.
func NewTxWrapper(tx *sql.Tx) *TxWrapper {
	return &TxWrapper{Tx: tx}
}

// Query executes a query within the transaction.
func (t *TxWrapper) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row within the transaction.
func (t *TxWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.Tx.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning rows within the transaction.
func (t *TxWrapper) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, query, args...)
}

// Begin is not supported for transactions.
func (t *TxWrapper) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, sql.ErrTxDone
}

// Ping is not supported for transactions.
func (t *TxWrapper) Ping(ctx context.Context) error {
	return nil
}

// Stats is not supported for transactions.
func (t *TxWrapper) Stats() sql.DBStats {
	return sql.DBStats{}
}

// Close is not supported for transactions.
func (t *TxWrapper) Close() error {
	return nil
}

// ConnectionConfig holds database connection configuration.
type ConnectionConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConnectionConfig returns default connection configuration.
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}
}

// ConfigureConnection applies connection configuration to a database instance.
func ConfigureConnection(db *sql.DB, config ConnectionConfig) {
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}
}
