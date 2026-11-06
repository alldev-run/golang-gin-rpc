package poolrw

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/rwproxy"
)

// fakeSQLClient adapts a *sql.DB to db.SQLClient so a fake driver can feed
// the whole read-write pool from a sqlmock database.
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

const poolrwFakeDriverType = db.Type("poolrw-sqlmock")

var poolrwFakeDriverOnce sync.Once

// newPoolWithSQLMock builds a Pool whose master and replica are fake driver
// clients over a permissive sqlmock DB (master and replica configs are equal,
// so the factory caches one shared client per pool). The permissive DB
// pre-registers every SQL shape the Pool-method tests use, because sqlmock
// requires an expectation for each driver call.
func newPoolWithSQLMock(t *testing.T) *Pool {
	t.Helper()
	poolrwFakeDriverOnce.Do(func() {
		db.RegisterDriver(poolrwFakeDriverType, func(cfg db.Config) (db.Client, error) {
			return &fakeSQLClient{db: newPermissiveMockDB(t)}, nil
		})
	})

	pool, err := New(RWPoolConfig{
		MasterConfig:   db.Config{Type: poolrwFakeDriverType},
		ReplicaConfigs: []db.Config{{Type: poolrwFakeDriverType}},
		Strategy:       rwproxy.LBStrategyRoundRobin,
		PoolConfig:     PoolSettings{HealthCheckPeriod: time.Hour}, // keep the background checker idle
	}, db.NewFactory())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return pool
}

// newPermissiveMockDB returns a sqlmock DB pre-loaded with expectations for
// all call shapes used below. MatchExpectationsInOrder(false) lets each call
// match whichever remaining expectation fits.
func newPermissiveMockDB(t *testing.T) *sql.DB {
	t.Helper()
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	mock.MatchExpectationsInOrder(false)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectBegin()
	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectCommit()
	mock.ExpectRollback()
	mock.ExpectPrepare("SELECT")
	return raw
}

func TestPool_QueryOnReplica(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	rows, err := pool.Query(t.Context(), "SELECT id FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if !rows.Next() {
		t.Fatal("expected a row")
	}
	rows.Close()
}

func TestPool_QueryRowOnReplica(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	var id int
	if err := pool.QueryRow(t.Context(), "SELECT id FROM users").Scan(&id); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if id != 1 && id != 2 {
		t.Errorf("id = %d, want 1 or 2", id)
	}
}

func TestPool_ExecOnMaster(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	result, err := pool.Exec(t.Context(), "UPDATE users SET name = 'x'")
	if err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected() error = %v", err)
	}
	if affected != 1 {
		t.Errorf("rows affected = %d, want 1", affected)
	}
}

func TestPool_BeginAndTransaction(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	tx, err := pool.Begin(t.Context(), nil)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec("UPDATE users SET name = 'tx'"); err != nil {
		t.Fatalf("tx.Exec() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	err = pool.Transaction(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE users SET name = 'tx2'")
		return err
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}
}

func TestPool_TransactionRollback(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	origErr := errors.New("rollback me")
	if err := pool.Transaction(t.Context(), func(tx *sql.Tx) error {
		return origErr
	}); !errors.Is(err, origErr) {
		t.Errorf("Transaction() error = %v, want original fn error", err)
	}
}

func TestPool_Prepare(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	stmt, err := pool.Prepare(t.Context(), "SELECT id FROM users WHERE name = ?")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := stmt.Close(); err != nil {
		t.Errorf("stmt.Close() error = %v", err)
	}
}

func TestPool_ForceMasterAndStats(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	if pool.IsMasterForced() {
		t.Error("IsMasterForced() = true initially, want false")
	}

	pool.ForceMaster(true)
	if !pool.IsMasterForced() {
		t.Error("IsMasterForced() = false after ForceMaster(true)")
	}

	stats := pool.GetStats()
	if !stats.ForceMaster {
		t.Error("GetStats().ForceMaster = false after ForceMaster(true)")
	}
	if !stats.MasterConnected {
		t.Error("GetStats().MasterConnected = false, want true")
	}
	if stats.ReplicaCount < 1 {
		t.Errorf("GetStats().ReplicaCount = %d, want >= 1", stats.ReplicaCount)
	}

	pool.ForceMaster(false)
	if pool.IsMasterForced() {
		t.Error("IsMasterForced() = true after ForceMaster(false)")
	}
}

func TestPool_PingAll(t *testing.T) {
	pool := newPoolWithSQLMock(t)
	defer func() { _ = pool.Close() }()

	if err := pool.Ping(t.Context()); err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestPool_NilRWClientErrors(t *testing.T) {
	// Existing coverage file tests nil-client errors; this complements it with
	// Transaction, which previously had no coverage at all.
	pool := &Pool{}
	if err := pool.Transaction(t.Context(), func(tx *sql.Tx) error { return nil }); err == nil {
		t.Error("Transaction() on nil-rwClient pool should fail")
	}
}
