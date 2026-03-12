package sqlprevention

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.StrictMode {
		t.Error("StrictMode should be true by default")
	}
	if cfg.MaxLength != 1000 {
		t.Errorf("MaxLength = %d, want 1000", cfg.MaxLength)
	}
	if cfg.AllowedChars == "" {
		t.Error("AllowedChars should not be empty")
	}
	if len(cfg.ForbiddenWords) == 0 {
		t.Error("ForbiddenWords should not be empty")
	}
}

func TestNewValidator(t *testing.T) {
	cfg := DefaultConfig()
	v := New(cfg)

	if v == nil {
		t.Fatal("New() returned nil")
	}
	if !v.strictMode {
		t.Error("strictMode should be true")
	}
	if v.maxLength != 1000 {
		t.Error("maxLength mismatch")
	}
}

func TestValidateInputEmpty(t *testing.T) {
	v := New(DefaultConfig())

	err := v.ValidateInput("")
	if err != nil {
		t.Errorf("Empty input should be valid, got: %v", err)
	}
}

func TestValidateInputMaxLength(t *testing.T) {
	cfg := Config{
		StrictMode: true,
		MaxLength:  10,
	}
	v := New(cfg)

	err := v.ValidateInput("this is a very long string")
	if err == nil {
		t.Error("Should fail for input exceeding max length")
	}
	if !strings.Contains(err.Error(), "maximum length") {
		t.Errorf("Error message should mention max length, got: %v", err)
	}
}

func TestValidateInputForbiddenWords(t *testing.T) {
	cfg := Config{
		StrictMode:     true,
		ForbiddenWords: []string{"DROP", "DELETE"},
	}
	v := New(cfg)

	err := v.ValidateInput("DROP TABLE users")
	if err == nil {
		t.Error("Should detect forbidden word DROP")
	}

	err = v.ValidateInput("delete from users")
	if err == nil {
		t.Error("Should detect forbidden word DELETE (case insensitive)")
	}
}

func TestValidateInputSafe(t *testing.T) {
	v := New(DefaultConfig())

	tests := []string{
		"john_doe",
		"user@example.com",
		"Test.User-123",
		"Normal text with spaces",
	}

	for _, input := range tests {
		err := v.ValidateInput(input)
		if err != nil {
			t.Errorf("Input %q should be valid, got: %v", input, err)
		}
	}
}

func TestDetectInjectionUnionSelect(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"' UNION SELECT * FROM users --", true},
		{"1 UNION ALL SELECT password FROM admin", true},
		{"normal text", false},
		{"UNION SELECT", true}, // Without quotes
	}

	for _, tt := range tests {
		result := DetectInjection(tt.input)
		if result.IsInjected != tt.expected {
			t.Errorf("DetectInjection(%q).IsInjected = %v, want %v", 
				tt.input, result.IsInjected, tt.expected)
		}
	}
}

func TestDetectInjectionOrAttack(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"' OR '1'='1", true},
		{"1 OR 1=1", true},
		{"normal user input", false},
		{"order by name", false}, // Contains 'or' but not as attack
	}

	for _, tt := range tests {
		result := DetectInjection(tt.input)
		if result.IsInjected != tt.expected {
			t.Errorf("DetectInjection(%q).IsInjected = %v, want %v", 
				tt.input, result.IsInjected, tt.expected)
		}
	}
}

func TestDetectInjectionTimeBased(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1 AND SLEEP(5)", true},
		{"benchmark(1000000, md5('test'))", true},
		{"pg_sleep(10)", true},
		{"normal query", false},
	}

	for _, tt := range tests {
		result := DetectInjection(tt.input)
		if result.IsInjected != tt.expected {
			t.Errorf("DetectInjection(%q).IsInjected = %v, want %v", 
				tt.input, result.IsInjected, tt.expected)
		}
	}
}

func TestDetectInjectionStackedQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"'; DROP TABLE users; --", true},
		{"1; DELETE FROM users", true},
		{"valid;input", false}, // Semicolon without SQL
		{"normal text", false},
	}

	for _, tt := range tests {
		result := DetectInjection(tt.input)
		if result.IsInjected != tt.expected {
			t.Errorf("DetectInjection(%q).IsInjected = %v, want %v", 
				tt.input, result.IsInjected, tt.expected)
		}
	}
}

func TestDetectInjectionSeverity(t *testing.T) {
	tests := []struct {
		input            string
		expectedSeverity string
	}{
		{"UNION SELECT * FROM passwords", "critical"},
		{"' OR '1'='1", "medium"},  // Detected as boolean_based or comment
		{"AND 1=1", "medium"},
		{"-- comment", "medium"},
		{"safe input", ""},
	}

	for _, tt := range tests {
		result := DetectInjection(tt.input)
		if result.Severity != tt.expectedSeverity {
			t.Errorf("DetectInjection(%q).Severity = %q, want %q", 
				tt.input, result.Severity, tt.expectedSeverity)
		}
	}
}

func TestSanitizeInput(t *testing.T) {
	v := New(DefaultConfig())

	tests := []struct {
		input    string
		expected string
	}{
		{"' OR '1'='1", "'' OR ''1''=''1"},       // Quote escaping
		{"test/*comment*/here", "testcommenthere"},       // Comment removal (content preserved)
		{"data--comment", "datacomment"},                 // Comment removal (content preserved)
		{"  spaced  ", "spaced"},                  // Trimming
		{"\x00null\x00byte", "nullbyte"},          // Null byte removal
	}

	for _, tt := range tests {
		result := v.SanitizeInput(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeInput(%q) = %q, want %q", 
				tt.input, result, tt.expected)
		}
	}
}

func TestQuickCheck(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"' or 1=1", true},
		{"' and 1=1", true},
		{"; drop table users", true},
		{"union select", true},
		{"normal input", false},
		{"short", false},
	}

	for _, tt := range tests {
		result := QuickCheck(tt.input)
		if result != tt.expected {
			t.Errorf("QuickCheck(%q) = %v, want %v", 
				tt.input, result, tt.expected)
		}
	}
}

func TestQuoteEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"it's", "it''s"},
		{"' OR '1'='1", "'' OR ''1''=''1"},
		{"no quotes", "no quotes"},
		{"''", "''''"},
	}

	for _, tt := range tests {
		result := QuoteEscape(tt.input)
		if result != tt.expected {
			t.Errorf("QuoteEscape(%q) = %q, want %q", 
				tt.input, result, tt.expected)
		}
	}
}

func TestSafeString(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"users", true},
		{"user_name", true},
		{"table123", true},
		{"_private", true},
		{"123table", false}, // Can't start with digit
		{"table-name", false}, // Hyphen not allowed
		{"", false},           // Empty
		{"SELECT", false},     // SQL keyword
		{"select", false},     // SQL keyword (case insensitive)
		{"table name", false}, // Space not allowed
	}

	for _, tt := range tests {
		result := SafeString(tt.input)
		if result != tt.expected {
			t.Errorf("SafeString(%q) = %v, want %v", 
				tt.input, result, tt.expected)
		}
	}
}

func TestSafeInt64(t *testing.T) {
	tests := []struct {
		input       string
		expectedVal int64
		expectedOK  bool
	}{
		{"123", 123, true},
		{"0", 0, true},
		{"-456", -456, true},
		{" 789 ", 789, true},
		{"abc", 0, false},
		{"12.34", 0, false},
		{"12abc", 0, false},
		{"", 0, false},
		{"-", 0, false},
	}

	for _, tt := range tests {
		val, ok := SafeInt64(tt.input)
		if ok != tt.expectedOK {
			t.Errorf("SafeInt64(%q) ok = %v, want %v", 
				tt.input, ok, tt.expectedOK)
			continue
		}
		if ok && val != tt.expectedVal {
			t.Errorf("SafeInt64(%q) = %d, want %d", 
				tt.input, val, tt.expectedVal)
		}
	}
}

func TestParameterizedQuery(t *testing.T) {
	pq := NewParameterizedQuery("SELECT * FROM users WHERE id = ? AND name = ?")
	
	pq.AddParam(123).AddParam("john")
	
	query, params := pq.Build()
	if query != "SELECT * FROM users WHERE id = ? AND name = ?" {
		t.Errorf("Unexpected query: %s", query)
	}
	if len(params) != 2 {
		t.Errorf("Expected 2 params, got %d", len(params))
	}
	if params[0] != 123 {
		t.Errorf("First param = %v, want 123", params[0])
	}
	if params[1] != "john" {
		t.Errorf("Second param = %v, want 'john'", params[1])
	}
}

func TestCleanLikePattern(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test%", "test\\%"},     // Percent escape
		{"test_", "test\\_"},     // Underscore escape
		{"[test]", "\\[test\\]"}, // Bracket escape
		{"\x00test", "test"},     // Null byte removal
		{"normal", "normal"},     // No change
	}

	for _, tt := range tests {
		result := CleanLikePattern(tt.input)
		if result != tt.expected {
			t.Errorf("CleanLikePattern(%q) = %q, want %q", 
				tt.input, result, tt.expected)
		}
	}
}

func TestBuildInClause(t *testing.T) {
	tests := []struct {
		column   string
		values   []any
		expected string
		len      int
	}{
		{"id", []any{1, 2, 3}, "id IN (?, ?, ?)", 3},
		{"name", []any{"a", "b"}, "name IN (?, ?)", 2},
		{"status", []any{}, "1=0", 0},
	}

	for _, tt := range tests {
		query, params := BuildInClause(tt.column, tt.values)
		if query != tt.expected {
			t.Errorf("BuildInClause query = %q, want %q", query, tt.expected)
		}
		if len(params) != tt.len {
			t.Errorf("BuildInClause params len = %d, want %d", len(params), tt.len)
		}
	}
}

func TestSecurityHelper(t *testing.T) {
	sh := NewSecurityHelper()

	// Test ValidateAndSanitize
	input := "' OR '1'='1"
	sanitized, err := sh.ValidateAndSanitize(input)
	if err == nil {
		t.Error("Should detect SQL injection in strict mode")
	}

	// Test with safe input
	safeInput := "john_doe"
	sanitized, err = sh.ValidateAndSanitize(safeInput)
	if err != nil {
		t.Errorf("Safe input should pass, got: %v", err)
	}
	if sanitized != safeInput {
		t.Errorf("Sanitized = %q, want %q", sanitized, safeInput)
	}

	// Test IsSafeIdentifier
	if !sh.IsSafeIdentifier("users") {
		t.Error("'users' should be safe identifier")
	}
	if sh.IsSafeIdentifier("123table") {
		t.Error("'123table' should not be safe identifier")
	}
}

func TestNonStrictMode(t *testing.T) {
	cfg := Config{
		StrictMode:     false, // Non-strict
		MaxLength:      100,
		ForbiddenWords: []string{"DROP"},
	}
	v := New(cfg)

	// Should still check forbidden words
	err := v.ValidateInput("DROP TABLE")
	if err == nil {
		t.Error("Should still detect forbidden words in non-strict mode")
	}

	// Should allow input that would be injection in strict mode
	// (as long as it passes other checks)
	err = v.ValidateInput("normal text with or and")
	if err != nil {
		t.Errorf("Non-strict should allow safe input, got: %v", err)
	}
}

// Test complex SQL injection patterns
func TestComplexInjectionPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"union_in_comment", "'/**/union/**/select*/", true},
		{"case_variation", "' UnIoN SeLeCt ", true},
		{"nested_quotes", "''or''1''=''1''", true},
		{"hex_encoding", "0x44454C455445", false}, // Hex pattern detection
		{"time_based_alt", "benchmark(10000000,sha1('test'))", true},
		{"out_of_band", "' into outfile '/tmp/test'", true},
		{"safe_text", "Hello World", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectInjection(tt.input)
			if result.IsInjected != tt.expected {
				t.Errorf("DetectInjection(%q) = %v, want %v", 
					tt.input, result.IsInjected, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkDetectInjection(b *testing.B) {
	input := "' UNION SELECT * FROM users WHERE '1'='1"
	
	for i := 0; i < b.N; i++ {
		DetectInjection(input)
	}
}

func BenchmarkValidateInput(b *testing.B) {
	v := New(DefaultConfig())
	input := "normal_user_input_123"
	
	for i := 0; i < b.N; i++ {
		v.ValidateInput(input)
	}
}

func BenchmarkQuickCheck(b *testing.B) {
	input := "' or 1=1 --"
	
	for i := 0; i < b.N; i++ {
		QuickCheck(input)
	}
}
