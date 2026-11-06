package orm

import (
	"testing"
)

// ==================== validateBatchData ====================

func TestValidateBatchData(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := validateBatchData(nil)
		if err != nil {
			t.Errorf("unexpected error for nil data: %v", err)
		}
		err = validateBatchData([]map[string]interface{}{})
		if err != nil {
			t.Errorf("unexpected error for empty data: %v", err)
		}
	})
	t.Run("valid", func(t *testing.T) {
		data := []map[string]interface{}{
			{"name": "alice", "age": 30},
			{"name": "bob", "age": 25},
		}
		err := validateBatchData(data)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("inconsistent_field_count", func(t *testing.T) {
		data := []map[string]interface{}{
			{"name": "alice", "age": 30},
			{"name": "bob"},
		}
		err := validateBatchData(data)
		if err == nil {
			t.Error("expected error for inconsistent field count")
		}
	})
	t.Run("missing_field", func(t *testing.T) {
		data := []map[string]interface{}{
			{"name": "alice", "age": 30},
			{"name": "bob", "email": "bob@example.com"},
		}
		err := validateBatchData(data)
		if err == nil {
			t.Error("expected error for missing field")
		}
	})
	t.Run("invalid_column_name", func(t *testing.T) {
		data := []map[string]interface{}{
			{"1bad": "alice"},
		}
		err := validateBatchData(data)
		if err == nil {
			t.Error("expected error for invalid column name")
		}
	})
	t.Run("invalid_data_type", func(t *testing.T) {
		data := []map[string]interface{}{
			{"name": struct{}{}},
		}
		err := validateBatchData(data)
		if err == nil {
			t.Error("expected error for invalid data type")
		}
	})
}

// ==================== getColumnsFromBatch ====================

func TestGetColumnsFromBatch(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		cols := getColumnsFromBatch(nil)
		if cols != nil {
			t.Errorf("expected nil for empty batch, got %v", cols)
		}
	})
	t.Run("with_data", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"a": 1, "b": 2, "c": 3},
		}
		cols := getColumnsFromBatch(batch)
		if len(cols) != 3 {
			t.Errorf("expected 3 columns, got %d", len(cols))
		}
	})
}

// ==================== buildBatchInsertQuery ====================

func TestBuildBatchInsertQuery(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("empty_batch", func(t *testing.T) {
		q, args, err := buildBatchInsertQuery("users", nil, nil, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q != "" {
			t.Errorf("expected empty query for nil batch, got %q", q)
		}
		if args != nil {
			t.Errorf("expected nil args for nil batch, got %v", args)
		}
	})
	t.Run("single_row", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"name": "alice", "age": 30},
		}
		columns := []string{"name", "age"}
		q, args, err := buildBatchInsertQuery("users", batch, columns, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q == "" {
			t.Error("query should not be empty")
		}
		if len(args) != 2 {
			t.Errorf("args count = %d, want 2", len(args))
		}
	})
	t.Run("multiple_rows", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"name": "alice", "age": 30},
			{"name": "bob", "age": 25},
		}
		columns := []string{"name", "age"}
		q, args, err := buildBatchInsertQuery("users", batch, columns, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q == "" {
			t.Error("query should not be empty")
		}
		if len(args) != 4 {
			t.Errorf("args count = %d, want 4", len(args))
		}
	})
}

// ==================== buildBatchInsertQueryWithReturning ====================

func TestBuildBatchInsertQueryWithReturning(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("with_columns", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"name": "alice"},
		}
		q, args, err := buildBatchInsertQueryWithReturning("users", batch, []string{"id", "name"}, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q == "" {
			t.Error("query should not be empty")
		}
		if len(args) != 1 {
			t.Errorf("args count = %d, want 1", len(args))
		}
	})
	t.Run("without_columns", func(t *testing.T) {
		batch := []map[string]interface{}{
			{"name": "alice"},
		}
		q, _, err := buildBatchInsertQueryWithReturning("users", batch, nil, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q == "" {
			t.Error("query should not be empty")
		}
	})
}
