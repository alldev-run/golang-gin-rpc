// Package mysql provides a MySQL database client with connection pooling,
// configuration management, and common query operations.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Config holds MySQL connection configuration.
type Config struct {
	Host            string        `yaml:"host" json:"host"`
	Port            int           `yaml:"port" json:"port"`
	Database        string        `yaml:"database" json:"database"`
	Username        string        `yaml:"username" json:"username"`
	Password        string        `yaml:"password" json:"password"`
	Charset         string        `yaml:"charset" json:"charset"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

// DefaultConfig returns default MySQL configuration.
func DefaultConfig() Config {
	return Config{
		Host:            "localhost",
		Port:            3306,
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}
}

// Client wraps sql.DB with additional functionality.
type Client struct {
	db                  *sql.DB
	config              Config
	defaultVersionField string
}

var ErrClientNotInitialized = errors.New("mysql client is nil or uninitialized")

var mysqlIdentifierPartPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

func quoteIdentifier(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", fmt.Errorf("invalid identifier: empty")
	}

	parts := strings.Split(identifier, ".")
	quoted := make([]string, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if !mysqlIdentifierPartPattern.MatchString(part) {
			return "", fmt.Errorf("invalid identifier: %s", identifier)
		}
		quoted[i] = "`" + part + "`"
	}

	return strings.Join(quoted, "."), nil
}

func normalizeConfig(config Config) Config {
	defaults := DefaultConfig()

	if config.Charset == "" {
		config.Charset = defaults.Charset
	}
	if config.MaxOpenConns <= 0 {
		config.MaxOpenConns = defaults.MaxOpenConns
	}
	if config.MaxIdleConns <= 0 {
		config.MaxIdleConns = defaults.MaxIdleConns
	}
	if config.ConnMaxLifetime <= 0 {
		config.ConnMaxLifetime = defaults.ConnMaxLifetime
	}
	if config.ConnMaxIdleTime <= 0 {
		config.ConnMaxIdleTime = defaults.ConnMaxIdleTime
	}

	if config.MaxIdleConns > config.MaxOpenConns {
		config.MaxIdleConns = config.MaxOpenConns
	}

	return config
}

func (c *Client) ensureDB() error {
	if c == nil || c.db == nil {
		return ErrClientNotInitialized
	}
	return nil
}

// New creates a new MySQL client from config.
func New(config Config) (*Client, error) {
	config = normalizeConfig(config)

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.Charset,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}

	return &Client{
		db:                  db,
		config:              config,
		defaultVersionField: "version",
	}, nil
}

// DB returns the underlying sql.DB instance.
func (c *Client) DB() *sql.DB {
	return c.db
}

// Close closes the database connection.
func (c *Client) Close() error {
	if err := c.ensureDB(); err != nil {
		return err
	}
	return c.db.Close()
}

// Ping checks the database connection health.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.ensureDB(); err != nil {
		return err
	}
	return c.db.PingContext(ctx)
}

// Stats returns database connection statistics.
func (c *Client) Stats() sql.DBStats {
	if err := c.ensureDB(); err != nil {
		return sql.DBStats{}
	}
	return c.db.Stats()
}

// Query executes a query that returns multiple rows.
func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if err := c.ensureDB(); err != nil {
		return nil, err
	}
	return c.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if err := c.ensureDB(); err != nil {
		return &sql.Row{}
	}
	return c.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning rows (INSERT, UPDATE, DELETE).
func (c *Client) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if err := c.ensureDB(); err != nil {
		return nil, err
	}
	return c.db.ExecContext(ctx, query, args...)
}

// InsertGetID executes an INSERT statement and returns the last inserted ID.
func (c *Client) InsertGetID(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := c.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Update executes an UPDATE statement and returns the number of rows affected.
func (c *Client) Update(ctx context.Context, query string, args ...any) (int64, error) {
	res, err := c.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetFieldByID updates a single field on a row identified by an ID column.
func (c *Client) SetFieldByID(ctx context.Context, table, idColumn string, id interface{}, field string, value interface{}) (int64, error) {
	quotedTable, err := quoteIdentifier(table)
	if err != nil {
		return 0, err
	}
	quotedField, err := quoteIdentifier(field)
	if err != nil {
		return 0, err
	}
	quotedIDColumn, err := quoteIdentifier(idColumn)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", quotedTable, quotedField, quotedIDColumn)
	res, err := c.Exec(ctx, query, value, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Save inserts or updates a row based on whether the provided id is zero.
// If id is zero or nil, it performs an INSERT and returns the new row ID.
// Otherwise, it performs an UPDATE and returns the number of affected rows.
func (c *Client) Save(ctx context.Context, table, idColumn string, id interface{}, data map[string]interface{}) (int64, error) {
	if isZero(id) {
		query, args, err := c.buildInsertQuery(table, data)
		if err != nil {
			return 0, err
		}
		return c.InsertGetID(ctx, query, args...)
	}
	query, args, err := c.buildUpdateQuery(table, idColumn, id, data)
	if err != nil {
		return 0, err
	}
	return c.Update(ctx, query, args...)
}

func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case int, int8, int16, int32, int64:
		return val == 0
	case uint, uint8, uint16, uint32, uint64:
		return val == 0
	case float32, float64:
		return val == 0
	case string:
		return val == ""
	case bool:
		return !val
	default:
		return false
	}
}

func (c *Client) buildInsertQuery(table string, data map[string]interface{}) (string, []interface{}, error) {
	if _, ok := data[c.defaultVersionField]; !ok {
		data[c.defaultVersionField] = 1
	}

	if len(data) == 0 {
		return "", nil, nil
	}

	quotedTable, err := quoteIdentifier(table)
	if err != nil {
		return "", nil, err
	}

	cols := make([]string, 0, len(data))
	for k := range data {
		cols = append(cols, k)
	}
	sort.Strings(cols)

	placeholders := make([]string, len(cols))
	args := make([]interface{}, len(cols))
	quotedCols := make([]string, len(cols))
	for i, col := range cols {
		quotedCol, err := quoteIdentifier(col)
		if err != nil {
			return "", nil, err
		}
		placeholders[i] = "?"
		quotedCols[i] = quotedCol
		args[i] = data[col]
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quotedTable, strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "))
	return query, args, nil
}

func (c *Client) buildUpdateQuery(table, idColumn string, id interface{}, data map[string]interface{}) (string, []interface{}, error) {
	hasVersion := false
	var oldVersion interface{}
	if v, ok := data[c.defaultVersionField]; ok {
		hasVersion = true
		oldVersion = v
		delete(data, c.defaultVersionField)
	}

	if len(data) == 0 {
		return "", nil, nil
	}

	quotedTable, err := quoteIdentifier(table)
	if err != nil {
		return "", nil, err
	}
	quotedIDColumn, err := quoteIdentifier(idColumn)
	if err != nil {
		return "", nil, err
	}
	quotedVersionField, err := quoteIdentifier(c.defaultVersionField)
	if err != nil {
		return "", nil, err
	}

	cols := make([]string, 0, len(data))
	for k := range data {
		if k == idColumn {
			continue
		}
		cols = append(cols, k)
	}
	sort.Strings(cols)

	setStmts := make([]string, 0, len(cols))
	args := make([]interface{}, 0, len(cols)+1)
	for _, col := range cols {
		quotedCol, err := quoteIdentifier(col)
		if err != nil {
			return "", nil, err
		}
		if col == c.defaultVersionField {
			setStmts = append(setStmts, quotedVersionField+" = "+quotedVersionField+" + 1")
		} else {
			setStmts = append(setStmts, fmt.Sprintf("%s = ?", quotedCol))
			args = append(args, data[col])
		}
	}
	args = append(args, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quotedTable, strings.Join(setStmts, ", "), quotedIDColumn)
	if hasVersion {
		query += " AND " + quotedVersionField + " = ?"
		args = append(args, oldVersion)
	}
	return query, args, nil
}

// Begin starts a new transaction.
func (c *Client) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if err := c.ensureDB(); err != nil {
		return nil, err
	}
	return c.db.BeginTx(ctx, opts)
}

// Transaction executes a function within a transaction.
func (c *Client) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.Begin(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Prepare creates a prepared statement.
func (c *Client) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	if err := c.ensureDB(); err != nil {
		return nil, err
	}
	return c.db.PrepareContext(ctx, query)
}
