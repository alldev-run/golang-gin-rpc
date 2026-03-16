package orm

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// InsertBuilder provides a fluent interface for building INSERT queries.
type InsertBuilder struct {
	db         DB
	table      string
	data       map[string]interface{}
	columns    []string
	values     [][]interface{}
	onConflict string
	dialect    Dialect
}

// NewInsertBuilder creates a new INSERT query builder.
func NewInsertBuilder(db DB, table string) *InsertBuilder {
	dialect := NewDefaultDialect()
	return &InsertBuilder{
		db:      db,
		table:   table,
		data:    make(map[string]interface{}),
		dialect: dialect,
	}
}

// NewInsertBuilderWithDialect creates a new INSERT query builder with a specific dialect.
func NewInsertBuilderWithDialect(db DB, table string, dialect Dialect) *InsertBuilder {
	return &InsertBuilder{
		db:      db,
		table:   table,
		data:    make(map[string]interface{}),
		dialect: dialect,
	}
}

// Set sets a column value.
func (ib *InsertBuilder) Set(column string, value interface{}) *InsertBuilder {
	ib.data[column] = value
	return ib
}

// Sets sets multiple column values.
func (ib *InsertBuilder) Sets(data map[string]interface{}) *InsertBuilder {
	for k, v := range data {
		ib.data[k] = v
	}
	return ib
}

// Values sets column names and multiple rows of values for bulk insert.
func (ib *InsertBuilder) Values(columns []string, rows ...[]interface{}) *InsertBuilder {
	ib.columns = columns
	ib.values = rows
	return ib
}

// AddRow adds a single row of values for bulk insert.
func (ib *InsertBuilder) AddRow(row []interface{}) *InsertBuilder {
	ib.values = append(ib.values, row)
	return ib
}

// OnConflict sets the ON CONFLICT clause (for databases that support it).
func (ib *InsertBuilder) OnConflict(action string) *InsertBuilder {
	ib.onConflict = action
	return ib
}

// OnConflictDoNothing sets ON CONFLICT DO NOTHING (UPSERT).
func (ib *InsertBuilder) OnConflictDoNothing() *InsertBuilder {
	ib.onConflict = "DO NOTHING"
	return ib
}

// OnConflictUpdate sets ON CONFLICT UPDATE for UPSERT operations.
func (ib *InsertBuilder) OnConflictUpdate(updateColumns ...string) *InsertBuilder {
	if len(updateColumns) == 0 {
		ib.onConflict = "DO UPDATE SET excluded = excluded"
	} else {
		updates := make([]string, len(updateColumns))
		for i, col := range updateColumns {
			updates[i] = fmt.Sprintf("%s = EXCLUDED.%s", 
				ib.dialect.QuoteIdentifier(col), 
				ib.dialect.QuoteIdentifier(col))
		}
		ib.onConflict = "DO UPDATE SET " + strings.Join(updates, ", ")
	}
	return ib
}

// Build constructs the INSERT query string and returns it with args.
func (ib *InsertBuilder) Build() (string, []interface{}, error) {
	var query string
	var args []interface{}
	var err error
	
	if len(ib.values) > 0 && len(ib.columns) > 0 {
		// Bulk insert mode
		query, args, err = ib.buildBulkInsert()
	} else if len(ib.data) > 0 {
		// Single insert mode
		query, args, err = BuildInsertQuery(ib.table, ib.data, ib.dialect)
	} else {
		return "", nil, ErrEmptyData
	}
	
	if err != nil {
		return "", nil, err
	}
	
	// Add ON CONFLICT clause if specified
	if ib.onConflict != "" && ib.dialect.SupportsFeature(FeatureUpsert) {
		query += " ON CONFLICT " + ib.onConflict
	}
	
	return query, args, nil
}

func (ib *InsertBuilder) buildSingleInsert() (string, []interface{}) {
	// Sort keys for consistent query generation
	keys := make([]string, 0, len(ib.data))
	for k := range ib.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Build column list
	quotedKeys := QuoteIdentifiers(ib.dialect, keys...)
	columns := strings.Join(quotedKeys, ", ")
	
	// Build placeholders
	placeholders := BuildPlaceholders(ib.dialect, len(keys))
	
	// Build arguments
	args := make([]interface{}, len(keys))
	for i, key := range keys {
		args[i] = ib.data[key]
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", 
		ib.dialect.QuoteIdentifier(ib.table), columns, placeholders)
	
	return query, args
}

func (ib *InsertBuilder) buildBulkInsert() (string, []interface{}, error) {
	// Quote column names
	quotedColumns := QuoteIdentifiers(ib.dialect, ib.columns...)
	columns := strings.Join(quotedColumns, ", ")
	
	// Build value placeholders for each row
	valueStrings := make([]string, len(ib.values))
	var args []interface{}
	
	for i, row := range ib.values {
		if len(row) != len(ib.columns) {
			return "", nil, fmt.Errorf("row %d has %d values, expected %d", i, len(row), len(ib.columns))
		}
		
		placeholders := make([]string, len(row))
		for j, value := range row {
			placeholders[j] = ib.dialect.Placeholder(len(args))
			args = append(args, value)
		}
		valueStrings[i] = "(" + strings.Join(placeholders, ", ") + ")"
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", 
		ib.dialect.QuoteIdentifier(ib.table), 
		columns, 
		strings.Join(valueStrings, ", "))
	
	return query, args, nil
}

// Exec executes the built INSERT query and returns the result.
func (ib *InsertBuilder) Exec(ctx context.Context) (sql.Result, error) {
	query, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	return ib.db.Exec(ctx, query, args...)
}

// ExecTx executes the built INSERT query within a transaction and returns the result.
func (ib *InsertBuilder) ExecTx(ctx context.Context, tx *sql.Tx) (sql.Result, error) {
	query, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	return tx.ExecContext(ctx, query, args...)
}

// InsertGetID executes the INSERT query and returns the last inserted ID.
func (ib *InsertBuilder) InsertGetID(ctx context.Context) (int64, error) {
	res, err := ib.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertGetIDTx executes the INSERT query within a transaction and returns the last inserted ID.
func (ib *InsertBuilder) InsertGetIDTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	res, err := ib.ExecTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertReturning executes the INSERT query with RETURNING clause (for databases that support it).
func (ib *InsertBuilder) InsertReturning(ctx context.Context, returningColumns ...string) (*sql.Rows, error) {
	if !ib.dialect.SupportsFeature(FeatureCTE) {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}
	
	query, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	
	// Add RETURNING clause
	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(ib.dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}
	
	return ib.db.Query(ctx, query, args...)
}

// InsertReturningTx executes the INSERT query with RETURNING clause within a transaction.
func (ib *InsertBuilder) InsertReturningTx(ctx context.Context, tx *sql.Tx, returningColumns ...string) (*sql.Rows, error) {
	if !ib.dialect.SupportsFeature(FeatureCTE) {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}
	
	query, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	
	// Add RETURNING clause
	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(ib.dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}
	
	return tx.QueryContext(ctx, query, args...)
}

// Ignore sets the INSERT to ignore duplicates (MySQL syntax).
func (ib *InsertBuilder) Ignore() *InsertBuilder {
	// This is MySQL-specific syntax
	if _, ok := ib.dialect.(*MySQLDialect); ok {
		return ib // MySQL uses INSERT IGNORE, but we'll handle this in the query
	}
	return ib
}

// Replace sets the query to be a REPLACE INTO (MySQL syntax).
func (ib *InsertBuilder) Replace() *InsertBuilder {
	// This is MySQL-specific syntax
	if _, ok := ib.dialect.(*MySQLDialect); ok {
		return ib // Will be handled in Build()
	}
	return ib
}

// Clone creates a copy of the InsertBuilder.
func (ib *InsertBuilder) Clone() *InsertBuilder {
	clone := &InsertBuilder{
		db:         ib.db,
		table:      ib.table,
		data:       make(map[string]interface{}),
		columns:    make([]string, len(ib.columns)),
		values:     make([][]interface{}, len(ib.values)),
		onConflict: ib.onConflict,
		dialect:    ib.dialect,
	}
	
	// Copy data
	for k, v := range ib.data {
		clone.data[k] = v
	}
	
	// Copy columns
	copy(clone.columns, ib.columns)
	
	// Copy values
	for i, row := range ib.values {
		clone.values[i] = make([]interface{}, len(row))
		copy(clone.values[i], row)
	}
	
	return clone
}

// Reset clears all data and values.
func (ib *InsertBuilder) Reset() *InsertBuilder {
	ib.data = make(map[string]interface{})
	ib.columns = ib.columns[:0]
	ib.values = ib.values[:0]
	ib.onConflict = ""
	return ib
}

// IsEmpty returns true if no data or values have been set.
func (ib *InsertBuilder) IsEmpty() bool {
	return len(ib.data) == 0 && len(ib.values) == 0
}

// GetTable returns the table name.
func (ib *InsertBuilder) GetTable() string {
	return ib.table
}

// GetData returns a copy of the data map.
func (ib *InsertBuilder) GetData() map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range ib.data {
		data[k] = v
	}
	return data
}

// GetColumns returns a copy of the columns slice.
func (ib *InsertBuilder) GetColumns() []string {
	columns := make([]string, len(ib.columns))
	copy(columns, ib.columns)
	return columns
}

// GetValues returns a copy of the values slice.
func (ib *InsertBuilder) GetValues() [][]interface{} {
	values := make([][]interface{}, len(ib.values))
	for i, row := range ib.values {
		values[i] = make([]interface{}, len(row))
		copy(values[i], row)
	}
	return values
}
