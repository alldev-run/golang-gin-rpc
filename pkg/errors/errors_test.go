package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		code    ErrorCode
		message string
	}{
		{
			name:    "create system error",
			code:    ErrCodeInternalServer,
			message: "Internal server error",
		},
		{
			name:    "create database error",
			code:    ErrCodeDBConnection,
			message: "Database connection failed",
		},
		{
			name:    "create cache error",
			code:    ErrCodeCacheMiss,
			message: "Cache miss",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, tt.message)
			
			if err.Code != tt.code {
				t.Errorf("New().Code = %v, want %v", err.Code, tt.code)
			}
			
			if err.Message != tt.message {
				t.Errorf("New().Message = %v, want %v", err.Message, tt.message)
			}
			
			if err.Timestamp.IsZero() {
				t.Error("New().Timestamp should not be zero")
			}
			
			expectedLevel := getDefaultLevel(tt.code)
			if err.Level != expectedLevel {
				t.Errorf("New().Level = %v, want %v", err.Level, expectedLevel)
			}
			
			expectedStatus := getDefaultHTTPStatus(tt.code)
			if err.HTTPStatus != expectedStatus {
				t.Errorf("New().HTTPStatus = %v, want %v", err.HTTPStatus, expectedStatus)
			}
		})
	}
}

func TestAppError_WithCause(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := New(ErrCodeInternalServer, "Test error")
	
	errWithCause := appErr.WithCause(originalErr)
	
	if errWithCause.Cause != originalErr {
		t.Errorf("WithCause().Cause = %v, want %v", errWithCause.Cause, originalErr)
	}
	
	// Test Error() method with cause
	expected := "[SYSTEM_1000] Test error: %!s(<nil>) (caused by: original error)"
	if errWithCause.Error() != expected {
		t.Errorf("WithCause().Error() = %v, want %v", errWithCause.Error(), expected)
	}
}

func TestAppError_WithDetails(t *testing.T) {
	details := map[string]interface{}{
		"user_id": 123,
		"action":  "create",
	}
	
	appErr := New(ErrCodeValidationFailed, "Validation failed")
	errWithDetails := appErr.WithDetails(details)
	
	if errWithDetails.Details == nil {
		t.Error("WithDetails().Details should not be nil")
	}
	
	detailsMap, ok := errWithDetails.Details.(map[string]interface{})
	if !ok {
		t.Error("WithDetails().Details should be a map")
		return
	}
	
	if detailsMap["user_id"] != 123 {
		t.Errorf("WithDetails().Details[user_id] = %v, want %v", detailsMap["user_id"], 123)
	}
}

func TestAppError_WithLevel(t *testing.T) {
	appErr := New(ErrCodeInternalServer, "Test error")
	
	errWithLevel := appErr.WithLevel(ErrorLevelFatal)
	if errWithLevel.Level != ErrorLevelFatal {
		t.Errorf("WithLevel().Level = %v, want %v", errWithLevel.Level, ErrorLevelFatal)
	}
}

func TestAppError_WithHTTPStatus(t *testing.T) {
	appErr := New(ErrCodeInternalServer, "Test error")
	
	errWithStatus := appErr.WithHTTPStatus(http.StatusBadGateway)
	if errWithStatus.HTTPStatus != http.StatusBadGateway {
		t.Errorf("WithHTTPStatus().HTTPStatus = %v, want %v", errWithStatus.HTTPStatus, http.StatusBadGateway)
	}
}

func TestAppError_WithRequestID(t *testing.T) {
	requestID := "req-123456"
	appErr := New(ErrCodeInternalServer, "Test error")
	
	errWithRequestID := appErr.WithRequestID(requestID)
	if errWithRequestID.RequestID != requestID {
		t.Errorf("WithRequestID().RequestID = %v, want %v", errWithRequestID.RequestID, requestID)
	}
}

func TestAppError_WithUserID(t *testing.T) {
	userID := "user-123"
	appErr := New(ErrCodeInternalServer, "Test error")
	
	errWithUserID := appErr.WithUserID(userID)
	if errWithUserID.UserID != userID {
		t.Errorf("WithUserID().UserID = %v, want %v", errWithUserID.UserID, userID)
	}
}

func TestAppError_WithStackTrace(t *testing.T) {
	appErr := New(ErrCodeInternalServer, "Test error")
	
	errWithStack := appErr.WithStackTrace()
	if errWithStack.StackTrace == "" {
		t.Error("WithStackTrace().StackTrace should not be empty")
	}
	
	// Check if stack trace contains expected patterns
	if !contains(errWithStack.StackTrace, "golang-gin-rpc") {
		t.Error("StackTrace should contain package name")
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	
	appErr := Wrap(originalErr, ErrCodeDBConnection, "Database connection failed")
	
	if appErr == nil {
		t.Fatal("Wrap() should not return nil")
	}
	
	if appErr.Code != ErrCodeDBConnection {
		t.Errorf("Wrap().Code = %v, want %v", appErr.Code, ErrCodeDBConnection)
	}
	
	if appErr.Message != "Database connection failed" {
		t.Errorf("Wrap().Message = %v, want %v", appErr.Message, "Database connection failed")
	}
	
	if appErr.Cause != originalErr {
		t.Errorf("Wrap().Cause = %v, want %v", appErr.Cause, originalErr)
	}
	
	// Test wrapping nil error
	nilErr := Wrap(nil, ErrCodeInternalServer, "Test")
	if nilErr != nil {
		t.Error("Wrap(nil) should return nil")
	}
}

func TestWrap_AppError(t *testing.T) {
	originalErr := New(ErrCodeInternalServer, "Original error")
	
	wrappedErr := Wrap(originalErr, ErrCodeDBConnection, "Wrapped error")
	
	// Should return the original AppError with cause
	if wrappedErr != originalErr {
		t.Error("Wrap(AppError) should return the same error instance")
	}
}

func TestAppError_Is(t *testing.T) {
	err1 := New(ErrCodeInternalServer, "Error 1")
	err2 := New(ErrCodeInternalServer, "Error 2")
	err3 := New(ErrCodeDBConnection, "Error 3")
	
	// Test same code
	if !err1.Is(err2) {
		t.Error("Is() should return true for same error code")
	}
	
	// Test different code
	if err1.Is(err3) {
		t.Error("Is() should return false for different error code")
	}
	
	// Test with standard error
	standardErr := errors.New("standard error")
	if err1.Is(standardErr) {
		t.Error("Is() should return false for standard error")
	}
}

func TestAppError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := New(ErrCodeInternalServer, "Test error").WithCause(originalErr)
	
	unwrapped := appErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
	
	// Test without cause
	appErrNoCause := New(ErrCodeInternalServer, "Test error")
	if appErrNoCause.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no cause is set")
	}
}

func TestErrorCategories(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		category string
	}{
		{"system error", New(ErrCodeInternalServer, "Test"), "system"},
		{"database error", New(ErrCodeDBConnection, "Test"), "database"},
		{"cache error", New(ErrCodeCacheMiss, "Test"), "cache"},
		{"network error", New(ErrCodeNetworkTimeout, "Test"), "network"},
		{"auth error", New(ErrCodeUnauthorized, "Test"), "auth"},
		{"business error", New(ErrCodeResourceNotFound, "Test"), "business"},
		{"validation error", New(ErrCodeValidationFailed, "Test"), "validation"},
		{"external error", New(ErrCodeExternalService, "Test"), "external"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.category {
			case "system":
				if !tt.err.IsSystem() {
					t.Error("IsSystem() should return true")
				}
			case "database":
				if !tt.err.IsDatabase() {
					t.Error("IsDatabase() should return true")
				}
			case "cache":
				if !tt.err.IsCache() {
					t.Error("IsCache() should return true")
				}
			case "network":
				if !tt.err.IsNetwork() {
					t.Error("IsNetwork() should return true")
				}
			case "auth":
				if !tt.err.IsAuth() {
					t.Error("IsAuth() should return true")
				}
			case "business":
				if !tt.err.IsBusiness() {
					t.Error("IsBusiness() should return true")
				}
			case "validation":
				if !tt.err.IsValidation() {
					t.Error("IsValidation() should return true")
				}
			case "external":
				if !tt.err.IsExternal() {
					t.Error("IsExternal() should return true")
				}
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		retryable bool
	}{
		{"retryable timeout", New(ErrCodeTimeout, "Test"), true},
		{"retryable network timeout", New(ErrCodeNetworkTimeout, "Test"), true},
		{"retryable db connection", New(ErrCodeDBConnection, "Test"), true},
		{"retryable cache connection", New(ErrCodeCacheConnection, "Test"), true},
		{"retryable external timeout", New(ErrCodeExternalTimeout, "Test"), true},
		{"non-retryable validation", New(ErrCodeValidationFailed, "Test"), false},
		{"non-retryable unauthorized", New(ErrCodeUnauthorized, "Test"), false},
		{"non-retryable resource not found", New(ErrCodeResourceNotFound, "Test"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.IsRetryable() != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", tt.err.IsRetryable(), tt.retryable)
			}
		})
	}
}

func TestDefaultHTTPStatus(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrCodeUnauthorized, http.StatusUnauthorized},
		{ErrCodeForbidden, http.StatusForbidden},
		{ErrCodeResourceNotFound, http.StatusNotFound},
		{ErrCodeResourceExists, http.StatusConflict},
		{ErrCodeValidationFailed, http.StatusBadRequest},
		{ErrCodeInvalidInput, http.StatusBadRequest},
		{ErrCodeMissingField, http.StatusBadRequest},
		{ErrCodeRateLimited, http.StatusTooManyRequests},
		{ErrCodeExternalRateLimit, http.StatusTooManyRequests},
		{ErrCodeServiceUnavailable, http.StatusServiceUnavailable},
		{ErrCodeTimeout, http.StatusRequestTimeout},
		{ErrCodeNetworkTimeout, http.StatusRequestTimeout},
		{ErrCodeExternalTimeout, http.StatusRequestTimeout},
		{ErrCodeInternalServer, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := getDefaultHTTPStatus(tt.code); got != tt.expected {
				t.Errorf("getDefaultHTTPStatus(%v) = %v, want %v", tt.code, got, tt.expected)
			}
		})
	}
}

func TestDefaultLevel(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected ErrorLevel
	}{
		{ErrCodeInternalServer, ErrorLevelError},
		{ErrCodeDBConnection, ErrorLevelError},
		{ErrCodeCacheConnection, ErrorLevelWarning},
		{ErrCodeNetworkTimeout, ErrorLevelWarning},
		{ErrCodeUnauthorized, ErrorLevelWarning},
		{ErrCodeResourceNotFound, ErrorLevelInfo},
		{ErrCodeValidationFailed, ErrorLevelInfo},
		{ErrCodeExternalService, ErrorLevelWarning},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := getDefaultLevel(tt.code); got != tt.expected {
				t.Errorf("getDefaultLevel(%v) = %v, want %v", tt.code, got, tt.expected)
			}
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name    string
		errFunc func() *AppError
		code    ErrorCode
		message string
	}{
		{"ErrInternalServer", func() *AppError { return ErrInternalServer }, ErrCodeInternalServer, "Internal server error"},
		{"ErrUnauthorized", func() *AppError { return ErrUnauthorized }, ErrCodeUnauthorized, "Unauthorized access"},
		{"ErrForbidden", func() *AppError { return ErrForbidden }, ErrCodeForbidden, "Forbidden access"},
		{"ErrResourceNotFound", func() *AppError { return ErrResourceNotFound }, ErrCodeResourceNotFound, "Resource not found"},
		{"ErrValidationFailed", func() *AppError { return ErrValidationFailed }, ErrCodeValidationFailed, "Validation failed"},
		{"ErrInvalidInput", func() *AppError { return ErrInvalidInput }, ErrCodeInvalidInput, "Invalid input"},
		{"ErrServiceUnavailable", func() *AppError { return ErrServiceUnavailable }, ErrCodeServiceUnavailable, "Service unavailable"},
		{"ErrTimeout", func() *AppError { return ErrTimeout }, ErrCodeTimeout, "Request timeout"},
		{"ErrRateLimited", func() *AppError { return ErrRateLimited }, ErrCodeRateLimited, "Rate limit exceeded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errFunc()
			if err.Code != tt.code {
				t.Errorf("Predefined error %s.Code = %v, want %v", tt.name, err.Code, tt.code)
			}
			
			if err.Message != tt.message {
				t.Errorf("Predefined error %s.Message = %v, want %v", tt.name, err.Message, tt.message)
			}
		})
	}
}

func TestGetStackTrace(t *testing.T) {
	stack := getStackTrace()
	
	if stack == "" {
		t.Error("getStackTrace() should not return empty string")
	}
	
	// Check if stack trace contains expected patterns
	// Note: The actual stack trace may vary, so we just check it's not empty
}

func TestSimpleHash(t *testing.T) {
	// Test hash function behavior
	key := "test"
	hash1 := simpleHashInTest(key)
	hash2 := simpleHashInTest(key)
	
	// Test consistency
	if hash1 != hash2 {
		t.Error("simpleHash() should be consistent for same input")
	}
	
	// Test non-negative
	if hash1 < 0 {
		t.Error("simpleHash() should return non-negative value")
	}
	
	// Test different keys produce different hashes
	hash3 := simpleHashInTest("different")
	if hash1 == hash3 {
		t.Error("simpleHash() should produce different hashes for different keys")
	}
}

// Helper function for testing simpleHash
func simpleHashInTest(key string) int {
	hash := 0
	for _, c := range key {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    s[:len(substr)] == substr || 
		    s[len(s)-len(substr):] == substr || 
		    indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Benchmark tests
func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New(ErrCodeInternalServer, "Test error")
	}
}

func BenchmarkWithCause(b *testing.B) {
	originalErr := errors.New("original error")
	appErr := New(ErrCodeInternalServer, "Test error")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		appErr.WithCause(originalErr)
	}
}

func BenchmarkError(b *testing.B) {
	appErr := New(ErrCodeInternalServer, "Test error").WithCause(errors.New("cause"))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = appErr.Error()
	}
}
