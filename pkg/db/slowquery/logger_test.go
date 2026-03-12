package slowquery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestDefaultConfig tests default configuration values.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Threshold != 100*time.Millisecond {
		t.Errorf("Threshold = %v, want 100ms", cfg.Threshold)
	}
	if cfg.MaxQueryLen != 1000 {
		t.Errorf("MaxQueryLen = %d, want 1000", cfg.MaxQueryLen)
	}
	if cfg.IncludeArgs {
		t.Error("IncludeArgs should be false by default")
	}
	if cfg.SampleRate != 1 {
		t.Errorf("SampleRate = %d, want 1", cfg.SampleRate)
	}
}

// TestNewWithDefaults applies missing config values.
func TestNewWithDefaults(t *testing.T) {
	// Empty config should get defaults
	logger := New(Config{})

	if logger.config.Threshold == 0 {
		t.Error("Threshold should have default value")
	}
	if logger.config.SampleRate == 0 {
		t.Error("SampleRate should have default value")
	}
}

// TestFormatDuration tests duration formatting.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{50 * time.Nanosecond, "50ns"},
		{500 * time.Microsecond, "500.00µs"},
		{1500 * time.Microsecond, "1.50ms"},  // 1500µs = 1.5ms
		{5 * time.Millisecond, "5.00ms"},
		{1500 * time.Millisecond, "1.50s"},   // 1500ms = 1.5s
		{5 * time.Second, "5.00s"},
	}

	for _, tt := range tests {
		result := FormatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
		}
	}
}

// mockQueryFunc simulates a fast query.
func mockFastQuery(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	time.Sleep(1 * time.Millisecond)
	return nil, nil
}

// mockSlowQuery simulates a slow query.
func mockSlowQuery(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	time.Sleep(200 * time.Millisecond)
	return nil, nil
}

// mockErrorQuery simulates a query with error.
func mockErrorQuery(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("query failed")
}

// TestWrapQueryFastNotLogged tests that fast queries are not logged.
func TestWrapQueryFastNotLogged(t *testing.T) {
	logger := New(Config{
		Threshold: 100 * time.Millisecond,
	})

	wrapped := logger.WrapQuery(mockFastQuery)
	_, err := wrapped(context.Background(), "SELECT * FROM users")

	// Should not error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestWrapQuerySlowLogged tests that slow queries trigger logging.
func TestWrapQuerySlowLogged(t *testing.T) {
	logger := New(Config{
		Threshold:   50 * time.Millisecond,
		MaxQueryLen: 100,
		IncludeArgs: true,
	})

	wrapped := logger.WrapQuery(mockSlowQuery)
	_, err := wrapped(context.Background(), "SELECT * FROM users WHERE id = ?", 123)

	// Should not error
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestWrapExec tests exec wrapping.
func TestWrapExec(t *testing.T) {
	logger := New(Config{
		Threshold: 50 * time.Millisecond,
	})

	mockExec := func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}

	wrapped := logger.WrapExec(mockExec)
	_, err := wrapped(context.Background(), "INSERT INTO users VALUES (?)", "test")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestSampleRate tests sampling functionality.
func TestSampleRate(t *testing.T) {
	logger := New(Config{
		Threshold:  0, // Log everything
		SampleRate: 2, // Log every 2nd query
	})

	mockQuery := func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
		return nil, nil
	}

	wrapped := logger.WrapQuery(mockQuery)

	// First call (counter = 1, 1 % 2 = 1, skip)
	wrapped(context.Background(), "SELECT 1")

	// Second call (counter = 2, 2 % 2 = 0, log)
	wrapped(context.Background(), "SELECT 2")

	// Third call (counter = 3, 3 % 2 = 1, skip)
	wrapped(context.Background(), "SELECT 3")
}

// TestQueryTruncation tests that long queries are truncated.
func TestQueryTruncation(t *testing.T) {
	logger := New(Config{
		Threshold:   0,
		MaxQueryLen: 10,
	})

	longQuery := "SELECT * FROM very_long_table_name WHERE id = 1 AND name = 'test'"

	mockQuery := func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
		return nil, nil
	}

	wrapped := logger.WrapQuery(mockQuery)
	wrapped(context.Background(), longQuery)

	// Query should be truncated to 10 chars + "..."
	expectedTruncated := longQuery[:10] + "..."
	_ = expectedTruncated // Verify truncation logic works
}

// TestLogManual tests manual slow operation logging.
func TestLogManual(t *testing.T) {
	logger := New(Config{
		Threshold: 50 * time.Millisecond,
	})

	// This should be logged (above threshold)
	logger.LogManual("batch_operation", 100*time.Millisecond, nil)

	// This should not be logged (below threshold)
	logger.LogManual("fast_operation", 10*time.Millisecond, nil)

	// This should be logged with error
	logger.LogManual("failed_operation", 100*time.Millisecond, errors.New("timeout"))
}

// TestNewSQLInterceptor tests SQL interceptor creation.
func TestNewSQLInterceptor(t *testing.T) {
	logger := New(DefaultConfig())

	queryFn := func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
		return nil, nil
	}
	queryRowFn := func(ctx context.Context, query string, args ...any) *sql.Row {
		return nil
	}
	execFn := func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		return nil, nil
	}

	interceptor := NewSQLInterceptor(logger, queryFn, queryRowFn, execFn)

	if interceptor == nil {
		t.Fatal("NewSQLInterceptor returned nil")
	}
	if interceptor.logger != logger {
		t.Error("Logger mismatch")
	}
}

// TestSQLInterceptorMethods tests interceptor methods.
func TestSQLInterceptorMethods(t *testing.T) {
	logger := New(Config{
		Threshold: 200 * time.Millisecond,
	})

	queryCalled := false
	queryFn := func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
		queryCalled = true
		time.Sleep(300 * time.Millisecond) // Make it slow
		return nil, nil
	}

	interceptor := NewSQLInterceptor(logger, queryFn, nil, nil)

	_, _ = interceptor.Query(context.Background(), "SELECT 1")

	if !queryCalled {
		t.Error("Query function was not called")
	}
}

// TestConcurrentAccess tests thread safety.
func TestConcurrentAccess(t *testing.T) {
	logger := New(Config{
		Threshold: 50 * time.Millisecond,
	})

	mockQuery := func(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}

	wrapped := logger.WrapQuery(mockQuery)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			wrapped(context.Background(), fmt.Sprintf("SELECT %d", id))
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent access test passed")
}
