package orm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SoftDelete provides soft delete functionality for database records.
type SoftDelete struct {
	DeletedAtColumn string        // Column name for deletion timestamp (default: "deleted_at")
	RetentionPeriod time.Duration // How long to keep soft-deleted records (default: 30 days)
}

// SoftDeleteResult contains the result of a soft delete operation.
type SoftDeleteResult struct {
	RowsAffected int64 // Number of rows affected
	DeletedAt    time.Time // When the records were marked as deleted
}

// NewSoftDelete creates a new SoftDelete instance with default settings.
func NewSoftDelete() *SoftDelete {
	return &SoftDelete{
		DeletedAtColumn: "deleted_at",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days
	}
}

// SoftDelete marks records as deleted by setting the deleted_at timestamp.
func (sd *SoftDelete) SoftDelete(ctx context.Context, db DB, table, idColumn string, ids ...interface{}) (*SoftDeleteResult, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no IDs provided for soft delete")
	}

	dialect := NewDefaultDialect()

	// Validate inputs
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}
	if err := ValidateColumnName(idColumn); err != nil {
		return nil, fmt.Errorf("invalid ID column name '%s': %w", idColumn, err)
	}
	if err := ValidateColumnName(sd.DeletedAtColumn); err != nil {
		return nil, fmt.Errorf("invalid deleted_at column name '%s': %w", sd.DeletedAtColumn, err)
	}

	now := time.Now().UTC()
	query, args := sd.buildSoftDeleteQuery(table, idColumn, ids, now, dialect)

	result, err := db.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	return &SoftDeleteResult{
		RowsAffected: rowsAffected,
		DeletedAt:    now,
	}, nil
}

// Restore undeletes records by clearing the deleted_at timestamp.
func (sd *SoftDelete) Restore(ctx context.Context, db DB, table, idColumn string, ids ...interface{}) (*SoftDeleteResult, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no IDs provided for restore")
	}

	dialect := NewDefaultDialect()

	// Validate inputs
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}
	if err := ValidateColumnName(idColumn); err != nil {
		return nil, fmt.Errorf("invalid ID column name '%s': %w", idColumn, err)
	}

	query, args := sd.buildRestoreQuery(table, idColumn, ids, dialect)

	result, err := db.Exec(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	return &SoftDeleteResult{
		RowsAffected: rowsAffected,
	}, nil
}

// FindNotDeleted returns a WHERE clause to find non-deleted records.
func (sd *SoftDelete) FindNotDeleted() string {
	return fmt.Sprintf("(%s IS NULL)", sd.DeletedAtColumn)
}

// FindSoftDeleted returns a WHERE clause to find soft-deleted records.
func (sd *SoftDelete) FindSoftDeleted() string {
	return fmt.Sprintf("(%s IS NOT NULL)", sd.DeletedAtColumn)
}

// FindNotDeletedOr returns a WHERE condition for non-deleted records (OR syntax).
func (sd *SoftDelete) FindNotDeletedOr() string {
	return fmt.Sprintf("%s IS NULL", sd.DeletedAtColumn)
}

// FindSoftDeletedOr returns a WHERE condition for soft-deleted records (OR syntax).
func (sd *SoftDelete) FindSoftDeletedOr() string {
	return fmt.Sprintf("%s IS NOT NULL", sd.DeletedAtColumn)
}

// CleanSoftDeleted permanently removes soft-deleted records older than the retention period.
func (sd *SoftDelete) CleanSoftDeleted(ctx context.Context, db DB, table string) (*SoftDeleteResult, error) {
	dialect := NewDefaultDialect()

	// Validate inputs
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	cutoffTime := time.Now().UTC().Add(-sd.RetentionPeriod)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s < %s",
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(sd.DeletedAtColumn),
		dialect.Placeholder(0))

	result, err := db.Exec(ctx, query, cutoffTime)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	return &SoftDeleteResult{
		RowsAffected: rowsAffected,
	}, nil
}

// IsSoftDeleted checks if a record is soft-deleted.
func (sd *SoftDelete) IsSoftDeleted(ctx context.Context, db DB, table, idColumn string, id interface{}) (bool, error) {
	dialect := NewDefaultDialect()

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = %s AND %s IS NOT NULL",
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(idColumn), dialect.Placeholder(0),
		dialect.QuoteIdentifier(sd.DeletedAtColumn))

	var count int
	row := db.QueryRow(ctx, query, id)
	if err := row.Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetDeletedAt returns the deletion timestamp for a record.
func (sd *SoftDelete) GetDeletedAt(ctx context.Context, db DB, table, idColumn string, id interface{}) (*time.Time, error) {
	dialect := NewDefaultDialect()

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s",
		dialect.QuoteIdentifier(sd.DeletedAtColumn),
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(idColumn), dialect.Placeholder(0))

	var deletedAt sql.NullTime
	row := db.QueryRow(ctx, query, id)
	if err := row.Scan(&deletedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("record not found")
		}
		return nil, err
	}

	if deletedAt.Valid {
		return &deletedAt.Time, nil
	}

	return nil, nil // Not soft-deleted
}

// buildSoftDeleteQuery builds the UPDATE query for soft deletion.
func (sd *SoftDelete) buildSoftDeleteQuery(table, idColumn string, ids []interface{}, deletedAt time.Time, dialect Dialect) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)

	args[0] = deletedAt
	for i, id := range ids {
		placeholders[i] = dialect.Placeholder(i + 1)
		args[i+1] = id
	}

	query := fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IN (%s)",
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(sd.DeletedAtColumn), dialect.Placeholder(0),
		dialect.QuoteIdentifier(idColumn),
		strings.Join(placeholders, ", "))

	return query, args
}

// buildRestoreQuery builds the UPDATE query for restoring soft-deleted records.
func (sd *SoftDelete) buildRestoreQuery(table, idColumn string, ids []interface{}, dialect Dialect) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))

	for i, id := range ids {
		placeholders[i] = dialect.Placeholder(i)
		args[i] = id
	}

	query := fmt.Sprintf("UPDATE %s SET %s = NULL WHERE %s IN (%s)",
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(sd.DeletedAtColumn),
		dialect.QuoteIdentifier(idColumn),
		strings.Join(placeholders, ", "))

	return query, args
}
