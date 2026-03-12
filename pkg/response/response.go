package response

import (
	"net/http"
	"time"

	"golang-gin-rpc/pkg/status_code"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code      int         `json:"code"`
	Msg       string      `json:"msg"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

var (
	SuccessCode = status_code.Success
	ErrorCode   = status_code.BadRequest
)

// Success sends a success response
func Success(c *gin.Context, data interface{}) {
	response := Response{
		Code:      int(status_code.Success),
		Msg:       status_code.Success.Message(),
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(SuccessCode), response)
}

// Error sends an error response
func Error(c *gin.Context, msg string, data interface{}) {
	response := Response{
		Code:      int(status_code.BadRequest),
		Msg:       msg,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(ErrorCode), response)
}

// ErrorWithCode sends an error response with custom status code
func ErrorWithCode(c *gin.Context, code status_code.StatusCode, msg string, data interface{}) {
	response := Response{
		Code:      int(code),
		Msg:       msg,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(code), response)
}

// InternalError sends an internal server error response
func InternalError(c *gin.Context, msg string) {
	response := Response{
		Code:      int(status_code.Internal),
		Msg:       msg,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(status_code.Internal), response)
}

// NotFound sends a not found response
func NotFound(c *gin.Context, msg string) {
	response := Response{
		Code:      int(status_code.NotFound),
		Msg:       msg,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(status_code.NotFound), response)
}

// Unauthorized sends an unauthorized response
func Unauthorized(c *gin.Context, msg string) {
	response := Response{
		Code:      http.StatusUnauthorized,
		Msg:       msg,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusUnauthorized, response)
}

// Forbidden sends a forbidden response
func Forbidden(c *gin.Context, msg string) {
	response := Response{
		Code:      http.StatusForbidden,
		Msg:       msg,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusForbidden, response)
}

// Created sends a created response (201)
func Created(c *gin.Context, data interface{}) {
	response := Response{
		Code:      http.StatusCreated,
		Msg:       "Created successfully",
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusCreated, response)
}

// NoContent sends a no content response (204)
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// PagedResponse represents a paginated response
type PagedResponse struct {
	Response
	 Pagination Pagination `json:"pagination"`
}

// Pagination contains pagination information
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Paged sends a paginated response
func Paged(c *gin.Context, data interface{}, page, pageSize, total int) {
	totalPages := (total + pageSize - 1) / pageSize // Ceiling division

	response := PagedResponse{
		Response: Response{
			Code:      int(status_code.Success),
			Msg:       status_code.Success.Message(),
			Data:      data,
			Timestamp: time.Now().Unix(),
		},
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(SuccessCode), response)
}

// ValidationError represents a validation error response
type ValidationError struct {
	Response
	Errors map[string]string `json:"errors"`
}

// ValidationFailed sends a validation error response
func ValidationFailed(c *gin.Context, errors map[string]string) {
	response := ValidationError{
		Response: Response{
			Code:      int(status_code.BadRequest),
			Msg:       "Validation failed",
			Timestamp: time.Now().Unix(),
		},
		Errors: errors,
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(int(status_code.BadRequest), response)
}

// RateLimitExceeded sends a rate limit exceeded response
func RateLimitExceeded(c *gin.Context, msg string) {
	response := Response{
		Code:      http.StatusTooManyRequests,
		Msg:       msg,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(http.StatusTooManyRequests, response)
}

// Custom sends a custom response with any status code
func Custom(c *gin.Context, statusCode int, msg string, data interface{}) {
	response := Response{
		Code:      statusCode,
		Msg:       msg,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		response.RequestID = requestID.(string)
	}

	c.JSON(statusCode, response)
}
