// Package db provides global database helper functions for easy querying.
// This helper automatically uses the framework-managed connection pool.
package db

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
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
	return factory.GetMySQLSQLClient()
}

// Query executes a SELECT query and returns the result rows.
// Automatically uses the framework's connection pool.
func Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	client, err := MySQL()
	if err != nil {
		return nil, err
	}
	return client.Query(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	client, err := MySQL()
	if err != nil {
		// Return a row with error embedded
		return &sql.Row{}
	}
	return client.QueryRow(ctx, query, args...)
}

// Exec executes an INSERT, UPDATE, DELETE, or other non-SELECT query.
func Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	client, err := MySQL()
	if err != nil {
		return nil, err
	}
	return client.Exec(ctx, query, args...)
}

// Begin starts a transaction.
func Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	client, err := MySQL()
	if err != nil {
		return nil, err
	}
	return client.Begin(ctx, opts)
}

// Transaction executes a function within a transaction.
func Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	client, err := MySQL()
	if err != nil {
		return err
	}
	return client.Transaction(ctx, fn)
}

// DB returns the underlying *sql.DB for advanced operations.
func DB() (*sql.DB, error) {
	client, err := MySQL()
	if err != nil {
		return nil, err
	}
	return client.DB(), nil
}

// DBQuery is a fluent query builder wrapper that supports multiple database types.
type DBQuery struct {
	dbType DBType
	table  string
}

// Using creates a new DBQuery for the specified database type.
// Usage: db.Using(db.DBTypePostgres).Select("orders")
func Using(dbType DBType) *DBQuery {
	return &DBQuery{dbType: dbType}
}

// Select creates a SELECT builder for the specified database and table.
func (q *DBQuery) Select(table string) (*orm.SelectBuilder, error) {
	return SelectOn(q.dbType, table)
}

// Insert creates an INSERT builder for the specified database and table.
func (q *DBQuery) Insert(table string) (*orm.InsertBuilder, error) {
	return InsertOn(q.dbType, table)
}

// Update creates an UPDATE builder for the specified database and table.
func (q *DBQuery) Update(table string) (*orm.UpdateBuilder, error) {
	return UpdateOn(q.dbType, table)
}

// Delete creates a DELETE builder for the specified database and table.
func (q *DBQuery) Delete(table string) (*orm.DeleteBuilder, error) {
	return DeleteOn(q.dbType, table)
}

// DBType specifies which database to use for ORM operations.
type DBType string

const (
	DBTypeMySQL    DBType = "mysql"
	DBTypePostgres DBType = "postgres"
)

// SelectOn creates a SELECT builder for the specified database type.
func SelectOn(dbType DBType, table string) (*orm.SelectBuilder, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}

	switch dbType {
	case DBTypeMySQL, "":
		client, err := factory.GetMySQL()
		if err != nil {
			return nil, err
		}
		return orm.NewSelectBuilder(client, table), nil
	case DBTypePostgres:
		client, err := factory.GetPostgres()
		if err != nil {
			return nil, err
		}
		return orm.NewSelectBuilder(client, table), nil
	default:
		return nil, &DBError{Message: "unsupported database type: " + string(dbType)}
	}
}

// InsertOn creates an INSERT builder for the specified database type.
func InsertOn(dbType DBType, table string) (*orm.InsertBuilder, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}

	switch dbType {
	case DBTypeMySQL, "":
		client, err := factory.GetMySQL()
		if err != nil {
			return nil, err
		}
		return orm.NewInsertBuilder(client, table), nil
	case DBTypePostgres:
		client, err := factory.GetPostgres()
		if err != nil {
			return nil, err
		}
		return orm.NewInsertBuilder(client, table), nil
	default:
		return nil, &DBError{Message: "unsupported database type: " + string(dbType)}
	}
}

// UpdateOn creates an UPDATE builder for the specified database type.
func UpdateOn(dbType DBType, table string) (*orm.UpdateBuilder, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}

	switch dbType {
	case DBTypeMySQL, "":
		client, err := factory.GetMySQL()
		if err != nil {
			return nil, err
		}
		return orm.NewUpdateBuilder(client, table), nil
	case DBTypePostgres:
		client, err := factory.GetPostgres()
		if err != nil {
			return nil, err
		}
		return orm.NewUpdateBuilder(client, table), nil
	default:
		return nil, &DBError{Message: "unsupported database type: " + string(dbType)}
	}
}

// DeleteOn creates a DELETE builder for the specified database type.
func DeleteOn(dbType DBType, table string) (*orm.DeleteBuilder, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}

	switch dbType {
	case DBTypeMySQL, "":
		client, err := factory.GetMySQL()
		if err != nil {
			return nil, err
		}
		return orm.NewDeleteBuilder(client, table), nil
	case DBTypePostgres:
		client, err := factory.GetPostgres()
		if err != nil {
			return nil, err
		}
		return orm.NewDeleteBuilder(client, table), nil
	default:
		return nil, &DBError{Message: "unsupported database type: " + string(dbType)}
	}
}

// MySQLClient returns the global MySQL client for ORM operations.
func MySQLClient() (*mysql.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetMySQL()
}

// Select creates a new SELECT query builder using the global MySQL client.
// If a database has been set via Use(), the table will be prefixed with "database."
func Select(table string) (*orm.SelectBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}
	return orm.NewSelectBuilder(client, withDB(table)), nil
}

// Insert creates a new INSERT query builder using the global MySQL client.
// If a database has been set via Use(), the table will be prefixed with "database."
func Insert(table string) (*orm.InsertBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}
	return orm.NewInsertBuilder(client, withDB(table)), nil
}

// Update creates a new UPDATE query builder using the global MySQL client.
// If a database has been set via Use(), the table will be prefixed with "database."
func Update(table string) (*orm.UpdateBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}
	return orm.NewUpdateBuilder(client, withDB(table)), nil
}

// Delete creates a new DELETE query builder using the global MySQL client.
// If a database has been set via Use(), the table will be prefixed with "database."
func Delete(table string) (*orm.DeleteBuilder, error) {
	client, err := MySQLClient()
	if err != nil {
		return nil, err
	}
	return orm.NewDeleteBuilder(client, withDB(table)), nil
}

// ==================== Multi-Database Support (Same MySQL Instance) ====================

var (
	currentDB   string
	currentDBMu sync.RWMutex
)

// Use sets the current database for subsequent queries.
// This affects the default database used by Select/Insert/Update/Delete.
// Usage:
//
//	db.Use("userdb")    // switch to userdb
//	db.Select("users")  // queries userdb.users
//	db.Use("orderdb")   // switch to orderdb
//	db.Select("orders") // queries orderdb.orders
//
// To reset to default (no prefix), call Use("") or UseDefault().
func Use(database string) {
	currentDBMu.Lock()
	defer currentDBMu.Unlock()
	currentDB = database
}

// UseDefault resets the current database to default (no prefix).
func UseDefault() {
	Use("")
}

// GetDB returns the current database name.
func GetDB() string {
	currentDBMu.RLock()
	defer currentDBMu.RUnlock()
	return currentDB
}

// withDB adds database prefix to table if currentDB is set and table doesn't already have a dot.
func withDB(table string) string {
	currentDBMu.RLock()
	defer currentDBMu.RUnlock()
	if currentDB != "" && !strings.Contains(table, ".") {
		return currentDB + "." + table
	}
	return table
}

// ==================== PostgreSQL ORM Helpers ====================

// PostgresSelect creates a SELECT builder for PostgreSQL.
func PostgresSelect(table string) (*orm.SelectBuilder, error) {
	return SelectOn(DBTypePostgres, table)
}

// PostgresInsert creates an INSERT builder for PostgreSQL.
func PostgresInsert(table string) (*orm.InsertBuilder, error) {
	return InsertOn(DBTypePostgres, table)
}

// PostgresUpdate creates an UPDATE builder for PostgreSQL.
func PostgresUpdate(table string) (*orm.UpdateBuilder, error) {
	return UpdateOn(DBTypePostgres, table)
}

// PostgresDelete creates a DELETE builder for PostgreSQL.
func PostgresDelete(table string) (*orm.DeleteBuilder, error) {
	return DeleteOn(DBTypePostgres, table)
}

// ==================== Redis Helper Functions ====================

// Redis returns the global Redis client.
func Redis() (*redis.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetRedis()
}

// ==================== PostgreSQL Helper Functions ====================

// Postgres returns the global PostgreSQL client.
func Postgres() (*postgres.Client, error) {
	factory := GetGlobalFactory()
	if factory == nil {
		return nil, ErrFactoryNotInitialized
	}
	return factory.GetPostgres()
}

// ==================== Common Errors ====================

var ErrFactoryNotInitialized = &DBError{Message: "database factory not initialized, ensure bootstrap.InitializeDatabases() was called"}

// DBError represents a database helper error.
type DBError struct {
	Message string
}

func (e *DBError) Error() string {
	return e.Message
}
