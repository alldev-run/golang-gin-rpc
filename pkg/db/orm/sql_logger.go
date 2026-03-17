package orm

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"alldev-gin-rpc/pkg/logger"
)

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


// SQLLogger provides configurable logging for SQL operations.
type SQLLogger struct {
	level              LogLevel
	slowQueryThreshold time.Duration
}

// QueryLog represents a logged SQL query.
type QueryLog struct {
	Query     string        // The SQL query
	Args      []interface{} // Query arguments
	Duration  time.Duration // How long the query took
	Error     error         // Any error that occurred
	Timestamp time.Time     // When the query was executed
}

// NewSQLLogger creates a new SQL logger with default settings.
func NewSQLLogger() *SQLLogger {
	return &SQLLogger{
		level:              LogLevelInfo,
		slowQueryThreshold: 100 * time.Millisecond,
	}
}

// NewSQLLoggerWithConfig creates a new SQL logger with specific level and threshold.
func NewSQLLoggerWithConfig(level LogLevel, threshold time.Duration) *SQLLogger {
	return &SQLLogger{
		level:              level,
		slowQueryThreshold: threshold,
	}
}

// SetLevel sets the logging level.
func (sl *SQLLogger) SetLevel(level LogLevel) {
	sl.level = level
}

// SetSlowQueryThreshold sets the threshold for logging slow queries.
func (sl *SQLLogger) SetSlowQueryThreshold(threshold time.Duration) {
	sl.slowQueryThreshold = threshold
}


// LogQuery logs a query execution.
func (sl *SQLLogger) LogQuery(queryLog *QueryLog) {
	if sl.level < LogLevelInfo {
		return
	}

	msg := "SQL query executed"
	if queryLog.Error != nil {
		msg = "SQL query failed"
	} else if queryLog.Duration > sl.slowQueryThreshold {
		msg = "Slow SQL query detected"
	}

	// Build log message with key details
	logMsg := msg +
		" | query: " + queryLog.Query +
		" | duration: " + queryLog.Duration.String() +
		" | timestamp: " + queryLog.Timestamp.Format("2006-01-02 15:04:05")

	if len(queryLog.Args) > 0 {
		logMsg += " | args: " + fmt.Sprintf("%v", queryLog.Args)
	}

	if queryLog.Error != nil {
		logMsg += " | error: " + queryLog.Error.Error()
		logger.Errorf(logMsg)
		return
	}

	// Check for slow queries
	if queryLog.Duration > sl.slowQueryThreshold {
		logMsg += " | threshold: " + sl.slowQueryThreshold.String()
		logger.Warn(logMsg)
		return
	}

	if sl.level >= LogLevelInfo {
		logger.Info(logMsg)
	}
}

// LogTransaction logs transaction operations.
func (sl *SQLLogger) LogTransaction(operation string, duration time.Duration, err error) {
	logMsg := "Transaction completed" +
		" | operation: " + operation +
		" | duration: " + duration.String()

	if err != nil {
		logMsg = "Transaction failed" +
			" | operation: " + operation +
			" | duration: " + duration.String() +
			" | error: " + err.Error()
		logger.Errorf(logMsg)
	} else if sl.level >= LogLevelInfo {
		logger.Info(logMsg)
	}
}

// LoggedDB wraps a DB interface to provide automatic query logging.
type LoggedDB struct {
	db     DB
	logger *SQLLogger
}

// NewLoggedDB creates a new logged database wrapper.
func NewLoggedDB(db DB, logger *SQLLogger) *LoggedDB {
	if logger == nil {
		logger = NewSQLLogger()
	}
	return &LoggedDB{
		db:     db,
		logger: logger,
	}
}

// Exec executes a query and logs it.
func (ldb *LoggedDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := ldb.db.Exec(ctx, query, args...)
	duration := time.Since(start)

	ldb.logger.LogQuery(&QueryLog{
		Query:     query,
		Args:      args,
		Duration:  duration,
		Error:     err,
		Timestamp: start,
	})

	return result, err
}

// Query executes a SELECT query and logs it.
func (ldb *LoggedDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := ldb.db.Query(ctx, query, args...)
	duration := time.Since(start)

	ldb.logger.LogQuery(&QueryLog{
		Query:     query,
		Args:      args,
		Duration:  duration,
		Error:     err,
		Timestamp: start,
	})

	return rows, err
}

// QueryRow executes a query that returns a single row and logs it.
func (ldb *LoggedDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()

	// For QueryRow, we log at trace level since we can't measure duration
	if ldb.logger.level >= LogLevelTrace {
		logMsg := "SQL query row" +
			" | query: " + query +
			" | timestamp: " + start.Format("2006-01-02 15:04:05")
		if len(args) > 0 {
			logMsg += " | args: " + fmt.Sprintf("%v", args)
		}
		logger.Debug(logMsg)
	}

	return ldb.db.QueryRow(ctx, query, args...)
}

// Begin starts a transaction and logs it.
func (ldb *LoggedDB) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	start := time.Now()
	tx, err := ldb.db.Begin(ctx, opts)
	duration := time.Since(start)

	ldb.logger.LogTransaction("begin", duration, err)

	return tx, err
}

// Ping checks the database connection and logs it.
func (ldb *LoggedDB) Ping(ctx context.Context) error {
	start := time.Now()
	err := ldb.db.Ping(ctx)
	duration := time.Since(start)

	ldb.logger.LogQuery(&QueryLog{
		Query:     "PING",
		Args:      nil,
		Duration:  duration,
		Error:     err,
		Timestamp: start,
	})

	return err
}

// Close closes the database connection.
func (ldb *LoggedDB) Close() error {
	return ldb.db.Close()
}

// Stats returns database statistics.
func (ldb *LoggedDB) Stats() sql.DBStats {
	return ldb.db.Stats()
}

// SetLogger sets the logger for the logged database.
func (ldb *LoggedDB) SetLogger(logger *SQLLogger) {
	ldb.logger = logger
}

// GetLogger returns the current logger.
func (ldb *LoggedDB) GetLogger() *SQLLogger {
	return ldb.logger
}
