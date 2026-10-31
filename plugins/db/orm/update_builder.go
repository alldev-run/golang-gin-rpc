package orm

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// SQLExpr represents an explicit SQL expression used in SET clauses.
// Use this only with trusted SQL snippets.
type SQLExpr struct {
	SQL  string
	Args []interface{}
}

// Expr creates an explicit SQL expression value for use with SetExpr.
func Expr(sql string, args ...interface{}) SQLExpr {
	return SQLExpr{SQL: sql, Args: args}
}

// UpdateBuilder provides a fluent interface for building UPDATE queries.
type UpdateBuilder struct {
	db           DB
	table        string
	data         map[string]interface{}
	where        *WhereBuilder
	joins        []Join
	orderBy      []string
	limit        int
	dialect      Dialect
	versionField string
}

// NewUpdateBuilder creates a new UPDATE query builder.
func NewUpdateBuilder(db DB, table string) *UpdateBuilder {
	dialect := NewDefaultDialect()
	return &UpdateBuilder{
		db:           db,
		table:        table,
		data:         make(map[string]interface{}),
		where:        NewWhereBuilder(dialect),
		dialect:      dialect,
		versionField: "version",
	}
}

// NewUpdateBuilderWithDialect creates a new UPDATE query builder with a specific dialect.
func NewUpdateBuilderWithDialect(db DB, table string, dialect Dialect) *UpdateBuilder {
	return &UpdateBuilder{
		db:           db,
		table:        table,
		data:         make(map[string]interface{}),
		where:        NewWhereBuilder(dialect),
		dialect:      dialect,
		versionField: "version",
	}
}

// Set sets a column value.
func (ub *UpdateBuilder) Set(column string, value interface{}) *UpdateBuilder {
	ub.data[column] = value
	return ub
}

// Sets sets multiple column values.
func (ub *UpdateBuilder) Sets(data map[string]interface{}) *UpdateBuilder {
	for k, v := range data {
		ub.data[k] = v
	}
	return ub
}

// SetVersionField sets the version field name for optimistic locking.
func (ub *UpdateBuilder) SetVersionField(field string) *UpdateBuilder {
	ub.versionField = field
	return ub
}

// Inc increments a numeric column by the specified amount.
func (ub *UpdateBuilder) Inc(column string, amount interface{}) *UpdateBuilder {
	quotedColumn := ub.dialect.QuoteIdentifier(column)
	return ub.SetExpr(column, fmt.Sprintf("%s + ?", quotedColumn), amount)
}

// Dec decrements a numeric column by the specified amount.
func (ub *UpdateBuilder) Dec(column string, amount interface{}) *UpdateBuilder {
	quotedColumn := ub.dialect.QuoteIdentifier(column)
	return ub.SetExpr(column, fmt.Sprintf("%s - ?", quotedColumn), amount)
}

// SetExpr sets a column to an explicit SQL expression.
// Example: SetExpr("count", "`count` + ?", 1)
func (ub *UpdateBuilder) SetExpr(column string, expr string, args ...interface{}) *UpdateBuilder {
	ub.data[column] = Expr(expr, args...)
	return ub
}

// Where adds WHERE conditions.
func (ub *UpdateBuilder) Where(condition string, args ...interface{}) *UpdateBuilder {
	ub.where.Where(condition, args...)
	return ub
}

// And adds AND conditions.
func (ub *UpdateBuilder) And(condition string, args ...interface{}) *UpdateBuilder {
	ub.where.And(condition, args...)
	return ub
}

// Or adds OR conditions.
func (ub *UpdateBuilder) Or(condition string, args ...interface{}) *UpdateBuilder {
	ub.where.Or(condition, args...)
	return ub
}

// WhereBuilder returns the WHERE builder for advanced conditions.
func (ub *UpdateBuilder) WhereBuilder() *WhereBuilder {
	return ub.where
}

// Eq adds an equality condition.
func (ub *UpdateBuilder) Eq(column string, value interface{}) *UpdateBuilder {
	ub.where.Eq(column, value)
	return ub
}

// Ne adds a not equal condition.
func (ub *UpdateBuilder) Ne(column string, value interface{}) *UpdateBuilder {
	ub.where.Ne(column, value)
	return ub
}

// Gt adds a greater than condition.
func (ub *UpdateBuilder) Gt(column string, value interface{}) *UpdateBuilder {
	ub.where.Gt(column, value)
	return ub
}

// Gte adds a greater than or equal condition.
func (ub *UpdateBuilder) Gte(column string, value interface{}) *UpdateBuilder {
	ub.where.Gte(column, value)
	return ub
}

// Lt adds a less than condition.
func (ub *UpdateBuilder) Lt(column string, value interface{}) *UpdateBuilder {
	ub.where.Lt(column, value)
	return ub
}

// Lte adds a less than or equal condition.
func (ub *UpdateBuilder) Lte(column string, value interface{}) *UpdateBuilder {
	ub.where.Lte(column, value)
	return ub
}

// Like adds a LIKE condition.
func (ub *UpdateBuilder) Like(column string, value interface{}) *UpdateBuilder {
	ub.where.Like(column, value)
	return ub
}

// ILike adds a case-insensitive LIKE condition.
func (ub *UpdateBuilder) ILike(column string, value interface{}) *UpdateBuilder {
	ub.where.ILike(column, value)
	return ub
}

// In adds an IN condition.
func (ub *UpdateBuilder) In(column string, values ...interface{}) *UpdateBuilder {
	ub.where.In(column, values...)
	return ub
}

// NotIn adds a NOT IN condition.
func (ub *UpdateBuilder) NotIn(column string, values ...interface{}) *UpdateBuilder {
	ub.where.NotIn(column, values...)
	return ub
}

// IsNull adds an IS NULL condition.
func (ub *UpdateBuilder) IsNull(column string) *UpdateBuilder {
	ub.where.IsNull(column)
	return ub
}

// IsNotNull adds an IS NOT NULL condition.
func (ub *UpdateBuilder) IsNotNull(column string) *UpdateBuilder {
	ub.where.IsNotNull(column)
	return ub
}

// Between adds a BETWEEN condition.
func (ub *UpdateBuilder) Between(column string, start, end interface{}) *UpdateBuilder {
	ub.where.Between(column, start, end)
	return ub
}

// OrderBy sets the ORDER BY clause (supported by some databases).
func (ub *UpdateBuilder) OrderBy(order ...string) *UpdateBuilder {
	for _, item := range order {
		safeItem, err := BuildSafeOrderByItem(ub.dialect, item)
		if err != nil {
			continue
		}
		ub.orderBy = append(ub.orderBy, safeItem)
	}
	return ub
}

// OrderByRaw appends raw ORDER BY expressions.
// Use this only with trusted SQL snippets.
func (ub *UpdateBuilder) OrderByRaw(order ...string) *UpdateBuilder {
	ub.orderBy = append(ub.orderBy, order...)
	return ub
}

// Limit sets the LIMIT clause (supported by some databases).
func (ub *UpdateBuilder) Limit(limit int) *UpdateBuilder {
	ub.limit = limit
	return ub
}

// Join adds a JOIN clause for UPDATE with JOIN (supported by some databases).
func (ub *UpdateBuilder) Join(table, condition string, args ...interface{}) *UpdateBuilder {
	return ub.JoinWithType("INNER", table, condition, args...)
}

// JoinWithType adds a JOIN clause with specified type for UPDATE with JOIN.
func (ub *UpdateBuilder) JoinWithType(joinType, table, condition string, args ...interface{}) *UpdateBuilder {
	normalizedJoinType, err := NormalizeJoinType(joinType)
	if err != nil {
		return ub
	}
	if err := ValidateJoinTableReference(table); err != nil {
		return ub
	}
	ub.joins = append(ub.joins, Join{
		Type:      normalizedJoinType,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return ub
}

// JoinWithTypeRaw adds a raw JOIN clause for UPDATE queries.
// Use this only with trusted SQL snippets.
func (ub *UpdateBuilder) JoinWithTypeRaw(joinType, table, condition string, args ...interface{}) *UpdateBuilder {
	ub.joins = append(ub.joins, Join{
		Type:      joinType,
		Table:     table,
		RawTable:  true,
		Condition: condition,
		Args:      args,
	})
	return ub
}

// LeftJoin adds a LEFT JOIN clause for UPDATE with JOIN.
func (ub *UpdateBuilder) LeftJoin(table, condition string, args ...interface{}) *UpdateBuilder {
	return ub.JoinWithType("LEFT", table, condition, args...)
}

// Build constructs the UPDATE query string and returns it with args.
func (ub *UpdateBuilder) Build() (string, []interface{}) {
	if len(ub.data) == 0 {
		return "", nil
	}

	// Sort keys for consistent query generation
	keys := make([]string, 0, len(ub.data))
	for k := range ub.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build SET clause
	setParts := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))

	for _, key := range keys {
		quotedKey := ub.dialect.QuoteIdentifier(key)

		if expr, ok := ub.data[key].(SQLExpr); ok {
			exprSQL := replaceConditionPlaceholders(expr.SQL, ub.dialect, len(args))
			setParts = append(setParts, fmt.Sprintf("%s = %s", quotedKey, exprSQL))
			args = append(args, expr.Args...)
		} else {
			setParts = append(setParts, fmt.Sprintf("%s = %s", quotedKey, ub.dialect.Placeholder(len(args))))
			args = append(args, ub.data[key])
		}
	}

	query := fmt.Sprintf("UPDATE %s SET %s", 
		ub.dialect.QuoteIdentifier(ub.table), 
		strings.Join(setParts, ", "))

	var allArgs []interface{}
	allArgs = append(allArgs, args...)

	// Add JOIN clauses if any
	for _, join := range ub.joins {
		tableExpr := ""
		if join.RawTable {
			tableExpr = shiftPlaceholdersIfNeeded(join.Table, ub.dialect, len(allArgs))
			if len(join.TableArgs) > 0 {
				allArgs = append(allArgs, join.TableArgs...)
			}
		} else {
			tableExpr = ub.dialect.QuoteIdentifier(join.Table)
		}
		cond := replaceConditionPlaceholders(join.Condition, ub.dialect, len(allArgs))
		query += fmt.Sprintf(" %s JOIN %s ON %s", 
			join.Type, 
			tableExpr, 
			cond)
		allArgs = append(allArgs, join.Args...)
	}

	// Add WHERE clause
	whereClause, whereArgs := ub.where.BuildWithOffset(len(allArgs))
	if whereClause != "" {
		query += " " + whereClause
		allArgs = append(allArgs, whereArgs...)
	}

	// Add ORDER BY clause if specified
	if len(ub.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(ub.orderBy, ", ")
	}

	// Add LIMIT clause if specified
	if ub.limit > 0 {
		limitClause := ub.dialect.LimitOffset(ub.limit, 0)
		if limitClause != "" {
			query += " " + limitClause
		}
	}

	return query, allArgs
}

// Exec executes the built UPDATE query and returns the result.
func (ub *UpdateBuilder) Exec(ctx context.Context) (sql.Result, error) {
	query, args := ub.Build()
	return ub.db.Exec(ctx, query, args...)
}

// ExecTx executes the built UPDATE query within a transaction and returns the result.
func (ub *UpdateBuilder) ExecTx(ctx context.Context, tx *sql.Tx) (sql.Result, error) {
	query, args := ub.Build()
	return tx.ExecContext(ctx, query, args...)
}

// Update executes the UPDATE query and returns the number of rows affected.
func (ub *UpdateBuilder) Update(ctx context.Context) (int64, error) {
	res, err := ub.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateTx executes the UPDATE query within a transaction and returns the number of rows affected.
func (ub *UpdateBuilder) UpdateTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	res, err := ub.ExecTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateWithVersion executes the UPDATE query with optimistic locking.
func (ub *UpdateBuilder) UpdateWithVersion(ctx context.Context, id interface{}, idColumn string, currentVersion interface{}) (int64, error) {
	// Add version condition to WHERE clause
	versionCondition := fmt.Sprintf("%s = %s", 
		ub.dialect.QuoteIdentifier(ub.versionField), 
		ub.dialect.Placeholder(len(ub.where.GetArgs())))
	
	ub.where.And(versionCondition, currentVersion)
	
	// Increment version field
	ub.SetExpr(ub.versionField, fmt.Sprintf("%s + ?", ub.dialect.QuoteIdentifier(ub.versionField)), 1)
	
	// Add ID condition
	ub.where.Eq(idColumn, id)
	
	return ub.Update(ctx)
}

// UpdateWithVersionTx executes the UPDATE query with optimistic locking within a transaction.
func (ub *UpdateBuilder) UpdateWithVersionTx(ctx context.Context, tx *sql.Tx, id interface{}, idColumn string, currentVersion interface{}) (int64, error) {
	// Add version condition to WHERE clause
	versionCondition := fmt.Sprintf("%s = %s", 
		ub.dialect.QuoteIdentifier(ub.versionField), 
		ub.dialect.Placeholder(len(ub.where.GetArgs())))
	
	ub.where.And(versionCondition, currentVersion)
	
	// Increment version field
	ub.SetExpr(ub.versionField, fmt.Sprintf("%s + ?", ub.dialect.QuoteIdentifier(ub.versionField)), 1)
	
	// Add ID condition
	ub.where.Eq(idColumn, id)
	
	return ub.UpdateTx(ctx, tx)
}

// UpdateReturning executes the UPDATE query with RETURNING clause (for databases that support it).
func (ub *UpdateBuilder) UpdateReturning(ctx context.Context, returningColumns ...string) (*sql.Rows, error) {
	if !ub.dialect.SupportsFeature(FeatureCTE) {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}
	
	query, args := ub.Build()
	
	// Add RETURNING clause
	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(ub.dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}
	
	return ub.db.Query(ctx, query, args...)
}

// UpdateReturningTx executes the UPDATE query with RETURNING clause within a transaction.
func (ub *UpdateBuilder) UpdateReturningTx(ctx context.Context, tx *sql.Tx, returningColumns ...string) (*sql.Rows, error) {
	if !ub.dialect.SupportsFeature(FeatureCTE) {
		return nil, fmt.Errorf("RETURNING clause not supported by this dialect")
	}
	
	query, args := ub.Build()
	
	// Add RETURNING clause
	if len(returningColumns) > 0 {
		quotedColumns := QuoteIdentifiers(ub.dialect, returningColumns...)
		query += " RETURNING " + strings.Join(quotedColumns, ", ")
	} else {
		query += " RETURNING *"
	}
	
	return tx.QueryContext(ctx, query, args...)
}

// Clone creates a copy of the UpdateBuilder.
func (ub *UpdateBuilder) Clone() *UpdateBuilder {
	clone := &UpdateBuilder{
		db:           ub.db,
		table:        ub.table,
		data:         make(map[string]interface{}),
		joins:        make([]Join, len(ub.joins)),
		orderBy:      make([]string, len(ub.orderBy)),
		limit:        ub.limit,
		dialect:      ub.dialect,
		versionField: ub.versionField,
	}
	
	// Copy data
	for k, v := range ub.data {
		clone.data[k] = v
	}
	
	// Copy joins
	copy(clone.joins, ub.joins)
	
	// Copy orderBy
	copy(clone.orderBy, ub.orderBy)
	
	clone.where = ub.where.Clone()
	
	return clone
}

// Reset clears all data and conditions.
func (ub *UpdateBuilder) Reset() *UpdateBuilder {
	ub.data = make(map[string]interface{})
	ub.where = NewWhereBuilder(ub.dialect)
	ub.joins = ub.joins[:0]
	ub.orderBy = ub.orderBy[:0]
	ub.limit = 0
	return ub
}

// IsEmpty returns true if no data has been set.
func (ub *UpdateBuilder) IsEmpty() bool {
	return len(ub.data) == 0
}

// GetTable returns the table name.
func (ub *UpdateBuilder) GetTable() string {
	return ub.table
}

// GetData returns a copy of the data map.
func (ub *UpdateBuilder) GetData() map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range ub.data {
		data[k] = v
	}
	return data
}

// GetVersionField returns the version field name.
func (ub *UpdateBuilder) GetVersionField() string {
	return ub.versionField
}
