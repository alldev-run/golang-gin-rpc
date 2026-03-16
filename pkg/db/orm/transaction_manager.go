package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TransactionConfig holds configuration for transaction operations.
type TransactionConfig struct {
	MaxRetries     int           // Maximum number of retries for retryable operations
	RetryDelay     time.Duration // Delay between retries
	IsolationLevel sql.IsolationLevel // Transaction isolation level
	ReadOnly       bool          // Whether transaction is read-only
}

// TransactionManager provides enhanced transaction management with retry logic.
type TransactionManager struct {
	config TransactionConfig
}

// TransactionResult contains the result of a transaction operation.
type TransactionResult struct {
	Success      bool          // Whether the transaction succeeded
	Duration     time.Duration // How long the transaction took
	Retries      int           // Number of retries performed
	Error        error         // Any error that occurred
}

// NewTransactionManager creates a new TransactionManager with default settings.
func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		config: TransactionConfig{
			MaxRetries:     3,
			RetryDelay:     100 * time.Millisecond,
			IsolationLevel: sql.LevelDefault,
			ReadOnly:       false,
		},
	}
}

// WithTransaction executes a function within a transaction with retry capability.
func (tm *TransactionManager) WithTransaction(ctx context.Context, db DB, fn func(*ORM) error) (*TransactionResult, error) {
	startTime := time.Now()
	var lastErr error

	for attempt := 0; attempt <= tm.config.MaxRetries; attempt++ {
		result, err := tm.executeTransactionOnce(ctx, db, fn)
		if err == nil {
			result.Duration = time.Since(startTime)
			result.Retries = attempt
			result.Success = true
			return result, nil
		}

		lastErr = err
		result.Error = err

		// Check if error is retryable
		if !tm.isRetryableError(err) {
			result.Duration = time.Since(startTime)
			result.Retries = attempt
			return result, err
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < tm.config.MaxRetries {
			select {
			case <-time.After(tm.config.RetryDelay):
				// Continue to next attempt
			case <-ctx.Done():
				result.Duration = time.Since(startTime)
				result.Retries = attempt
				result.Error = ctx.Err()
				return result, ctx.Err()
			}
		}
	}

	result := &TransactionResult{
		Success:  false,
		Duration: time.Since(startTime),
		Retries:  tm.config.MaxRetries,
		Error:    lastErr,
	}

	return result, fmt.Errorf("transaction failed after %d attempts: %w", tm.config.MaxRetries+1, lastErr)
}

// WithRetry executes a function with retry logic (without transaction).
func (tm *TransactionManager) WithRetry(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= tm.config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !tm.isRetryableError(err) {
			return err
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < tm.config.MaxRetries {
			select {
			case <-time.After(tm.config.RetryDelay):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", tm.config.MaxRetries+1, lastErr)
}

// WithNestedTransaction executes a function within a nested transaction (savepoint).
func (tm *TransactionManager) WithNestedTransaction(ctx context.Context, orm *ORM, savepointName string, fn func(*ORM) error) error {
	if savepointName == "" {
		savepointName = fmt.Sprintf("sp_%d", time.Now().UnixNano())
	}

	// Create savepoint
	if err := tm.createSavepoint(ctx, orm.DB(), savepointName); err != nil {
		return fmt.Errorf("failed to create savepoint: %w", err)
	}

	// Execute function
	if err := fn(orm); err != nil {
		// Rollback to savepoint on error
		if rollbackErr := tm.rollbackToSavepoint(ctx, orm.DB(), savepointName); rollbackErr != nil {
			return fmt.Errorf("failed to rollback to savepoint: %w (original error: %w)", rollbackErr, err)
		}
		return err
	}

	// Release savepoint on success
	if err := tm.releaseSavepoint(ctx, orm.DB(), savepointName); err != nil {
		return fmt.Errorf("failed to release savepoint: %w", err)
	}

	return nil
}

// executeTransactionOnce executes a transaction function once.
func (tm *TransactionManager) executeTransactionOnce(ctx context.Context, db DB, fn func(*ORM) error) (*TransactionResult, error) {
	// Begin transaction
	tx, err := db.(*DBWrapper).db.BeginTx(ctx, &sql.TxOptions{
		Isolation: tm.config.IsolationLevel,
		ReadOnly:  tm.config.ReadOnly,
	})
	if err != nil {
		return &TransactionResult{Error: err}, err
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
		return &TransactionResult{Error: err}, err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &TransactionResult{Error: err}, err
	}

	committed = true
	return &TransactionResult{Success: true}, nil
}

// isRetryableError determines if an error is worth retrying.
func (tm *TransactionManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common retryable database errors
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"connection lost",
		"deadlock",
		"lock wait timeout",
		"serialization failure",
		"could not serialize access",
		"timeout",
		"temporary",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}

	return false
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
