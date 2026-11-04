package orm

import (
	"testing"
	"time"
)

// ==================== ToCamelCase ====================

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"hello", "hello"},
		{"hello_world", "helloWorld"},
		{"user_id", "userId"},
		{"created_at", "createdAt"},
		{"a_b_c", "aBC"},
		{"already", "already"},
		{"with___triple", "withTriple"},
		{"trailing_", "trailing"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToCamelCase(tt.input)
			if got != tt.want {
				t.Errorf("ToCamelCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ==================== IsZero ====================

func TestIsZero_AllTypes(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if !IsZero(nil) {
			t.Error("nil should be zero")
		}
	})
	t.Run("bool_false", func(t *testing.T) {
		if !IsZero(false) {
			t.Error("false should be zero")
		}
	})
	t.Run("bool_true", func(t *testing.T) {
		if IsZero(true) {
			t.Error("true should not be zero")
		}
	})
	t.Run("int_zero", func(t *testing.T) {
		if !IsZero(0) {
			t.Error("0 should be zero")
		}
	})
	t.Run("int_nonzero", func(t *testing.T) {
		if IsZero(42) {
			t.Error("42 should not be zero")
		}
	})
	t.Run("int8_zero", func(t *testing.T) {
		var v int8
		if !IsZero(v) {
			t.Error("int8(0) should be zero")
		}
	})
	t.Run("int16_zero", func(t *testing.T) {
		var v int16
		if !IsZero(v) {
			t.Error("int16(0) should be zero")
		}
	})
	t.Run("int32_zero", func(t *testing.T) {
		var v int32
		if !IsZero(v) {
			t.Error("int32(0) should be zero")
		}
	})
	t.Run("int64_zero", func(t *testing.T) {
		var v int64
		if !IsZero(v) {
			t.Error("int64(0) should be zero")
		}
	})
	t.Run("uint_zero", func(t *testing.T) {
		var v uint
		if !IsZero(v) {
			t.Error("uint(0) should be zero")
		}
	})
	t.Run("uint8_zero", func(t *testing.T) {
		var v uint8
		if !IsZero(v) {
			t.Error("uint8(0) should be zero")
		}
	})
	t.Run("uint16_zero", func(t *testing.T) {
		var v uint16
		if !IsZero(v) {
			t.Error("uint16(0) should be zero")
		}
	})
	t.Run("uint32_zero", func(t *testing.T) {
		var v uint32
		if !IsZero(v) {
			t.Error("uint32(0) should be zero")
		}
	})
	t.Run("uint64_zero", func(t *testing.T) {
		var v uint64
		if !IsZero(v) {
			t.Error("uint64(0) should be zero")
		}
	})
	t.Run("float32_zero", func(t *testing.T) {
		var v float32
		if !IsZero(v) {
			t.Error("float32(0) should be zero")
		}
	})
	t.Run("float64_zero", func(t *testing.T) {
		var v float64
		if !IsZero(v) {
			t.Error("float64(0) should be zero")
		}
	})
	t.Run("string_empty", func(t *testing.T) {
		if !IsZero("") {
			t.Error("empty string should be zero")
		}
	})
	t.Run("string_nonempty", func(t *testing.T) {
		if IsZero("hello") {
			t.Error("non-empty string should not be zero")
		}
	})
	t.Run("pointer_nil", func(t *testing.T) {
		var p *int
		if !IsZero(p) {
			t.Error("nil pointer should be zero")
		}
	})
	t.Run("pointer_nonnil", func(t *testing.T) {
		v := 42
		if IsZero(&v) {
			t.Error("non-nil pointer should not be zero")
		}
	})
	t.Run("slice_nil", func(t *testing.T) {
		var s []int
		if !IsZero(s) {
			t.Error("nil slice should be zero")
		}
	})
	t.Run("slice_empty", func(t *testing.T) {
		if !IsZero([]int{}) {
			t.Error("empty slice should be zero")
		}
	})
	t.Run("slice_nonempty", func(t *testing.T) {
		if IsZero([]int{1}) {
			t.Error("non-empty slice should not be zero")
		}
	})
	t.Run("map_nil", func(t *testing.T) {
		var m map[string]int
		if !IsZero(m) {
			t.Error("nil map should be zero")
		}
	})
	t.Run("map_empty", func(t *testing.T) {
		if !IsZero(map[string]int{}) {
			t.Error("empty map should be zero")
		}
	})
	t.Run("map_nonempty", func(t *testing.T) {
		if IsZero(map[string]int{"a": 1}) {
			t.Error("non-empty map should not be zero")
		}
	})
	t.Run("array_zero", func(t *testing.T) {
		var a [0]int
		if !IsZero(a) {
			t.Error("zero-length array should be zero")
		}
	})
	t.Run("time_zero", func(t *testing.T) {
		if !IsZero(time.Time{}) {
			t.Error("zero time should be zero")
		}
	})
	t.Run("time_nonzero", func(t *testing.T) {
		if IsZero(time.Now()) {
			t.Error("non-zero time should not be zero")
		}
	})
	t.Run("struct_nonzero", func(t *testing.T) {
		type s struct{ X int }
		if IsZero(s{X: 1}) {
			t.Error("struct with fields should not be zero")
		}
	})
}

// ==================== BuildDeleteQuery ====================

func TestBuildDeleteQuery(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("basic", func(t *testing.T) {
		q, err := BuildDeleteQuery("users", "WHERE id = ?", 0, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "DELETE FROM `users` WHERE id = ?"
		if q != want {
			t.Errorf("got %q, want %q", q, want)
		}
	})
	t.Run("with_limit", func(t *testing.T) {
		q, err := BuildDeleteQuery("users", "", 10, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "DELETE FROM `users` LIMIT 10"
		if q != want {
			t.Errorf("got %q, want %q", q, want)
		}
	})
	t.Run("invalid_table", func(t *testing.T) {
		_, err := BuildDeleteQuery("", "", 0, dialect)
		if err == nil {
			t.Error("expected error for empty table name")
		}
	})
	t.Run("no_where_no_limit", func(t *testing.T) {
		q, err := BuildDeleteQuery("users", "", 0, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "DELETE FROM `users`"
		if q != want {
			t.Errorf("got %q, want %q", q, want)
		}
	})
}

// ==================== MapToSlice / SliceToMap ====================

func TestMapToSlice(t *testing.T) {
	data := map[string]interface{}{"a": 1, "b": "two"}
	result := MapToSlice(data)
	if len(result) != 2 {
		t.Fatalf("MapToSlice returned %d items, want 2", len(result))
	}
	found := map[string]interface{}{}
	for _, item := range result {
		found[item["key"].(string)] = item["value"]
	}
	if found["a"] != 1 || found["b"] != "two" {
		t.Errorf("MapToSlice roundtrip mismatch: %v", found)
	}
}

func TestSliceToMap(t *testing.T) {
	slice := []map[string]interface{}{
		{"key": "x", "value": 10},
		{"key": "y", "value": "hello"},
		{"key": "bad"}, // missing string key
	}
	result := SliceToMap(slice)
	if result["x"] != 10 {
		t.Errorf("SliceToMap x = %v, want 10", result["x"])
	}
	if result["y"] != "hello" {
		t.Errorf("SliceToMap y = %v, want hello", result["y"])
	}
}

// ==================== FilterEmptyValues ====================

func TestFilterEmptyValues(t *testing.T) {
	data := map[string]interface{}{
		"a": "hello",
		"b": "",
		"c": 0,
		"d": 42,
		"e": nil,
		"f": false,
		"g": true,
	}
	result := FilterEmptyValues(data)
	if _, ok := result["a"]; !ok {
		t.Error("non-empty string should be kept")
	}
	if _, ok := result["b"]; ok {
		t.Error("empty string should be filtered")
	}
	if _, ok := result["c"]; ok {
		t.Error("zero int should be filtered")
	}
	if _, ok := result["d"]; !ok {
		t.Error("non-zero int should be kept")
	}
	if _, ok := result["e"]; ok {
		t.Error("nil should be filtered")
	}
	if _, ok := result["f"]; ok {
		t.Error("false should be filtered")
	}
	if _, ok := result["g"]; !ok {
		t.Error("true should be kept")
	}
}

// ==================== MergeMaps ====================

func TestMergeMaps(t *testing.T) {
	m1 := map[string]interface{}{"a": 1, "b": 2}
	m2 := map[string]interface{}{"b": 3, "c": 4}
	result := MergeMaps(m1, m2)
	if result["a"] != 1 {
		t.Error("a should be 1")
	}
	if result["b"] != 3 {
		t.Error("b should be 3 (m2 overrides)")
	}
	if result["c"] != 4 {
		t.Error("c should be 4")
	}
}

func TestMergeMapsEmpty(t *testing.T) {
	result := MergeMaps()
	if len(result) != 0 {
		t.Errorf("MergeMaps() should return empty map, got %v", result)
	}
}

// ==================== GetMapKeys ====================

func TestGetMapKeys(t *testing.T) {
	data := map[string]interface{}{"c": 3, "a": 1, "b": 2}
	keys := GetMapKeys(data)
	if len(keys) != 3 {
		t.Fatalf("GetMapKeys returned %d keys, want 3", len(keys))
	}
	// Should be sorted
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("GetMapKeys not sorted: %v", keys)
	}
}

func TestGetMapKeysEmpty(t *testing.T) {
	keys := GetMapKeys(map[string]interface{}{})
	if len(keys) != 0 {
		t.Errorf("GetMapKeys(empty) = %v, want empty", keys)
	}
}

// ==================== ValidateTableName ====================

func TestValidateTableName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "users", false},
		{"valid_underscore", "user_profiles", false},
		{"empty", "", true},
		{"starts_with_number", "1table", true},
		{"has_space", "my table", true},
		{"has_dash", "my-table", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTableName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTableName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ==================== ValidateColumnName ====================

func TestValidateColumnName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "id", false},
		{"qualified", "users.name", false},
		{"empty", "", true},
		{"starts_number", "1col", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateColumnName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateColumnName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ==================== ValidateQualifiedIdentifier ====================

func TestValidateQualifiedIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "users", false},
		{"dotted", "schema.users", false},
		{"empty", "", true},
		{"invalid", "1bad", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQualifiedIdentifier(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQualifiedIdentifier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ==================== BuildSafeOrderByItem ====================

func TestBuildSafeOrderByItem(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("column_asc", func(t *testing.T) {
		got, err := BuildSafeOrderByItem(dialect, "name ASC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "name ASC" {
			t.Errorf("got %q, want %q", got, "name ASC")
		}
	})
	t.Run("column_desc", func(t *testing.T) {
		got, err := BuildSafeOrderByItem(dialect, "created_at DESC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "created_at DESC" {
			t.Errorf("got %q, want %q", got, "created_at DESC")
		}
	})
	t.Run("column_only", func(t *testing.T) {
		got, err := BuildSafeOrderByItem(dialect, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "id" {
			t.Errorf("got %q, want %q", got, "id")
		}
	})
	t.Run("invalid", func(t *testing.T) {
		_, err := BuildSafeOrderByItem(dialect, "123invalid")
		if err == nil {
			t.Error("expected error for invalid order by item")
		}
	})
}

// ==================== NormalizeJoinType ====================

func TestNormalizeJoinType(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"INNER", "INNER", false},
		{"left", "LEFT", false},
		{"  RIGHT  ", "RIGHT", false},
		{"full outer", "FULL OUTER", false},
		{"CROSS", "CROSS", false},
		{"NATURAL", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeJoinType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeJoinType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NormalizeJoinType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ==================== ValidateJoinTableReference ====================

func TestValidateJoinTableReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "users", false},
		{"with_alias", "users u", false},
		{"with_as_alias", "users AS u", false},
		{"qualified", "schema.users u", false},
		{"empty", "", true},
		{"too_many_parts", "a b c d", true},
		{"invalid_base", "1bad", true},
		{"as_wrong", "users BY u", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJoinTableReference(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJoinTableReference(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// ==================== isValidDataType ====================

func TestIsValidDataType(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"nil", nil, true},
		{"bool", true, true},
		{"int", 42, true},
		{"int8", int8(1), true},
		{"int16", int16(1), true},
		{"int32", int32(1), true},
		{"int64", int64(1), true},
		{"uint", uint(1), true},
		{"uint8", uint8(1), true},
		{"uint16", uint16(1), true},
		{"uint32", uint32(1), true},
		{"uint64", uint64(1), true},
		{"float32", float32(1.0), true},
		{"float64", float64(1.0), true},
		{"string", "hello", true},
		{"bytes", []byte("data"), true},
		{"time", time.Now(), true},
		{"struct", struct{}{}, false},
		{"ptr_int", func() interface{} { v := 42; return &v }(), true},
		{"ptr_struct", func() interface{} { v := struct{}{}; return &v }(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidDataType(tt.value)
			if got != tt.want {
				t.Errorf("isValidDataType(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// ==================== BuildLikeCondition ====================

func TestBuildLikeCondition(t *testing.T) {
	dialect := NewMySQLDialect()
	got := BuildLikeCondition("name", "%test%", dialect)
	want := "`name` LIKE ? ESCAPE '\\'"
	if got != want {
		t.Errorf("BuildLikeCondition = %q, want %q", got, want)
	}
}

// ==================== BuildInsertQuery error paths ====================

func TestBuildInsertQuery_Errors(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("invalid_table", func(t *testing.T) {
		_, _, err := BuildInsertQuery("", map[string]interface{}{"a": 1}, dialect)
		if err == nil {
			t.Error("expected error for empty table")
		}
	})
	t.Run("empty_data", func(t *testing.T) {
		_, _, err := BuildInsertQuery("users", map[string]interface{}{}, dialect)
		if err == nil {
			t.Error("expected error for empty data")
		}
	})
	t.Run("invalid_column", func(t *testing.T) {
		_, _, err := BuildInsertQuery("users", map[string]interface{}{"1bad": 1}, dialect)
		if err == nil {
			t.Error("expected error for invalid column name")
		}
	})
	t.Run("invalid_data_type", func(t *testing.T) {
		_, _, err := BuildInsertQuery("users", map[string]interface{}{"col": struct{}{}}, dialect)
		if err == nil {
			t.Error("expected error for invalid data type")
		}
	})
}

// ==================== BuildUpdateQuery error paths ====================

func TestBuildUpdateQuery_Errors(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("invalid_table", func(t *testing.T) {
		_, _, err := BuildUpdateQuery("", "id", 1, map[string]interface{}{"a": 1}, dialect)
		if err == nil {
			t.Error("expected error for empty table")
		}
	})
	t.Run("invalid_id_column", func(t *testing.T) {
		_, _, err := BuildUpdateQuery("users", "1bad", 1, map[string]interface{}{"a": 1}, dialect)
		if err == nil {
			t.Error("expected error for invalid id column")
		}
	})
	t.Run("empty_data", func(t *testing.T) {
		_, _, err := BuildUpdateQuery("users", "id", 1, map[string]interface{}{}, dialect)
		if err == nil {
			t.Error("expected error for empty data")
		}
	})
	t.Run("only_id_in_data", func(t *testing.T) {
		_, _, err := BuildUpdateQuery("users", "id", 1, map[string]interface{}{"id": 1}, dialect)
		if err == nil {
			t.Error("expected error when only id column in data")
		}
	})
}

// ==================== BuildSelectQuery error paths ====================

func TestBuildSelectQuery_Errors(t *testing.T) {
	dialect := NewMySQLDialect()

	t.Run("invalid_table", func(t *testing.T) {
		_, err := BuildSelectQuery("", nil, "", "", 0, 0, dialect)
		if err == nil {
			t.Error("expected error for empty table")
		}
	})
	t.Run("unsafe_column", func(t *testing.T) {
		_, err := BuildSelectQuery("users", []string{"id; DROP TABLE users"}, "", "", 0, 0, dialect)
		if err == nil {
			t.Error("expected error for unsafe column")
		}
	})
	t.Run("invalid_column_name", func(t *testing.T) {
		_, err := BuildSelectQuery("users", []string{"1bad"}, "", "", 0, 0, dialect)
		if err == nil {
			t.Error("expected error for invalid column name")
		}
	})
	t.Run("unsafe_where", func(t *testing.T) {
		_, err := BuildSelectQuery("users", nil, "WHERE 1=1; DROP TABLE", "", 0, 0, dialect)
		if err == nil {
			t.Error("expected error for unsafe where clause")
		}
	})
	t.Run("invalid_where_prefix", func(t *testing.T) {
		_, err := BuildSelectQuery("users", nil, "id = 1", "", 0, 0, dialect)
		if err == nil {
			t.Error("expected error for where clause without WHERE prefix")
		}
	})
	t.Run("unsafe_orderby", func(t *testing.T) {
		_, err := BuildSelectQuery("users", nil, "", "id; DROP", 0, 0, dialect)
		if err == nil {
			t.Error("expected error for unsafe order by")
		}
	})
	t.Run("default_columns", func(t *testing.T) {
		q, err := BuildSelectQuery("users", nil, "", "", 0, 0, dialect)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q != "SELECT * FROM `users`" {
			t.Errorf("got %q", q)
		}
	})
}

// ==================== BuildSelectQueryWithOptions error paths ====================

func TestBuildSelectQueryWithOptions_Errors(t *testing.T) {
	t.Run("unsafe_where_condition", func(t *testing.T) {
		_, _, err := BuildSelectQueryWithOptions("users", SelectQueryOptions{
			WhereConditions: []string{"id; DROP TABLE"},
			Dialect:         NewMySQLDialect(),
		})
		if err == nil {
			t.Error("expected error for unsafe where condition")
		}
	})
	t.Run("invalid_order_by", func(t *testing.T) {
		_, _, err := BuildSelectQueryWithOptions("users", SelectQueryOptions{
			OrderByItems: []string{"123invalid"},
			Dialect:      NewMySQLDialect(),
		})
		if err == nil {
			t.Error("expected error for invalid order by item")
		}
	})
	t.Run("nil_dialect_uses_default", func(t *testing.T) {
		q, _, err := BuildSelectQueryWithOptions("users", SelectQueryOptions{
			Columns: []string{"id"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q == "" {
			t.Error("expected non-empty query")
		}
	})
}

// ==================== hasUnsafeSQLBoundary additional ====================

func TestHasUnsafeSQLBoundary(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"normal", false},
		{"with;semicolon", true},
		{"with--comment", true},
		{"with/*comment", true},
		{"with*/comment", true},
		{"with\x00null", true},
		{"union all select", true},
		{"union select", true},
		{"into outfile", true},
		{"drop table", true},
		{"truncate table", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := hasUnsafeSQLBoundary(tt.input)
			if got != tt.want {
				t.Errorf("hasUnsafeSQLBoundary(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
