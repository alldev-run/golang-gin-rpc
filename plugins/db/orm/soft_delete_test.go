package orm

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"
)

// ==================== NewSoftDelete ====================

func TestNewSoftDelete(t *testing.T) {
	sd := NewSoftDelete()
	if sd.DeletedAtColumn != "deleted_at" {
		t.Errorf("DeletedAtColumn = %q, want deleted_at", sd.DeletedAtColumn)
	}
	if sd.RetentionPeriod != 30*24*time.Hour {
		t.Errorf("RetentionPeriod = %v, want 720h", sd.RetentionPeriod)
	}
}

// ==================== FindNotDeleted / FindSoftDeleted ====================

func TestSoftDelete_FindQueries(t *testing.T) {
	sd := NewSoftDelete()

	if got := sd.FindNotDeleted(); got != "(deleted_at IS NULL)" {
		t.Errorf("FindNotDeleted() = %q", got)
	}
	if got := sd.FindSoftDeleted(); got != "(deleted_at IS NOT NULL)" {
		t.Errorf("FindSoftDeleted() = %q", got)
	}
	if got := sd.FindNotDeletedOr(); got != "deleted_at IS NULL" {
		t.Errorf("FindNotDeletedOr() = %q", got)
	}
	if got := sd.FindSoftDeletedOr(); got != "deleted_at IS NOT NULL" {
		t.Errorf("FindSoftDeletedOr() = %q", got)
	}
}

func TestSoftDelete_CustomColumn(t *testing.T) {
	sd := &SoftDelete{DeletedAtColumn: "removed_at"}
	if got := sd.FindNotDeleted(); got != "(removed_at IS NULL)" {
		t.Errorf("FindNotDeleted() = %q", got)
	}
}

// ==================== SoftDelete with MockDB ====================

func TestSoftDelete_SoftDelete(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 1}, nil
		},
	}
	sd := NewSoftDelete()
	result, err := sd.SoftDelete(context.Background(), mock, "users", "id", 1, 2)
	if err != nil {
		t.Fatalf("SoftDelete error: %v", err)
	}
	if result.RowsAffected != 1 {
		t.Errorf("RowsAffected = %d, want 1", result.RowsAffected)
	}
	if result.DeletedAt.IsZero() {
		t.Error("DeletedAt should not be zero")
	}
}

func TestSoftDelete_SoftDeleteNoIDs(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.SoftDelete(context.Background(), &MockDB{}, "users", "id")
	if err == nil {
		t.Error("expected error with no IDs")
	}
}

func TestSoftDelete_SoftDeleteInvalidTable(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.SoftDelete(context.Background(), &MockDB{}, "", "id", 1)
	if err == nil {
		t.Error("expected error with empty table")
	}
}

func TestSoftDelete_SoftDeleteInvalidColumn(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.SoftDelete(context.Background(), &MockDB{}, "users", "1bad", 1)
	if err == nil {
		t.Error("expected error with invalid id column")
	}
}

func TestSoftDelete_SoftDeleteInvalidDeletedAtColumn(t *testing.T) {
	sd := &SoftDelete{DeletedAtColumn: "1bad", RetentionPeriod: time.Hour}
	_, err := sd.SoftDelete(context.Background(), &MockDB{}, "users", "id", 1)
	if err == nil {
		t.Error("expected error with invalid deleted_at column")
	}
}

func TestSoftDelete_SoftDeleteExecError(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, sql.ErrNoRows
		},
	}
	sd := NewSoftDelete()
	_, err := sd.SoftDelete(context.Background(), mock, "users", "id", 1)
	if err == nil {
		t.Error("expected error from exec")
	}
}

// ==================== Restore ====================

func TestSoftDelete_Restore(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 2}, nil
		},
	}
	sd := NewSoftDelete()
	result, err := sd.Restore(context.Background(), mock, "users", "id", 1, 2)
	if err != nil {
		t.Fatalf("Restore error: %v", err)
	}
	if result.RowsAffected != 2 {
		t.Errorf("RowsAffected = %d, want 2", result.RowsAffected)
	}
}

func TestSoftDelete_RestoreNoIDs(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.Restore(context.Background(), &MockDB{}, "users", "id")
	if err == nil {
		t.Error("expected error with no IDs")
	}
}

func TestSoftDelete_RestoreInvalidTable(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.Restore(context.Background(), &MockDB{}, "", "id", 1)
	if err == nil {
		t.Error("expected error with empty table")
	}
}

func TestSoftDelete_RestoreInvalidColumn(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.Restore(context.Background(), &MockDB{}, "users", "1bad", 1)
	if err == nil {
		t.Error("expected error with invalid id column")
	}
}

// ==================== CleanSoftDeleted ====================

func TestSoftDelete_CleanSoftDeleted(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{affected: 5}, nil
		},
	}
	sd := NewSoftDelete()
	result, err := sd.CleanSoftDeleted(context.Background(), mock, "users")
	if err != nil {
		t.Fatalf("CleanSoftDeleted error: %v", err)
	}
	if result.RowsAffected != 5 {
		t.Errorf("RowsAffected = %d, want 5", result.RowsAffected)
	}
}

func TestSoftDelete_CleanSoftDeletedInvalidTable(t *testing.T) {
	sd := NewSoftDelete()
	_, err := sd.CleanSoftDeleted(context.Background(), &MockDB{}, "")
	if err == nil {
		t.Error("expected error with empty table")
	}
}

// ==================== buildSoftDeleteQuery / buildRestoreQuery ====================

func TestSoftDelete_BuildQueries(t *testing.T) {
	sd := NewSoftDelete()
	dialect := NewDefaultDialect()
	now := time.Now()

	q, args := sd.buildSoftDeleteQuery("users", "id", []interface{}{1, 2}, now, dialect)
	if q == "" {
		t.Error("buildSoftDeleteQuery returned empty query")
	}
	if len(args) != 3 { // now + 2 ids
		t.Errorf("args count = %d, want 3", len(args))
	}

	q2, args2 := sd.buildRestoreQuery("users", "id", []interface{}{1, 2}, dialect)
	if q2 == "" {
		t.Error("buildRestoreQuery returned empty query")
	}
	if len(args2) != 2 {
		t.Errorf("restore args count = %d, want 2", len(args2))
	}
}

// ==================== mockResult ====================

type mockResult struct {
	affected int64
	id       int64
}

func (r *mockResult) LastInsertId() (int64, error) { return r.id, nil }
func (r *mockResult) RowsAffected() (int64, error) { return r.affected, nil }

// Ensure mockResult satisfies driver.Result
var _ driver.Result = (*mockResult)(nil)
