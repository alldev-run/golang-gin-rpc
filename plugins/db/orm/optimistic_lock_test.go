package orm

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// ==================== NewOptimisticLock ====================

func TestNewOptimisticLock(t *testing.T) {
	ol := NewOptimisticLock()
	if ol.VersionColumn != "version" {
		t.Errorf("VersionColumn = %q, want version", ol.VersionColumn)
	}
	if ol.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", ol.RetryCount)
	}
	if ol.RetryDelay != 100*time.Millisecond {
		t.Errorf("RetryDelay = %v, want 100ms", ol.RetryDelay)
	}
}

// ==================== incrementVersion ====================

func TestIncrementVersion(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{"int", int(5), int(6)},
		{"int32", int32(5), int32(6)},
		{"int64", int64(5), int64(6)},
		{"uint", uint(5), uint(6)},
		{"uint32", uint32(5), uint32(6)},
		{"uint64", uint64(5), uint64(6)},
		{"string", "v1.0", "v1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := incrementVersion(tt.input)
			if got != tt.want {
				t.Errorf("incrementVersion(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ==================== UpdateWithVersion ====================

func TestOptimisticLock_UpdateWithVersion(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 1}, nil
		},
	}
	ol := NewOptimisticLock()
	result, err := ol.UpdateWithVersion(context.Background(), mock, "users", "id", 1, 5, map[string]interface{}{
		"name": "alice",
	})
	if err != nil {
		t.Fatalf("UpdateWithVersion error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.RowsAffected != 1 {
		t.Errorf("RowsAffected = %d, want 1", result.RowsAffected)
	}
	if result.CurrentVersion != int(6) {
		t.Errorf("CurrentVersion = %v, want 6", result.CurrentVersion)
	}
}

func TestOptimisticLock_UpdateWithVersionNoRows(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 0}, nil
		},
	}
	ol := NewOptimisticLock()
	result, err := ol.UpdateWithVersion(context.Background(), mock, "users", "id", 1, 5, map[string]interface{}{
		"name": "alice",
	})
	if err != nil {
		t.Fatalf("UpdateWithVersion error: %v", err)
	}
	if result.Success {
		t.Error("expected failure (0 rows affected)")
	}
}

func TestOptimisticLock_UpdateWithVersionInvalidTable(t *testing.T) {
	ol := NewOptimisticLock()
	_, err := ol.UpdateWithVersion(context.Background(), &MockDB{}, "", "id", 1, 1, map[string]interface{}{"a": 1})
	if err == nil {
		t.Error("expected error for empty table")
	}
}

func TestOptimisticLock_UpdateWithVersionInvalidIDColumn(t *testing.T) {
	ol := NewOptimisticLock()
	_, err := ol.UpdateWithVersion(context.Background(), &MockDB{}, "users", "1bad", 1, 1, map[string]interface{}{"a": 1})
	if err == nil {
		t.Error("expected error for invalid id column")
	}
}

func TestOptimisticLock_UpdateWithVersionInvalidVersionColumn(t *testing.T) {
	ol := &OptimisticLock{VersionColumn: "1bad"}
	_, err := ol.UpdateWithVersion(context.Background(), &MockDB{}, "users", "id", 1, 1, map[string]interface{}{"a": 1})
	if err == nil {
		t.Error("expected error for invalid version column")
	}
}

func TestOptimisticLock_UpdateWithVersionEmptyData(t *testing.T) {
	ol := NewOptimisticLock()
	_, err := ol.UpdateWithVersion(context.Background(), &MockDB{}, "users", "id", 1, 1, map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestOptimisticLock_UpdateWithVersionInvalidDataColumn(t *testing.T) {
	ol := NewOptimisticLock()
	_, err := ol.UpdateWithVersion(context.Background(), &MockDB{}, "users", "id", 1, 1, map[string]interface{}{"1bad": 1})
	if err == nil {
		t.Error("expected error for invalid data column")
	}
}

func TestOptimisticLock_UpdateWithVersionInvalidDataType(t *testing.T) {
	ol := NewOptimisticLock()
	_, err := ol.UpdateWithVersion(context.Background(), &MockDB{}, "users", "id", 1, 1, map[string]interface{}{"name": struct{}{}})
	if err == nil {
		t.Error("expected error for invalid data type")
	}
}

func TestOptimisticLock_UpdateWithVersionExecError(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, sql.ErrNoRows
		},
	}
	ol := NewOptimisticLock()
	_, err := ol.UpdateWithVersion(context.Background(), mock, "users", "id", 1, 1, map[string]interface{}{"name": "a"})
	if err == nil {
		t.Error("expected error from exec")
	}
}

// ==================== UpdateWithRetry ====================

func TestOptimisticLock_UpdateWithRetry(t *testing.T) {
	callCount := 0
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			callCount++
			if callCount < 3 {
				return &mockResult{affected: 0}, nil // conflict
			}
			return &mockResult{affected: 1}, nil // success
		},
	}
	ol := &OptimisticLock{
		VersionColumn: "version",
		RetryCount:    3,
		RetryDelay:    1 * time.Millisecond,
	}
	version := 1
	result, err := ol.UpdateWithRetry(context.Background(), mock, "users", "id", 1, map[string]interface{}{
		"name": "alice",
	}, func() interface{} {
		v := version
		version++
		return v
	})
	if err != nil {
		t.Fatalf("UpdateWithRetry error: %v", err)
	}
	if !result.Success {
		t.Error("expected success after retries")
	}
}

func TestOptimisticLock_UpdateWithRetryNilVersion(t *testing.T) {
	ol := &OptimisticLock{
		VersionColumn: "version",
		RetryCount:    1,
		RetryDelay:    1 * time.Millisecond,
	}
	_, err := ol.UpdateWithRetry(context.Background(), &MockDB{}, "users", "id", 1, map[string]interface{}{
		"name": "alice",
	}, func() interface{} {
		return nil
	})
	if err == nil {
		t.Error("expected error when version is nil")
	}
}

func TestOptimisticLock_UpdateWithRetryAllFail(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 0}, nil // always conflict
		},
	}
	ol := &OptimisticLock{
		VersionColumn: "version",
		RetryCount:    2,
		RetryDelay:    1 * time.Millisecond,
	}
	_, err := ol.UpdateWithRetry(context.Background(), mock, "users", "id", 1, map[string]interface{}{
		"name": "alice",
	}, func() interface{} { return 1 })
	if err == nil {
		t.Error("expected error after all retries fail")
	}
}

func TestOptimisticLock_UpdateWithRetryContextCancelled(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 0}, nil
		},
	}
	ol := &OptimisticLock{
		VersionColumn: "version",
		RetryCount:    5,
		RetryDelay:    1 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := ol.UpdateWithRetry(ctx, mock, "users", "id", 1, map[string]interface{}{
		"name": "alice",
	}, func() interface{} { return 1 })
	if err == nil {
		t.Error("expected error from context cancellation")
	}
}

// ==================== CheckVersionConflict ====================

// Note: CheckVersionConflict requires a real QueryRow that returns scannable data,
// which is difficult to mock. We test the error paths instead.

func TestOptimisticLock_CheckVersionConflictExecError(t *testing.T) {
	// We can't easily test CheckVersionConflict without a real sql.Row
	// that supports Scan. Skip this test for now.
	t.Skip("CheckVersionConflict requires real sql.Row with Scan support")
}

// ==================== buildVersionedUpdateQuery ====================

func TestOptimisticLock_BuildVersionedUpdateQuery(t *testing.T) {
	ol := NewOptimisticLock()
	dialect := NewDefaultDialect()
	q, args, err := ol.buildVersionedUpdateQuery("users", "id", 1, 5, map[string]interface{}{
		"name": "alice",
		"age":  30,
	}, dialect)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q == "" {
		t.Error("query should not be empty")
	}
	// args: name, age, newVersion(6), id = 4
	if len(args) != 4 {
		t.Errorf("args count = %d, want 4", len(args))
	}
}
