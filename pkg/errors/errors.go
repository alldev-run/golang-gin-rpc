package errors

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// ErrorCode represents the error code type
type ErrorCode string

// Error categories
const (
	// System errors (1000-1999)
	CategorySystem    ErrorCode = "SYSTEM"
	CategoryDatabase  ErrorCode = "DATABASE"
	CategoryCache     ErrorCode = "CACHE"
	CategoryNetwork   ErrorCode = "NETWORK"
	CategoryAuth      ErrorCode = "AUTH"
	CategoryBusiness  ErrorCode = "BUSINESS"
	CategoryValidation ErrorCode = "VALIDATION"
	CategoryExternal  ErrorCode = "EXTERNAL"
)

// Specific error codes
const (
	// System errors (1000-1999)
	ErrCodeInternalServer    ErrorCode = "SYSTEM_1000"
	ErrCodePanicRecovered    ErrorCode = "SYSTEM_1001"
	ErrCodeConfigInvalid     ErrorCode = "SYSTEM_1002"
	ErrCodeServiceUnavailable ErrorCode = "SYSTEM_1003"
	ErrCodeTimeout           ErrorCode = "SYSTEM_1004"

	// Database errors (2000-2999)
	ErrCodeDBConnection      ErrorCode = "DATABASE_2000"
	ErrCodeDBQuery          ErrorCode = "DATABASE_2001"
	ErrCodeDBTransaction    ErrorCode = "DATABASE_2002"
	ErrCodeDBMigration      ErrorCode = "DATABASE_2003"
	ErrCodeDBDeadlock       ErrorCode = "DATABASE_2004"

	// Cache errors (3000-3999)
	ErrCodeCacheConnection   ErrorCode = "CACHE_3000"
	ErrCodeCacheMiss        ErrorCode = "CACHE_3001"
	ErrCodeCacheSerialization ErrorCode = "CACHE_3002"

	// Network errors (4000-4999)
	ErrCodeNetworkTimeout    ErrorCode = "NETWORK_4000"
	ErrCodeNetworkConnection ErrorCode = "NETWORK_4001"
	ErrCodeRateLimited       ErrorCode = "NETWORK_4002"

	// Auth errors (5000-5999)
	ErrCodeUnauthorized      ErrorCode = "AUTH_5000"
	ErrCodeForbidden         ErrorCode = "AUTH_5001"
	ErrCodeTokenExpired      ErrorCode = "AUTH_5002"
	ErrCodeTokenInvalid      ErrorCode = "AUTH_5003"

	// Business errors (6000-6999)
	ErrCodeResourceNotFound  ErrorCode = "BUSINESS_6000"
	ErrCodeResourceExists    ErrorCode = "BUSINESS_6001"
	ErrCodeBusinessRule      ErrorCode = "BUSINESS_6002"

	// Validation errors (7000-7999)
	ErrCodeValidationFailed  ErrorCode = "VALIDATION_7000"
	ErrCodeInvalidInput      ErrorCode = "VALIDATION_7001"
	ErrCodeMissingField      ErrorCode = "VALIDATION_7002"

	// External service errors (8000-8999)
	ErrCodeExternalService  ErrorCode = "EXTERNAL_8000"
	ErrCodeExternalTimeout   ErrorCode = "EXTERNAL_8001"
	ErrCodeExternalRateLimit ErrorCode = "EXTERNAL_8002"
)

// ErrorLevel represents the severity level of an error
type ErrorLevel string

const (
	ErrorLevelDebug   ErrorLevel = "DEBUG"
	ErrorLevelInfo    ErrorLevel = "INFO"
	ErrorLevelWarning ErrorLevel = "WARNING"
	ErrorLevelError   ErrorLevel = "ERROR"
	ErrorLevelFatal   ErrorLevel = "FATAL"
)

// AppError represents a structured application error
type AppError struct {
	Code        ErrorCode   `json:"code"`
	Message     string      `json:"message"`
	Level       ErrorLevel  `json:"level"`
	HTTPStatus  int         `json:"http_status"`
	Details     interface{} `json:"details,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
	RequestID   string      `json:"request_id,omitempty"`
	UserID      string      `json:"user_id,omitempty"`
	StackTrace  string      `json:"stack_trace,omitempty"`
	Cause       error       `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (caused by: %v)", e.Code, e.Message, e.Details, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target
func (e *AppError) Is(target error) bool {
	if t, ok := target.(*AppError); ok {
		return e.Code == t.Code
	}
	return false
}

// New creates a new application error
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Level:      getDefaultLevel(code),
		HTTPStatus: getDefaultHTTPStatus(code),
		Timestamp:  time.Now(),
	}
}

// WithCause adds a cause to the error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details interface{}) *AppError {
	e.Details = details
	return e
}

// WithLevel sets the error level
func (e *AppError) WithLevel(level ErrorLevel) *AppError {
	e.Level = level
	return e
}

// WithHTTPStatus sets the HTTP status code
func (e *AppError) WithHTTPStatus(status int) *AppError {
	e.HTTPStatus = status
	return e
}

// WithRequestID adds request ID to the error
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithUserID adds user ID to the error
func (e *AppError) WithUserID(userID string) *AppError {
	e.UserID = userID
	return e
}

// WithStackTrace adds stack trace to the error
func (e *AppError) WithStackTrace() *AppError {
	e.StackTrace = getStackTrace()
	return e
}

// Wrap wraps an existing error with application context
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}

	// If it's already an AppError, just add context
	if appErr, ok := err.(*AppError); ok {
		return appErr.WithCause(err)
	}

	return New(code, message).WithCause(err)
}

// IsSystem checks if the error is a system error
func (e *AppError) IsSystem() bool {
	return strings.HasPrefix(string(e.Code), string(CategorySystem))
}

// IsDatabase checks if the error is a database error
func (e *AppError) IsDatabase() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryDatabase))
}

// IsCache checks if the error is a cache error
func (e *AppError) IsCache() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryCache))
}

// IsNetwork checks if the error is a network error
func (e *AppError) IsNetwork() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryNetwork))
}

// IsAuth checks if the error is an auth error
func (e *AppError) IsAuth() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryAuth))
}

// IsBusiness checks if the error is a business error
func (e *AppError) IsBusiness() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryBusiness))
}

// IsValidation checks if the error is a validation error
func (e *AppError) IsValidation() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryValidation))
}

// IsExternal checks if the error is an external service error
func (e *AppError) IsExternal() bool {
	return strings.HasPrefix(string(e.Code), string(CategoryExternal))
}

// IsRetryable checks if the error is retryable
func (e *AppError) IsRetryable() bool {
	retryableCodes := map[ErrorCode]bool{
		ErrCodeNetworkTimeout:    true,
		ErrCodeNetworkConnection: true,
		ErrCodeDBConnection:      true,
		ErrCodeDBDeadlock:        true,
		ErrCodeCacheConnection:   true,
		ErrCodeExternalTimeout:   true,
		ErrCodeExternalRateLimit: true,
		ErrCodeTimeout:           true,
	}
	return retryableCodes[e.Code]
}

// getDefaultLevel returns the default error level for a given error code
func getDefaultLevel(code ErrorCode) ErrorLevel {
	switch {
	case strings.HasPrefix(string(code), string(CategorySystem)):
		return ErrorLevelError
	case strings.HasPrefix(string(code), string(CategoryDatabase)):
		return ErrorLevelError
	case strings.HasPrefix(string(code), string(CategoryCache)):
		return ErrorLevelWarning
	case strings.HasPrefix(string(code), string(CategoryNetwork)):
		return ErrorLevelWarning
	case strings.HasPrefix(string(code), string(CategoryAuth)):
		return ErrorLevelWarning
	case strings.HasPrefix(string(code), string(CategoryBusiness)):
		return ErrorLevelInfo
	case strings.HasPrefix(string(code), string(CategoryValidation)):
		return ErrorLevelInfo
	case strings.HasPrefix(string(code), string(CategoryExternal)):
		return ErrorLevelWarning
	default:
		return ErrorLevelError
	}
}

// getDefaultHTTPStatus returns the default HTTP status for a given error code
func getDefaultHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeResourceNotFound:
		return http.StatusNotFound
	case ErrCodeResourceExists:
		return http.StatusConflict
	case ErrCodeValidationFailed, ErrCodeInvalidInput, ErrCodeMissingField:
		return http.StatusBadRequest
	case ErrCodeRateLimited, ErrCodeExternalRateLimit:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeTimeout, ErrCodeNetworkTimeout, ErrCodeExternalTimeout:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

// getStackTrace captures the current stack trace
func getStackTrace() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:])
	if n == 0 {
		return ""
	}

	var sb strings.Builder
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		sb.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}
	return sb.String()
}

// Predefined common errors for convenience
var (
	ErrInternalServer     = New(ErrCodeInternalServer, "Internal server error")
	ErrUnauthorized       = New(ErrCodeUnauthorized, "Unauthorized access")
	ErrForbidden          = New(ErrCodeForbidden, "Forbidden access")
	ErrResourceNotFound   = New(ErrCodeResourceNotFound, "Resource not found")
	ErrValidationFailed   = New(ErrCodeValidationFailed, "Validation failed")
	ErrInvalidInput       = New(ErrCodeInvalidInput, "Invalid input")
	ErrServiceUnavailable = New(ErrCodeServiceUnavailable, "Service unavailable")
	ErrTimeout            = New(ErrCodeTimeout, "Request timeout")
	ErrRateLimited        = New(ErrCodeRateLimited, "Rate limit exceeded")
)
