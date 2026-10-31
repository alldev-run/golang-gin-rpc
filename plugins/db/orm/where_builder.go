package orm

import (
	"fmt"
	"strings"
)

// WhereBuilder provides a fluent interface for building WHERE clauses.
type WhereBuilder struct {
	conditions []string
	args       []interface{}
	dialect    Dialect
}

// NewWhereBuilder creates a new WHERE builder.
func NewWhereBuilder(dialect Dialect) *WhereBuilder {
	if dialect == nil {
		dialect = NewDefaultDialect()
	}
	return &WhereBuilder{
		dialect: dialect,
	}
}

// Where adds a WHERE condition.
func (wb *WhereBuilder) Where(condition string, args ...interface{}) *WhereBuilder {
	wb.conditions = append(wb.conditions, condition)
	wb.args = append(wb.args, args...)
	return wb
}

// WhereRaw adds a raw WHERE condition string.
// Use this only with trusted SQL snippets.
func (wb *WhereBuilder) WhereRaw(condition string, args ...interface{}) *WhereBuilder {
	return wb.Where(condition, args...)
}

func (wb *WhereBuilder) Group(fn func(*WhereBuilder)) *WhereBuilder {
	subBuilder := NewWhereBuilder(wb.dialect)
	fn(subBuilder)

	if len(subBuilder.conditions) > 0 {
		subCondition := strings.Join(subBuilder.conditions, " ")
		if len(wb.conditions) > 0 {
			wb.conditions = append(wb.conditions, "AND ("+subCondition+")")
		} else {
			wb.conditions = append(wb.conditions, "("+subCondition+")")
		}
		wb.args = append(wb.args, subBuilder.args...)
	}
	return wb
}

func (wb *WhereBuilder) AndGroup(fn func(*WhereBuilder)) *WhereBuilder {
	return wb.AndWhere(fn)
}

func (wb *WhereBuilder) OrGroup(fn func(*WhereBuilder)) *WhereBuilder {
	return wb.OrWhere(fn)
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

// AndRaw adds a raw AND condition string.
// Use this only with trusted SQL snippets.
func (wb *WhereBuilder) AndRaw(condition string, args ...interface{}) *WhereBuilder {
	return wb.And(condition, args...)
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

// OrRaw adds a raw OR condition string.
// Use this only with trusted SQL snippets.
func (wb *WhereBuilder) OrRaw(condition string, args ...interface{}) *WhereBuilder {
	return wb.Or(condition, args...)
}

// Eq adds an equality condition (column = value).
func (wb *WhereBuilder) Eq(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s = %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// Ne adds a not equal condition (column != value).
func (wb *WhereBuilder) Ne(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s != %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// Gt adds a greater than condition (column > value).
func (wb *WhereBuilder) Gt(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s > %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// Gte adds a greater than or equal condition (column >= value).
func (wb *WhereBuilder) Gte(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s >= %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// Lt adds a less than condition (column < value).
func (wb *WhereBuilder) Lt(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s < %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// Lte adds a less than or equal condition (column <= value).
func (wb *WhereBuilder) Lte(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s <= %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// Like adds a LIKE condition.
func (wb *WhereBuilder) Like(column string, value interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s LIKE %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)))
	return wb.Where(condition, value)
}

// ILike adds a case-insensitive LIKE condition (if supported).
func (wb *WhereBuilder) ILike(column string, value interface{}) *WhereBuilder {
	var condition string
	switch wb.dialect.(type) {
	case *PostgreSQLDialect:
		condition = fmt.Sprintf("%s ILIKE %s", 
			wb.dialect.QuoteIdentifier(column), 
			wb.dialect.Placeholder(len(wb.args)))
	default:
		// Fallback to LIKE for databases that don't support ILIKE
		condition = fmt.Sprintf("LOWER(%s) LIKE LOWER(%s)", 
			wb.dialect.QuoteIdentifier(column), 
			wb.dialect.Placeholder(len(wb.args)))
	}
	return wb.Where(condition, value)
}

// In adds an IN condition.
func (wb *WhereBuilder) In(column string, values ...interface{}) *WhereBuilder {
	if len(values) == 0 {
		return wb.Where("1=0") // Always false
	}
	
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = wb.dialect.Placeholder(len(wb.args) + i)
	}
	
	condition := fmt.Sprintf("%s IN (%s)", 
		wb.dialect.QuoteIdentifier(column), 
		strings.Join(placeholders, ", "))
	
	wb.args = append(wb.args, values...)
	return wb.Where(condition)
}

// NotIn adds a NOT IN condition.
func (wb *WhereBuilder) NotIn(column string, values ...interface{}) *WhereBuilder {
	if len(values) == 0 {
		return wb.Where("1=1") // Always true
	}
	
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = wb.dialect.Placeholder(len(wb.args) + i)
	}
	
	condition := fmt.Sprintf("%s NOT IN (%s)", 
		wb.dialect.QuoteIdentifier(column), 
		strings.Join(placeholders, ", "))
	
	wb.args = append(wb.args, values...)
	return wb.Where(condition)
}

// IsNull adds an IS NULL condition.
func (wb *WhereBuilder) IsNull(column string) *WhereBuilder {
	condition := fmt.Sprintf("%s IS NULL", wb.dialect.QuoteIdentifier(column))
	return wb.Where(condition)
}

// IsNotNull adds an IS NOT NULL condition.
func (wb *WhereBuilder) IsNotNull(column string) *WhereBuilder {
	condition := fmt.Sprintf("%s IS NOT NULL", wb.dialect.QuoteIdentifier(column))
	return wb.Where(condition)
}

// Between adds a BETWEEN condition.
func (wb *WhereBuilder) Between(column string, start, end interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s BETWEEN %s AND %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)), 
		wb.dialect.Placeholder(len(wb.args)+1))
	
	wb.args = append(wb.args, start, end)
	return wb.Where(condition)
}

// NotBetween adds a NOT BETWEEN condition.
func (wb *WhereBuilder) NotBetween(column string, start, end interface{}) *WhereBuilder {
	condition := fmt.Sprintf("%s NOT BETWEEN %s AND %s", 
		wb.dialect.QuoteIdentifier(column), 
		wb.dialect.Placeholder(len(wb.args)), 
		wb.dialect.Placeholder(len(wb.args)+1))
	
	wb.args = append(wb.args, start, end)
	return wb.Where(condition)
}

// Exists adds an EXISTS condition with a subquery.
func (wb *WhereBuilder) Exists(subquery string, args ...interface{}) *WhereBuilder {
	condition := fmt.Sprintf("EXISTS (%s)", subquery)
	wb.args = append(wb.args, args...)
	return wb.Where(condition)
}

// NotExists adds a NOT EXISTS condition with a subquery.
func (wb *WhereBuilder) NotExists(subquery string, args ...interface{}) *WhereBuilder {
	condition := fmt.Sprintf("NOT EXISTS (%s)", subquery)
	wb.args = append(wb.args, args...)
	return wb.Where(condition)
}

// AndWhere starts a new AND group with multiple conditions.
func (wb *WhereBuilder) AndWhere(fn func(*WhereBuilder)) *WhereBuilder {
	subBuilder := NewWhereBuilder(wb.dialect)
	fn(subBuilder)
	
	if len(subBuilder.conditions) > 0 {
		subCondition := strings.Join(subBuilder.conditions, " ")
		if len(wb.conditions) > 0 {
			wb.conditions = append(wb.conditions, "AND ("+subCondition+")")
		} else {
			wb.conditions = append(wb.conditions, "("+subCondition+")")
		}
		wb.args = append(wb.args, subBuilder.args...)
	}
	return wb
}

// OrWhere starts a new OR group with multiple conditions.
func (wb *WhereBuilder) OrWhere(fn func(*WhereBuilder)) *WhereBuilder {
	subBuilder := NewWhereBuilder(wb.dialect)
	fn(subBuilder)
	
	if len(subBuilder.conditions) > 0 {
		subCondition := strings.Join(subBuilder.conditions, " ")
		if len(wb.conditions) > 0 {
			wb.conditions = append(wb.conditions, "OR ("+subCondition+")")
		} else {
			wb.conditions = append(wb.conditions, "("+subCondition+")")
		}
		wb.args = append(wb.args, subBuilder.args...)
	}
	return wb
}

// Raw adds a raw condition string.
func (wb *WhereBuilder) Raw(condition string, args ...interface{}) *WhereBuilder {
	return wb.Where(condition, args...)
}

func (wb *WhereBuilder) ExistsSubquery(sub *SelectBuilder) *WhereBuilder {
	if sub == nil {
		return wb
	}
	q, args := sub.Build()
	q = shiftPlaceholdersIfNeeded(q, wb.dialect, len(wb.args))
	wb.args = append(wb.args, args...)
	wb.conditions = append(wb.conditions, fmt.Sprintf("EXISTS (%s)", q))
	return wb
}

func (wb *WhereBuilder) NotExistsSubquery(sub *SelectBuilder) *WhereBuilder {
	if sub == nil {
		return wb
	}
	q, args := sub.Build()
	q = shiftPlaceholdersIfNeeded(q, wb.dialect, len(wb.args))
	wb.args = append(wb.args, args...)
	wb.conditions = append(wb.conditions, fmt.Sprintf("NOT EXISTS (%s)", q))
	return wb
}

func (wb *WhereBuilder) InSubquery(column string, sub *SelectBuilder) *WhereBuilder {
	if sub == nil {
		return wb
	}
	q, args := sub.Build()
	q = shiftPlaceholdersIfNeeded(q, wb.dialect, len(wb.args))
	wb.args = append(wb.args, args...)
	wb.conditions = append(wb.conditions, fmt.Sprintf("%s IN (%s)", wb.dialect.QuoteIdentifier(column), q))
	return wb
}

func (wb *WhereBuilder) NotInSubquery(column string, sub *SelectBuilder) *WhereBuilder {
	if sub == nil {
		return wb
	}
	q, args := sub.Build()
	q = shiftPlaceholdersIfNeeded(q, wb.dialect, len(wb.args))
	wb.args = append(wb.args, args...)
	wb.conditions = append(wb.conditions, fmt.Sprintf("%s NOT IN (%s)", wb.dialect.QuoteIdentifier(column), q))
	return wb
}

// Build constructs the WHERE clause string and returns conditions with args.

func (wb *WhereBuilder) BuildWithOffset(startIndex int) (string, []interface{}) {
	if len(wb.conditions) == 0 {
		return "", nil
	}
	
	// Replace placeholders with dialect-specific ones
	conditions := make([]string, len(wb.conditions))
	argIndex := startIndex
	for i, condition := range wb.conditions {
		conditions[i] = condition
		// Replace ? placeholders with dialect-specific ones
		for j := 0; j < strings.Count(condition, "?"); j++ {
			conditions[i] = strings.Replace(conditions[i], "?", wb.dialect.Placeholder(argIndex), 1)
			argIndex++
		}
	}
	
	return "WHERE " + strings.Join(conditions, " "), wb.args
}

// Build constructs the WHERE clause string and returns conditions with args.
func (wb *WhereBuilder) Build() (string, []interface{}) {
	return wb.BuildWithOffset(0)
}

// Clone creates a copy of the WhereBuilder.
func (wb *WhereBuilder) Clone() *WhereBuilder {
	clone := &WhereBuilder{
		conditions: make([]string, len(wb.conditions)),
		args:       make([]interface{}, len(wb.args)),
		dialect:    wb.dialect,
	}
	
	copy(clone.conditions, wb.conditions)
	copy(clone.args, wb.args)
	
	return clone
}

// Reset clears all conditions and arguments.
func (wb *WhereBuilder) Reset() *WhereBuilder {
	wb.conditions = wb.conditions[:0]
	wb.args = wb.args[:0]
	return wb
}

// IsEmpty returns true if no conditions have been added.
func (wb *WhereBuilder) IsEmpty() bool {
	return len(wb.conditions) == 0
}

// Count returns the number of conditions.
func (wb *WhereBuilder) Count() int {
	return len(wb.conditions)
}

// GetArgs returns a copy of the arguments.
func (wb *WhereBuilder) GetArgs() []interface{} {
	args := make([]interface{}, len(wb.args))
	copy(args, wb.args)
	return args
}

// GetConditions returns a copy of the conditions.
func (wb *WhereBuilder) GetConditions() []string {
	conditions := make([]string, len(wb.conditions))
	copy(conditions, wb.conditions)
	return conditions
}
