package orm

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// ==================== LogLevel ====================

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelError, "ERROR"},
		{LogLevelWarn, "WARN"},
		{LogLevelInfo, "INFO"},
		{LogLevelDebug, "DEBUG"},
		{LogLevelTrace, "TRACE"},
		{LogLevel(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// ==================== NewSQLLogger ====================

func TestNewSQLLogger(t *testing.T) {
	sl := NewSQLLogger()
	if sl.level != LogLevelInfo {
		t.Errorf("level = %v, want Info", sl.level)
	}
	if sl.slowQueryThreshold != 100*time.Millisecond {
		t.Errorf("threshold = %v, want 100ms", sl.slowQueryThreshold)
	}
}

func TestNewSQLLoggerWithConfig(t *testing.T) {
	sl := NewSQLLoggerWithConfig(LogLevelDebug, 50*time.Millisecond)
	if sl.level != LogLevelDebug {
		t.Errorf("level = %v, want Debug", sl.level)
	}
	if sl.slowQueryThreshold != 50*time.Millisecond {
		t.Errorf("threshold = %v, want 50ms", sl.slowQueryThreshold)
	}
}

func TestSQLLogger_SetLevel(t *testing.T) {
	sl := NewSQLLogger()
	sl.SetLevel(LogLevelDebug)
	if sl.level != LogLevelDebug {
		t.Errorf("level = %v, want Debug", sl.level)
	}
}

func TestSQLLogger_SetSlowQueryThreshold(t *testing.T) {
	sl := NewSQLLogger()
	sl.SetSlowQueryThreshold(200 * time.Millisecond)
	if sl.slowQueryThreshold != 200*time.Millisecond {
		t.Errorf("threshold = %v, want 200ms", sl.slowQueryThreshold)
	}
}

// ==================== LogQuery ====================

func TestSQLLogger_LogQuery(t *testing.T) {
	// Just ensure no panics for various log levels
	levels := []LogLevel{LogLevelError, LogLevelWarn, LogLevelInfo, LogLevelDebug, LogLevelTrace}
	for _, level := range levels {
		sl := NewSQLLoggerWithConfig(level, 10*time.Millisecond)

		// Normal query
		sl.LogQuery(&QueryLog{
			Query:     "SELECT 1",
			Args:      []interface{}{1},
			Duration:  5 * time.Millisecond,
			Timestamp: time.Now(),
		})

		// Slow query
		sl.LogQuery(&QueryLog{
			Query:     "SELECT SLEEP(1)",
			Duration:  1 * time.Second,
			Timestamp: time.Now(),
		})

		// Error query
		sl.LogQuery(&QueryLog{
			Query:     "INVALID SQL",
			Error:     sql.ErrNoRows,
			Duration:  time.Millisecond,
			Timestamp: time.Now(),
		})

		// No args
		sl.LogQuery(&QueryLog{
			Query:     "SELECT 1",
			Duration:  time.Millisecond,
			Timestamp: time.Now(),
		})
	}
}

func TestSQLLogger_LogQueryBelowInfo(t *testing.T) {
	sl := NewSQLLoggerWithConfig(LogLevelError, time.Hour)
	// Should not log anything (level < Info)
	sl.LogQuery(&QueryLog{
		Query:     "SELECT 1",
		Duration:  time.Millisecond,
		Timestamp: time.Now(),
	})
}

// ==================== LogTransaction ====================

func TestSQLLogger_LogTransaction(t *testing.T) {
	sl := NewSQLLogger()

	// Success
	sl.LogTransaction("begin", time.Millisecond, nil)

	// Error
	sl.LogTransaction("commit", time.Millisecond, sql.ErrNoRows)
}

func TestSQLLogger_LogTransactionBelowInfo(t *testing.T) {
	sl := NewSQLLoggerWithConfig(LogLevelError, time.Hour)
	// Should not log (level < Info)
	sl.LogTransaction("begin", time.Millisecond, nil)
}

// ==================== LoggedDB ====================

func TestLoggedDB_Exec(t *testing.T) {
	execCalled := false
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			execCalled = true
			return &mockResult{affected: 1}, nil
		},
	}
	sl := NewSQLLogger()
	ldb := NewLoggedDB(mock, sl)

	res, err := ldb.Exec(context.Background(), "INSERT INTO t VALUES (?)", 1)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if !execCalled {
		t.Error("underlying Exec should have been called")
	}
	if res == nil {
		t.Error("result should not be nil")
	}
}

func TestLoggedDB_Query(t *testing.T) {
	queryCalled := false
	mock := &MockDB{
		QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
			queryCalled = true
			return nil, nil
		},
	}
	sl := NewSQLLogger()
	ldb := NewLoggedDB(mock, sl)

	_, err := ldb.Query(context.Background(), "SELECT * FROM t")
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if !queryCalled {
		t.Error("underlying Query should have been called")
	}
}

func TestLoggedDB_QueryRow(t *testing.T) {
	mock := &MockDB{
		QueryRowFunc: func(ctx context.Context, query string, args ...interface{}) *sql.Row {
			return nil
		},
	}
	// Test with trace level
	sl := NewSQLLoggerWithConfig(LogLevelTrace, time.Hour)
	ldb := NewLoggedDB(mock, sl)
	row := ldb.QueryRow(context.Background(), "SELECT 1", 42)
	_ = row
}

func TestLoggedDB_Begin(t *testing.T) {
	mock := &MockDB{
		BeginFunc: func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
			return nil, nil
		},
	}
	sl := NewSQLLogger()
	ldb := NewLoggedDB(mock, sl)
	_, err := ldb.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin error: %v", err)
	}
}

func TestLoggedDB_Ping(t *testing.T) {
	mock := &MockDB{
		PingFunc: func(ctx context.Context) error {
			return nil
		},
	}
	sl := NewSQLLogger()
	ldb := NewLoggedDB(mock, sl)
	err := ldb.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestLoggedDB_Close(t *testing.T) {
	closed := false
	mock := &MockDB{
		CloseFunc: func() error {
			closed = true
			return nil
		},
	}
	sl := NewSQLLogger()
	ldb := NewLoggedDB(mock, sl)
	err := ldb.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if !closed {
		t.Error("underlying Close should have been called")
	}
}

func TestLoggedDB_Stats(t *testing.T) {
	mock := &MockDB{
		StatsFunc: func() sql.DBStats {
			return sql.DBStats{OpenConnections: 3}
		},
	}
	sl := NewSQLLogger()
	ldb := NewLoggedDB(mock, sl)
	stats := ldb.Stats()
	if stats.OpenConnections != 3 {
		t.Errorf("OpenConnections = %d, want 3", stats.OpenConnections)
	}
}

func TestLoggedDB_SetGetLogger(t *testing.T) {
	mock := &MockDB{}
	sl1 := NewSQLLogger()
	sl2 := NewSQLLoggerWithConfig(LogLevelDebug, time.Second)
	ldb := NewLoggedDB(mock, sl1)

	if ldb.GetLogger() != sl1 {
		t.Error("GetLogger should return initial logger")
	}
	ldb.SetLogger(sl2)
	if ldb.GetLogger() != sl2 {
		t.Error("GetLogger should return updated logger")
	}
}

func TestNewLoggedDB_NilLogger(t *testing.T) {
	mock := &MockDB{}
	ldb := NewLoggedDB(mock, nil)
	if ldb.logger == nil {
		t.Error("nil logger should be replaced with default")
	}
}
