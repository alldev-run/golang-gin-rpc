// Package orm provides a database-agnostic ORM layer with query builders.
package orm

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// DB defines the interface for database operations.
type DB interface {
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Dialect defines database-specific SQL dialects.
type Dialect interface {
	LockForUpdate() string
	LockInShareMode() string
	QuoteIdentifier(identifier string) string
}

// DefaultDialect provides MySQL-compatible dialect.
type DefaultDialect struct{}

func (d DefaultDialect) LockForUpdate() string {
	return "FOR UPDATE"
}

func (d DefaultDialect) LockInShareMode() string {
	return "LOCK IN SHARE MODE"
}

func (d DefaultDialect) QuoteIdentifier(identifier string) string {
	return "`" + identifier + "`"
}

// WhereBuilder provides a fluent interface for building WHERE clauses.
type WhereBuilder struct {
	conditions []string
	args       []interface{}
}

// NewWhereBuilder creates a new WHERE builder.
func NewWhereBuilder() *WhereBuilder {
	return &WhereBuilder{}
}

// Where adds a WHERE condition.
func (wb *WhereBuilder) Where(condition string, args ...interface{}) *WhereBuilder {
	wb.conditions = append(wb.conditions, condition)
	wb.args = append(wb.args, args...)
	return wb
}

// And adds an AND condition.
func (wb *WhereBuilder) And(condition string, args ...interface{}) *WhereBuilder {
	if len(wb.conditions) > 0 {
		wb.conditions = append(wb.conditions, "AND "+condition)
	} else {
		wb.conditions = append(wb.conditions, condition)
	}
	wb.args = append(wb.args, args...)
	return wb
}

// Or adds an OR condition.
func (wb *WhereBuilder) Or(condition string, args ...interface{}) *WhereBuilder {
	if len(wb.conditions) > 0 {
		wb.conditions = append(wb.conditions, "OR "+condition)
	} else {
		wb.conditions = append(wb.conditions, condition)
	}
	wb.args = append(wb.args, args...)
	return wb
}

// Build constructs the WHERE clause string and returns conditions with args.
func (wb *WhereBuilder) Build() (string, []interface{}) {
	if len(wb.conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(wb.conditions, " "), wb.args
}

// Join represents a JOIN clause.
type Join struct {
	Type      string
	Table     string
	Condition string
	Args      []interface{}
}

// SelectBuilder provides a fluent interface for building SELECT queries.
type SelectBuilder struct {
	db       DB
	table    string
	columns  []string
	joins    []Join
	where    *WhereBuilder
	groupBy  []string
	having   *WhereBuilder
	orderBy  string
	limit    int
	offset   int
	lockMode string
	dialect  Dialect
}

// NewSelectBuilder creates a new SELECT query builder.
func NewSelectBuilder(db DB, table string) *SelectBuilder {
	return &SelectBuilder{
		db:      db,
		table:   table,
		columns: []string{"*"},
		joins:   []Join{},
		where:   NewWhereBuilder(),
		groupBy: []string{},
		having:  NewWhereBuilder(),
		dialect: DefaultDialect{},
	}
}

// Columns sets the columns to select.
func (sb *SelectBuilder) Columns(columns ...string) *SelectBuilder {
	sb.columns = columns
	return sb
}

// Where adds WHERE conditions.
func (sb *SelectBuilder) Where(condition string, args ...interface{}) *SelectBuilder {
	sb.where.Where(condition, args...)
	return sb
}

// And adds AND conditions.
func (sb *SelectBuilder) And(condition string, args ...interface{}) *SelectBuilder {
	sb.where.And(condition, args...)
	return sb
}

// Or adds OR conditions.
func (sb *SelectBuilder) Or(condition string, args ...interface{}) *SelectBuilder {
	sb.where.Or(condition, args...)
	return sb
}

// OrderBy sets the ORDER BY clause.
func (sb *SelectBuilder) OrderBy(order string) *SelectBuilder {
	sb.orderBy = order
	return sb
}

// Limit sets the LIMIT clause.
func (sb *SelectBuilder) Limit(limit int) *SelectBuilder {
	sb.limit = limit
	return sb
}

// Offset sets the OFFSET clause.
func (sb *SelectBuilder) Offset(offset int) *SelectBuilder {
	sb.offset = offset
	return sb
}

// ForUpdate adds FOR UPDATE lock to the query.
func (sb *SelectBuilder) ForUpdate() *SelectBuilder {
	sb.lockMode = sb.dialect.LockForUpdate()
	return sb
}

// LockInShareMode adds LOCK IN SHARE MODE to the query.
func (sb *SelectBuilder) LockInShareMode() *SelectBuilder {
	sb.lockMode = sb.dialect.LockInShareMode()
	return sb
}

// Lock sets a custom lock mode.
func (sb *SelectBuilder) Lock(lockMode string) *SelectBuilder {
	sb.lockMode = lockMode
	return sb
}

// Join adds an INNER JOIN clause.
func (sb *SelectBuilder) Join(table, condition string, args ...interface{}) *SelectBuilder {
	return sb.JoinWithType("INNER", table, condition, args...)
}

// JoinWithType adds a JOIN clause with specified type.
func (sb *SelectBuilder) JoinWithType(joinType, table, condition string, args ...interface{}) *SelectBuilder {
	sb.joins = append(sb.joins, Join{
		Type:      joinType,
		Table:     table,
		Condition: condition,
		Args:      args,
	})
	return sb
}

// LeftJoin adds a LEFT JOIN clause.
func (sb *SelectBuilder) LeftJoin(table, condition string, args ...interface{}) *SelectBuilder {
	return sb.JoinWithType("LEFT", table, condition, args...)
}

// RightJoin adds a RIGHT JOIN clause.
func (sb *SelectBuilder) RightJoin(table, condition string, args ...interface{}) *SelectBuilder {
	return sb.JoinWithType("RIGHT", table, condition, args...)
}

// FullOuterJoin adds a FULL OUTER JOIN clause.
func (sb *SelectBuilder) FullOuterJoin(table, condition string, args ...interface{}) *SelectBuilder {
	return sb.JoinWithType("FULL OUTER", table, condition, args...)
}

// GroupBy sets the GROUP BY clause.
func (sb *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	sb.groupBy = append(sb.groupBy, columns...)
	return sb
}

// Having adds HAVING conditions (used with GROUP BY).
func (sb *SelectBuilder) Having(condition string, args ...interface{}) *SelectBuilder {
	sb.having.Where(condition, args...)
	return sb
}

// HavingAnd adds AND conditions to HAVING.
func (sb *SelectBuilder) HavingAnd(condition string, args ...interface{}) *SelectBuilder {
	sb.having.And(condition, args...)
	return sb
}

// HavingOr adds OR conditions to HAVING.
func (sb *SelectBuilder) HavingOr(condition string, args ...interface{}) *SelectBuilder {
	sb.having.Or(condition, args...)
	return sb
}

// Build constructs the SELECT query string and returns it with args.
func (sb *SelectBuilder) Build() (string, []interface{}) {
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(sb.columns, ", "), sb.table)

	var allArgs []interface{}

	// Add JOIN clauses
	for _, join := range sb.joins {
		query += fmt.Sprintf(" %s JOIN %s ON %s", join.Type, join.Table, join.Condition)
		allArgs = append(allArgs, join.Args...)
	}

	// Add WHERE clause
	whereClause, whereArgs := sb.where.Build()
	if whereClause != "" {
		query += " " + whereClause
		allArgs = append(allArgs, whereArgs...)
	}

	// Add GROUP BY clause
	if len(sb.groupBy) > 0 {
		query += " GROUP BY " + strings.Join(sb.groupBy, ", ")
	}

	// Add HAVING clause
	havingClause, havingArgs := sb.having.Build()
	if havingClause != "" {
		query += " " + strings.Replace(havingClause, "WHERE", "HAVING", 1)
		allArgs = append(allArgs, havingArgs...)
	}

	// Add ORDER BY clause
	if sb.orderBy != "" {
		query += " ORDER BY " + sb.orderBy
	}

	// Add LIMIT clause
	if sb.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", sb.limit)
	}

	// Add OFFSET clause
	if sb.offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", sb.offset)
	}

	// Add lock clause
	if sb.lockMode != "" {
		query += " " + sb.lockMode
	}

	return query, allArgs
}

// Query executes the built SELECT query and returns rows.
func (sb *SelectBuilder) Query(ctx context.Context) (*sql.Rows, error) {
	query, args := sb.Build()
	return sb.db.Query(ctx, query, args...)
}

// QueryRow executes the built SELECT query and returns a single row.
func (sb *SelectBuilder) QueryRow(ctx context.Context) *sql.Row {
	query, args := sb.Build()
	return sb.db.QueryRow(ctx, query, args...)
}

// QueryTx executes the built SELECT query within a transaction and returns rows.
func (sb *SelectBuilder) QueryTx(ctx context.Context, tx *sql.Tx) (*sql.Rows, error) {
	query, args := sb.Build()
	return tx.QueryContext(ctx, query, args...)
}

// QueryRowTx executes the built SELECT query within a transaction and returns a single row.
func (sb *SelectBuilder) QueryRowTx(ctx context.Context, tx *sql.Tx) *sql.Row {
	query, args := sb.Build()
	return tx.QueryRowContext(ctx, query, args...)
}

// DeleteBuilder provides a fluent interface for building DELETE queries.
type DeleteBuilder struct {
	db      DB
	table   string
	where   *WhereBuilder
	limit   int
	dialect Dialect
}

// NewDeleteBuilder creates a new DELETE query builder.
func NewDeleteBuilder(db DB, table string) *DeleteBuilder {
	return &DeleteBuilder{
		db:      db,
		table:   table,
		where:   NewWhereBuilder(),
		dialect: DefaultDialect{},
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

// Limit sets the LIMIT clause.
func (db *DeleteBuilder) Limit(limit int) *DeleteBuilder {
	db.limit = limit
	return db
}

// Build constructs the DELETE query string and returns it with args.
func (db *DeleteBuilder) Build() (string, []interface{}) {
	query := fmt.Sprintf("DELETE FROM %s", db.table)

	whereClause, args := db.where.Build()
	if whereClause != "" {
		query += " " + whereClause
	}

	if db.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", db.limit)
	}

	return query, args
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

// Helper functions for common operations

// InsertGetID executes an INSERT statement and returns the last inserted ID.
func InsertGetID(ctx context.Context, db DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update executes an UPDATE statement and returns the number of rows affected.
func Update(ctx context.Context, db DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetFieldByID updates a single field on a row identified by an ID column.
func SetFieldByID(ctx context.Context, db DB, table, idColumn string, id interface{}, field string, value interface{}) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", table, field, idColumn)
	res, err := db.Exec(ctx, query, value, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Save inserts or updates a row based on whether the provided id is zero.
// If id is zero or nil, it performs an INSERT and returns the new row ID.
// Otherwise, it performs an UPDATE and returns the number of affected rows.
// Supports optimistic locking with version field.
func Save(ctx context.Context, db DB, table, idColumn string, id interface{}, data map[string]interface{}) (int64, error) {
	if isZero(id) {
		query, args := buildInsertQuery(table, data)
		return InsertGetID(ctx, db, query, args...)
	}
	query, args := buildUpdateQuery(table, idColumn, id, data)
	return Update(ctx, db, query, args...)
}

func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case int:
		return val == 0
	case int8:
		return val == 0
	case int16:
		return val == 0
	case int32:
		return val == 0
	case int64:
		return val == 0
	case uint:
		return val == 0
	case uint8:
		return val == 0
	case uint16:
		return val == 0
	case uint32:
		return val == 0
	case uint64:
		return val == 0
	case float32:
		return val == 0
	case float64:
		return val == 0
	case string:
		return val == ""
	case bool:
		return !val
	default:
		return false
	}
}

func buildInsertQuery(table string, data map[string]interface{}) (string, []interface{}) {
	const defaultVersionField = "version"
	if _, ok := data[defaultVersionField]; !ok {
		data[defaultVersionField] = 1
	}

	if len(data) == 0 {
		return "", nil
	}

	cols := make([]string, 0, len(data))
	for k := range data {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	placeholders := make([]string, len(cols))
	args := make([]interface{}, len(cols))
	for i, col := range cols {
		placeholders[i] = "?"
		args[i] = data[col]
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	return query, args
}

func buildUpdateQuery(table, idColumn string, id interface{}, data map[string]interface{}) (string, []interface{}) {
	const defaultVersionField = "version"
	hasVersion := false
	var oldVersion interface{}
	if v, ok := data[defaultVersionField]; ok {
		hasVersion = true
		oldVersion = v
		delete(data, defaultVersionField)
	}

	if len(data) == 0 {
		return "", nil
	}

	cols := make([]string, 0, len(data))
	for k := range data {
		if k != idColumn {
			cols = append(cols, k)
		}
	}
	sort.Strings(cols)

	setStmts := make([]string, 0, len(cols)+1)  // +1 for version
	args := make([]interface{}, 0, len(cols)+2) // +2 for id and possibly version
	for _, col := range cols {
		if col == defaultVersionField {
			setStmts = append(setStmts, defaultVersionField+" = "+defaultVersionField+" + 1")
		} else {
			setStmts = append(setStmts, fmt.Sprintf("%s = ?", col))
			args = append(args, data[col])
		}
	}

	// Always update version field
	setStmts = append(setStmts, defaultVersionField+" = "+defaultVersionField+" + 1")

	args = append(args, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", table, strings.Join(setStmts, ", "), idColumn)
	if hasVersion {
		query += " AND " + defaultVersionField + " = ?"
		args = append(args, oldVersion)
	}
	return query, args
}
