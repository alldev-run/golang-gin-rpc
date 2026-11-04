package postgres

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// newSQLMockClient builds a Client on top of a sqlmock DB, mirroring the
// struct-injection pattern used by the mysql client tests.
func newSQLMockClient(t *testing.T) (*Client, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Client{db: db}, mock, db
}

func TestClient_Query(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM users WHERE id = $1")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice"))

	rows, err := client.Query(t.Context(), "SELECT id, name FROM users WHERE id = $1", 1)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if id != 1 || name != "alice" {
		t.Errorf("row = (%d, %s), want (1, alice)", id, name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet SQL expectations: %v", err)
	}
}

func TestClient_QueryError(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectQuery("SELECT").WillReturnError(errors.New("relation does not exist"))

	if _, err := client.Query(t.Context(), "SELECT 1"); err == nil {
		t.Error("Query() error = nil, want query failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet SQL expectations: %v", err)
	}
}

func TestClient_QueryRow(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectQuery("SELECT count").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(7),
	)

	var count int
	if err := client.QueryRow(t.Context(), "SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if count != 7 {
		t.Errorf("count = %d, want 7", count)
	}
}

func TestClient_Exec(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name = $1 WHERE id = $2")).
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := client.Exec(t.Context(), "UPDATE users SET name = $1 WHERE id = $2", "bob", 1)
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

func TestClient_ExecError(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectExec("INSERT").WillReturnError(errors.New("duplicate key"))

	if _, err := client.Exec(t.Context(), "INSERT INTO users VALUES (1)"); err == nil {
		t.Error("Exec() error = nil, want exec failure")
	}
}

func TestClient_BeginAndCommit(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := client.Begin(t.Context(), nil)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec("INSERT INTO users VALUES ($1)", 1); err != nil {
		t.Fatalf("tx.Exec() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet SQL expectations: %v", err)
	}
}

func TestClient_Transaction_Commit(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ran := false
	err := client.Transaction(t.Context(), func(tx *sql.Tx) error {
		ran = true
		_, err := tx.Exec("INSERT INTO users VALUES ($1)", 1)
		return err
	})
	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}
	if !ran {
		t.Error("fn did not run")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet SQL expectations: %v", err)
	}
}

func TestClient_Transaction_RollbackOnError(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	origErr := errors.New("constraint violated")
	err := client.Transaction(t.Context(), func(tx *sql.Tx) error {
		return origErr
	})
	if !errors.Is(err, origErr) {
		t.Errorf("Transaction() error = %v, want original fn error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet SQL expectations: %v", err)
	}
}

func TestClient_Transaction_RollbackOnPanic(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Error("expected panic to propagate after rollback")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet SQL expectations: %v", err)
		}
	}()

	_ = client.Transaction(t.Context(), func(tx *sql.Tx) error {
		panic("boom inside transaction")
	})
}

func TestClient_Transaction_BeginError(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectBegin().WillReturnError(errors.New("pool exhausted"))

	if err := client.Transaction(t.Context(), func(tx *sql.Tx) error { return nil }); err == nil {
		t.Error("Transaction() error = nil, want begin failure")
	}
}

func TestClient_Prepare(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectPrepare(regexp.QuoteMeta("SELECT name FROM users WHERE id = $1"))

	stmt, err := client.Prepare(t.Context(), "SELECT name FROM users WHERE id = $1")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := stmt.Close(); err != nil {
		t.Errorf("stmt.Close() error = %v", err)
	}
}

func TestClient_Ping(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectPing()

	if err := client.Ping(t.Context()); err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}
}

func TestClient_PingError(t *testing.T) {
	client, mock, _ := newSQLMockClient(t)

	mock.ExpectPing().WillReturnError(errors.New("server gone away"))

	if err := client.Ping(t.Context()); err == nil {
		t.Error("Ping() error = nil, want ping failure")
	}
}

func TestClient_StatsAndDB(t *testing.T) {
	client, _, db := newSQLMockClient(t)

	if got := client.DB(); got != db {
		t.Error("DB() should return the underlying *sql.DB")
	}
	// Stats must not panic and reflect the sqlmock DB.
	_ = client.Stats()
}

func TestClient_Close(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	client := &Client{db: db}

	mock.ExpectClose()
	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestClient_SetSQLLogger(t *testing.T) {
	t.Run("replaces logger when sqlLogger exists", func(t *testing.T) {
		client, _, _ := newSQLMockClient(t)
		client.sqlLogger = NewSQLLogger("info", time.Second)

		client.SetSQLLogger(zap.NewNop())
		if client.sqlLogger.logger == nil {
			t.Error("SetSQLLogger should replace the internal zap logger")
		}
	})

	t.Run("no-op without sqlLogger", func(t *testing.T) {
		client, _, _ := newSQLMockClient(t)
		client.SetSQLLogger(zap.NewNop()) // must not panic
	})
}

// ==================== SQL query logging integration ====================

func TestClient_LoggingPaths(t *testing.T) {
	// Negative threshold makes every call take the slow-query branch
	// deterministically, regardless of coarse clock granularity.
	client, mock, _ := newSQLMockClient(t)
	client.sqlLogger = NewSQLLogger("debug", -1*time.Millisecond)

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
	if _, err := client.Query(t.Context(), "SELECT 1"); err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
	_ = client.QueryRow(t.Context(), "SELECT 1")

	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	if _, err := client.Exec(t.Context(), "INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}
}

func TestSQLLogger_NilInternalLoggerFallback(t *testing.T) {
	// A SQLLogger built without an internal zap logger falls back to the
	// package-level logger; all branches must be panic-free.
	threshold := time.Second
	for _, level := range []LogLevel{LogLevelError, LogLevelWarn, LogLevelInfo, LogLevelDebug, LogLevelTrace, LogLevel(99)} {
		logger := &SQLLogger{level: level, slowQueryThreshold: threshold}

		logger.LogQuery("SELECT 1", nil, time.Millisecond, nil)
		logger.LogQuery("SELECT 1", []any{1}, time.Millisecond, errors.New("boom"))
	}
	// Slow-query branch: negative threshold always triggers the warn path.
	slow := &SQLLogger{level: LogLevelInfo, slowQueryThreshold: -time.Millisecond}
	slow.LogQuery("SELECT 1", nil, time.Millisecond, nil)
}

func TestSQLLogger_SetLogger(t *testing.T) {
	logger := NewSQLLogger("info", time.Second)
	logger.SetLogger(zap.NewNop())
	if logger.logger == nil {
		t.Error("SetLogger should install the custom logger")
	}
	logger.LogQuery("SELECT 1", nil, time.Millisecond, nil)
}
