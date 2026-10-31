package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// OptimisticLock provides version-based optimistic locking functionality.
type OptimisticLock struct {
	VersionColumn string        // Column name for version (default: "version")
	RetryCount    int           // Number of retry attempts (default: 3)
	RetryDelay    time.Duration // Delay between retries (default: 100ms)
}

// OptimisticLockResult contains the result of an optimistic lock operation.
type OptimisticLockResult struct {
	Success        bool        // Whether the operation succeeded
	RowsAffected   int64       // Number of rows affected
	CurrentVersion interface{} // Current version after operation
}

// NewOptimisticLock creates a new OptimisticLock instance with default settings.
func NewOptimisticLock() *OptimisticLock {
	return &OptimisticLock{
		VersionColumn: "version",
		RetryCount:    3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// UpdateWithVersion performs an update with optimistic locking using version checking.
func (ol *OptimisticLock) UpdateWithVersion(ctx context.Context, db DB, table, idColumn string, id, expectedVersion interface{}, data map[string]interface{}) (*OptimisticLockResult, error) {
	dialect := NewDefaultDialect()

	// Validate inputs
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}
	if err := ValidateColumnName(idColumn); err != nil {
		return nil, fmt.Errorf("invalid ID column name '%s': %w", idColumn, err)
	}
	if err := ValidateColumnName(ol.VersionColumn); err != nil {
		return nil, fmt.Errorf("invalid version column name '%s': %w", ol.VersionColumn, err)
	}

	// Ensure version is not in the update data (it will be handled separately)
	if _, hasVersion := data[ol.VersionColumn]; hasVersion {
		delete(data, ol.VersionColumn)
	}

	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	// Validate data types
	for column, value := range data {
		if err := ValidateColumnName(column); err != nil {
			return nil, fmt.Errorf("invalid column name '%s': %w", column, err)
		}
		if !isValidDataType(value) {
			return nil, fmt.Errorf("invalid data type for column '%s'", column)
		}
	}

	// Build update query with version check
	query, args, err := ol.buildVersionedUpdateQuery(table, idColumn, id, expectedVersion, data, dialect)
	if err != nil {
		return nil, err
	}

	// Execute update
	res, err := db.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	result := &OptimisticLockResult{
		Success:      rowsAffected > 0,
		RowsAffected: rowsAffected,
	}

	// If update succeeded, increment the version
	if result.Success {
		result.CurrentVersion = incrementVersion(expectedVersion)
	}

	return result, nil
}

// UpdateWithRetry performs an update with automatic retry on version conflicts.
func (ol *OptimisticLock) UpdateWithRetry(ctx context.Context, db DB, table, idColumn string, id interface{}, data map[string]interface{}, getCurrentVersion func() interface{}) (*OptimisticLockResult, error) {
	var lastErr error

	for attempt := 0; attempt <= ol.RetryCount; attempt++ {
		// Get current version
		currentVersion := getCurrentVersion()
		if currentVersion == nil {
			return nil, errors.New("failed to get current version")
		}

		// Attempt update
		result, err := ol.UpdateWithVersion(ctx, db, table, idColumn, id, currentVersion, data)
		if err != nil {
			return nil, err
		}

		if result.Success {
			return result, nil
		}

		lastErr = fmt.Errorf("optimistic lock conflict on attempt %d", attempt+1)

		// If this wasn't the last attempt, wait before retrying
		if attempt < ol.RetryCount {
			select {
			case <-time.After(ol.RetryDelay):
				// Continue to next attempt
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("optimistic lock failed after %d attempts: %w", ol.RetryCount+1, lastErr)
}

// CheckVersionConflict checks if there's a version conflict without performing an update.
func (ol *OptimisticLock) CheckVersionConflict(ctx context.Context, db DB, table, idColumn string, id, expectedVersion interface{}) (bool, error) {
	dialect := NewDefaultDialect()

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s AND %s = %s",
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(idColumn), dialect.Placeholder(0),
		dialect.QuoteIdentifier(ol.VersionColumn), dialect.Placeholder(1))

	var count int
	row := db.QueryRow(ctx, query, id, expectedVersion)
	if err := row.Scan(&count); err != nil {
		return false, err
	}

	return count == 0, nil // true if conflict (no rows match expected version)
}

// GetCurrentVersion retrieves the current version of a record.
func (ol *OptimisticLock) GetCurrentVersion(ctx context.Context, db DB, table, idColumn string, id interface{}) (interface{}, error) {
	dialect := NewDefaultDialect()

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s",
		dialect.QuoteIdentifier(ol.VersionColumn),
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(idColumn), dialect.Placeholder(0))

	var version interface{}
	row := db.QueryRow(ctx, query, id)
	if err := row.Scan(&version); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("record not found")
		}
		return nil, err
	}

	return version, nil
}

// buildVersionedUpdateQuery builds an UPDATE query with version checking.
func (ol *OptimisticLock) buildVersionedUpdateQuery(table, idColumn string, id, expectedVersion interface{}, data map[string]interface{}, dialect Dialect) (string, []interface{}, error) {
	// Sort keys for consistent query generation
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build SET clause
	setParts := make([]string, len(keys))
	args := make([]interface{}, len(keys)+2) // +2 for ID and version

	for i, key := range keys {
		quotedKey := dialect.QuoteIdentifier(key)
		setParts[i] = fmt.Sprintf("%s = %s", quotedKey, dialect.Placeholder(i))
		args[i] = data[key]
	}

	// Add version increment to SET clause
	newVersion := incrementVersion(expectedVersion)
	setParts = append(setParts, fmt.Sprintf("%s = %s", dialect.QuoteIdentifier(ol.VersionColumn), dialect.Placeholder(len(keys))))
	args[len(keys)] = newVersion

	// Add WHERE conditions
	args[len(keys)+1] = id

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s AND %s = %s",
		dialect.QuoteIdentifier(table),
		strings.Join(setParts, ", "),
		dialect.QuoteIdentifier(idColumn), dialect.Placeholder(len(keys)+1),
		dialect.QuoteIdentifier(ol.VersionColumn), dialect.Placeholder(len(keys)))

	return query, args, nil
}

// incrementVersion increments a version value.
func incrementVersion(version interface{}) interface{} {
	switch v := version.(type) {
	case int:
		return v + 1
	case int32:
		return v + 1
	case int64:
		return v + 1
	case uint:
		return v + 1
	case uint32:
		return v + 1
	case uint64:
		return v + 1
	default:
		// For non-numeric versions, return as-is
		return v
	}
}
