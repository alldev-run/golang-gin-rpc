package orm

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// sqlmockNewDB is a test helper that creates a sqlmock database.
func sqlmockNewDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return db, mock
}

// ==================== DBWrapper ====================

func TestDBWrapper_Query(t *testing.T) {
	db, mock := sqlmockNewDB(t)
	defer db.Close()

	w := NewDBWrapper(db)

	mock.ExpectQuery("SELECT 1").WillReturnRows(
		sqlmock.NewRows([]string{"1"}).AddRow(1),
	)

	rows, err := w.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if rows != nil {
		rows.Close()
	}
}

func TestDBWrapper_QueryRow(t *testing.T) {
	db, mock := sqlmockNewDB(t)
	defer db.Close()

	w := NewDBWrapper(db)

	mock.ExpectQuery("SELECT 1").WillReturnRows(
		sqlmock.NewRows([]string{"1"}).AddRow(1),
	)

	row := w.QueryRow(context.Background(), "SELECT 1")
	if row == nil {
		t.Fatal("QueryRow returned nil")
	}
	var val int
	if err := row.Scan(&val); err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if val != 1 {
		t.Errorf("val = %d, want 1", val)
	}
}

func TestDBWrapper_Exec(t *testing.T) {
	db, mock := sqlmockNewDB(t)
	defer db.Close()

	w := NewDBWrapper(db)

	mock.ExpectExec("INSERT INTO t").WillReturnResult(
		sqlmock.NewResult(1, 1),
	)

	res, err := w.Exec(context.Background(), "INSERT INTO t VALUES (1)")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if res == nil {
		t.Fatal("result should not be nil")
	}
}

func TestDBWrapper_Begin(t *testing.T) {
	db, mock := sqlmockNewDB(t)
	defer db.Close()

	w := NewDBWrapper(db)

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := w.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin error: %v", err)
	}
	if tx == nil {
		t.Fatal("tx should not be nil")
	}
	_ = tx.Rollback()
}

func TestDBWrapper_Ping(t *testing.T) {
	db, mock := sqlmockNewDB(t)
	defer db.Close()

	w := NewDBWrapper(db)

	mock.ExpectPing()

	if err := w.Ping(context.Background()); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestDBWrapper_Stats(t *testing.T) {
	db, _ := sqlmockNewDB(t)
	defer db.Close()

	w := NewDBWrapper(db)
	stats := w.Stats()
	_ = stats // Just ensure no panic
}

func TestDBWrapper_Close(t *testing.T) {
	db, mock := sqlmockNewDB(t)

	w := NewDBWrapper(db)

	mock.ExpectClose()

	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

// ==================== TxWrapper ====================

func TestTxWrapper_Begin(t *testing.T) {
	tx := &TxWrapper{}
	_, err := tx.Begin(context.Background(), nil)
	if err != sql.ErrTxDone {
		t.Errorf("Begin error = %v, want sql.ErrTxDone", err)
	}
}

func TestTxWrapper_Ping(t *testing.T) {
	tx := &TxWrapper{}
	if err := tx.Ping(context.Background()); err != nil {
		t.Errorf("Ping error = %v, want nil", err)
	}
}

func TestTxWrapper_Stats(t *testing.T) {
	tx := &TxWrapper{}
	stats := tx.Stats()
	if stats.OpenConnections != 0 {
		t.Errorf("Stats().OpenConnections = %d, want 0", stats.OpenConnections)
	}
}

func TestTxWrapper_Close(t *testing.T) {
	tx := &TxWrapper{}
	if err := tx.Close(); err != nil {
		t.Errorf("Close error = %v, want nil", err)
	}
}

func TestNewTxWrapper(t *testing.T) {
	tx := NewTxWrapper(nil)
	if tx == nil {
		t.Fatal("NewTxWrapper returned nil")
	}
}

// ==================== ConnectionConfig ====================

func TestDefaultConnectionConfig(t *testing.T) {
	cfg := DefaultConnectionConfig()
	if cfg.MaxOpenConns != 25 {
		t.Errorf("MaxOpenConns = %d, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("MaxIdleConns = %d, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("ConnMaxLifetime = %v, want 1h", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 30*time.Minute {
		t.Errorf("ConnMaxIdleTime = %v, want 30m", cfg.ConnMaxIdleTime)
	}
}

func TestConfigureConnection(t *testing.T) {
	db, _ := sqlmockNewDB(t)
	defer db.Close()

	cfg := ConnectionConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    20,
		ConnMaxLifetime: 2 * time.Hour,
		ConnMaxIdleTime: time.Hour,
	}
	ConfigureConnection(db, cfg)

	stats := db.Stats()
	if stats.MaxOpenConnections != 50 {
		t.Errorf("MaxOpenConnections = %d, want 50", stats.MaxOpenConnections)
	}
}

func TestConfigureConnection_ZeroValues(t *testing.T) {
	db, _ := sqlmockNewDB(t)
	defer db.Close()

	// Zero values should not change defaults
	ConfigureConnection(db, ConnectionConfig{})
}
