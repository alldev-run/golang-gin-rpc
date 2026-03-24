// Package slowquery provides slow query logging functionality for SQL databases.
// It integrates with the project's logger module to record queries exceeding
// a configurable threshold.
package slowquery

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// Config holds slow query logger configuration.
type Config struct {
	// Threshold is the minimum duration to consider a query as slow
	Threshold time.Duration
	// MaxQueryLen limits the logged query length (0 = unlimited)
	MaxQueryLen int
	// IncludeArgs controls whether to log query arguments
	IncludeArgs bool
	// SampleRate limits logging to 1/N queries (1 = log all slow queries)
	SampleRate int
}

// DefaultConfig returns default slow query configuration.
func DefaultConfig() Config {
	return Config{
		Threshold:   100 * time.Millisecond,
		MaxQueryLen: 1000,
		IncludeArgs: false,
		SampleRate:  1,
	}
}

// Logger wraps a SQL client with slow query logging.
type Logger struct {
	config  Config
	counter uint64 // sample counter
}

// New creates a new slow query logger.
func New(config Config) *Logger {
	if config.Threshold == 0 {
		config.Threshold = DefaultConfig().Threshold
	}
	if config.SampleRate == 0 {
		config.SampleRate = 1
	}
	return &Logger{
		config: config,
	}
}

// QueryFunc is the function signature for database queries.
type QueryFunc func(ctx context.Context, query string, args ...any) (*sql.Rows, error)

// QueryRowFunc is the function signature for database query row.
type QueryRowFunc func(ctx context.Context, query string, args ...any) *sql.Row

// ExecFunc is the function signature for database exec.
type ExecFunc func(ctx context.Context, query string, args ...any) (sql.Result, error)

// WrapQuery wraps a query function with slow query logging.
func (l *Logger) WrapQuery(fn QueryFunc) QueryFunc {
	return func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
		start := time.Now()
		rows, err := fn(ctx, query, args...)
		duration := time.Since(start)

		l.logIfSlow(query, args, duration, err)
		return rows, err
	}
}

// WrapQueryRow wraps a query row function with slow query logging.
func (l *Logger) WrapQueryRow(fn QueryRowFunc) QueryRowFunc {
	return func(ctx context.Context, query string, args ...any) *sql.Row {
		start := time.Now()
		row := fn(ctx, query, args...)
		duration := time.Since(start)

		// Note: Can't get error until Scan, so we log optimistically
		l.logIfSlow(query, args, duration, nil)
		return row
	}
}

// WrapExec wraps an exec function with slow query logging.
func (l *Logger) WrapExec(fn ExecFunc) ExecFunc {
	return func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		start := time.Now()
		result, err := fn(ctx, query, args...)
		duration := time.Since(start)

		l.logIfSlow(query, args, duration, err)
		return result, err
	}
}

// logIfSlow logs the query if it exceeds the threshold.
func (l *Logger) logIfSlow(query string, args []any, duration time.Duration, err error) {
	// Check threshold
	if duration < l.config.Threshold {
		return
	}

	// Sample rate limiting (simple implementation)
	if l.config.SampleRate > 1 {
		l.counter++
		if l.counter%uint64(l.config.SampleRate) != 0 {
			return
		}
	}

	// Truncate query if needed
	loggedQuery := query
	if l.config.MaxQueryLen > 0 && len(query) > l.config.MaxQueryLen {
		loggedQuery = query[:l.config.MaxQueryLen] + "..."
	}

	// Build log fields
	fields := []zap.Field{
		zap.Duration("duration", duration),
		zap.String("query", loggedQuery),
	}

	if l.config.IncludeArgs && len(args) > 0 {
		fields = append(fields, zap.Any("args", args))
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	// Log as warning (slow query is a performance issue)
	logger.Warn("slow query detected", fields...)
}

// LogManual allows manual logging of slow operations.
func (l *Logger) LogManual(operation string, duration time.Duration, err error, extraFields ...zap.Field) {
	if duration < l.config.Threshold {
		return
	}

	fields := []zap.Field{
		zap.Duration("duration", duration),
		zap.String("operation", operation),
	}
	fields = append(fields, extraFields...)

	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	logger.Warn("slow operation detected", fields...)
}

// SQLInterceptor wraps a SQL client with slow query logging.
// It implements the SQLClient interface from pkg/db.
type SQLInterceptor struct {
	logger   *Logger
	query    func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	queryRow func(ctx context.Context, query string, args ...any) *sql.Row
	exec     func(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// NewSQLInterceptor creates a new SQL interceptor wrapping existing functions.
func NewSQLInterceptor(
	logger *Logger,
	queryFn func(ctx context.Context, query string, args ...any) (*sql.Rows, error),
	queryRowFn func(ctx context.Context, query string, args ...any) *sql.Row,
	execFn func(ctx context.Context, query string, args ...any) (sql.Result, error),
) *SQLInterceptor {
	return &SQLInterceptor{
		logger:   logger,
		query:    logger.WrapQuery(queryFn),
		queryRow: logger.WrapQueryRow(queryRowFn),
		exec:     logger.WrapExec(execFn),
	}
}

// Query executes a query with slow query logging.
func (s *SQLInterceptor) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.query(ctx, query, args...)
}

// QueryRow executes a query row with slow query logging.
func (s *SQLInterceptor) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.queryRow(ctx, query, args...)
}

// Exec executes a command with slow query logging.
func (s *SQLInterceptor) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.exec(ctx, query, args...)
}

// Helper function to format duration for human readability.
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
