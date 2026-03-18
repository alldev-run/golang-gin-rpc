package orm

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

// generateTransactionID generates a unique transaction ID.
func (tm *TransactionManager) generateTransactionID() string {
	return fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), atomic.AddInt64(&tm.totalTxCount, 1))
}

// updateMetrics updates transaction metrics.
func (tm *TransactionManager) updateMetrics(result *TransactionResult) {
	if !tm.config.EnableMetrics {
		return
	}

	tm.metrics.mu.Lock()
	defer tm.metrics.mu.Unlock()

	tm.metrics.TotalTransactions++
	
	if result.Success {
		tm.metrics.SuccessfulTransactions++
	} else {
		tm.metrics.FailedTransactions++
	}

	if result.Retries > 0 {
		tm.metrics.RetriedTransactions++
	}

	// Update duration metrics
	if tm.metrics.MinDuration == 0 || result.Duration < tm.metrics.MinDuration {
		tm.metrics.MinDuration = result.Duration
	}
	if result.Duration > tm.metrics.MaxDuration {
		tm.metrics.MaxDuration = result.Duration
	}

	// Calculate average duration
	totalDuration := tm.metrics.AvgDuration * time.Duration(tm.metrics.TotalTransactions-1)
	tm.metrics.AvgDuration = (totalDuration + result.Duration) / time.Duration(tm.metrics.TotalTransactions)

	// Update Prometheus metrics if available
	// TODO: Implement metrics integration when metrics package is available
	/*
	if metrics.DefaultRegistry != nil {
		metrics.DefaultRegistry.Counter("orm_transactions_total").Inc()
		if result.Success {
			metrics.DefaultRegistry.Counter("orm_transactions_success_total").Inc()
		} else {
			metrics.DefaultRegistry.Counter("orm_transactions_failed_total").Inc()
		}
		metrics.DefaultRegistry.Histogram("orm_transaction_duration_seconds").Observe(result.Duration.Seconds())
		if result.Retries > 0 {
			metrics.DefaultRegistry.Counter("orm_transactions_retried_total").Add(float64(result.Retries))
		}
	}
	*/
}

// GetMetrics returns current transaction metrics.
func (tm *TransactionManager) GetMetrics() *TransactionMetrics {
	tm.metrics.mu.RLock()
	defer tm.metrics.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := &TransactionMetrics{
		TotalTransactions:     tm.metrics.TotalTransactions,
		SuccessfulTransactions: tm.metrics.SuccessfulTransactions,
		FailedTransactions:     tm.metrics.FailedTransactions,
		RetriedTransactions:    tm.metrics.RetriedTransactions,
		AvgDuration:           tm.metrics.AvgDuration,
		MaxDuration:           tm.metrics.MaxDuration,
		MinDuration:           tm.metrics.MinDuration,
		LastReset:             tm.metrics.LastReset,
	}
	
	return metrics
}

// ResetMetrics resets all transaction metrics.
func (tm *TransactionManager) ResetMetrics() {
	tm.metrics.mu.Lock()
	defer tm.metrics.mu.Unlock()
	
	tm.metrics = &TransactionMetrics{LastReset: time.Now()}
}

// DefaultErrorClassifier provides default error classification.
type DefaultErrorClassifier struct {
	retryablePatterns    []*regexp.Regexp
	transientPatterns    []*regexp.Regexp
	deadlockPatterns     []*regexp.Regexp
	timeoutPatterns      []*regexp.Regexp
	connectionPatterns   []*regexp.Regexp
	constraintPatterns   []*regexp.Regexp
	permissionPatterns   []*regexp.Regexp
	resourcePatterns     []*regexp.Regexp
}

// NewDefaultErrorClassifier creates a new default error classifier.
func NewDefaultErrorClassifier() *DefaultErrorClassifier {
	return &DefaultErrorClassifier{
		retryablePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)connection.*refused`),
			regexp.MustCompile(`(?i)connection.*reset`),
			regexp.MustCompile(`(?i)connection.*lost`),
			regexp.MustCompile(`(?i)deadlock`),
			regexp.MustCompile(`(?i)lock wait timeout`),
			regexp.MustCompile(`(?i)serialization failure`),
			regexp.MustCompile(`(?i)could not serialize access`),
			regexp.MustCompile(`(?i)timeout`),
			regexp.MustCompile(`(?i)temporary`),
			regexp.MustCompile(`(?i)too many connections`),
			regexp.MustCompile(`(?i)connection pool exhausted`),
			regexp.MustCompile(`(?i)network.*error`),
			regexp.MustCompile(`(?i)read.*timeout`),
			regexp.MustCompile(`(?i)write.*timeout`),
		},
		transientPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)connection.*refused`),
			regexp.MustCompile(`(?i)connection.*reset`),
			regexp.MustCompile(`(?i)connection.*lost`),
			regexp.MustCompile(`(?i)timeout`),
			regexp.MustCompile(`(?i)temporary`),
			regexp.MustCompile(`(?i)network.*error`),
		},
		deadlockPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)deadlock`),
			regexp.MustCompile(`(?i)lock wait timeout`),
			regexp.MustCompile(`(?i)serialization failure`),
			regexp.MustCompile(`(?i)could not serialize access`),
		},
		timeoutPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)timeout`),
			regexp.MustCompile(`(?i)deadline exceeded`),
			regexp.MustCompile(`(?i)context.*canceled`),
		},
		connectionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)connection.*refused`),
			regexp.MustCompile(`(?i)connection.*reset`),
			regexp.MustCompile(`(?i)connection.*lost`),
			regexp.MustCompile(`(?i)too many connections`),
			regexp.MustCompile(`(?i)connection pool exhausted`),
		},
		constraintPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)unique constraint`),
			regexp.MustCompile(`(?i)foreign key constraint`),
			regexp.MustCompile(`(?i)check constraint`),
			regexp.MustCompile(`(?i)not null constraint`),
			regexp.MustCompile(`(?i)duplicate key`),
		},
		permissionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)permission denied`),
			regexp.MustCompile(`(?i)access denied`),
			regexp.MustCompile(`(?i)unauthorized`),
		},
		resourcePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)out of memory`),
			regexp.MustCompile(`(?i)disk full`),
			regexp.MustCompile(`(?i)quota exceeded`),
		},
	}
}

// ClassifyError classifies an error and returns its classification.
func (ec *DefaultErrorClassifier) ClassifyError(err error) ErrorClassification {
	if err == nil {
		return ErrorClassification{
			Type:      ErrorTypeUnknown,
			Retryable: false,
			Transient: false,
			Severity:  SeverityLow,
		}
	}

	errStr := err.Error()
	
	// Check for deadlock errors (highest severity)
	for _, pattern := range ec.deadlockPatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypeDeadlock,
				Retryable: true,
				Transient: true,
				Severity:  SeverityHigh,
				Category:  "concurrency",
			}
		}
	}

	// Check for timeout errors
	for _, pattern := range ec.timeoutPatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypeTimeout,
				Retryable: true,
				Transient: true,
				Severity:  SeverityMedium,
				Category:  "performance",
			}
		}
	}

	// Check for connection errors
	for _, pattern := range ec.connectionPatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypeConnection,
				Retryable: true,
				Transient: true,
				Severity:  SeverityHigh,
				Category:  "infrastructure",
			}
		}
	}

	// Check for constraint errors (not retryable)
	for _, pattern := range ec.constraintPatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypeConstraint,
				Retryable: false,
				Transient: false,
				Severity:  SeverityMedium,
				Category:  "data_integrity",
			}
		}
	}

	// Check for permission errors (not retryable)
	for _, pattern := range ec.permissionPatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypePermission,
				Retryable: false,
				Transient: false,
				Severity:  SeverityCritical,
				Category:  "security",
			}
		}
	}

	// Check for resource errors
	for _, pattern := range ec.resourcePatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypeResource,
				Retryable: false,
				Transient: false,
				Severity:  SeverityCritical,
				Category:  "infrastructure",
			}
		}
	}

	// Check for other retryable patterns
	for _, pattern := range ec.retryablePatterns {
		if pattern.MatchString(errStr) {
			return ErrorClassification{
				Type:      ErrorTypeConnection,
				Retryable: true,
				Transient: true,
				Severity:  SeverityMedium,
				Category:  "infrastructure",
			}
		}
	}

	// Default classification
	return ErrorClassification{
		Type:      ErrorTypeLogic,
		Retryable: false,
		Transient: false,
		Severity:  SeverityLow,
		Category:  "application",
	}
}

// IsRetryable returns true if the error is retryable.
func (ec *DefaultErrorClassifier) IsRetryable(err error) bool {
	classification := ec.ClassifyError(err)
	return classification.Retryable
}

// IsTransient returns true if the error is transient.
func (ec *DefaultErrorClassifier) IsTransient(err error) bool {
	classification := ec.ClassifyError(err)
	return classification.Transient
}

// GetRetryDelay returns the recommended retry delay for the error.
func (ec *DefaultErrorClassifier) GetRetryDelay(attempt int, err error) time.Duration {
	classification := ec.ClassifyError(err)
	
	switch classification.Type {
	case ErrorTypeDeadlock:
		// Exponential backoff for deadlocks
		return time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
	case ErrorTypeTimeout:
		// Longer backoff for timeouts
		return time.Duration(math.Pow(2, float64(attempt))) * 200 * time.Millisecond
	case ErrorTypeConnection:
		// Moderate backoff for connection issues
		return time.Duration(math.Pow(1.5, float64(attempt))) * 100 * time.Millisecond
	default:
		// Default backoff
		return time.Duration(attempt+1) * 100 * time.Millisecond
	}
}

// ExponentialBackoffStrategy implements exponential backoff retry strategy.
type ExponentialBackoffStrategy struct {
	config TransactionConfig
}

// NewExponentialBackoffStrategy creates a new exponential backoff strategy.
func NewExponentialBackoffStrategy(config TransactionConfig) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{config: config}
}

// ShouldRetry returns true if the operation should be retried.
func (ebs *ExponentialBackoffStrategy) ShouldRetry(attempt int, err error) bool {
	if attempt >= ebs.config.MaxRetries {
		return false
	}
	
	// Use error classifier to determine if retryable
	classifier := NewDefaultErrorClassifier()
	return classifier.IsRetryable(err)
}

// GetDelay returns the delay before the next retry.
func (ebs *ExponentialBackoffStrategy) GetDelay(attempt int, err error) time.Duration {
	classifier := NewDefaultErrorClassifier()
	baseDelay := classifier.GetRetryDelay(attempt, err)
	
	// Apply exponential backoff with jitter
	delay := time.Duration(float64(baseDelay) * math.Pow(ebs.config.RetryBackoffFactor, float64(attempt)))
	
	// Add jitter to prevent thundering herd
	jitter := time.Duration(float64(delay) * 0.1 * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0))
	delay += jitter
	
	// Cap at maximum delay
	if delay > ebs.config.MaxRetryDelay {
		delay = ebs.config.MaxRetryDelay
	}
	
	return delay
}

// GetMaxAttempts returns the maximum number of retry attempts.
func (ebs *ExponentialBackoffStrategy) GetMaxAttempts() int {
	return ebs.config.MaxRetries
}

// DefaultTransactionMonitor provides default transaction monitoring.
type DefaultTransactionMonitor struct {
	config TransactionConfig
}

// NewDefaultTransactionMonitor creates a new default transaction monitor.
func NewDefaultTransactionMonitor(config TransactionConfig) *DefaultTransactionMonitor {
	return &DefaultTransactionMonitor{config: config}
}

// OnTransactionStart is called when a transaction starts.
func (dtm *DefaultTransactionMonitor) OnTransactionStart(txID string, config TransactionConfig) {
	if dtm.config.EnableMetrics {
		logDebugf("Transaction started: tx_id=%s, isolation=%s, read_only=%v, timeout=%v",
			txID, config.IsolationLevel.String(), config.ReadOnly, config.Timeout)
	}
}

// OnTransactionCommit is called when a transaction commits.
func (dtm *DefaultTransactionMonitor) OnTransactionCommit(txID string, result *TransactionResult) {
	if dtm.config.EnableMetrics {
		logInfof("Transaction committed: tx_id=%s, duration=%v, retries=%d",
			txID, result.Duration, result.Retries)
	}
}

// OnTransactionRollback is called when a transaction rolls back.
func (dtm *DefaultTransactionMonitor) OnTransactionRollback(txID string, result *TransactionResult) {
	if dtm.config.EnableMetrics {
		logWarnf("Transaction rolled back: tx_id=%s, duration=%v, retries=%d, error=%s",
			txID, result.Duration, result.Retries, result.Error.Error())
	}
}

// OnTransactionRetry is called when a transaction is retried.
func (dtm *DefaultTransactionMonitor) OnTransactionRetry(txID string, attempt int, err error) {
	if dtm.config.EnableMetrics {
		logDebugf("Transaction retry: tx_id=%s, attempt=%d, error=%s",
			txID, attempt, err.Error())
	}
}

// OnSlowQuery is called when a slow query is detected.
func (dtm *DefaultTransactionMonitor) OnSlowQuery(txID string, query string, duration time.Duration) {
	if dtm.config.LogSlowQueries {
		logWarnf("Slow query detected: tx_id=%s, query=%s, duration=%v",
			txID, query, duration)
	}
}

// ConnectionPool manages database connections with enterprise features.
type ConnectionPool struct {
	config TransactionConfig
	db     *sql.DB
	mu     sync.RWMutex
}

// NewConnectionPool creates a new connection pool.
func NewConnectionPool(config TransactionConfig) *ConnectionPool {
	return &ConnectionPool{
		config: config,
	}
}

// Initialize initializes the connection pool with a database connection.
func (cp *ConnectionPool) Initialize(db *sql.DB) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	cp.db = db
	
	// Configure connection pool
	db.SetMaxOpenConns(cp.config.MaxOpenConns)
	db.SetMaxIdleConns(cp.config.MaxIdleConns)
	db.SetConnMaxLifetime(cp.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cp.config.ConnMaxIdleTime)
	
	return nil
}

// GetDB returns the database connection.
func (cp *ConnectionPool) GetDB() *sql.DB {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.db
}

// Close closes the connection pool.
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	if cp.db != nil {
		return cp.db.Close()
	}
	return nil
}

// HTTPCoordinator provides HTTP-based distributed transaction coordination.
type HTTPCoordinator struct {
	baseURL string
}

// NewHTTPCoordinator creates a new HTTP coordinator.
func NewHTTPCoordinator(baseURL string) *HTTPCoordinator {
	return &HTTPCoordinator{baseURL: baseURL}
}

// BeginTransaction begins a distributed transaction.
func (hc *HTTPCoordinator) BeginTransaction(ctx context.Context, txID string) error {
	// Placeholder implementation
	// In a real implementation, this would make an HTTP request to the coordinator
	logDebugf("Beginning distributed transaction: tx_id=%s", txID)
	return nil
}

// CommitTransaction commits a distributed transaction.
func (hc *HTTPCoordinator) CommitTransaction(ctx context.Context, txID string) error {
	// Placeholder implementation
	logDebugf("Committing distributed transaction: tx_id=%s", txID)
	return nil
}

// RollbackTransaction rolls back a distributed transaction.
func (hc *HTTPCoordinator) RollbackTransaction(ctx context.Context, txID string) error {
	// Placeholder implementation
	logDebugf("Rolling back distributed transaction: tx_id=%s", txID)
	return nil
}

// GetTransactionStatus gets the status of a distributed transaction.
func (hc *HTTPCoordinator) GetTransactionStatus(ctx context.Context, txID string) (TransactionStatus, error) {
	// Placeholder implementation
	return TxStatusActive, nil
}

// JaegerTracer provides Jaeger-based distributed tracing.
type JaegerTracer struct {
	serviceName string
}

// NewJaegerTracer creates a new Jaeger tracer.
func NewJaegerTracer(serviceName string) *JaegerTracer {
	return &JaegerTracer{serviceName: serviceName}
}

// StartSpan starts a new trace span.
func (jt *JaegerTracer) StartSpan(ctx context.Context, operationName string) (context.Context, Span) {
	// Placeholder implementation
	// In a real implementation, this would use the Jaeger client
	return ctx, &MockSpan{operationName: operationName}
}

// FinishSpan finishes a trace span.
func (jt *JaegerTracer) FinishSpan(span Span) {
	// Placeholder implementation
}

// InjectContext injects trace context into headers.
func (jt *JaegerTracer) InjectContext(ctx context.Context, headers map[string]string) {
	// Placeholder implementation
}

// ExtractContext extracts trace context from headers.
func (jt *JaegerTracer) ExtractContext(headers map[string]string) (context.Context, error) {
	// Placeholder implementation
	return context.Background(), nil
}

// MockSpan provides a mock implementation of Span interface.
type MockSpan struct {
	operationName string
	tags          map[string]interface{}
	baggage       map[string]string
}

// SetTag sets a tag on the span.
func (ms *MockSpan) SetTag(key string, value interface{}) {
	if ms.tags == nil {
		ms.tags = make(map[string]interface{})
	}
	ms.tags[key] = value
}

// SetBaggageItem sets a baggage item on the span.
func (ms *MockSpan) SetBaggageItem(key, value string) {
	if ms.baggage == nil {
		ms.baggage = make(map[string]string)
	}
	ms.baggage[key] = value
}

// Finish finishes the span.
func (ms *MockSpan) Finish() {
	logDebugf("Span finished: operation=%s, tags=%+v", ms.operationName, ms.tags)
}

// Context returns the span context.
func (ms *MockSpan) Context() SpanContext {
	return &MockSpanContext{baggage: ms.baggage}
}

// MockSpanContext provides a mock implementation of SpanContext interface.
type MockSpanContext struct {
	baggage map[string]string
}

// ForeachBaggageItem iterates over baggage items.
func (msc *MockSpanContext) ForeachBaggageItem(handler func(key, value string) string) {
	for k, v := range msc.baggage {
		handler(k, v)
	}
}
