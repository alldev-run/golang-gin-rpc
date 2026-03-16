package orm

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// BatchInsertConfig holds configuration for batch insert operations.
type BatchInsertConfig struct {
	BatchSize      int  // Number of records per batch (default: 100)
	UseTransaction bool // Whether to wrap batch in transaction (default: true)
	ReturnIDs      bool // Whether to return inserted IDs (default: false)
}

// BatchInsertResult contains the result of a batch insert operation.
type BatchInsertResult struct {
	RowsAffected int64   // Total rows affected
	InsertedIDs  []int64 // Inserted IDs (only if ReturnIDs is true)
}

// BatchInsert performs efficient bulk insert operations with configurable batch sizes.
// It automatically splits large datasets into smaller batches for optimal performance.
func BatchInsert(ctx context.Context, db DB, table string, data []map[string]interface{}, config *BatchInsertConfig) (*BatchInsertResult, error) {
	if config == nil {
		config = &BatchInsertConfig{
			BatchSize:      100,
			UseTransaction: true,
			ReturnIDs:      false,
		}
	}

	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}

	if len(data) == 0 {
		return &BatchInsertResult{RowsAffected: 0}, nil
	}

	dialect := NewDefaultDialect()

	// Validate table name and data
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	// Validate all data entries have the same structure
	if err := validateBatchData(data); err != nil {
		return nil, err
	}

	var result BatchInsertResult
	var tx *sql.Tx
	var txWrapper *TxWrapper

	// Start transaction if requested
	if config.UseTransaction {
		var err error
		tx, err = db.(*DBWrapper).db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				panic(p)
			}
		}()
		txWrapper = NewTxWrapper(tx)
		db = txWrapper
	}

	// Process data in batches
	for i := 0; i < len(data); i += config.BatchSize {
		end := i + config.BatchSize
		if end > len(data) {
			end = len(data)
		}

		batch := data[i:end]
		batchResult, err := insertBatch(ctx, db, table, batch, dialect, config.ReturnIDs)
		if err != nil {
			if config.UseTransaction && tx != nil {
				tx.Rollback()
			}
			return nil, fmt.Errorf("batch insert failed at offset %d: %w", i, err)
		}

		result.RowsAffected += batchResult.RowsAffected
		result.InsertedIDs = append(result.InsertedIDs, batchResult.InsertedIDs...)
	}

	// Commit transaction
	if config.UseTransaction && tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

// BatchInsertWithReturning performs batch insert and returns specified columns.
func BatchInsertWithReturning(ctx context.Context, db DB, table string, data []map[string]interface{}, returningColumns []string, config *BatchInsertConfig) (*sql.Rows, error) {
	if config == nil {
		config = &BatchInsertConfig{
			BatchSize:      100,
			UseTransaction: true,
		}
	}

	if len(data) == 0 {
		return nil, nil
	}

	dialect := NewDefaultDialect()

	// Validate inputs
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	if err := validateBatchData(data); err != nil {
		return nil, err
	}

	// Check if dialect supports RETURNING
	if !dialect.SupportsFeature(FeatureCTE) && len(returningColumns) > 0 {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}

	var tx *sql.Tx
	var txWrapper *TxWrapper

	// Start transaction if requested
	if config.UseTransaction {
		var err error
		tx, err = db.(*DBWrapper).db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				panic(p)
			}
		}()
		txWrapper = NewTxWrapper(tx)
		db = txWrapper
	}

	// Build bulk insert query with RETURNING
	query, args, err := buildBatchInsertQueryWithReturning(table, data, returningColumns, dialect)
	if err != nil {
		if config.UseTransaction && tx != nil {
			tx.Rollback()
		}
		return nil, err
	}

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		if config.UseTransaction && tx != nil {
			tx.Rollback()
		}
		return nil, err
	}

	// Note: Transaction will be committed when rows are closed
	// This is handled by the caller

	return rows.(*sql.Rows), nil
}

// validateBatchData ensures all data entries have consistent structure.
func validateBatchData(data []map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	// Get reference keys from first entry
	var refKeys []string
	for key := range data[0] {
		refKeys = append(refKeys, key)
	}

	// Check all entries have the same keys
	for i, entry := range data {
		if len(entry) != len(refKeys) {
			return fmt.Errorf("entry %d has different number of fields (%d vs %d)", i, len(entry), len(refKeys))
		}

		for _, key := range refKeys {
			if _, exists := entry[key]; !exists {
				return fmt.Errorf("entry %d missing field '%s'", i, key)
			}
		}
	}

	// Validate each field
	for _, entry := range data {
		for column, value := range entry {
			if err := ValidateColumnName(column); err != nil {
				return fmt.Errorf("invalid column name '%s': %w", column, err)
			}
			if !isValidDataType(value) {
				return fmt.Errorf("invalid data type for column '%s'", column)
			}
		}
	}

	return nil
}

// insertBatch inserts a single batch of data.
func insertBatch(ctx context.Context, db DB, table string, batch []map[string]interface{}, dialect Dialect, returnIDs bool) (*BatchInsertResult, error) {
	if len(batch) == 0 {
		return &BatchInsertResult{}, nil
	}

	// Get column names from first entry
	var columns []string
	for key := range batch[0] {
		columns = append(columns, key)
	}

	// Build query and args
	query, args, err := buildBatchInsertQuery(table, batch, columns, dialect)
	if err != nil {
		return nil, err
	}

	var result BatchInsertResult

	if returnIDs {
		// Use RETURNING if supported and requested
		if dialect.SupportsFeature(FeatureCTE) {
			returningQuery := query + " RETURNING id"
			rows, err := db.Query(ctx, returningQuery, args...)
			if err != nil {
				return nil, err
			}
			defer rows.Close()

			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					return nil, err
				}
				result.InsertedIDs = append(result.InsertedIDs, id)
			}

			if err := rows.Err(); err != nil {
				return nil, err
			}

			result.RowsAffected = int64(len(result.InsertedIDs))
		} else {
			// Execute without RETURNING
			res, err := db.Exec(ctx, query, args...)
			if err != nil {
				return nil, err
			}
			result.RowsAffected, _ = res.RowsAffected()
		}
	} else {
		// Simple execution
		res, err := db.Exec(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		result.RowsAffected, _ = res.RowsAffected()
	}

	return &result, nil
}

// buildBatchInsertQuery builds the INSERT query for a batch of data.
func buildBatchInsertQuery(table string, batch []map[string]interface{}, columns []string, dialect Dialect) (string, []interface{}, error) {
	if len(batch) == 0 {
		return "", nil, nil
	}

	// Sort columns for consistent ordering
	sortedColumns := make([]string, len(columns))
	copy(sortedColumns, columns)
	sort.Strings(sortedColumns)

	quotedColumns := QuoteIdentifiers(dialect, sortedColumns...)
	columnsStr := strings.Join(quotedColumns, ", ")

	var placeholders []string
	var args []interface{}

	for _, row := range batch {
		var rowPlaceholders []string
		for _, col := range sortedColumns {
			rowPlaceholders = append(rowPlaceholders, dialect.Placeholder(len(args)))
			args = append(args, row[col])
		}
		placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		dialect.QuoteIdentifier(table),
		columnsStr,
		strings.Join(placeholders, ", "))

	return query, args, nil
}

// buildBatchInsertQueryWithReturning builds batch insert query with RETURNING clause.
func buildBatchInsertQueryWithReturning(table string, batch []map[string]interface{}, returningColumns []string, dialect Dialect) (string, []interface{}, error) {
	query, args, err := buildBatchInsertQuery(table, batch, getColumnsFromBatch(batch), dialect)
	if err != nil {
		return "", nil, err
	}

	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}

	return query, args, nil
}

// getColumnsFromBatch extracts column names from batch data.
func getColumnsFromBatch(batch []map[string]interface{}) []string {
	if len(batch) == 0 {
		return nil
	}

	var columns []string
	for key := range batch[0] {
		columns = append(columns, key)
	}
	return columns
}
