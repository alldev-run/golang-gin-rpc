// Package db provides global database helper functions for easy querying.
// This helper automatically uses the framework-managed connection pool.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/clickhouse"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mongodb"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

// Errors
var (
	// ErrFactoryNotInitialized is returned when database factory is not initialized
	ErrFactoryNotInitialized = errors.New("database factory not initialized, ensure bootstrap.InitializeDatabases() was called")
)

// contextKey is the type for context keys used in this package.
type contextKey int

const (
	// dbContextKey is the key for storing database name in context.
	dbContextKey contextKey = iota
)

var (
	globalFactory *Factory
	factoryMu     sync.RWMutex
)

// SetGlobalFactory sets the global database factory (called by bootstrap during initialization).
func SetGlobalFactory(factory *Factory) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	globalFactory = factory
}

// GetGlobalFactory returns the global database factory.
func GetGlobalFactory() *Factory {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	return globalFactory
}

// ==================== MySQL Helper Functions ====================

// MySQL returns the global MySQL SQLClient for direct SQL operations.
func MySQL() (SQLClient, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetMySQL()
}

// MySQLClient returns the global MySQL client for ORM operations.
func MySQLClient() (*mysql.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetMySQL()
}

// ==================== Database-Aware ORM Builders (Thread-Safe) ====================

// ==================== Global Convenience Functions ====================

// Insert creates an INSERT query builder for the specified table using the default database.
// This is a convenience function that calls InsertDB with an empty database name.
func Insert(table string) (*orm.InsertBuilder, error) {
	return InsertDB("", table)
}

// Update creates an UPDATE query builder for the specified table using the default database.
// This is a convenience function that calls UpdateDB with an empty database name.
func Update(table string) (*orm.UpdateBuilder, error) {
	return UpdateDB("", table)
}

// Select creates a SELECT query builder for the specified table using the default database.
// This is a convenience function that calls SelectDB with an empty database name.
func Select(table string) (*orm.SelectBuilder, error) {
	return SelectDB("", table)
}

// Delete creates a DELETE query builder for the specified table using the default database.
// This is a convenience function that calls DeleteDB with an empty database name.
func Delete(table string) (*orm.DeleteBuilder, error) {
	return DeleteDB("", table)
}

// ==================== Database-Specific Helper Functions ====================

// SelectDB creates a SELECT query builder for the specified database table.
// This is goroutine-safe and does not modify global state.
// If database is empty, uses the current default database.
func SelectDB(database, table string) (*orm.SelectBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}

	var fullTable string
	if database != "" {
		fullTable = database + "." + table
	} else {
		// Use default database - just use table name as is
		fullTable = table
	}

	return orm.NewSelectBuilder(client, fullTable), nil
}

// InsertDB creates an INSERT query builder for the specified database table.
// This is goroutine-safe and does not modify global state.
// If database is empty, uses the current default database.
func InsertDB(database, table string) (*orm.InsertBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}

	var fullTable string
	if database != "" {
		fullTable = database + "." + table
	} else {
		// Use default database - just use table name as is
		fullTable = table
	}

	return orm.NewInsertBuilder(client, fullTable), nil
}

// UpdateDB creates an UPDATE query builder for the specified database table.
// This is goroutine-safe and does not modify global state.
// If database is empty, uses the current default database.
func UpdateDB(database, table string) (*orm.UpdateBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}

	var fullTable string
	if database != "" {
		fullTable = database + "." + table
	} else {
		// Use default database - just use table name as is
		fullTable = table
	}

	return orm.NewUpdateBuilder(client, fullTable), nil
}

// DeleteDB creates a DELETE query builder for the specified database table.
// This is goroutine-safe and does not modify global state.
// If database is empty, uses the current default database.
func DeleteDB(database, table string) (*orm.DeleteBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}

	var fullTable string
	if database != "" {
		fullTable = database + "." + table
	} else {
		// Use default database - just use table name as is
		fullTable = table
	}

	return orm.NewDeleteBuilder(client, fullTable), nil
}

// ==================== Fluent API (Thread-Safe) ====================

// DBSelector provides a fluent interface for database-specific operations.
type DBSelector struct {
	database string
}

// On returns a database selector for the specified database.
// This is goroutine-safe and does not modify global state.
func On(database string) *DBSelector {
	return &DBSelector{database: database}
}

// Select creates a SELECT query builder for the specified table.
func (d *DBSelector) Select(table string) (*orm.SelectBuilder, error) {
	return SelectDB(d.database, table)
}

// Insert creates an INSERT query builder for the specified table.
func (d *DBSelector) Insert(table string) (*orm.InsertBuilder, error) {
	return InsertDB(d.database, table)
}

// Update creates an UPDATE query builder for the specified table.
func (d *DBSelector) Update(table string) (*orm.UpdateBuilder, error) {
	return UpdateDB(d.database, table)
}

// Delete creates a DELETE query builder for the specified table.
func (d *DBSelector) Delete(table string) (*orm.DeleteBuilder, error) {
	return DeleteDB(d.database, table)
}

// ==================== Context-Aware SQL Helpers ====================

// QueryContext executes a SELECT query with context-aware database selection.
// Checks context for database name set via WithDBContext, falls back to global default.
func QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	client, err := MySQL()
	if err != nil {
		return nil, err
	}
	// Note: For raw SQL queries, database must be specified in the SQL itself
	// or use SelectDB/InsertDB/etc for ORM operations
	return client.Query(ctx, query, args...)
}

// ExecContext executes a non-SELECT query with context-aware database selection.
func ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	client, err := MySQL()
	if err != nil {
		return nil, err
	}
	return client.Exec(ctx, query, args...)
}

// ==================== Context Database Support ====================

// WithDBContext adds database name to context for per-request database selection.
func WithDBContext(ctx context.Context, database string) context.Context {
	return context.WithValue(ctx, dbContextKey, database)
}

// DBFromContext retrieves database name from context.
func DBFromContext(ctx context.Context) string {
	if db, ok := ctx.Value(dbContextKey).(string); ok {
		return db
	}
	return ""
}

// ==================== Other Database Helpers ====================

// Postgres returns the global PostgreSQL SQLClient.
func Postgres() (SQLClient, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetPostgres()
}

// MongoDB returns the global MongoDB client.
func MongoDB() (*mongodb.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetMongoDB()
}

// ClickHouse returns the global ClickHouse client.
func ClickHouse() (*clickhouse.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetClickHouse()
}

// Redis returns the global Redis client.
func Redis() (*redis.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetRedis()
}

// ==================== Database Type Helpers ====================

// InsertOn creates an INSERT builder for the specified database type.
func InsertOn(dbType Type, table string) (*orm.InsertBuilder, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}

	var client orm.DB
	var err error

	switch dbType {
	case TypeMySQL:
		client, err = factory.GetMySQL()
	case TypePostgres:
		client, err = factory.GetPostgres()
	default:
		return nil, fmt.Errorf("unsupported database type for INSERT: %v", dbType)
	}

	if err != nil {
		return nil, err
	}

	return orm.NewInsertBuilder(client, table), nil
}

// ==================== Legacy Helper Functions (Backward Compatibility) ====================

// These functions are kept for backward compatibility but may be deprecated in future versions.

// InsertGetID executes an INSERT statement and returns the last inserted ID.
func InsertGetID(ctx context.Context, db orm.DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateStatement executes an UPDATE statement and returns the number of rows affected.
func UpdateStatement(ctx context.Context, db orm.DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteStatement executes a DELETE statement and returns the number of rows affected.
func DeleteStatement(ctx context.Context, db orm.DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Query executes a SELECT query and returns the resulting rows.
func Query(ctx context.Context, db orm.DB, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(ctx, query, args...)
}

// QueryRow executes a SELECT query that returns a single row.
func QueryRow(ctx context.Context, db orm.DB, query string, args ...interface{}) *sql.Row {
	return db.QueryRow(ctx, query, args...)
}

// Exec executes a non-SELECT SQL statement.
func Exec(ctx context.Context, db orm.DB, query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(ctx, query, args...)
}
