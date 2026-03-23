package orm

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type cteClause struct {
	name  string
	query *SelectBuilder
	recursive bool
}

func (sb *SelectBuilder) JoinOn(table string, fn func(*JoinOnBuilder)) *SelectBuilder {
	jb := NewJoinOnBuilder(sb.dialect)
	if fn != nil {
		fn(jb)
	}
	cond, args := jb.Build()
	sb.joins = append(sb.joins, Join{Type: "INNER", Table: table, RawTable: false, Condition: cond, Args: args})
	return sb
}

func (sb *SelectBuilder) LeftJoinOn(table string, fn func(*JoinOnBuilder)) *SelectBuilder {
	jb := NewJoinOnBuilder(sb.dialect)
	if fn != nil {
		fn(jb)
	}
	cond, args := jb.Build()
	sb.joins = append(sb.joins, Join{Type: "LEFT", Table: table, RawTable: false, Condition: cond, Args: args})
	return sb
}

func (sb *SelectBuilder) RightJoinOn(table string, fn func(*JoinOnBuilder)) *SelectBuilder {
	jb := NewJoinOnBuilder(sb.dialect)
	if fn != nil {
		fn(jb)
	}
	cond, args := jb.Build()
	sb.joins = append(sb.joins, Join{Type: "RIGHT", Table: table, RawTable: false, Condition: cond, Args: args})
	return sb
}

func (sb *SelectBuilder) JoinSubqueryOn(sub *SelectBuilder, alias string, fn func(*JoinOnBuilder)) *SelectBuilder {
	if sub == nil {
		return sb
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "t"
	}
	q, subArgs := sub.Build()
	jb := NewJoinOnBuilder(sb.dialect)
	if fn != nil {
		fn(jb)
	}
	cond, args := jb.Build()
	tableExpr := fmt.Sprintf("(%s) %s", q, alias)
	sb.joins = append(sb.joins, Join{Type: "INNER", Table: tableExpr, RawTable: true, TableArgs: subArgs, Condition: cond, Args: args})
	return sb
}

func (sb *SelectBuilder) LeftJoinSubqueryOn(sub *SelectBuilder, alias string, fn func(*JoinOnBuilder)) *SelectBuilder {
	if sub == nil {
		return sb
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "t"
	}
	q, subArgs := sub.Build()
	jb := NewJoinOnBuilder(sb.dialect)
	if fn != nil {
		fn(jb)
	}
	cond, args := jb.Build()
	tableExpr := fmt.Sprintf("(%s) %s", q, alias)
	sb.joins = append(sb.joins, Join{Type: "LEFT", Table: tableExpr, RawTable: true, TableArgs: subArgs, Condition: cond, Args: args})
	return sb
}

type unionClause struct {
	all   bool
	query *SelectBuilder
}

var postgresPlaceholderRe = regexp.MustCompile(`\$(\d+)`)

func shiftPlaceholdersIfNeeded(query string, dialect Dialect, offset int) string {
	if offset == 0 {
		return query
	}
	if _, ok := dialect.(*PostgreSQLDialect); !ok {
		return query
	}
	return postgresPlaceholderRe.ReplaceAllStringFunc(query, func(m string) string {
		nStr := strings.TrimPrefix(m, "$")
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return m
		}
		return fmt.Sprintf("$%d", n+offset)
	})
}

// Join represents a JOIN clause.
type Join struct {
	Type      string
	Table     string
	RawTable  bool
	TableArgs []interface{}
	Condition string
	Args      []interface{}
}

func replaceConditionPlaceholders(condition string, dialect Dialect, startIndex int) string {
	if condition == "" {
		return condition
	}
	out := condition
	idx := startIndex
	for j := 0; j < strings.Count(condition, "?"); j++ {
		out = strings.Replace(out, "?", dialect.Placeholder(idx), 1)
		idx++
	}
	return out
}

// SelectBuilder provides a fluent interface for building SELECT queries.
type SelectBuilder struct {
	db       DB
	table    string
	rawTable bool
	tableArgs []interface{}
	columns  []string
	joins    []Join
	where    *WhereBuilder
	groupBy  []string
	having   *WhereBuilder
	orderBy  []string
	limit    int
	offset   int
	lockMode string
	dialect  Dialect
	ctes     []cteClause
	unions   []unionClause
}

// NewSelectBuilder creates a new SELECT query builder.
func NewSelectBuilder(db DB, table string) *SelectBuilder {
	dialect := NewDefaultDialect()
	return &SelectBuilder{
		db:      db,
		table:   table,
		columns: []string{"*"},
		joins:   []Join{},
		where:   NewWhereBuilder(dialect),
		groupBy: []string{},
		having:  NewWhereBuilder(dialect),
		dialect: dialect,
	}
}

// NewSelectBuilderWithDialect creates a new SELECT query builder with a specific dialect.
func NewSelectBuilderWithDialect(db DB, table string, dialect Dialect) *SelectBuilder {
	return &SelectBuilder{
		db:      db,
		table:   table,
		columns: []string{"*"},
		joins:   []Join{},
		where:   NewWhereBuilder(dialect),
		groupBy: []string{},
		having:  NewWhereBuilder(dialect),
		dialect: dialect,
	}
}

func (sb *SelectBuilder) FromRaw(tableExpr string, args ...interface{}) *SelectBuilder {
	sb.table = tableExpr
	sb.rawTable = true
	sb.tableArgs = append([]interface{}(nil), args...)
	return sb
}

func (sb *SelectBuilder) FromSubquery(sub *SelectBuilder, alias string) *SelectBuilder {
	if sub == nil {
		return sb
	}
	q, args := sub.Build()
	q = shiftPlaceholdersIfNeeded(q, sb.dialect, 0)
	sb.table = fmt.Sprintf("(%s) %s", q, alias)
	sb.rawTable = true
	sb.tableArgs = append([]interface{}(nil), args...)
	return sb
}

func (sb *SelectBuilder) With(name string, q *SelectBuilder) *SelectBuilder {
	name = strings.TrimSpace(name)
	if name == "" || q == nil {
		return sb
	}
	sb.ctes = append(sb.ctes, cteClause{name: name, query: q})
	return sb
}

func (sb *SelectBuilder) WithRecursive(name string, seed *SelectBuilder, recursive *SelectBuilder) *SelectBuilder {
	name = strings.TrimSpace(name)
	if name == "" || seed == nil || recursive == nil {
		return sb
	}
	combined := seed.Clone().UnionAll(recursive)
	sb.ctes = append(sb.ctes, cteClause{name: name, query: combined, recursive: true})
	return sb
}

func (sb *SelectBuilder) Union(q *SelectBuilder) *SelectBuilder {
	if q == nil {
		return sb
	}
	sb.unions = append(sb.unions, unionClause{all: false, query: q})
	return sb
}

func (sb *SelectBuilder) UnionAll(q *SelectBuilder) *SelectBuilder {
	if q == nil {
		return sb
	}
	sb.unions = append(sb.unions, unionClause{all: true, query: q})
	return sb
}

func (sb *SelectBuilder) AsDerived(alias string) *SelectBuilder {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "t"
	}
	q, args := sb.Build()
	outer := NewSelectBuilderWithDialect(sb.db, "ignored", sb.dialect)
	outer.FromRaw(fmt.Sprintf("(%s) %s", q, alias), args...)
	return outer
}

func (sb *SelectBuilder) JoinSubquery(sub *SelectBuilder, alias string, condition string, args ...interface{}) *SelectBuilder {
	if sub == nil {
		return sb
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "t"
	}
	q, subArgs := sub.Build()
	tableExpr := fmt.Sprintf("(%s) %s", q, alias)
	sb.joins = append(sb.joins, Join{Type: "INNER", Table: tableExpr, RawTable: true, TableArgs: subArgs, Condition: condition, Args: args})
	return sb
}

func (sb *SelectBuilder) LeftJoinSubquery(sub *SelectBuilder, alias string, condition string, args ...interface{}) *SelectBuilder {
	if sub == nil {
		return sb
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "t"
	}
	q, subArgs := sub.Build()
	tableExpr := fmt.Sprintf("(%s) %s", q, alias)
	sb.joins = append(sb.joins, Join{Type: "LEFT", Table: tableExpr, RawTable: true, TableArgs: subArgs, Condition: condition, Args: args})
	return sb
}

func (sb *SelectBuilder) RightJoinSubquery(sub *SelectBuilder, alias string, condition string, args ...interface{}) *SelectBuilder {
	if sub == nil {
		return sb
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "t"
	}
	q, subArgs := sub.Build()
	tableExpr := fmt.Sprintf("(%s) %s", q, alias)
	sb.joins = append(sb.joins, Join{Type: "RIGHT", Table: tableExpr, RawTable: true, TableArgs: subArgs, Condition: condition, Args: args})
	return sb
}

// Columns sets the columns to select.
func (sb *SelectBuilder) Columns(columns ...string) *SelectBuilder {
	sb.columns = columns
	return sb
}

// Column adds a single column to select.
func (sb *SelectBuilder) Column(column string) *SelectBuilder {
	sb.columns = append(sb.columns, column)
	return sb
}

// Select is an alias for Columns.
func (sb *SelectBuilder) Select(columns ...string) *SelectBuilder {
	return sb.Columns(columns...)
}

// Distinct adds DISTINCT to the query.
func (sb *SelectBuilder) Distinct() *SelectBuilder {
	sb.columns = append([]string{"DISTINCT"}, sb.columns...)
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

// WhereBuilder returns the WHERE builder for advanced conditions.
func (sb *SelectBuilder) WhereBuilder() *WhereBuilder {
	return sb.where
}

// Eq adds an equality condition.
func (sb *SelectBuilder) Eq(column string, value interface{}) *SelectBuilder {
	sb.where.Eq(column, value)
	return sb
}

// Ne adds a not equal condition.
func (sb *SelectBuilder) Ne(column string, value interface{}) *SelectBuilder {
	sb.where.Ne(column, value)
	return sb
}

// Gt adds a greater than condition.
func (sb *SelectBuilder) Gt(column string, value interface{}) *SelectBuilder {
	sb.where.Gt(column, value)
	return sb
}

// Gte adds a greater than or equal condition.
func (sb *SelectBuilder) Gte(column string, value interface{}) *SelectBuilder {
	sb.where.Gte(column, value)
	return sb
}

// Lt adds a less than condition.
func (sb *SelectBuilder) Lt(column string, value interface{}) *SelectBuilder {
	sb.where.Lt(column, value)
	return sb
}

// Lte adds a less than or equal condition.
func (sb *SelectBuilder) Lte(column string, value interface{}) *SelectBuilder {
	sb.where.Lte(column, value)
	return sb
}

// Like adds a LIKE condition.
func (sb *SelectBuilder) Like(column string, value interface{}) *SelectBuilder {
	sb.where.Like(column, value)
	return sb
}

// ILike adds a case-insensitive LIKE condition.
func (sb *SelectBuilder) ILike(column string, value interface{}) *SelectBuilder {
	sb.where.ILike(column, value)
	return sb
}

// In adds an IN condition.
func (sb *SelectBuilder) In(column string, values ...interface{}) *SelectBuilder {
	sb.where.In(column, values...)
	return sb
}

// NotIn adds a NOT IN condition.
func (sb *SelectBuilder) NotIn(column string, values ...interface{}) *SelectBuilder {
	sb.where.NotIn(column, values...)
	return sb
}

// IsNull adds an IS NULL condition.
func (sb *SelectBuilder) IsNull(column string) *SelectBuilder {
	sb.where.IsNull(column)
	return sb
}

// IsNotNull adds an IS NOT NULL condition.
func (sb *SelectBuilder) IsNotNull(column string) *SelectBuilder {
	sb.where.IsNotNull(column)
	return sb
}

// Between adds a BETWEEN condition.
func (sb *SelectBuilder) Between(column string, start, end interface{}) *SelectBuilder {
	sb.where.Between(column, start, end)
	return sb
}

// OrderBy sets the ORDER BY clause.
func (sb *SelectBuilder) OrderBy(order ...string) *SelectBuilder {
	sb.orderBy = append(sb.orderBy, order...)
	return sb
}

// OrderByAsc adds an ascending ORDER BY clause.
func (sb *SelectBuilder) OrderByAsc(column string) *SelectBuilder {
	sb.orderBy = append(sb.orderBy, sb.dialect.QuoteIdentifier(column)+" ASC")
	return sb
}

// OrderByDesc adds a descending ORDER BY clause.
func (sb *SelectBuilder) OrderByDesc(column string) *SelectBuilder {
	sb.orderBy = append(sb.orderBy, sb.dialect.QuoteIdentifier(column)+" DESC")
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

func (sb *SelectBuilder) ForUpdateNowait() *SelectBuilder {
	sb.lockMode = sb.dialect.LockForUpdate()
	if sb.lockMode != "" {
		sb.lockMode += " NOWAIT"
	}
	return sb
}

func (sb *SelectBuilder) ForUpdateSkipLocked() *SelectBuilder {
	sb.lockMode = sb.dialect.LockForUpdate()
	if sb.lockMode != "" {
		sb.lockMode += " SKIP LOCKED"
	}
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
	if sb.dialect != nil && !sb.dialect.SupportsFeature(FeatureFullOuterJoin) {
		return sb.JoinWithType("LEFT", table, condition, args...)
	}
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

// HavingBuilder returns the HAVING builder for advanced conditions.
func (sb *SelectBuilder) HavingBuilder() *WhereBuilder {
	return sb.having
}

// Build constructs the SELECT query string and returns it with args.
func (sb *SelectBuilder) Build() (string, []interface{}) {
	var allArgs []interface{}

	if len(sb.ctes) > 0 {
		cteParts := make([]string, 0, len(sb.ctes))
		hasRecursive := false
		for _, c := range sb.ctes {
			if c.recursive {
				hasRecursive = true
			}
			q, args := c.query.Build()
			q = shiftPlaceholdersIfNeeded(q, sb.dialect, len(allArgs))
			allArgs = append(allArgs, args...)
			cteParts = append(cteParts, fmt.Sprintf("%s AS (%s)", sb.dialect.QuoteIdentifier(c.name), q))
		}
		prefix := "WITH "
		if hasRecursive {
			prefix = "WITH RECURSIVE "
		}
		prefix += strings.Join(cteParts, ", ") + " "
		// Build main query below then prepend
		mainQuery, mainArgs := sb.buildSelectCore()
		mainQuery = shiftPlaceholdersIfNeeded(mainQuery, sb.dialect, len(allArgs))
		allArgs = append(allArgs, mainArgs...)
		finalQuery := prefix + mainQuery
		finalQuery, allArgs = sb.appendUnions(finalQuery, allArgs)
		return finalQuery, allArgs
	}

	query, args := sb.buildSelectCore()
	allArgs = append(allArgs, args...)
	query, allArgs = sb.appendUnions(query, allArgs)
	return query, allArgs
}

func (sb *SelectBuilder) buildSelectCore() (string, []interface{}) {
	// Quote columns
	quotedColumns := make([]string, len(sb.columns))
	for i, column := range sb.columns {
		if column == "*" || strings.HasPrefix(column, "DISTINCT ") {
			quotedColumns[i] = column
		} else {
			quotedColumns[i] = sb.dialect.QuoteIdentifier(column)
		}
	}

	fromExpr := ""
	if sb.rawTable {
		fromExpr = sb.table
	} else {
		fromExpr = sb.dialect.QuoteIdentifier(sb.table)
	}
	query := fmt.Sprintf("SELECT %s FROM %s",
		strings.Join(quotedColumns, ", "),
		fromExpr)

	var allArgs []interface{}
	if len(sb.tableArgs) > 0 {
		allArgs = append(allArgs, sb.tableArgs...)
	}

	// Add JOIN clauses
	for _, join := range sb.joins {
		tableExpr := ""
		if join.RawTable {
			tableExpr = shiftPlaceholdersIfNeeded(join.Table, sb.dialect, len(allArgs))
			if len(join.TableArgs) > 0 {
				allArgs = append(allArgs, join.TableArgs...)
			}
		} else {
			tableExpr = sb.dialect.QuoteIdentifier(join.Table)
		}
		cond := replaceConditionPlaceholders(join.Condition, sb.dialect, len(allArgs))
		query += fmt.Sprintf(" %s JOIN %s ON %s", 
			join.Type, 
			tableExpr, 
			cond)
		allArgs = append(allArgs, join.Args...)
	}

	// Add WHERE clause
	whereClause, whereArgs := sb.where.BuildWithOffset(len(allArgs))
	if whereClause != "" {
		query += " " + whereClause
		allArgs = append(allArgs, whereArgs...)
	}

	// Add GROUP BY clause
	if len(sb.groupBy) > 0 {
		quotedGroupBy := QuoteIdentifiers(sb.dialect, sb.groupBy...)
		query += " GROUP BY " + strings.Join(quotedGroupBy, ", ")
	}

	// Add HAVING clause
	havingClause, havingArgs := sb.having.BuildWithOffset(len(allArgs))
	if havingClause != "" {
		query += " " + strings.Replace(havingClause, "WHERE", "HAVING", 1)
		allArgs = append(allArgs, havingArgs...)
	}

	// Add ORDER BY clause
	if len(sb.orderBy) > 0 {
		query += " ORDER BY " + strings.Join(sb.orderBy, ", ")
	}

	// Add LIMIT and OFFSET clauses
	limitOffsetClause := sb.dialect.LimitOffset(sb.limit, sb.offset)
	if limitOffsetClause != "" {
		query += " " + limitOffsetClause
	}

	// Add lock clause
	if sb.lockMode != "" {
		query += " " + sb.lockMode
	}

	return query, allArgs
}

func (sb *SelectBuilder) appendUnions(baseQuery string, baseArgs []interface{}) (string, []interface{}) {
	if len(sb.unions) == 0 {
		return baseQuery, baseArgs
	}
	query := baseQuery
	args := baseArgs
	for _, u := range sb.unions {
		q, a := u.query.Build()
		q = shiftPlaceholdersIfNeeded(q, sb.dialect, len(args))
		if u.all {
			query += " UNION ALL " + q
		} else {
			query += " UNION " + q
		}
		args = append(args, a...)
	}
	return query, args
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

// Count builds a COUNT query.
func (sb *SelectBuilder) Count() *SelectBuilder {
	sb.columns = []string{"COUNT(*)"}
	return sb
}

// CountColumn builds a COUNT(column) query.
func (sb *SelectBuilder) CountColumn(column string) *SelectBuilder {
	quotedColumn := sb.dialect.QuoteIdentifier(column)
	sb.columns = []string{fmt.Sprintf("COUNT(%s)", quotedColumn)}
	return sb
}

// Sum builds a SUM(column) query.
func (sb *SelectBuilder) Sum(column string) *SelectBuilder {
	quotedColumn := sb.dialect.QuoteIdentifier(column)
	sb.columns = []string{fmt.Sprintf("SUM(%s)", quotedColumn)}
	return sb
}

// Avg builds an AVG(column) query.
func (sb *SelectBuilder) Avg(column string) *SelectBuilder {
	quotedColumn := sb.dialect.QuoteIdentifier(column)
	sb.columns = []string{fmt.Sprintf("AVG(%s)", quotedColumn)}
	return sb
}

// Max builds a MAX(column) query.
func (sb *SelectBuilder) Max(column string) *SelectBuilder {
	quotedColumn := sb.dialect.QuoteIdentifier(column)
	sb.columns = []string{fmt.Sprintf("MAX(%s)", quotedColumn)}
	return sb
}

// Min builds a MIN(column) query.
func (sb *SelectBuilder) Min(column string) *SelectBuilder {
	quotedColumn := sb.dialect.QuoteIdentifier(column)
	sb.columns = []string{fmt.Sprintf("MIN(%s)", quotedColumn)}
	return sb
}

// Clone creates a copy of the SelectBuilder.
func (sb *SelectBuilder) Clone() *SelectBuilder {
	clone := &SelectBuilder{
		db:      sb.db,
		table:   sb.table,
		columns: make([]string, len(sb.columns)),
		joins:   make([]Join, len(sb.joins)),
		groupBy: make([]string, len(sb.groupBy)),
		orderBy: make([]string, len(sb.orderBy)),
		limit:   sb.limit,
		offset:  sb.offset,
		lockMode: sb.lockMode,
		dialect: sb.dialect,
	}
	
	copy(clone.columns, sb.columns)
	copy(clone.joins, sb.joins)
	copy(clone.groupBy, sb.groupBy)
	copy(clone.orderBy, sb.orderBy)
	
	clone.where = sb.where.Clone()
	clone.having = sb.having.Clone()
	
	return clone
}

// Reset clears all query conditions except table and dialect.
func (sb *SelectBuilder) Reset() *SelectBuilder {
	sb.columns = []string{"*"}
	sb.joins = sb.joins[:0]
	sb.where = NewWhereBuilder(sb.dialect)
	sb.groupBy = sb.groupBy[:0]
	sb.having = NewWhereBuilder(sb.dialect)
	sb.orderBy = sb.orderBy[:0]
	sb.limit = 0
	sb.offset = 0
	sb.lockMode = ""
	return sb
}
