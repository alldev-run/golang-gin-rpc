package orm

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestUpdateWithVersion_UsesExpressionIncrement(t *testing.T) {
	var capturedQuery string
	var capturedArgs []interface{}

	mockDB := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			capturedQuery = query
			capturedArgs = append([]interface{}(nil), args...)
			return &MockResult{rowsAffected: 1}, nil
		},
	}

	ub := NewUpdateBuilder(mockDB, "users")
	ub.Set("name", "alice")

	affected, err := ub.UpdateWithVersion(context.Background(), 42, "id", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected rows affected 1, got %d", affected)
	}

	if !strings.Contains(capturedQuery, "`version` = `version` + ?") {
		t.Fatalf("expected version increment expression in SET clause, got query: %s", capturedQuery)
	}

	expectedArgs := []interface{}{"alice", 1, 7, 42}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Fatalf("unexpected args: got=%v want=%v", capturedArgs, expectedArgs)
	}
}
