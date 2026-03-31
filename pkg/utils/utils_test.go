package utils

import (
	"strings"
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
	result, err := RandomString(16)
	if err != nil {
		t.Errorf("RandomString() error = %v", err)
	}
	if len(result) != 16 {
		t.Errorf("RandomString() returned length %d, want 16", len(result))
	}
}

func TestRandomHex(t *testing.T) {
	result, err := RandomHex(8)
	if err != nil {
		t.Errorf("RandomHex() error = %v", err)
	}
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

// Additional comprehensive tests for missing functions

func TestStringContainsAny(t *testing.T) {
	tests := []struct {
		s      string
		substr []string
		want   bool
	}{
		{"hello world", []string{"hello", "test"}, true},
		{"hello world", []string{"test", "world"}, true},
		{"hello world", []string{"test", "foo"}, false},
		{"", []string{"test"}, false},
		{"hello", []string{}, false},
	}

	for _, tt := range tests {
		result := StringContainsAny(tt.s, tt.substr...)
		if result != tt.want {
			t.Errorf("StringContainsAny(%q, %v) = %v, want %v", tt.s, tt.substr, result, tt.want)
		}
	}
}

func TestStringFilterEmpty(t *testing.T) {
	input := []string{"a", "", "  ", "b", "\t", "c"}
	result := StringFilterEmpty(input)
	expected := []string{"a", "b", "c"}
	
	if len(result) != len(expected) {
		t.Errorf("StringFilterEmpty() returned %d elements, want %d", len(result), len(expected))
	}
	
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("StringFilterEmpty()[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestSafeFromJSON(t *testing.T) {
	// Test valid JSON
	validJSON := `{"name": "test", "value": 123}`
	var result map[string]interface{}
	err := SafeFromJSON(validJSON, &result)
	if err != nil {
		t.Errorf("SafeFromJSON() with valid JSON error = %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("SafeFromJSON() name = %v, want test", result["name"])
	}

	// Test invalid JSON that would cause panic
	invalidJSON := `{"name": "test", "value":}` // This will cause a panic in json.Unmarshal
	var result2 map[string]interface{}
	err = SafeFromJSON(invalidJSON, &result2)
	if err == nil {
		t.Error("SafeFromJSON() with invalid JSON should return error")
	}
	// The error should contain our panic recovery message
	if !strings.Contains(err.Error(), "json unmarshal panic") && !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("SafeFromJSON() error should mention panic or parsing error, got %v", err)
	}
}

func TestToInt64Extended(t *testing.T) {
	tests := []struct {
		input    any
		expected int64
		wantErr  bool
	}{
		{int64(123456789), 123456789, false},
		{123, 123, false},
		{"123456789", 123456789, false},
		{float64(123.45), 123, false},
		{uint64(123), 123, false},
		{"invalid", 0, true},
		{nil, 0, true},
	}

	for _, tt := range tests {
		result, err := ToInt64(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ToInt64(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && result != tt.expected {
			t.Errorf("ToInt64(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestToFloat64Extended(t *testing.T) {
	tests := []struct {
		input    any
		expected float64
		wantErr  bool
	}{
		{float64(3.14), 3.14, false},
		{float32(3.14), 3.140000104904175, false}, // Updated expected value for float32 precision
		{123, 123.0, false},
		{"3.14", 3.14, false},
		{"invalid", 0, true},
		{nil, 0, true},
	}

	for _, tt := range tests {
		result, err := ToFloat64(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ToFloat64(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && result != tt.expected {
			t.Errorf("ToFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestMustToBool(t *testing.T) {
	tests := []struct {
		input    any
		expected bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"false", false},
		{1, true},
		{0, false},
		{"invalid", false}, // Should return false on error
		{nil, false},      // Should return false on error
	}

	for _, tt := range tests {
		result := MustToBool(tt.input)
		if result != tt.expected {
			t.Errorf("MustToBool(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestContainsIntSlice(t *testing.T) {
	if !Contains([]int{1, 2, 3}, 2) {
		t.Error("Contains() should return true for int slice")
	}
	if Contains([]int{1, 2, 3}, 4) {
		t.Error("Contains() should return false for missing element")
	}
}

func TestContainsInt64Slice(t *testing.T) {
	if !Contains([]int64{1, 2, 3}, int64(2)) {
		t.Error("Contains() should return true for int64 slice")
	}
	if Contains([]int64{1, 2, 3}, int64(4)) {
		t.Error("Contains() should return false for missing element")
	}
}

func TestContainsFloat64Slice(t *testing.T) {
	if !Contains([]float64{1.1, 2.2, 3.3}, 2.2) {
		t.Error("Contains() should return true for float64 slice")
	}
	if Contains([]float64{1.1, 2.2, 3.3}, 4.4) {
		t.Error("Contains() should return false for missing element")
	}
}

func TestContainsUnsupportedType(t *testing.T) {
	// Test with unsupported slice type
	if Contains([]bool{true, false}, true) {
		t.Error("Contains() should return false for unsupported slice type")
	}
}

func TestDerefInt(t *testing.T) {
	i := 42
	if DerefInt(&i, 0) != 42 {
		t.Error("DerefInt() with non-nil pointer incorrect")
	}
	if DerefInt(nil, 0) != 0 {
		t.Error("DerefInt() with nil pointer incorrect")
	}
	if DerefInt(nil, 99) != 99 {
		t.Error("DerefInt() with nil pointer and default incorrect")
	}
}

func TestPointerHelpersExtended(t *testing.T) {
	// Test all pointer helpers
	strPtr := String("test")
	if strPtr == nil || *strPtr != "test" {
		t.Error("String() pointer incorrect")
	}

	intPtr := Int(42)
	if intPtr == nil || *intPtr != 42 {
		t.Error("Int() pointer incorrect")
	}

	int64Ptr := Int64(1234567890)
	if int64Ptr == nil || *int64Ptr != 1234567890 {
		t.Error("Int64() pointer incorrect")
	}

	boolPtr := Bool(true)
	if boolPtr == nil || *boolPtr != true {
		t.Error("Bool() pointer incorrect")
	}

	float64Ptr := Float64(3.14)
	if float64Ptr == nil || *float64Ptr != 3.14 {
		t.Error("Float64() pointer incorrect")
	}
}

func TestRandomStringEdgeCases(t *testing.T) {
	// Test with different lengths
	for n := 0; n < 20; n++ {
		result, err := RandomString(n)
		if err != nil {
			t.Errorf("RandomString(%d) error = %v", n, err)
			continue
		}
		if len(result) != n {
			t.Errorf("RandomString(%d) returned length %d, want %d", n, len(result), n)
		}
	}
	
	// Test that random strings are different
	s1, err1 := RandomString(10)
	s2, err2 := RandomString(10)
	if err1 != nil || err2 != nil {
		t.Errorf("RandomString() errors: %v, %v", err1, err2)
		return
	}
	if s1 == s2 {
		t.Error("RandomString() should produce different strings")
	}
}

func TestRandomHexEdgeCases(t *testing.T) {
	// Test with different lengths
	for n := 0; n < 10; n++ {
		result, err := RandomHex(n)
		if err != nil {
			t.Errorf("RandomHex(%d) error = %v", n, err)
			continue
		}
		expectedLen := n * 2 // hex encoding doubles the length
		if len(result) != expectedLen {
			t.Errorf("RandomHex(%d) returned length %d, want %d", n, len(result), expectedLen)
		}
	}
	
	// Test that hex strings contain only valid hex characters
	result, err := RandomHex(5)
	if err != nil {
		t.Errorf("RandomHex(5) error = %v", err)
		return
	}
	for _, c := range result {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			t.Errorf("RandomHex() contains invalid hex character: %c", c)
		}
	}
}

func TestChunkEdgeCases(t *testing.T) {
	// Test empty slice
	result := Chunk([]string{}, 3)
	if len(result) != 0 {
		t.Errorf("Chunk() with empty slice should return empty slice, got %d chunks", len(result))
	}
	
	// Test size larger than slice
	result = Chunk([]string{"a", "b"}, 5)
	if len(result) != 1 || len(result[0]) != 2 {
		t.Error("Chunk() with size > slice length should return one chunk")
	}
	
	// Test size 0 (should return single chunk)
	result = Chunk([]string{"a", "b", "c"}, 0)
	if len(result) != 1 || len(result[0]) != 3 {
		t.Error("Chunk() with size 0 should return single chunk")
	}
	
	// Test size 1
	result = Chunk([]string{"a", "b", "c"}, 1)
	if len(result) != 3 {
		t.Error("Chunk() with size 1 should return 3 chunks")
	}
	for i, chunk := range result {
		if len(chunk) != 1 {
			t.Errorf("Chunk() size 1 chunk %d should have length 1", i)
		}
	}
}

func TestToStringComplexTypes(t *testing.T) {
	// Test with slice
	slice := []string{"a", "b"}
	result := ToString(slice)
	if result == "" {
		t.Error("ToString() with slice should not be empty")
	}
	
	// Test with map
	m := map[string]int{"a": 1}
	result = ToString(m)
	if result == "" {
		t.Error("ToString() with map should not be empty")
	}
	
	// Test with struct
	type TestStruct struct {
		Field string
	}
	s := TestStruct{Field: "test"}
	result = ToString(s)
	if result == "" {
		t.Error("ToString() with struct should not be empty")
	}
}
