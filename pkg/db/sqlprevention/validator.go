// Package sqlprevention provides SQL injection prevention utilities including
// input validation, sanitization, and SQL injection pattern detection.
// This package helps protect against common SQL injection attacks.
package sqlprevention

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Common SQL injection patterns
type patterns struct {
	unionSelect   *regexp.Regexp
	sleepDelay    *regexp.Regexp
	booleanBased  *regexp.Regexp
	stackedQuery  *regexp.Regexp
	commentAttack *regexp.Regexp
	orAttack      *regexp.Regexp
	andAttack     *regexp.Regexp
	singleQuote   *regexp.Regexp
	semicolon     *regexp.Regexp
	hexEscape     *regexp.Regexp
	timeBased     *regexp.Regexp
	outOfBand     *regexp.Regexp
}

var sqlPatterns *patterns

func init() {
	sqlPatterns = &patterns{
		// Union-based SQL injection: UNION SELECT, UNION ALL SELECT
		unionSelect: regexp.MustCompile(`(?i)(union\s+all\s+select|union\s+select)`),
		// Time-based blind SQL injection: SLEEP, BENCHMARK, WAITFOR DELAY, pg_sleep
		sleepDelay: regexp.MustCompile(`(?i)(sleep\s*\(|benchmark\s*\(|waitfor\s+delay|pg_sleep)`),
		// Boolean-based blind SQL injection: AND 1=1, OR 'a'='a', etc.
		booleanBased: regexp.MustCompile(`(?i)((and|or)\s+\d+\s*=\s*\d+|(and|or)\s+['"]\w+['"]\s*=\s*['"]\w+['"])`),
		// Stacked queries: ; DELETE, ; DROP, ; INSERT, etc.
		stackedQuery: regexp.MustCompile(`(?i)(;\s*(delete|drop|insert|update|create|alter|truncate|merge|grant|revoke))`),
		// Comment attacks: /**/, --, ;--, etc.
		commentAttack: regexp.MustCompile(`(?i)(/\*|\*/|--|;--|#)`),
		// OR-based attacks: OR 1=1, OR 'x'='x', etc.
		orAttack: regexp.MustCompile(`(?i)(\bor\s+\d+\s*=\s*\d+|\bor\s+['"]\w+['"]\s*=\s*['"]\w+['"])`),
		// AND-based attacks: AND 1=1, etc.
		andAttack: regexp.MustCompile(`(?i)(\band\s+\d+\s*=\s*\d+|\band\s+['"]\w+['"]\s*=\s*['"]\w+['"])`),
		// Single quote escaping attempts
		singleQuote: regexp.MustCompile(`'+`),
		// Semicolon for query termination
		semicolon: regexp.MustCompile(`;`),
		// Hexadecimal escape sequences: 0x, \x, etc.
		hexEscape: regexp.MustCompile(`(?i)(0x[0-9a-f]+|\\x[0-9a-f]+)`),
		// Time-based functions
		timeBased: regexp.MustCompile(`(?i)(sleep\s*\(|benchmark\s*\(|pg_sleep|waitfor\s+)`),
		// Out-of-band extraction
		outOfBand: regexp.MustCompile(`(?i)(load_file|into\s+outfile|into\s+dumpfile)`),
	}
}

// Validator provides SQL injection prevention capabilities
type Validator struct {
	strictMode      bool
	maxLength       int
	allowedChars    *regexp.Regexp
	forbiddenWords  []string
	blockPatterns   []*regexp.Regexp
}

// Config holds validator configuration
type Config struct {
	StrictMode     bool     // If true, reject any suspicious input
	MaxLength      int      // Maximum allowed length for input
	AllowedChars   string   // Regex pattern for allowed characters
	ForbiddenWords []string // List of forbidden SQL keywords
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		StrictMode:   true,
		MaxLength:    1000,
		AllowedChars: `^[a-zA-Z0-9_\-\s\.\@\:]+$`,
		ForbiddenWords: []string{
			"DROP", "DELETE", "TRUNCATE", "INSERT", "UPDATE",
			"CREATE", "ALTER", "GRANT", "REVOKE", "EXEC",
			"UNION", "SELECT", "FROM", "WHERE", "OR", "AND",
		},
	}
}

// New creates a new SQL injection validator
func New(config Config) *Validator {
	var allowedPattern *regexp.Regexp
	if config.AllowedChars != "" {
		allowedPattern = regexp.MustCompile(config.AllowedChars)
	}

	// Compile custom patterns
	blockPatterns := make([]*regexp.Regexp, 0)
	for _, word := range config.ForbiddenWords {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
		blockPatterns = append(blockPatterns, pattern)
	}

	return &Validator{
		strictMode:     config.StrictMode,
		maxLength:      config.MaxLength,
		allowedChars:   allowedPattern,
		forbiddenWords: config.ForbiddenWords,
		blockPatterns:  blockPatterns,
	}
}

// ValidateInput checks if the input is safe from SQL injection
func (v *Validator) ValidateInput(input string) error {
	if len(input) == 0 {
		return nil // Empty input is safe
	}

	if v.maxLength > 0 && len(input) > v.maxLength {
		return fmt.Errorf("input exceeds maximum length of %d characters", v.maxLength)
	}

	// Check allowed characters
	if v.allowedChars != nil && !v.allowedChars.MatchString(input) {
		return errors.New("input contains invalid characters")
	}

	// Check for forbidden words
	for i, pattern := range v.blockPatterns {
		if pattern.MatchString(input) {
			return fmt.Errorf("input contains forbidden keyword: %s", v.forbiddenWords[i])
		}
	}

	// Run SQL injection detection
	if v.strictMode {
		if result := DetectInjection(input); result.IsInjected {
			return fmt.Errorf("potential SQL injection detected: %s", result.Pattern)
		}
	}

	return nil
}

// SanitizeInput sanitizes input by removing or escaping dangerous characters
func (v *Validator) SanitizeInput(input string) string {
	if len(input) == 0 {
		return input
	}

	// Limit length
	if v.maxLength > 0 && len(input) > v.maxLength {
		input = input[:v.maxLength]
	}

	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Escape single quotes
	input = strings.ReplaceAll(input, "'", "''")

	// Remove comment sequences
	input = strings.ReplaceAll(input, "/*", "")
	input = strings.ReplaceAll(input, "*/", "")
	input = strings.ReplaceAll(input, "--", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	return input
}

// InjectionResult represents the result of SQL injection detection
type InjectionResult struct {
	IsInjected bool   // Whether injection was detected
	Pattern    string // The matched pattern
	Severity   string // low, medium, high, critical
	Position   int    // Position where pattern was found
}

// DetectInjection checks if a string contains SQL injection patterns
func DetectInjection(input string) InjectionResult {
	if len(input) == 0 {
		return InjectionResult{IsInjected: false}
	}

	// Check critical patterns first
	if sqlPatterns.unionSelect.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "union_select",
			Severity:   "critical",
			Position:   sqlPatterns.unionSelect.FindStringIndex(input)[0],
		}
	}

	if sqlPatterns.stackedQuery.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "stacked_query",
			Severity:   "critical",
			Position:   sqlPatterns.stackedQuery.FindStringIndex(input)[0],
		}
	}

	if sqlPatterns.orAttack.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "or_attack",
			Severity:   "high",
			Position:   sqlPatterns.orAttack.FindStringIndex(input)[0],
		}
	}

	if sqlPatterns.sleepDelay.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "time_based",
			Severity:   "high",
			Position:   sqlPatterns.sleepDelay.FindStringIndex(input)[0],
		}
	}

	if sqlPatterns.booleanBased.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "boolean_based",
			Severity:   "medium",
			Position:   sqlPatterns.booleanBased.FindStringIndex(input)[0],
		}
	}

	if sqlPatterns.commentAttack.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "comment_attack",
			Severity:   "medium",
			Position:   sqlPatterns.commentAttack.FindStringIndex(input)[0],
		}
	}

	if sqlPatterns.outOfBand.MatchString(input) {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "out_of_band",
			Severity:   "high",
			Position:   sqlPatterns.outOfBand.FindStringIndex(input)[0],
		}
	}

	// Check for excessive quote escaping
	quoteCount := strings.Count(input, "'")
	if quoteCount > 3 {
		return InjectionResult{
			IsInjected: true,
			Pattern:    "quote_escaping",
			Severity:   "medium",
		}
	}

	return InjectionResult{IsInjected: false}
}

// QuickCheck performs a fast check for obvious SQL injection attempts
func QuickCheck(input string) bool {
	if len(input) == 0 {
		return false
	}

	// Fast path: check for obvious patterns
	lower := strings.ToLower(input)

	// Check for common SQL keywords in suspicious contexts
	dangerousPatterns := []string{
		"' or ",
		"' and ",
		"; drop ",
		"; delete ",
		"union select",
		"exec(",
		"eval(",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Check for excessive length
	if len(input) > 500 {
		return true
	}

	return false
}

// QuoteEscape escapes single quotes for SQL string literals
func QuoteEscape(input string) string {
	return strings.ReplaceAll(input, "'", "''")
}

// SafeString checks if a string is safe to use as an identifier (table/column name)
func SafeString(input string) bool {
	if len(input) == 0 {
		return false
	}

	// Must start with letter or underscore
	first := rune(input[0])
	if !unicode.IsLetter(first) && first != '_' {
		return false
	}

	// Must contain only letters, digits, underscores
	for _, r := range input {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}

	// Check for SQL keywords
	sqlKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE",
		"ALTER", "TABLE", "FROM", "WHERE", "AND", "OR", "NOT",
		"NULL", "TRUE", "FALSE", "UNION", "JOIN", "ORDER", "GROUP",
	}

	upper := strings.ToUpper(input)
	for _, keyword := range sqlKeywords {
		if upper == keyword {
			return false
		}
	}

	return true
}

// SafeInt64 checks if a string represents a safe integer
func SafeInt64(input string) (int64, bool) {
	// Remove whitespace
	input = strings.TrimSpace(input)

	// Check for non-digit characters (except leading -)
	if len(input) == 0 {
		return 0, false
	}

	start := 0
	if input[0] == '-' {
		start = 1
		if len(input) == 1 {
			return 0, false
		}
	}

	for i := start; i < len(input); i++ {
		if !unicode.IsDigit(rune(input[i])) {
			return 0, false
		}
	}

	// Parse to validate range
	var result int64
	_, err := fmt.Sscanf(input, "%d", &result)
	if err != nil {
		return 0, false
	}

	return result, true
}

// ParameterizedQuery provides a safe way to build parameterized queries
type ParameterizedQuery struct {
	Query  string
	Params []any
}

// NewParameterizedQuery creates a new parameterized query builder
func NewParameterizedQuery(baseQuery string) *ParameterizedQuery {
	return &ParameterizedQuery{
		Query:  baseQuery,
		Params: make([]any, 0),
	}
}

// AddParam adds a parameter to the query
func (pq *ParameterizedQuery) AddParam(value any) *ParameterizedQuery {
	pq.Params = append(pq.Params, value)
	return pq
}

// Build returns the final query and parameters
func (pq *ParameterizedQuery) Build() (string, []any) {
	return pq.Query, pq.Params
}

// Execute executes the parameterized query
func (pq *ParameterizedQuery) Execute(db *sql.DB) (*sql.Rows, error) {
	return db.Query(pq.Query, pq.Params...)
}

// SecurityHelper provides common security utilities
type SecurityHelper struct {
	validator *Validator
}

// NewSecurityHelper creates a new security helper
func NewSecurityHelper() *SecurityHelper {
	return &SecurityHelper{
		validator: New(DefaultConfig()),
	}
}

// ValidateAndSanitize validates input and returns sanitized version
func (sh *SecurityHelper) ValidateAndSanitize(input string) (string, error) {
	if err := sh.validator.ValidateInput(input); err != nil {
		return "", err
	}
	return sh.validator.SanitizeInput(input), nil
}

// IsSafeIdentifier checks if string is safe as SQL identifier
func (sh *SecurityHelper) IsSafeIdentifier(input string) bool {
	return SafeString(input)
}

// CleanLikePattern sanitizes LIKE pattern input
func CleanLikePattern(pattern string) string {
	// Escape special LIKE characters
	pattern = strings.ReplaceAll(pattern, "%", "\\%")
	pattern = strings.ReplaceAll(pattern, "_", "\\_")
	pattern = strings.ReplaceAll(pattern, "[", "\\[")
	pattern = strings.ReplaceAll(pattern, "]", "\\]")

	// Remove null bytes
	pattern = strings.ReplaceAll(pattern, "\x00", "")

	return pattern
}

// BuildInClause safely builds an IN clause with parameters
func BuildInClause(column string, values []any) (string, []any) {
	if len(values) == 0 {
		return "1=0", nil // Return false condition for empty slice
	}

	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", "))
	return query, values
}
