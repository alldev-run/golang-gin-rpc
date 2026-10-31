// Package orm provides a database-agnostic ORM layer with query builders.
package orm

import (
	"context"
	"database/sql"
	"fmt"
)

// ORM provides the main entry point for ORM operations.
type ORM struct {
	db      DB
	dialect Dialect
}

// NewORM creates a new ORM instance.
func NewORM(db *DBWrapper, dialect Dialect) *ORM {
	if dialect == nil {
		dialect = NewDefaultDialect()
	}
	return &ORM{
		db:      db,
		dialect: dialect,
	}
}

// NewORMWithDB creates a new ORM instance from a sql.DB.
func NewORMWithDB(db *sql.DB, dialect Dialect) *ORM {
	return NewORM(NewDBWrapper(db), dialect)
}

// DB returns the underlying database interface.
func (o *ORM) DB() DB {
	return o.db
}

// Dialect returns the database dialect.
func (o *ORM) Dialect() Dialect {
	return o.dialect
}

// Select creates a new SELECT query builder.
func (o *ORM) Select(table string) *SelectBuilder {
	return NewSelectBuilderWithDialect(o.db, table, o.dialect)
}

// SelectWithScopes creates a new SELECT query builder with scopes support.
// func (o *ORM) SelectWithScopes(table string) *ScopedQueryBuilder {
// 	return NewScopedQueryBuilder(o.Select(table))
// }

// Insert creates a new INSERT query builder.
func (o *ORM) Insert(table string) *InsertBuilder {
	return NewInsertBuilderWithDialect(o.db, table, o.dialect)
}

// Update creates a new UPDATE query builder.
func (o *ORM) Update(table string) *UpdateBuilder {
	return NewUpdateBuilderWithDialect(o.db, table, o.dialect)
}

// Delete creates a new DELETE query builder.
func (o *ORM) Delete(table string) *DeleteBuilder {
	return NewDeleteBuilderWithDialect(o.db, table, o.dialect)
}

// Where creates a new WHERE builder.
func (o *ORM) Where() *WhereBuilder {
	return NewWhereBuilder(o.dialect)
}

// Transaction executes a function within a transaction.
func (o *ORM) Transaction(ctx context.Context, fn func(*ORM) error) error {
	tx, err := o.db.Begin(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txORM := &ORM{
		db:      NewTxWrapper(tx),
		dialect: o.dialect,
	}

	if err := fn(txORM); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Ping checks the database connection health.
func (o *ORM) Ping(ctx context.Context) error {
	return o.db.Ping(ctx)
}

// Close closes the database connection.
func (o *ORM) Close() error {
	return o.db.Close()
}

// Stats returns database connection statistics.
func (o *ORM) Stats() sql.DBStats {
	return o.db.Stats()
}

// Legacy helper functions for backward compatibility

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
	dialect := NewDefaultDialect()
	if err := ValidateTableName(table); err != nil {
		return 0, err
	}
	if err := ValidateColumnName(idColumn); err != nil {
		return 0, err
	}
	if err := ValidateColumnName(field); err != nil {
		return 0, err
	}

	query := fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s = %s",
		dialect.QuoteIdentifier(table),
		dialect.QuoteIdentifier(field),
		dialect.Placeholder(0),
		dialect.QuoteIdentifier(idColumn),
		dialect.Placeholder(1),
	)
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
	dialect := NewDefaultDialect()

	if IsZero(id) {
		query, args, err := BuildInsertQuery(table, data, dialect)
		if err != nil {
			return 0, err
		}
		return InsertGetID(ctx, db, query, args...)
	}

	query, args, err := BuildUpdateQuery(table, idColumn, id, data, dialect)
	if err != nil {
		return 0, err
	}
	return Update(ctx, db, query, args...)
}
