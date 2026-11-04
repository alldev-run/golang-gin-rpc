package poolcb

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/pool"
)

// fakeSQLClient adapts a *sql.DB to db.SQLClient so the breaker-wrapped SQL
// pool can run against a sqlmock database.
type fakeSQLClient struct {
	db *sql.DB
}

func (c *fakeSQLClient) DB() *sql.DB                    { return c.db }
func (c *fakeSQLClient) Ping(ctx context.Context) error { return c.db.PingContext(ctx) }
func (c *fakeSQLClient) Close() error                   { return c.db.Close() }
func (c *fakeSQLClient) Query(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, q, args...)
}
func (c *fakeSQLClient) QueryRow(ctx context.Context, q string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, q, args...)
}
func (c *fakeSQLClient) Exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, q, args...)
}
func (c *fakeSQLClient) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, opts)
}
func (c *fakeSQLClient) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.db.BeginTx(ctx, nil)
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

const poolcbFakeDriverType = db.Type("poolcb-sqlmock")

var (
	poolcbFakeDriverOnce sync.Once
	// poolcbMockSink receives the sqlmock handle of each client the fake
	// driver creates; newSQLMockPool swaps it per test so expectations land on
	// the mock actually backing the pool.
	poolcbMockSink func(sqlmock.Sqlmock)
)

// newSQLMockPool registers the fake driver and returns a SQL pool whose
// "master" connection is backed by a sqlmock DB prepared by the caller.
func newSQLMockPool(t *testing.T, expect func(mock sqlmock.Sqlmock)) *SQLPoolWithBreaker {
	t.Helper()
	poolcbFakeDriverOnce.Do(func() {
		db.RegisterDriver(poolcbFakeDriverType, func(cfg db.Config) (db.Client, error) {
			raw, mock, err := sqlmock.New()
			if err != nil {
				return nil, err
			}
			if poolcbMockSink != nil {
				poolcbMockSink(mock)
			}
			return &fakeSQLClient{db: raw}, nil
		})
	})

	mockCh := make(chan sqlmock.Sqlmock, 1)
	poolcbMockSink = func(m sqlmock.Sqlmock) { mockCh <- m }
	defer func() { poolcbMockSink = nil }()

	p := NewSQL(Config{
		PoolConfig: pool.Config{
			MaxSize:           2,
			InitialSize:       1,
			HealthCheckPeriod: time.Hour, // keep the background checker idle
		},
	}, db.NewFactory())
	if err := p.Register("master", db.Config{Type: poolcbFakeDriverType}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	expect(<-mockCh)
	return p
}

func TestSQLPoolWithBreaker_QuerySuccess(t *testing.T) {
	p := newSQLMockPool(t, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
	})
	defer func() { _ = p.Close() }()

	rows, err := p.Query(t.Context(), "master", "SELECT 1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var v int
	if err := rows.Scan(&v); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if v != 1 {
		t.Errorf("v = %d, want 1", v)
	}
}

func TestSQLPoolWithBreaker_QueryRowSuccess(t *testing.T) {
	p := newSQLMockPool(t, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(9))
	})
	defer func() { _ = p.Close() }()

	var v int
	if err := p.QueryRow(t.Context(), "master", "SELECT 9").Scan(&v); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if v != 9 {
		t.Errorf("v = %d, want 9", v)
	}
}

func TestSQLPoolWithBreaker_ExecSuccess(t *testing.T) {
	p := newSQLMockPool(t, func(mock sqlmock.Sqlmock) {
		mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 2))
	})
	defer func() { _ = p.Close() }()

	result, err := p.Exec(t.Context(), "master", "UPDATE t SET a = 1")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 2 {
		t.Errorf("rows affected = %d (err %v), want 2", affected, err)
	}
}

func TestSQLPoolWithBreaker_TransactionSuccess(t *testing.T) {
	p := newSQLMockPool(t, func(mock sqlmock.Sqlmock) {
		mock.ExpectBegin()
		mock.ExpectCommit()
	})
	defer func() { _ = p.Close() }()

	ran := false
	if err := p.Transaction(t.Context(), "master", func(tx *sql.Tx) error {
		ran = true
		return nil
	}); err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}
	if !ran {
		t.Error("fn did not run inside the transaction")
	}
}

func TestPoolWithBreaker_AcquirePingClose(t *testing.T) {
	p := newSQLMockPool(t, func(mock sqlmock.Sqlmock) {
		mock.ExpectClose()
	})
	defer func() { _ = p.Close() }()

	client, err := p.Acquire(t.Context(), "master")
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := client.Ping(t.Context()); err != nil {
		t.Errorf("protectedClient.Ping() error = %v, want nil", err)
	}
	if _, ok := client.(*fakeSQLClient); ok {
		t.Error("Acquire should return a breaker-protected client, not the raw fake")
	}
	if err := client.Close(); err != nil {
		t.Errorf("protectedClient.Close() error = %v, want nil", err)
	}
}

func TestPoolWithBreaker_AcquireUnknownConnection(t *testing.T) {
	p := newSQLMockPool(t, func(sqlmock.Sqlmock) {})
	defer func() { _ = p.Close() }()

	if _, err := p.Acquire(t.Context(), "no-such-connection"); err == nil {
		t.Error("Acquire() for an unregistered connection should fail")
	}
}
