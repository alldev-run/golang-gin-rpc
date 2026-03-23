package orm

import (
	"fmt"
	"strings"
)

// Dialect defines database-specific SQL dialects.
type Dialect interface {
	// LockForUpdate returns the FOR UPDATE clause syntax
	LockForUpdate() string
	// LockInShareMode returns the LOCK IN SHARE MODE clause syntax
	LockInShareMode() string
	// QuoteIdentifier quotes database identifiers (table names, column names, etc.)
	QuoteIdentifier(identifier string) string
	// Placeholder returns the parameter placeholder for the given index
	Placeholder(index int) string
	// SupportsFeature checks if the dialect supports a specific feature
	SupportsFeature(feature Feature) bool
	// LimitOffset returns the LIMIT/OFFSET clause syntax
	LimitOffset(limit, offset int) string
	// GetLastInsertID returns the query to get the last inserted ID
	GetLastInsertID() string
}

// Feature represents database features that may vary between dialects.
type Feature int

const (
	FeatureWindowFunctions Feature = iota
	FeatureCTE
	FeatureFullOuterJoin
	FeatureJSON
	FeatureArray
	FeatureUUID
	FeatureUpsert
)

// MySQLDialect provides MySQL-compatible dialect.
type MySQLDialect struct{}

// NewMySQLDialect creates a new MySQL dialect.
func NewMySQLDialect() *MySQLDialect {
	return &MySQLDialect{}
}

func (d *MySQLDialect) LockForUpdate() string {
	return "FOR UPDATE"
}

func (d *MySQLDialect) LockInShareMode() string {
	return "LOCK IN SHARE MODE"
}

func (d *MySQLDialect) QuoteIdentifier(identifier string) string {
	return "`" + identifier + "`"
}

func (d *MySQLDialect) Placeholder(index int) string {
	return "?"
}

func (d *MySQLDialect) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeatureWindowFunctions, FeatureCTE, FeatureJSON:
		return true
	case FeatureFullOuterJoin, FeatureArray, FeatureUUID, FeatureUpsert:
		return false
	default:
		return false
	}
}

func (d *MySQLDialect) LimitOffset(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	if offset <= 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	}
	if limit <= 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

func (d *MySQLDialect) GetLastInsertID() string {
	return "SELECT LAST_INSERT_ID()"
}

// PostgreSQLDialect provides PostgreSQL-compatible dialect.
type PostgreSQLDialect struct{}

// NewPostgreSQLDialect creates a new PostgreSQL dialect.
func NewPostgreSQLDialect() *PostgreSQLDialect {
	return &PostgreSQLDialect{}
}

func (d *PostgreSQLDialect) LockForUpdate() string {
	return "FOR UPDATE"
}

func (d *PostgreSQLDialect) LockInShareMode() string {
	return "FOR SHARE"
}

func (d *PostgreSQLDialect) QuoteIdentifier(identifier string) string {
	return `"` + identifier + `"`
}

func (d *PostgreSQLDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index+1)
}

func (d *PostgreSQLDialect) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeatureWindowFunctions, FeatureCTE, FeatureFullOuterJoin, FeatureJSON, FeatureArray, FeatureUUID, FeatureUpsert:
		return true
	default:
		return false
	}
}

func (d *PostgreSQLDialect) LimitOffset(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	if offset <= 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	}
	if limit <= 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

func (d *PostgreSQLDialect) GetLastInsertID() string {
	return "RETURNING id"
}

// SQLiteDialect provides SQLite-compatible dialect.
type SQLiteDialect struct{}

// NewSQLiteDialect creates a new SQLite dialect.
func NewSQLiteDialect() *SQLiteDialect {
	return &SQLiteDialect{}
}

func (d *SQLiteDialect) LockForUpdate() string {
	return ""
}

func (d *SQLiteDialect) LockInShareMode() string {
	return ""
}

func (d *SQLiteDialect) QuoteIdentifier(identifier string) string {
	return `"` + identifier + `"`
}

func (d *SQLiteDialect) Placeholder(index int) string {
	return "?"
}

func (d *SQLiteDialect) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeatureWindowFunctions, FeatureCTE, FeatureJSON:
		return true
	case FeatureFullOuterJoin, FeatureArray, FeatureUUID, FeatureUpsert:
		return false
	default:
		return false
	}
}

func (d *SQLiteDialect) LimitOffset(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	if offset <= 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	}
	if limit <= 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

func (d *SQLiteDialect) GetLastInsertID() string {
	return "SELECT last_insert_rowid()"
}

// ClickHouseDialect provides ClickHouse-compatible dialect.
// Note: ClickHouse SQL differs from OLTP databases; this dialect targets the
// query builder output (quoting/placeholders/limit) that is commonly accepted
// by ClickHouse drivers.
type ClickHouseDialect struct{}

// NewClickHouseDialect creates a new ClickHouse dialect.
func NewClickHouseDialect() *ClickHouseDialect {
	return &ClickHouseDialect{}
}

func (d *ClickHouseDialect) LockForUpdate() string {
	return ""
}

func (d *ClickHouseDialect) LockInShareMode() string {
	return ""
}

func (d *ClickHouseDialect) QuoteIdentifier(identifier string) string {
	return "`" + identifier + "`"
}

func (d *ClickHouseDialect) Placeholder(index int) string {
	return "?"
}

func (d *ClickHouseDialect) SupportsFeature(feature Feature) bool {
	switch feature {
	case FeatureWindowFunctions, FeatureCTE, FeatureJSON:
		return true
	default:
		return false
	}
}

func (d *ClickHouseDialect) LimitOffset(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	if offset <= 0 {
		return fmt.Sprintf("LIMIT %d", limit)
	}
	if limit <= 0 {
		return fmt.Sprintf("OFFSET %d", offset)
	}
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

func (d *ClickHouseDialect) GetLastInsertID() string {
	return ""
}

// DefaultDialect provides MySQL-compatible dialect as default.
type DefaultDialect struct {
	*MySQLDialect
}

// NewDefaultDialect creates a new default dialect (MySQL-compatible).
func NewDefaultDialect() *DefaultDialect {
	return &DefaultDialect{MySQLDialect: NewMySQLDialect()}
}

// DialectType represents the type of database dialect.
type DialectType string

const (
	DialectMySQL      DialectType = "mysql"
	DialectPostgreSQL DialectType = "postgresql"
	DialectSQLite     DialectType = "sqlite"
	DialectClickHouse DialectType = "clickhouse"
)

// NewDialect creates a new dialect based on the dialect type.
func NewDialect(dialectType DialectType) Dialect {
	switch dialectType {
	case DialectMySQL:
		return NewMySQLDialect()
	case DialectPostgreSQL:
		return NewPostgreSQLDialect()
	case DialectSQLite:
		return NewSQLiteDialect()
	case DialectClickHouse:
		return NewClickHouseDialect()
	default:
		return NewDefaultDialect()
	}
}

// QuoteIdentifiers quotes multiple identifiers using the dialect.
func QuoteIdentifiers(dialect Dialect, identifiers ...string) []string {
	quoted := make([]string, len(identifiers))
	for i, identifier := range identifiers {
		quoted[i] = dialect.QuoteIdentifier(identifier)
	}
	return quoted
}

// BuildPlaceholders builds a string of placeholders for the given count.
func BuildPlaceholders(dialect Dialect, count int) string {
	if count <= 0 {
		return ""
	}
	
	placeholders := make([]string, count)
	for i := 0; i < count; i++ {
		placeholders[i] = dialect.Placeholder(i)
	}
	
	return strings.Join(placeholders, ", ")
}
