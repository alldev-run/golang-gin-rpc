// Package utils provides common utility functions for string manipulation,
// JSON conversion, type conversion, and other frequently used operations.
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// ==================== Buffer Pool for High Concurrency ====================

// bufferPool provides reusable buffers to reduce GC pressure under high concurrency
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 0, 1024) // Start with 1KB capacity
	},
}

// getBuffer gets a buffer from the pool
func getBuffer() []byte {
	return bufferPool.Get().([]byte)
}

// putBuffer returns a buffer to the pool after resetting it
func putBuffer(buf []byte) {
	if cap(buf) <= 64*1024 { // Only pool buffers up to 64KB
		bufferPool.Put(buf[:0]) // Reset length but keep capacity
	}
}

// ==================== String Utilities ====================

// StringJoin joins string slice with separator
func StringJoin(elems []string, sep string) string {
	return strings.Join(elems, sep)
}

// StringJoinNonEmpty joins non-empty strings only using buffer pool for efficiency
// This function is concurrent-safe and optimized for high concurrency
func StringJoinNonEmpty(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	if len(elems) == 1 {
		return elems[0]
	}
	
	// Use buffer pool to reduce allocations
	buf := getBuffer()
	defer putBuffer(buf)
	
	first := true
	for _, s := range elems {
		if s != "" {
			if !first {
				buf = append(buf, sep...)
			} else {
				first = false
			}
			buf = append(buf, s...)
		}
	}
	return string(buf)
}

// StringContains checks if slice contains string
func StringContains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// StringContainsAny checks if string contains any of the substrings
func StringContainsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// StringTruncate truncates string to max length with suffix
func StringTruncate(s string, maxLen int, suffix string) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	if len(suffix) >= maxLen {
		return s[:maxLen]
	}
	return s[:maxLen-len(suffix)] + suffix
}

// StringRemoveDuplicates removes duplicate strings from slice
// This function is concurrent-safe as it operates on local data only
func StringRemoveDuplicates(slice []string) []string {
	if len(slice) <= 1 {
		return slice // Early return for empty/single-element slices
	}
	seen := make(map[string]bool, len(slice))
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// StringTrimSlice trims whitespace from all strings in slice
// This function is concurrent-safe as it operates on local data only
func StringTrimSlice(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	result := make([]string, len(slice))
	for i, s := range slice {
		result[i] = strings.TrimSpace(s)
	}
	return result
}

// StringFilterEmpty removes empty strings from slice
// This function is concurrent-safe as it operates on local data only
func StringFilterEmpty(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}

// StringPadLeft pads string on the left to reach min length
func StringPadLeft(s string, minLen int, padChar rune) string {
	if len(s) >= minLen {
		return s
	}
	padding := strings.Repeat(string(padChar), minLen-len(s))
	return padding + s
}

// StringPadRight pads string on the right to reach min length
func StringPadRight(s string, minLen int, padChar rune) string {
	if len(s) >= minLen {
		return s
	}
	padding := strings.Repeat(string(padChar), minLen-len(s))
	return s + padding
}

// ==================== JSON Utilities ====================

// ToJSON marshals value to JSON string with size limits
// This function is concurrent-safe as it operates on local data only
func ToJSON(v any) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if len(bytes) > 10*1024*1024 { // 10MB limit
		return "", fmt.Errorf("JSON output too large: %d bytes", len(bytes))
	}
	return string(bytes), nil
}

// ToJSONPretty marshals value to indented JSON string
func ToJSONPretty(v any) (string, error) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// MustToJSON marshals to JSON, returns empty string on error
func MustToJSON(v any) string {
	s, _ := ToJSON(v)
	return s
}

// FromJSON unmarshals JSON string to target
func FromJSON(s string, target any) error {
	return json.Unmarshal([]byte(s), target)
}

// SafeFromJSON unmarshals with error recovery and size limits
// This function is concurrent-safe as it operates on local data only
func SafeFromJSON(s string, target any) (err error) {
	// Add size limit to prevent DoS attacks
	if len(s) > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("JSON input too large: %d bytes", len(s))
	}
	
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("json unmarshal panic: %v", r)
		}
	}()
	return json.Unmarshal([]byte(s), target)
}

// IsValidJSON checks if string is valid JSON
func IsValidJSON(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}

// ==================== Type Conversion ====================

// ToInt converts value to int
func ToInt(v any) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int8:
		return int(val), nil
	case int16:
		return int(val), nil
	case int32:
		return int(val), nil
	case int64:
		return int(val), nil
	case uint:
		return int(val), nil
	case uint8:
		return int(val), nil
	case uint16:
		return int(val), nil
	case uint32:
		return int(val), nil
	case uint64:
		return int(val), nil
	case float32:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// ToInt64 converts value to int64
func ToInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	case float64:
		return int64(val), nil
	case uint64:
		return int64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

// ToFloat64 converts value to float64
func ToFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// ToString converts value to string
func ToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return strconv.FormatBool(val)
	case []byte:
		return string(val)
	case nil:
		return ""
	default:
		bytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(bytes)
	}
}

// ToBool converts value to bool
func ToBool(v any) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return strconv.ParseBool(val)
	case int:
		return val != 0, nil
	case int64:
		return val != 0, nil
	case float64:
		return val != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

// MustToBool converts to bool, returns false on error
func MustToBool(v any) bool {
	b, _ := ToBool(v)
	return b
}

// ==================== Slice/Array Utilities ====================

// Contains checks if slice contains element (string, int, float64)
func Contains(slice any, elem any) bool {
	switch s := slice.(type) {
	case []string:
		str := ToString(elem)
		for _, v := range s {
			if v == str {
				return true
			}
		}
	case []int:
		n, err := ToInt(elem)
		if err != nil {
			return false
		}
		for _, v := range s {
			if v == n {
				return true
			}
		}
	case []int64:
		n, err := ToInt64(elem)
		if err != nil {
			return false
		}
		for _, v := range s {
			if v == n {
				return true
			}
		}
	case []float64:
		n, err := ToFloat64(elem)
		if err != nil {
			return false
		}
		for _, v := range s {
			if v == n {
				return true
			}
		}
	}
	return false
}

// Reverse reverses a string slice
func Reverse(slice []string) []string {
	result := make([]string, len(slice))
	for i, j := 0, len(slice)-1; i <= j; i, j = i+1, j-1 {
		result[i], result[j] = slice[j], slice[i]
	}
	return result
}

// Chunk splits slice into chunks of given size
func Chunk(slice []string, size int) [][]string {
	if size <= 0 {
		return [][]string{slice}
	}
	var chunks [][]string
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// ==================== Crypto/Random Utilities ====================

// RandomBytes generates random bytes of given length
func RandomBytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("invalid length: %d", n)
	}
	if n > 1024*1024 { // 1MB limit to prevent DoS
		return nil, fmt.Errorf("length too large: %d", n)
	}
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("random generation failed: %w", err)
	}
	return b, nil
}

// RandomString generates random string of given length (base64 URL safe)
func RandomString(n int) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("invalid length: %d", n)
	}
	if n == 0 {
		return "", nil
	}
	if n > 1024 { // Reasonable limit for string length
		return "", fmt.Errorf("length too large: %d", n)
	}
	
	// Calculate needed bytes for base64 encoding (4/3 ratio)
	byteLen := (n*3 + 3) / 4 // Round up
	b, err := RandomBytes(byteLen)
	if err != nil {
		return "", err
	}
	
	encoded := base64.URLEncoding.EncodeToString(b)
	if len(encoded) >= n {
		return encoded[:n], nil
	}
	return encoded, nil // Return what we have if shorter than expected
}

// RandomHex generates random hex string of given length (bytes*2)
func RandomHex(n int) (string, error) {
	if n < 0 {
		return "", fmt.Errorf("invalid length: %d", n)
	}
	if n == 0 {
		return "", nil
	}
	if n > 512 { // Reasonable limit for hex string
		return "", fmt.Errorf("length too large: %d", n)
	}
	
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ==================== Pointer Helpers ====================

// String returns pointer to string
func String(s string) *string {
	return &s
}

// Int returns pointer to int
func Int(i int) *int {
	return &i
}

// Int64 returns pointer to int64
func Int64(i int64) *int64 {
	return &i
}

// Bool returns pointer to bool
func Bool(b bool) *bool {
	return &b
}

// Float64 returns pointer to float64
func Float64(f float64) *float64 {
	return &f
}

// DerefString dereferences string pointer with default
func DerefString(s *string, defaultVal string) string {
	if s == nil {
		return defaultVal
	}
	return *s
}

// DerefInt dereferences int pointer with default
func DerefInt(i *int, defaultVal int) int {
	if i == nil {
		return defaultVal
	}
	return *i
}
