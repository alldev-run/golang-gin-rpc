package orm

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang-gin-rpc/pkg/logger"
)

// TransactionConfig holds configuration for transaction operations.
type TransactionConfig struct {
	// Retry configuration
	MaxRetries        int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay        time.Duration `yaml:"retry_delay" json:"retry_delay"`
	RetryBackoffFactor float64      `yaml:"retry_backoff_factor" json:"retry_backoff_factor"`
	MaxRetryDelay     time.Duration `yaml:"max_retry_delay" json:"max_retry_delay"`
	
	// Transaction configuration
	IsolationLevel    sql.IsolationLevel `yaml:"isolation_level" json:"isolation_level"`
	ReadOnly          bool              `yaml:"read_only" json:"read_only"`
	Timeout           time.Duration     `yaml:"timeout" json:"timeout"`
	
	// Monitoring and logging
	EnableMetrics     bool `yaml:"enable_metrics" json:"enable_metrics"`
	EnableTracing     bool `yaml:"enable_tracing" json:"enable_tracing"`
	LogSlowQueries    bool `yaml:"log_slow_queries" json:"log_slow_queries"`
	SlowQueryThreshold time.Duration `yaml:"slow_query_threshold" json:"slow_query_threshold"`
	
	// Distributed transaction support
	EnableDistributed bool   `yaml:"enable_distributed" json:"enable_distributed"`
	CoordinatorURL    string `yaml:"coordinator_url" json:"coordinator_url"`
	
	// Connection pool configuration
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

// TransactionManager provides enhanced transaction management with enterprise features.
type TransactionManager struct {
	config          TransactionConfig
	metrics         *TransactionMetrics
	mu              sync.RWMutex
	activeTxCount   int64
	totalTxCount    int64
	coordinator     DistributedTransactionCoordinator
	tracer          TransactionTracer
	monitor         TransactionMonitor
	errorClassifier ErrorClassifier
	retryStrategy   RetryStrategy
	connectionPool  *ConnectionPool
}

// DistributedTransactionCoordinator interface for distributed transaction management.
type DistributedTransactionCoordinator interface {
	BeginTransaction(ctx context.Context, txID string) error
	CommitTransaction(ctx context.Context, txID string) error
	RollbackTransaction(ctx context.Context, txID string) error
	GetTransactionStatus(ctx context.Context, txID string) (TransactionStatus, error)
}

// TransactionTracer interface for distributed tracing.
type TransactionTracer interface {
	StartSpan(ctx context.Context, operationName string) (context.Context, Span)
	FinishSpan(span Span)
	InjectContext(ctx context.Context, headers map[string]string)
	ExtractContext(headers map[string]string) (context.Context, error)
}

// TransactionMonitor interface for transaction monitoring.
type TransactionMonitor interface {
	OnTransactionStart(txID string, config TransactionConfig)
	OnTransactionCommit(txID string, result *TransactionResult)
	OnTransactionRollback(txID string, result *TransactionResult)
	OnTransactionRetry(txID string, attempt int, err error)
	OnSlowQuery(txID string, query string, duration time.Duration)
}

// ErrorClassifier interface for error classification.
type ErrorClassifier interface {
	ClassifyError(err error) ErrorClassification
	IsRetryable(err error) bool
	IsTransient(err error) bool
	GetRetryDelay(attempt int, err error) time.Duration
}

// RetryStrategy interface for retry strategies.
type RetryStrategy interface {
	ShouldRetry(attempt int, err error) bool
	GetDelay(attempt int, err error) time.Duration
	GetMaxAttempts() int
}

// Span interface for distributed tracing.
type Span interface {
	SetTag(key string, value interface{})
	SetBaggageItem(key, value string)
	Finish()
	Context() SpanContext
}

// SpanContext interface for span context.
type SpanContext interface {
	ForeachBaggageItem(handler func(key, value string) string)
}

// TransactionStatus represents transaction status.
type TransactionStatus string

const (
	TxStatusActive    TransactionStatus = "active"
	TxStatusCommitted TransactionStatus = "committed"
	TxStatusRolledBack TransactionStatus = "rolled_back"
	TxStatusFailed    TransactionStatus = "failed"
)

// ErrorClassification represents error classification.
type ErrorClassification struct {
	Type        ErrorType `json:"type"`
	Retryable   bool      `json:"retryable"`
	Transient   bool      `json:"transient"`
	Severity    ErrorSeverity `json:"severity"`
	Category    string    `json:"category"`
}

// ErrorType represents error types.
type ErrorType string

const (
	ErrorTypeConnection    ErrorType = "connection"
	ErrorTypeTimeout       ErrorType = "timeout"
	ErrorTypeDeadlock      ErrorType = "deadlock"
	ErrorTypeConstraint    ErrorType = "constraint"
	ErrorTypePermission    ErrorType = "permission"
	ErrorTypeResource      ErrorType = "resource"
	ErrorTypeLogic         ErrorType = "logic"
	ErrorTypeUnknown       ErrorType = "unknown"
)

// ErrorSeverity represents error severity.
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// TransactionMetrics holds transaction performance metrics.
type TransactionMetrics struct {
	TotalTransactions    int64         `json:"total_transactions"`
	SuccessfulTransactions int64      `json:"successful_transactions"`
	FailedTransactions   int64         `json:"failed_transactions"`
	RetriedTransactions  int64         `json:"retried_transactions"`
	AvgDuration          time.Duration `json:"avg_duration"`
	MaxDuration          time.Duration `json:"max_duration"`
	MinDuration          time.Duration `json:"min_duration"`
	LastReset            time.Time     `json:"last_reset"`
	mu                   sync.RWMutex  `json:"-"`
}

// TransactionResult contains the result of a transaction operation.
type TransactionResult struct {
	Success         bool          `json:"success"`
	Duration        time.Duration `json:"duration"`
	Retries         int           `json:"retries"`
	Error           error         `json:"error,omitempty"`
	TransactionID   string        `json:"transaction_id"`
	IsolationLevel  sql.IsolationLevel `json:"isolation_level"`
	ReadOnly        bool          `json:"read_only"`
	RetryAttempts   []RetryAttempt `json:"retry_attempts,omitempty"`
	Metrics         *DetailedMetrics `json:"metrics,omitempty"`
}

// RetryAttempt contains information about a retry attempt.
type RetryAttempt struct {
	AttemptNumber int           `json:"attempt_number"`
	Error        error         `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	Timestamp    time.Time     `json:"timestamp"`
}

// DetailedMetrics contains detailed transaction metrics.
type DetailedMetrics struct {
	QueryCount       int           `json:"query_count"`
	RowsAffected     int64         `json:"rows_affected"`
	LockWaitTime     time.Duration `json:"lock_wait_time"`
	IndexUsage       map[string]int64 `json:"index_usage"`
	TableAccess      map[string]int64 `json:"table_access"`
	MemoryUsage      int64         `json:"memory_usage"`
}

// NewTransactionManager creates a new TransactionManager with enterprise settings.
func NewTransactionManager(config TransactionConfig) *TransactionManager {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 100 * time.Millisecond
	}
	if config.RetryBackoffFactor == 0 {
		config.RetryBackoffFactor = 2.0
	}
	if config.MaxRetryDelay == 0 {
		config.MaxRetryDelay = 5 * time.Second
	}
	if config.SlowQueryThreshold == 0 {
		config.SlowQueryThreshold = 1 * time.Second
	}
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 30 * time.Minute
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 5 * time.Minute
	}

	tm := &TransactionManager{
		config:          config,
		metrics:         &TransactionMetrics{LastReset: time.Now()},
		errorClassifier: NewDefaultErrorClassifier(),
		retryStrategy:   NewExponentialBackoffStrategy(config),
		monitor:         NewDefaultTransactionMonitor(config),
	}

	// Initialize distributed transaction coordinator if enabled
	if config.EnableDistributed && config.CoordinatorURL != "" {
		tm.coordinator = NewHTTPCoordinator(config.CoordinatorURL)
	}

	// Initialize tracer if enabled
	if config.EnableTracing {
		tm.tracer = NewJaegerTracer("orm-transaction-manager")
	}

	// Initialize connection pool
	tm.connectionPool = NewConnectionPool(config)

	return tm
}

// NewDefaultTransactionManager creates a TransactionManager with default enterprise settings.
func NewDefaultTransactionManager() *TransactionManager {
	return NewTransactionManager(TransactionConfig{
		MaxRetries:         3,
		RetryDelay:         100 * time.Millisecond,
		RetryBackoffFactor: 2.0,
		MaxRetryDelay:      5 * time.Second,
		IsolationLevel:     sql.LevelReadCommitted,
		ReadOnly:           false,
		Timeout:            30 * time.Second,
		EnableMetrics:      true,
		EnableTracing:      true,
		LogSlowQueries:     true,
		SlowQueryThreshold: 1 * time.Second,
		EnableDistributed:  false,
		MaxOpenConns:       25,
		MaxIdleConns:       5,
		ConnMaxLifetime:    30 * time.Minute,
		ConnMaxIdleTime:    5 * time.Minute,
	})
}

// WithTransaction executes a function within a transaction with enterprise features.
func (tm *TransactionManager) WithTransaction(ctx context.Context, db DB, fn func(*ORM) error) (*TransactionResult, error) {
	txID := tm.generateTransactionID()
	startTime := time.Now()
	var retryAttempts []RetryAttempt
	var lastErr error

	// Update active transaction count
	atomic.AddInt64(&tm.activeTxCount, 1)
	defer atomic.AddInt64(&tm.activeTxCount, -1)

	// Update total transaction count
	atomic.AddInt64(&tm.totalTxCount, 1)

	// Start distributed transaction if enabled
	if tm.config.EnableDistributed && tm.coordinator != nil {
		if err := tm.coordinator.BeginTransaction(ctx, txID); err != nil {
			return &TransactionResult{
				Success:       false,
				Duration:      time.Since(startTime),
				Retries:       0,
				Error:         fmt.Errorf("failed to begin distributed transaction: %w", err),
				TransactionID: txID,
			}, err
		}
	}

	// Start tracing span if enabled
	var span Span
	if tm.config.EnableTracing && tm.tracer != nil {
		ctx, span = tm.tracer.StartSpan(ctx, "transaction")
		defer func() {
			if span != nil {
				tm.tracer.FinishSpan(span)
			}
		}()
		span.SetTag("transaction.id", txID)
		span.SetTag("transaction.isolation_level", tm.config.IsolationLevel.String())
		span.SetTag("transaction.read_only", tm.config.ReadOnly)
	}

	// Notify monitor
	if tm.monitor != nil {
		tm.monitor.OnTransactionStart(txID, tm.config)
	}

	// Apply timeout if configured
	if tm.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, tm.config.Timeout)
		defer cancel()
	}

	for attempt := 0; attempt <= tm.config.MaxRetries; attempt++ {
		attemptStartTime := time.Now()
		
		result, err := tm.executeTransactionOnce(ctx, db, fn, txID)
		
		// Record retry attempt
		retryAttempt := RetryAttempt{
			AttemptNumber: attempt + 1,
			Error:        err,
			Duration:     time.Since(attemptStartTime),
			Timestamp:    time.Now(),
		}
		retryAttempts = append(retryAttempts, retryAttempt)

		if err == nil {
			result.Success = true
			result.Duration = time.Since(startTime)
			result.Retries = attempt
			result.TransactionID = txID
			result.IsolationLevel = tm.config.IsolationLevel
			result.ReadOnly = tm.config.ReadOnly
			result.RetryAttempts = retryAttempts

			// Update metrics
		tm.updateMetrics(result)

			// Commit distributed transaction
			if tm.config.EnableDistributed && tm.coordinator != nil {
				if commitErr := tm.coordinator.CommitTransaction(ctx, txID); commitErr != nil {
					result.Success = false
					result.Error = fmt.Errorf("failed to commit distributed transaction: %w", commitErr)
					tm.monitor.OnTransactionRollback(txID, result)
					return result, commitErr
				}
			}

			// Notify monitor
			if tm.monitor != nil {
				tm.monitor.OnTransactionCommit(txID, result)
			}

			return result, nil
		}

		lastErr = err
		result.Error = err

		// Check if error is retryable
		if !tm.retryStrategy.ShouldRetry(attempt, err) {
			result.Success = false
			result.Duration = time.Since(startTime)
			result.Retries = attempt
			result.TransactionID = txID
			result.RetryAttempts = retryAttempts

			// Rollback distributed transaction
			if tm.config.EnableDistributed && tm.coordinator != nil {
				tm.coordinator.RollbackTransaction(ctx, txID)
			}

			tm.monitor.OnTransactionRollback(txID, result)
			return result, err
		}

		// Notify monitor of retry
		if tm.monitor != nil {
			tm.monitor.OnTransactionRetry(txID, attempt+1, err)
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < tm.config.MaxRetries {
			delay := tm.retryStrategy.GetDelay(attempt, err)
			
			// Add tracing tag
			if span != nil {
				span.SetTag("retry.attempt", attempt+1)
				span.SetTag("retry.delay", delay.String())
				span.SetTag("retry.error", err.Error())
			}

			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				result.Success = false
				result.Duration = time.Since(startTime)
				result.Retries = attempt
				result.TransactionID = txID
				result.Error = ctx.Err()
				result.RetryAttempts = retryAttempts

				// Rollback distributed transaction
				if tm.config.EnableDistributed && tm.coordinator != nil {
					tm.coordinator.RollbackTransaction(ctx, txID)
				}

				tm.monitor.OnTransactionRollback(txID, result)
				return result, ctx.Err()
			}
		}
	}

	result := &TransactionResult{
		Success:       false,
		Duration:      time.Since(startTime),
		Retries:       tm.config.MaxRetries,
		Error:         fmt.Errorf("transaction failed after %d attempts: %w", tm.config.MaxRetries+1, lastErr),
		TransactionID: txID,
		RetryAttempts: retryAttempts,
	}

	// Rollback distributed transaction
	if tm.config.EnableDistributed && tm.coordinator != nil {
		tm.coordinator.RollbackTransaction(ctx, txID)
	}

	tm.monitor.OnTransactionRollback(txID, result)
	return result, fmt.Errorf("transaction failed after %d attempts: %w", tm.config.MaxRetries+1, lastErr)
}

// WithRetry executes a function with enterprise retry logic.
func (tm *TransactionManager) WithRetry(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= tm.config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !tm.retryStrategy.ShouldRetry(attempt, err) {
			return err
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < tm.config.MaxRetries {
			delay := tm.retryStrategy.GetDelay(attempt, err)
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", tm.config.MaxRetries+1, lastErr)
}

// WithNestedTransaction executes a function within a nested transaction (savepoint) with enterprise features.
func (tm *TransactionManager) WithNestedTransaction(ctx context.Context, orm *ORM, savepointName string, fn func(*ORM) error) error {
	if savepointName == "" {
		savepointName = fmt.Sprintf("sp_%d_%d", time.Now().UnixNano(), atomic.AddInt64(&tm.totalTxCount, 1))
	}

	// Start tracing span if enabled
	var span Span
	if tm.config.EnableTracing && tm.tracer != nil {
		ctx, span = tm.tracer.StartSpan(ctx, "nested_transaction")
		defer func() {
			if span != nil {
				tm.tracer.FinishSpan(span)
			}
		}()
		span.SetTag("savepoint.name", savepointName)
	}

	startTime := time.Now()

	// Create savepoint
	if err := tm.createSavepoint(ctx, orm.DB(), savepointName); err != nil {
		return fmt.Errorf("failed to create savepoint %s: %w", savepointName, err)
	}

	// Execute function
	if err := fn(orm); err != nil {
		// Rollback to savepoint on error
		if rollbackErr := tm.rollbackToSavepoint(ctx, orm.DB(), savepointName); rollbackErr != nil {
			return fmt.Errorf("failed to rollback to savepoint %s: %w (original error: %w)", savepointName, rollbackErr, err)
		}
		
		// Log slow operations
		if tm.config.LogSlowQueries && time.Since(startTime) > tm.config.SlowQueryThreshold {
			logger.Warn("Slow nested transaction rollback",
				zap.String("savepoint", savepointName),
				zap.Duration("duration", time.Since(startTime)),
				zap.String("error", err.Error()))
		}
		
		return err
	}

	// Release savepoint on success
	if err := tm.releaseSavepoint(ctx, orm.DB(), savepointName); err != nil {
		return fmt.Errorf("failed to release savepoint %s: %w", savepointName, err)
	}

	// Log slow operations
	if tm.config.LogSlowQueries && time.Since(startTime) > tm.config.SlowQueryThreshold {
		logger.Info("Slow nested transaction",
			zap.String("savepoint", savepointName),
			zap.Duration("duration", time.Since(startTime)))
	}

	return nil
}

// executeTransactionOnce executes a transaction function once.
func (tm *TransactionManager) executeTransactionOnce(ctx context.Context, db DB, fn func(*ORM) error, txID string) (*TransactionResult, error) {
	startTime := time.Now()
	
	// Begin transaction
	tx, err := db.(*DBWrapper).DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: tm.config.IsolationLevel,
		ReadOnly:  tm.config.ReadOnly,
	})
	if err != nil {
		return &TransactionResult{
			Error:         err,
			TransactionID: txID,
		}, err
	}

	// Ensure transaction is cleaned up
	var committed bool
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Create ORM instance with transaction
	txORM := &ORM{
		db:      NewTxWrapper(tx),
		dialect: NewDefaultDialect(), // You might want to get this from the original ORM
	}

	// Execute function
	if err := fn(txORM); err != nil {
		return &TransactionResult{
			Error:         err,
			TransactionID: txID,
			Duration:      time.Since(startTime),
		}, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &TransactionResult{
			Error:         err,
			TransactionID: txID,
			Duration:      time.Since(startTime),
		}, err
	}

	committed = true
	return &TransactionResult{
		Success:       true,
		TransactionID: txID,
		Duration:      time.Since(startTime),
	}, nil
}

// createSavepoint creates a savepoint in the current transaction.
func (tm *TransactionManager) createSavepoint(ctx context.Context, db DB, name string) error {
	query := fmt.Sprintf("SAVEPOINT %s", name)
	_, err := db.Exec(ctx, query)
	return err
}

// rollbackToSavepoint rolls back to a savepoint.
func (tm *TransactionManager) rollbackToSavepoint(ctx context.Context, db DB, name string) error {
	query := fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", name)
	_, err := db.Exec(ctx, query)
	return err
}

// releaseSavepoint releases a savepoint.
func (tm *TransactionManager) releaseSavepoint(ctx context.Context, db DB, name string) error {
	query := fmt.Sprintf("RELEASE SAVEPOINT %s", name)
	_, err := db.Exec(ctx, query)
	return err
}

// SetConfig updates the transaction manager configuration.
func (tm *TransactionManager) SetConfig(config TransactionConfig) {
	tm.config = config
}

// GetConfig returns the current transaction manager configuration.
func (tm *TransactionManager) GetConfig() TransactionConfig {
	return tm.config
}
