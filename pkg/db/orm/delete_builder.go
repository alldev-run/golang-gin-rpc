package orm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// DeleteBuilder provides a fluent interface for building DELETE queries.
type DeleteBuilder struct {
	db      DB
	table   string
	where   *WhereBuilder
	joins   []Join
	orderBy []string
	limit   int
	dialect Dialect
}

// NewDeleteBuilder creates a new DELETE query builder.
func NewDeleteBuilder(db DB, table string) *DeleteBuilder {
	dialect := NewDefaultDialect()
	return &DeleteBuilder{
		db:      db,
		table:   table,
		where:   NewWhereBuilder(dialect),
		dialect: dialect,
	}
}

// NewDeleteBuilderWithDialect creates a new DELETE query builder with a specific dialect.
func NewDeleteBuilderWithDialect(db DB, table string, dialect Dialect) *DeleteBuilder {
	return &DeleteBuilder{
		db:      db,
		table:   table,
		where:   NewWhereBuilder(dialect),
		dialect: dialect,
	}
}

// Where adds WHERE conditions.
func (db *DeleteBuilder) Where(condition string, args ...interface{}) *DeleteBuilder {
	db.where.Where(condition, args...)
	return db
}

// And adds AND conditions.
func (db *DeleteBuilder) And(condition string, args ...interface{}) *DeleteBuilder {
	db.where.And(condition, args...)
	return db
}

// Or adds OR conditions.
func (db *DeleteBuilder) Or(condition string, args ...interface{}) *DeleteBuilder {
	db.where.Or(condition, args...)
	return db
}

// WhereBuilder returns the WHERE builder for advanced conditions.
func (db *DeleteBuilder) WhereBuilder() *WhereBuilder {
	return db.where
}

// Eq adds an equality condition.
func (db *DeleteBuilder) Eq(column string, value interface{}) *DeleteBuilder {
	db.where.Eq(column, value)
	return db
}

// Ne adds a not equal condition.
func (db *DeleteBuilder) Ne(column string, value interface{}) *DeleteBuilder {
	db.where.Ne(column, value)
	return db
}

// Gt adds a greater than condition.
func (db *DeleteBuilder) Gt(column string, value interface{}) *DeleteBuilder {
	db.where.Gt(column, value)
	return db
}

// Gte adds a greater than or equal condition.
func (db *DeleteBuilder) Gte(column string, value interface{}) *DeleteBuilder {
	db.where.Gte(column, value)
	return db
}

// Lt adds a less than condition.
func (db *DeleteBuilder) Lt(column string, value interface{}) *DeleteBuilder {
	db.where.Lt(column, value)
	return db
}

// Lte adds a less than or equal condition.
func (db *DeleteBuilder) Lte(column string, value interface{}) *DeleteBuilder {
	db.where.Lte(column, value)
	return db
}

// Like adds a LIKE condition.
func (db *DeleteBuilder) Like(column string, value interface{}) *DeleteBuilder {
	db.where.Like(column, value)
	return db
}

// ILike adds a case-insensitive LIKE condition.
func (db *DeleteBuilder) ILike(column string, value interface{}) *DeleteBuilder {
	db.where.ILike(column, value)
	return db
}

// In adds an IN condition.
func (db *DeleteBuilder) In(column string, values ...interface{}) *DeleteBuilder {
	db.where.In(column, values...)
	return db
}

// NotIn adds a NOT IN condition.
func (db *DeleteBuilder) NotIn(column string, values ...interface{}) *DeleteBuilder {
	db.where.NotIn(column, values...)
	return db
}

// IsNull adds an IS NULL condition.
func (db *DeleteBuilder) IsNull(column string) *DeleteBuilder {
	db.where.IsNull(column)
	return db
}

// IsNotNull adds an IS NOT NULL condition.
func (db *DeleteBuilder) IsNotNull(column string) *DeleteBuilder {
	db.where.IsNotNull(column)
	return db
}

// Between adds a BETWEEN condition.
func (db *DeleteBuilder) Between(column string, start, end interface{}) *DeleteBuilder {
	db.where.Between(column, start, end)
	return db
}

// OrderBy sets the ORDER BY clause (supported by some databases).
func (db *DeleteBuilder) OrderBy(order ...string) *DeleteBuilder {
	db.orderBy = append(db.orderBy, order...)
	return db
}

// OrderByAsc adds an ascending ORDER BY clause.
func (db *DeleteBuilder) OrderByAsc(column string) *DeleteBuilder {
	db.orderBy = append(db.orderBy, db.dialect.QuoteIdentifier(column)+" ASC")
	return db
}

// OrderByDesc adds a descending ORDER BY clause.
func (db *DeleteBuilder) OrderByDesc(column string) *DeleteBuilder {
	db.orderBy = append(db.orderBy, db.dialect.QuoteIdentifier(column)+" DESC")
	return db
}

// Limit sets the LIMIT clause (supported by some databases).
func (db *DeleteBuilder) Limit(limit int) *DeleteBuilder {
	db.limit = limit
	return db
}

// Join adds a JOIN clause for DELETE with JOIN (supported by some databases).
func (db *DeleteBuilder) Join(table, condition string, args ...interface{}) *DeleteBuilder {
	return db.JoinWithType("INNER", table, condition, args...)
}

// JoinWithType adds a JOIN clause with specified type for DELETE with JOIN.
func (db *DeleteBuilder) JoinWithType(joinType, table, condition string, args ...interface{}) *DeleteBuilder {
	db.joins = append(db.joins, Join{
		Type:      joinType,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return db
}

// LeftJoin adds a LEFT JOIN clause for DELETE with JOIN.
func (db *DeleteBuilder) LeftJoin(table, condition string, args ...interface{}) *DeleteBuilder {
	return db.JoinWithType("LEFT", table, condition, args...)
}

// Build constructs the DELETE query string and returns it with args.
func (db *DeleteBuilder) Build() (string, []interface{}) {
	query := fmt.Sprintf("DELETE FROM %s", db.dialect.QuoteIdentifier(db.table))

	var allArgs []interface{}

	// Add JOIN clauses if any
	for _, join := range db.joins {
		query += fmt.Sprintf(" %s JOIN %s ON %s", 
			join.Type, 
			db.dialect.QuoteIdentifier(join.Table), 
			join.Condition)
		allArgs = append(allArgs, join.Args...)
	}

	// Add WHERE clause
	whereClause, whereArgs := db.where.Build()
	if whereClause != "" {
		query += " " + whereClause
		allArgs = append(allArgs, whereArgs...)
	}

	// Add ORDER BY clause if specified
	if len(db.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(db.orderBy, ", ")
	}

	// Add LIMIT clause if specified
	if db.limit > 0 {
		limitClause := db.dialect.LimitOffset(db.limit, 0)
		if limitClause != "" {
			query += " " + limitClause
		}
	}

	return query, allArgs
}

// Exec executes the built DELETE query and returns the result.
func (db *DeleteBuilder) Exec(ctx context.Context) (sql.Result, error) {
	query, args := db.Build()
	return db.db.Exec(ctx, query, args...)
}

// ExecTx executes the built DELETE query within a transaction and returns the result.
func (db *DeleteBuilder) ExecTx(ctx context.Context, tx *sql.Tx) (sql.Result, error) {
	query, args := db.Build()
	return tx.ExecContext(ctx, query, args...)
}

// Delete executes the DELETE query and returns the number of rows affected.
func (db *DeleteBuilder) Delete(ctx context.Context) (int64, error) {
	res, err := db.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteTx executes the DELETE query within a transaction and returns the number of rows affected.
func (db *DeleteBuilder) DeleteTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	res, err := db.ExecTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteReturning executes the DELETE query with RETURNING clause (for databases that support it).
func (db *DeleteBuilder) DeleteReturning(ctx context.Context, returningColumns ...string) (*sql.Rows, error) {
	if !db.dialect.SupportsFeature(FeatureCTE) {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}
	
	query, args := db.Build()
	
	// Add RETURNING clause
	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(db.dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}
	
	return db.db.Query(ctx, query, args...)
}

// DeleteReturningTx executes the DELETE query with RETURNING clause within a transaction.
func (db *DeleteBuilder) DeleteReturningTx(ctx context.Context, tx *sql.Tx, returningColumns ...string) (*sql.Rows, error) {
	if !db.dialect.SupportsFeature(FeatureCTE) {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}
	
	query, args := db.Build()
	
	// Add RETURNING clause
	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(db.dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}
	
	return tx.QueryContext(ctx, query, args...)
}

// DeleteByID deletes a row by its ID.
func (db *DeleteBuilder) DeleteByID(ctx context.Context, idColumn string, id interface{}) (int64, error) {
	db.where.Eq(idColumn, id)
	return db.Delete(ctx)
}

// DeleteByIDTx deletes a row by its ID within a transaction.
func (db *DeleteBuilder) DeleteByIDTx(ctx context.Context, tx *sql.Tx, idColumn string, id interface{}) (int64, error) {
	db.where.Eq(idColumn, id)
	return db.DeleteTx(ctx, tx)
}

// DeleteByIDs deletes multiple rows by their IDs.
func (db *DeleteBuilder) DeleteByIDs(ctx context.Context, idColumn string, ids ...interface{}) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	db.where.In(idColumn, ids...)
	return db.Delete(ctx)
}

// DeleteByIDsTx deletes multiple rows by their IDs within a transaction.
func (db *DeleteBuilder) DeleteByIDsTx(ctx context.Context, tx *sql.Tx, idColumn string, ids ...interface{}) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	db.where.In(idColumn, ids...)
	return db.DeleteTx(ctx, tx)
}

// DeleteWithVersion deletes a row with optimistic locking.
func (db *DeleteBuilder) DeleteWithVersion(ctx context.Context, idColumn string, id interface{}, versionField string, currentVersion interface{}) (int64, error) {
	db.where.Eq(idColumn, id)
	db.where.Eq(versionField, currentVersion)
	return db.Delete(ctx)
}

// DeleteWithVersionTx deletes a row with optimistic locking within a transaction.
func (db *DeleteBuilder) DeleteWithVersionTx(ctx context.Context, tx *sql.Tx, idColumn string, id interface{}, versionField string, currentVersion interface{}) (int64, error) {
	db.where.Eq(idColumn, id)
	db.where.Eq(versionField, currentVersion)
	return db.DeleteTx(ctx, tx)
}

// Truncate truncates the table (deletes all rows). This is a DDL operation, not DML.
func (db *DeleteBuilder) Truncate(ctx context.Context) (sql.Result, error) {
	query := fmt.Sprintf("TRUNCATE TABLE %s", db.dialect.QuoteIdentifier(db.table))
	return db.db.Exec(ctx, query)
}

// TruncateTx truncates the table within a transaction (may not be supported by all databases).
func (db *DeleteBuilder) TruncateTx(ctx context.Context, tx *sql.Tx) (sql.Result, error) {
	query := fmt.Sprintf("TRUNCATE TABLE %s", db.dialect.QuoteIdentifier(db.table))
	return tx.ExecContext(ctx, query)
}

// Clone creates a copy of the DeleteBuilder.
func (db *DeleteBuilder) Clone() *DeleteBuilder {
	clone := &DeleteBuilder{
		db:      db.db,
		table:   db.table,
		joins:   make([]Join, len(db.joins)),
		orderBy: make([]string, len(db.orderBy)),
		limit:   db.limit,
		dialect: db.dialect,
	}
	
	// Copy joins
	copy(clone.joins, db.joins)
	
	// Copy orderBy
	copy(clone.orderBy, db.orderBy)
	
	clone.where = db.where.Clone()
	
	return clone
}

// Reset clears all conditions.
func (db *DeleteBuilder) Reset() *DeleteBuilder {
	db.where = NewWhereBuilder(db.dialect)
	db.joins = db.joins[:0]
	db.orderBy = db.orderBy[:0]
	db.limit = 0
	return db
}

// IsEmpty returns true if no WHERE conditions have been added.
func (db *DeleteBuilder) IsEmpty() bool {
	return db.where.IsEmpty()
}

// GetTable returns the table name.
func (db *DeleteBuilder) GetTable() string {
	return db.table
}

// HasJoins returns true if any JOIN clauses have been added.
func (db *DeleteBuilder) HasJoins() bool {
	return len(db.joins) > 0
}

// GetJoins returns a copy of the joins slice.
func (db *DeleteBuilder) GetJoins() []Join {
	joins := make([]Join, len(db.joins))
	copy(joins, db.joins)
	return joins
}
