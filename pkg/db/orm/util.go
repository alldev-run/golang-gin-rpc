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
	
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	
	return strings.ToLower(string(result))
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
	
	// Replace placeholders with dialect-specific ones
	for i := range conditions {
		conditions[i] = strings.ReplaceAll(conditions[i], "?", dialect.Placeholder(i))
	}
	
	whereClause := "WHERE " + strings.Join(conditions, " AND ")
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
		query += " " + whereClause
	}
	
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	
	limitOffsetClause := dialect.LimitOffset(limit, offset)
	if limitOffsetClause != "" {
		query += " " + limitOffsetClause
	}
	
	return query, nil
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
	
	if !columnNamePattern.MatchString(column) {
		return fmt.Errorf("%w: '%s' (must start with letter and contain only letters, numbers, underscores)", ErrInvalidColumnName, column)
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
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return value
}

// BuildLikeCondition builds a LIKE condition with proper escaping.
func BuildLikeCondition(column, pattern string, dialect Dialect) string {
	quotedColumn := dialect.QuoteIdentifier(column)
	return fmt.Sprintf("%s LIKE %s ESCAPE '\\'", quotedColumn, dialect.Placeholder(0))
}
