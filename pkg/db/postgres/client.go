// Package postgres provides a PostgreSQL database client with connection pooling,
// configuration management, and common query operations.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
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
	// Logging configuration
	LogEnabled         bool          `yaml:"log_enabled" json:"log_enabled"`
	LogLevel           string        `yaml:"log_level" json:"log_level"` // error, warn, info, debug, trace
	SlowQueryThreshold time.Duration `yaml:"slow_query_threshold" json:"slow_query_threshold"`
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

// SQLLogger provides configurable logging for SQL operations.
type SQLLogger struct {
	level              LogLevel
	slowQueryThreshold time.Duration
}

// LogLevel represents the logging level.
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelTrace:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// parseLogLevel parses log level from string.
func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "error":
		return LogLevelError
	case "warn":
		return LogLevelWarn
	case "info":
		return LogLevelInfo
	case "debug":
		return LogLevelDebug
	case "trace":
		return LogLevelTrace
	default:
		return LogLevelInfo
	}
}

// NewSQLLogger creates a new SQL logger with specific level and threshold.
func NewSQLLogger(level string, threshold time.Duration) *SQLLogger {
	return &SQLLogger{
		level:              parseLogLevel(level),
		slowQueryThreshold: threshold,
	}
}

// LogQuery logs a query execution.
func (sl *SQLLogger) LogQuery(query string, args []interface{}, duration time.Duration, err error) {
	msg := "SQL query executed"
	if err != nil {
		msg = "SQL query failed"
	} else if duration > sl.slowQueryThreshold {
		msg = "Slow SQL query detected"
	}

	// Build log message with key details
	logMsg := msg +
		" | query: " + query +
		" | duration: " + duration.String()

	if len(args) > 0 {
		logMsg += " | args: " + fmt.Sprintf("%v", args)
	}

	if err != nil {
		logMsg += " | error: " + err.Error()
		logger.Errorf(logMsg)
		return
	}

	// Check for slow queries
	if duration > sl.slowQueryThreshold {
		logMsg += " | threshold: " + sl.slowQueryThreshold.String()
		logger.Warn(logMsg)
		return
	}

	// Log based on level
	if sl.level >= LogLevelInfo {
		logger.Info(logMsg)
	} else if sl.level >= LogLevelDebug {
		logger.Debug(logMsg)
	}
}

// Client wraps sql.DB with additional functionality.
type Client struct {
	db        *sql.DB
	config    Config
	sqlLogger *SQLLogger
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

	client := &Client{
		db:     db,
		config: config,
	}

	// Initialize SQL logger if enabled
	if config.LogEnabled {
		client.sqlLogger = NewSQLLogger(config.LogLevel, config.SlowQueryThreshold)
		logger.Info("SQL logger initialized",
			logger.String("level", config.LogLevel),
			logger.String("threshold", config.SlowQueryThreshold.String()))
	}

	return client, nil
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
	start := time.Now()
	rows, err := c.db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	if c.sqlLogger != nil {
		c.sqlLogger.LogQuery(query, args, duration, err)
	}

	return rows, err
}

// QueryRow executes a query that returns a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := c.db.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	if c.sqlLogger != nil {
		c.sqlLogger.LogQuery(query, args, duration, nil)
	}

	return row
}

// Exec executes a query without returning rows (INSERT, UPDATE, DELETE).
func (c *Client) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := c.db.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	if c.sqlLogger != nil {
		c.sqlLogger.LogQuery(query, args, duration, err)
	}

	return result, err
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
