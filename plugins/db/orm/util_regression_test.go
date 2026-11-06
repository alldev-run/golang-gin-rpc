package orm

import (
	"reflect"
	"testing"
)

func TestBuildWhereClause_NoMutationAndSequentialPlaceholders(t *testing.T) {
	conds := []string{"a = ?", "b = ? OR c = ?"}
	orig := append([]string(nil), conds...)
	args := []interface{}{1, 2, 3}

	where, outArgs := BuildWhereClause(conds, args, NewPostgreSQLDialect())

	if where != "WHERE a = $1 AND b = $2 OR c = $3" {
		t.Fatalf("unexpected where clause: %s", where)
	}
	if !reflect.DeepEqual(conds, orig) {
		t.Fatalf("conditions mutated: before=%v after=%v", orig, conds)
	}
	if !reflect.DeepEqual(outArgs, args) {
		t.Fatalf("args changed: before=%v after=%v", args, outArgs)
	}
}

func TestEscapeLike_EscapesBackslashPercentUnderscore(t *testing.T) {
	in := `a\\b%c_d`
	got := EscapeLike(in)
	want := `a\\\\b\%c\_d`
	if got != want {
		t.Fatalf("unexpected escaped like string: got=%q want=%q", got, want)
	}
}

func TestHasUnsafeSQLBoundary_UnionSelect(t *testing.T) {
	if !hasUnsafeSQLBoundary("name ASC UNION SELECT password FROM users") {
		t.Fatalf("expected UNION SELECT to be treated as unsafe")
	}
}

func TestBuildSelectQueryWithOptions(t *testing.T) {
	query, args, err := BuildSelectQueryWithOptions("users", SelectQueryOptions{
		Columns:         []string{"id", "name"},
		WhereConditions: []string{"status = ?", "age > ?"},
		WhereArgs:       []interface{}{"active", 18},
		OrderByItems:    []string{"created_at DESC"},
		Limit:           10,
		Offset:          0,
		Dialect:         NewMySQLDialect(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if query != "SELECT `id`, `name` FROM `users` WHERE status = ? AND age > ? ORDER BY created_at DESC LIMIT 10" {
		t.Fatalf("unexpected query: %s", query)
	}
	if !reflect.DeepEqual(args, []interface{}{"active", 18}) {
		t.Fatalf("unexpected args: %v", args)
	}
}
