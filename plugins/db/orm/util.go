package orm

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	// ErrInvalidTableName is returned when a table name is invalid
	ErrInvalidTableName = errors.New("invalid table name")
	// ErrInvalidColumnName is returned when a column name is invalid
	ErrInvalidColumnName = errors.New("invalid column name")
	// ErrInvalidDataType is returned when data type is not supported
	ErrInvalidDataType = errors.New("invalid data type")
	// ErrEmptyData is returned when data map is empty
	ErrEmptyData = errors.New("data cannot be empty")
)

// Table name validation pattern - allow alphanumeric, underscore, and hyphen
var tableNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// Column name validation pattern - allow alphanumeric, underscore, and some special chars
var columnNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
var qualifiedIdentifierPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)*$`)
var quotedQualifiedIdentifierPattern = regexp.MustCompile("^`[a-zA-Z][a-zA-Z0-9_]*`(?:\\.`[a-zA-Z][a-zA-Z0-9_]*`)*$")
var jsonPathIdentifierPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(?:\.[a-zA-Z][a-zA-Z0-9_]*)?\s*->>?\s*'[^']+'$`)
var orderByItemPattern = regexp.MustCompile(`(?i)^\s*([a-zA-Z][a-zA-Z0-9_]*(?:\.[a-zA-Z][a-zA-Z0-9_]*)?)\s*(ASC|DESC)?\s*$`)
var unsafeSQLKeywordPattern = regexp.MustCompile(`(?i)\b(union\s+all\s+select|union\s+select|into\s+outfile|load_file\s*\(|sleep\s*\(|benchmark\s*\(|xp_cmdshell|drop\s+table|truncate\s+table)\b`)

var allowedJoinTypes = map[string]struct{}{
	"INNER":      {},
	"LEFT":       {},
	"RIGHT":      {},
	"FULL OUTER": {},
	"CROSS":      {},
}

// IsZero checks if a value is zero or nil.
func IsZero(v interface{}) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.String:
		return rv.String() == ""
	case reflect.Ptr:
		return rv.IsNil()
	case reflect.Slice, reflect.Map, reflect.Array:
		return rv.Len() == 0
	case reflect.Struct:
		// Special case for time.Time
		if t, ok := v.(time.Time); ok {
			return t.IsZero()
		}
		return false
	default:
		return false
	}
}

// ToSnakeCase converts a string from CamelCase to snake_case.
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	result := make([]rune, 0, len(runes)+4)

	for i, r := range runes {
		if i > 0 && isUpper(r) {
			prev := runes[i-1]
			var next rune
			hasNext := i+1 < len(runes)
			if hasNext {
				next = runes[i+1]
			}
			if isLower(prev) || isDigit(prev) || (isUpper(prev) && hasNext && isLower(next)) {
				result = append(result, '_')
			}
		}
		result = append(result, toLower(r))
	}

	return string(result)
}

// ToCamelCase converts a string from snake_case to CamelCase.
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}
	
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	
	return strings.Join(parts, "")
}

// BuildInsertQuery builds an INSERT query from table name and data with validation.
func BuildInsertQuery(table string, data map[string]interface{}, dialect Dialect) (string, []interface{}, error) {
	if err := ValidateTableName(table); err != nil {
		return "", nil, err
	}
	
	if len(data) == 0 {
		return "", nil, ErrEmptyData
	}
	
	// Validate column names and data types
	for column, value := range data {
		if err := ValidateColumnName(column); err != nil {
			return "", nil, fmt.Errorf("invalid column name '%s': %w", column, err)
		}
		if !isValidDataType(value) {
			return "", nil, fmt.Errorf("invalid data type for column '%s': %w", column, ErrInvalidDataType)
		}
	}
	
	// Sort keys for consistent query generation
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Build column list
	quotedKeys := QuoteIdentifiers(dialect, keys...)
	columns := strings.Join(quotedKeys, ", ")
	
	// Build placeholders
	placeholders := BuildPlaceholders(dialect, len(keys))
	
	// Build arguments
	args := make([]interface{}, len(keys))
	for i, key := range keys {
		args[i] = data[key]
	}
	
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", 
		dialect.QuoteIdentifier(table), columns, placeholders)
	
	return query, args, nil
}

// BuildUpdateQuery builds an UPDATE query from table name, ID, and data with validation.
func BuildUpdateQuery(table, idColumn string, id interface{}, data map[string]interface{}, dialect Dialect) (string, []interface{}, error) {
	if err := ValidateTableName(table); err != nil {
		return "", nil, err
	}
	
	if err := ValidateColumnName(idColumn); err != nil {
		return "", nil, fmt.Errorf("invalid ID column name '%s': %w", idColumn, err)
	}
	
	if len(data) == 0 {
		return "", nil, ErrEmptyData
	}
	
	// Validate column names and data types
	for column, value := range data {
		if err := ValidateColumnName(column); err != nil {
			return "", nil, fmt.Errorf("invalid column name '%s': %w", column, err)
		}
		if !isValidDataType(value) {
			return "", nil, fmt.Errorf("invalid data type for column '%s': %w", column, ErrInvalidDataType)
		}
	}
	
	// Sort keys for consistent query generation
	keys := make([]string, 0, len(data))
	for k := range data {
		if k != idColumn { // Skip ID column in SET clause
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	
	if len(keys) == 0 {
		return "", nil, fmt.Errorf("no columns to update after excluding ID column '%s'", idColumn)
	}
	
	// Build SET clause
	setParts := make([]string, len(keys))
	args := make([]interface{}, len(keys)+1) // +1 for ID
	
	for i, key := range keys {
		quotedKey := dialect.QuoteIdentifier(key)
		setParts[i] = fmt.Sprintf("%s = %s", quotedKey, dialect.Placeholder(i))
		args[i] = data[key]
	}
	
	// Add ID to arguments
	args[len(keys)] = id
	
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
		dialect.QuoteIdentifier(table),
		strings.Join(setParts, ", "),
		dialect.QuoteIdentifier(idColumn),
		dialect.Placeholder(len(keys)))
	
	return query, args, nil
}

// BuildWhereClause builds a WHERE clause from conditions.
func BuildWhereClause(conditions []string, args []interface{}, dialect Dialect) (string, []interface{}) {
	if len(conditions) == 0 {
		return "", nil
	}

	// Replace placeholders with dialect-specific ones without mutating input.
	replaced := make([]string, len(conditions))
	argIndex := 0
	for i, cond := range conditions {
		var builder strings.Builder
		builder.Grow(len(cond) + 16)
		for _, r := range cond {
			if r == '?' {
				builder.WriteString(dialect.Placeholder(argIndex))
				argIndex++
				continue
			}
			builder.WriteRune(r)
		}
		replaced[i] = builder.String()
	}

	whereClause := "WHERE " + strings.Join(replaced, " AND ")
	return whereClause, args
}

// BuildSelectQuery builds a SELECT query from components with validation.
func BuildSelectQuery(table string, columns []string, whereClause string, orderBy string, limit, offset int, dialect Dialect) (string, error) {
	if err := ValidateTableName(table); err != nil {
		return "", err
	}
	
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	
	// Validate column names
	for _, column := range columns {
		if hasUnsafeSQLBoundary(column) {
			return "", fmt.Errorf("unsafe column expression '%s'", column)
		}
		if column != "*" && !strings.Contains(column, "(") && !strings.Contains(column, " ") {
			// Skip validation for expressions like "COUNT(*)" or "DISTINCT column"
			if err := ValidateColumnName(column); err != nil {
				return "", fmt.Errorf("invalid column name '%s': %w", column, err)
			}
		}
	}
	
	quotedColumns := make([]string, len(columns))
	for i, column := range columns {
		if column == "*" || strings.HasPrefix(column, "DISTINCT ") || strings.Contains(column, "(") {
			quotedColumns[i] = column
		} else {
			quotedColumns[i] = dialect.QuoteIdentifier(column)
		}
	}
	
	query := fmt.Sprintf("SELECT %s FROM %s", 
		strings.Join(quotedColumns, ", "), 
		dialect.QuoteIdentifier(table))
	
	if whereClause != "" {
		if hasUnsafeSQLBoundary(whereClause) {
			return "", fmt.Errorf("unsafe where clause")
		}
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(whereClause)), "WHERE ") {
			return "", fmt.Errorf("invalid where clause, must start with WHERE")
		}
		query += " " + whereClause
	}
	
	if orderBy != "" {
		if hasUnsafeSQLBoundary(orderBy) {
			return "", fmt.Errorf("unsafe order by clause")
		}
		items := strings.Split(orderBy, ",")
		safeItems := make([]string, 0, len(items))
		for _, item := range items {
			safe, err := BuildSafeOrderByItem(dialect, item)
			if err != nil {
				return "", fmt.Errorf("invalid order by item '%s': %w", strings.TrimSpace(item), err)
			}
			safeItems = append(safeItems, safe)
		}
		query += " ORDER BY " + strings.Join(safeItems, ", ")
	}
	
	limitOffsetClause := dialect.LimitOffset(limit, offset)
	if limitOffsetClause != "" {
		query += " " + limitOffsetClause
	}
	
	return query, nil
}

// SelectQueryOptions defines structured inputs for safe select query construction.
type SelectQueryOptions struct {
	Columns         []string
	WhereConditions []string
	WhereArgs       []interface{}
	OrderByItems    []string
	Limit           int
	Offset          int
	Dialect         Dialect
}

// BuildSelectQueryWithOptions builds a SELECT query from structured options.
// This reduces direct raw string concatenation for WHERE/ORDER BY.
func BuildSelectQueryWithOptions(table string, opts SelectQueryOptions) (string, []interface{}, error) {
	dialect := opts.Dialect
	if dialect == nil {
		dialect = NewDefaultDialect()
	}

	for _, cond := range opts.WhereConditions {
		if hasUnsafeSQLBoundary(cond) {
			return "", nil, fmt.Errorf("unsafe where condition")
		}
	}

	whereClause, whereArgs := BuildWhereClause(opts.WhereConditions, opts.WhereArgs, dialect)

	orderBy := ""
	if len(opts.OrderByItems) > 0 {
		safeItems := make([]string, 0, len(opts.OrderByItems))
		for _, item := range opts.OrderByItems {
			safe, err := BuildSafeOrderByItem(dialect, item)
			if err != nil {
				return "", nil, err
			}
			safeItems = append(safeItems, safe)
		}
		orderBy = strings.Join(safeItems, ", ")
	}

	query, err := BuildSelectQuery(table, opts.Columns, whereClause, orderBy, opts.Limit, opts.Offset, dialect)
	if err != nil {
		return "", nil, err
	}

	return query, whereArgs, nil
}

// BuildDeleteQuery builds a DELETE query from table name and conditions with validation.
func BuildDeleteQuery(table string, whereClause string, limit int, dialect Dialect) (string, error) {
	if err := ValidateTableName(table); err != nil {
		return "", err
	}
	
	query := fmt.Sprintf("DELETE FROM %s", dialect.QuoteIdentifier(table))
	
	if whereClause != "" {
		query += " " + whereClause
	}
	
	if limit > 0 {
		limitClause := dialect.LimitOffset(limit, 0)
		if limitClause != "" {
			query += " " + limitClause
		}
	}
	
	return query, nil
}

// MapToSlice converts a map to a slice of key-value pairs.
func MapToSlice(data map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(data))
	i := 0
	for k, v := range data {
		result[i] = map[string]interface{}{"key": k, "value": v}
		i++
	}
	return result
}

// SliceToMap converts a slice of key-value pairs to a map.
func SliceToMap(slice []map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, item := range slice {
		if key, ok := item["key"].(string); ok {
			result[key] = item["value"]
		}
	}
	return result
}

// FilterEmptyValues removes empty values from a map.
func FilterEmptyValues(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		if !IsZero(v) {
			result[k] = v
		}
	}
	return result
}

// MergeMaps merges multiple maps, with later maps taking precedence.
func MergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// GetMapKeys returns all keys from a map as a slice.
func GetMapKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ValidateTableName checks if a table name is valid.
func ValidateTableName(table string) error {
	if table == "" {
		return ErrInvalidTableName
	}
	
	if !tableNamePattern.MatchString(table) {
		return fmt.Errorf("%w: '%s' (must start with letter and contain only letters, numbers, underscores)", ErrInvalidTableName, table)
	}
	
	return nil
}

// ValidateColumnName checks if a column name is valid.
func ValidateColumnName(column string) error {
	if column == "" {
		return ErrInvalidColumnName
	}

	column = strings.TrimSpace(column)

	if columnNamePattern.MatchString(column) {
		return nil
	}
	if qualifiedIdentifierPattern.MatchString(column) {
		return nil
	}
	if jsonPathIdentifierPattern.MatchString(column) {
		return nil
	}

	return fmt.Errorf("%w: '%s'", ErrInvalidColumnName, column)
}

// ValidateQualifiedIdentifier checks if a dotted identifier is valid (e.g. table.column).
func ValidateQualifiedIdentifier(identifier string) error {
	if identifier == "" {
		return ErrInvalidColumnName
	}

	if !qualifiedIdentifierPattern.MatchString(identifier) {
		return fmt.Errorf("%w: '%s' (must be one or more dot-separated identifiers)", ErrInvalidColumnName, identifier)
	}

	return nil
}

// BuildSafeOrderByItem validates and quotes a single ORDER BY item.
// Supported forms: "column", "column ASC", "table.column DESC".
func BuildSafeOrderByItem(dialect Dialect, item string) (string, error) {
	_ = dialect
	matches := orderByItemPattern.FindStringSubmatch(item)
	if len(matches) < 2 {
		return "", fmt.Errorf("invalid order by item: %s", item)
	}

	identifier := matches[1]
	if err := ValidateQualifiedIdentifier(identifier); err != nil {
		return "", err
	}

	direction := strings.ToUpper(strings.TrimSpace(matches[2]))
	if direction != "" && direction != "ASC" && direction != "DESC" {
		return "", fmt.Errorf("invalid order direction: %s", direction)
	}

	if direction == "" {
		return identifier, nil
	}

	return identifier + " " + direction, nil
}

// NormalizeJoinType validates and normalizes join type to a safe allowlist.
func NormalizeJoinType(joinType string) (string, error) {
	jt := strings.ToUpper(strings.TrimSpace(joinType))
	jt = strings.Join(strings.Fields(jt), " ")
	if _, ok := allowedJoinTypes[jt]; !ok {
		return "", fmt.Errorf("invalid join type: %s", joinType)
	}
	return jt, nil
}

// ValidateJoinTableReference checks whether join table expression is a safe
// identifier reference with optional alias, e.g. "users", "users u", "users AS u",
// "schema.users u", or "`schema`.`users` u".
func ValidateJoinTableReference(table string) error {
	table = strings.TrimSpace(table)
	if table == "" {
		return ErrInvalidTableName
	}

	parts := strings.Fields(table)
	if len(parts) == 0 || len(parts) > 3 {
		return fmt.Errorf("%w: '%s'", ErrInvalidTableName, table)
	}

	base := parts[0]
	if !(qualifiedIdentifierPattern.MatchString(base) || quotedQualifiedIdentifierPattern.MatchString(base)) {
		return fmt.Errorf("%w: '%s'", ErrInvalidTableName, table)
	}

	if len(parts) == 1 {
		return nil
	}

	if len(parts) == 2 {
		if err := ValidateColumnName(parts[1]); err != nil {
			return fmt.Errorf("%w: '%s'", ErrInvalidTableName, table)
		}
		return nil
	}

	if !strings.EqualFold(parts[1], "AS") {
		return fmt.Errorf("%w: '%s'", ErrInvalidTableName, table)
	}
	if err := ValidateColumnName(parts[2]); err != nil {
		return fmt.Errorf("%w: '%s'", ErrInvalidTableName, table)
	}

	return nil
}

// isValidDataType checks if the data type is supported.
func isValidDataType(value interface{}) bool {
	if value == nil {
		return true
	}
	
	switch value.(type) {
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, string, []byte, time.Time:
		return true
	default:
		// Check if it's a pointer to a supported type
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			switch rv.Elem().Kind() {
			case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				 reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				 reflect.Float32, reflect.Float64, reflect.String:
				return true
			}
		}
		return false
	}
}

// EscapeLike escapes special characters for LIKE queries.
func EscapeLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value
}

// BuildLikeCondition builds a LIKE condition with proper escaping.
func BuildLikeCondition(column, pattern string, dialect Dialect) string {
	_ = pattern
	quotedColumn := dialect.QuoteIdentifier(column)
	return fmt.Sprintf("%s LIKE ? ESCAPE '\\'", quotedColumn)
}

func hasUnsafeSQLBoundary(sqlFragment string) bool {
	trimmed := strings.TrimSpace(sqlFragment)
	s := strings.ToLower(trimmed)
	if strings.Contains(s, ";") ||
		strings.Contains(s, "--") ||
		strings.Contains(s, "/*") ||
		strings.Contains(s, "*/") ||
		strings.ContainsRune(s, '\x00') {
		return true
	}

	if unsafeSQLKeywordPattern.MatchString(trimmed) {
		return true
	}

	return false
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
func isDigit(r rune) bool { return r >= '0' && r <= '9' }
func toLower(r rune) rune {
	if isUpper(r) {
		return r + ('a' - 'A')
	}
	return r
}
