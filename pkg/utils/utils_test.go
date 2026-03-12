package utils

import (
	"testing"
)

// ==================== String Tests ====================

func TestStringJoin(t *testing.T) {
	tests := []struct {
		name     string
		elems    []string
		sep      string
		expected string
	}{
		{"basic", []string{"a", "b", "c"}, ",", "a,b,c"},
		{"empty sep", []string{"a", "b"}, "", "ab"},
		{"single", []string{"a"}, ",", "a"},
		{"empty slice", []string{}, ",", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringJoin(tt.elems, tt.sep)
			if result != tt.expected {
				t.Errorf("StringJoin() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStringJoinNonEmpty(t *testing.T) {
	input := []string{"a", "", "b", "", "c"}
	result := StringJoinNonEmpty(input, ",")
	expected := "a,b,c"
	if result != expected {
		t.Errorf("StringJoinNonEmpty() = %v, want %v", result, expected)
	}
}

func TestStringContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !StringContains(slice, "b") {
		t.Error("Should contain 'b'")
	}
	if StringContains(slice, "d") {
		t.Error("Should not contain 'd'")
	}
}

func TestStringTruncate(t *testing.T) {
	tests := []struct {
		s        string
		maxLen   int
		suffix   string
		expected string
	}{
		{"hello world", 8, "...", "hello..."},
		{"hello", 10, "...", "hello"},
		{"hello world", 0, "...", "hello world"},
	}

	for _, tt := range tests {
		result := StringTruncate(tt.s, tt.maxLen, tt.suffix)
		if result != tt.expected {
			t.Errorf("StringTruncate(%q, %d, %q) = %q, want %q",
				tt.s, tt.maxLen, tt.suffix, result, tt.expected)
		}
	}
}

func TestStringRemoveDuplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	result := StringRemoveDuplicates(input)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique elements, got %d", len(result))
	}
}

func TestStringTrimSlice(t *testing.T) {
	input := []string{"  a  ", " b", "c "}
	result := StringTrimSlice(input)
	expected := []string{"a", "b", "c"}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("StringTrimSlice()[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestStringPadLeft(t *testing.T) {
	result := StringPadLeft("5", 3, '0')
	if result != "005" {
		t.Errorf("StringPadLeft() = %q, want %q", result, "005")
	}
}

func TestStringPadRight(t *testing.T) {
	result := StringPadRight("5", 3, '0')
	if result != "500" {
		t.Errorf("StringPadRight() = %q, want %q", result, "500")
	}
}

// ==================== JSON Tests ====================

func TestToJSON(t *testing.T) {
	data := map[string]any{"name": "test", "value": 123}
	json, err := ToJSON(data)
	if err != nil {
		t.Errorf("ToJSON() error = %v", err)
	}
	if json == "" {
		t.Error("ToJSON() returned empty string")
	}
}

func TestToJSONPretty(t *testing.T) {
	data := map[string]any{"name": "test"}
	json, err := ToJSONPretty(data)
	if err != nil {
		t.Errorf("ToJSONPretty() error = %v", err)
	}
	// Pretty JSON should contain indentation whitespace
	if len(json) < 10 { // Pretty JSON is longer than compact
		t.Error("ToJSONPretty() should be longer than compact JSON")
	}
}

func TestMustToJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	json := MustToJSON(data)
	if json == "" {
		t.Error("MustToJSON() returned empty string")
	}
}

func TestFromJSON(t *testing.T) {
	json := `{"name":"test","value":123}`
	var result map[string]any
	err := FromJSON(json, &result)
	if err != nil {
		t.Errorf("FromJSON() error = %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("FromJSON() name = %v, want test", result["name"])
	}
}

func TestIsValidJSON(t *testing.T) {
	if !IsValidJSON(`{"a":1}`) {
		t.Error("Valid JSON marked as invalid")
	}
	if IsValidJSON(`{invalid}`) {
		t.Error("Invalid JSON marked as valid")
	}
}

// ==================== Type Conversion Tests ====================

func TestToInt(t *testing.T) {
	tests := []struct {
		input    any
		expected int
		wantErr  bool
	}{
		{42, 42, false},
		{int64(42), 42, false},
		{"42", 42, false},
		{42.5, 42, false},
		{true, 1, false},
		{false, 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		result, err := ToInt(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ToInt(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && result != tt.expected {
			t.Errorf("ToInt(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestToInt64(t *testing.T) {
	result, err := ToInt64("123456789")
	if err != nil {
		t.Errorf("ToInt64() error = %v", err)
	}
	if result != 123456789 {
		t.Errorf("ToInt64() = %v, want 123456789", result)
	}
}

func TestToFloat64(t *testing.T) {
	result, err := ToFloat64("3.14")
	if err != nil {
		t.Errorf("ToFloat64() error = %v", err)
	}
	if result != 3.14 {
		t.Errorf("ToFloat64() = %v, want 3.14", result)
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{42, "42"},
		{3.14, "3.14"},
		{true, "true"},
		{nil, ""},
	}

	for _, tt := range tests {
		result := ToString(tt.input)
		if result != tt.expected {
			t.Errorf("ToString(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		input    any
		expected bool
		wantErr  bool
	}{
		{true, true, false},
		{false, false, false},
		{"true", true, false},
		{"false", false, false},
		{"1", true, false},
		{"0", false, false},
		{1, true, false},
		{0, false, false},
		{"invalid", false, true},
	}

	for _, tt := range tests {
		result, err := ToBool(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ToBool(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if result != tt.expected {
			t.Errorf("ToBool(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// ==================== Slice Tests ====================

func TestContains(t *testing.T) {
	if !Contains([]string{"a", "b"}, "b") {
		t.Error("Contains() should return true for string slice")
	}
	if !Contains([]int{1, 2, 3}, 2) {
		t.Error("Contains() should return true for int slice")
	}
	if Contains([]string{"a", "b"}, "c") {
		t.Error("Contains() should return false for missing element")
	}
}

func TestReverse(t *testing.T) {
	input := []string{"a", "b", "c"}
	result := Reverse(input)
	expected := []string{"c", "b", "a"}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("Reverse()[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestChunk(t *testing.T) {
	input := []string{"a", "b", "c", "d", "e"}
	result := Chunk(input, 2)
	if len(result) != 3 {
		t.Errorf("Chunk() returned %d chunks, want 3", len(result))
	}
	if len(result[0]) != 2 || len(result[1]) != 2 || len(result[2]) != 1 {
		t.Error("Chunk() returned wrong chunk sizes")
	}
}

// ==================== Random Tests ====================

func TestRandomBytes(t *testing.T) {
	result, err := RandomBytes(16)
	if err != nil {
		t.Errorf("RandomBytes() error = %v", err)
	}
	if len(result) != 16 {
		t.Errorf("RandomBytes() returned %d bytes, want 16", len(result))
	}
}

func TestRandomString(t *testing.T) {
	result := RandomString(16)
	if len(result) != 16 {
		t.Errorf("RandomString() returned length %d, want 16", len(result))
	}
}

func TestRandomHex(t *testing.T) {
	result := RandomHex(8)
	if len(result) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("RandomHex() returned length %d, want 16", len(result))
	}
}

// ==================== Pointer Tests ====================

func TestPointerHelpers(t *testing.T) {
	strPtr := String("test")
	if *strPtr != "test" {
		t.Error("String() pointer incorrect")
	}

	intPtr := Int(42)
	if *intPtr != 42 {
		t.Error("Int() pointer incorrect")
	}

	boolPtr := Bool(true)
	if *boolPtr != true {
		t.Error("Bool() pointer incorrect")
	}
}

func TestDerefString(t *testing.T) {
	s := "hello"
	if DerefString(&s, "default") != "hello" {
		t.Error("DerefString() with non-nil pointer incorrect")
	}
	if DerefString(nil, "default") != "default" {
		t.Error("DerefString() with nil pointer incorrect")
	}
}
