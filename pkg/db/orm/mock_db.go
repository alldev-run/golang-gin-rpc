package orm

import (
	"context"
	"database/sql"
)

// MockDB implements the DB interface for testing and example purposes.
type MockDB struct {
	QueryFunc    func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowFunc func(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecFunc     func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	BeginFunc    func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	PingFunc     func(ctx context.Context) error
	StatsFunc    func() sql.DBStats
	CloseFunc    func() error
}

// Query calls the mock QueryFunc.
func (m *MockDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return nil, nil
}

// QueryRow calls the mock QueryRowFunc.
func (m *MockDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, query, args...)
	}
	return nil
}

// Exec calls the mock ExecFunc.
func (m *MockDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, query, args...)
	}
	return nil, nil
}

// Begin calls the mock BeginFunc.
func (m *MockDB) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if m.BeginFunc != nil {
		return m.BeginFunc(ctx, opts)
	}
	return nil, nil
}

// Ping calls the mock PingFunc.
func (m *MockDB) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// Stats calls the mock StatsFunc.
func (m *MockDB) Stats() sql.DBStats {
	if m.StatsFunc != nil {
		return m.StatsFunc()
	}
	return sql.DBStats{}
}

// Close calls the mock CloseFunc.
func (m *MockDB) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
