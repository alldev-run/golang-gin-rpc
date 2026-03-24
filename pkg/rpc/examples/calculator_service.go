// Package examples provides example RPC services
package examples

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/alldev-run/golang-gin-rpc/pkg/rpc"
)

// CalculatorService represents a calculator service
type CalculatorService struct {
	*rpc.BaseService
	history []CalculationRecord
	mutex   sync.RWMutex
}

// CalculationRecord represents a calculation history record
type CalculationRecord struct {
	Operation string    `json:"operation"`
	Operand1  float64   `json:"operand1"`
	Operand2  float64   `json:"operand2"`
	Result    float64   `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

// AddRequest represents an addition request
type AddRequest struct {
	Operand1 float64 `json:"operand1"`
	Operand2 float64 `json:"operand2"`
}

// AddResponse represents an addition response
type AddResponse struct {
	Result float64 `json:"result"`
}

// SubtractRequest represents a subtraction request
type SubtractRequest struct {
	Operand1 float64 `json:"operand1"`
	Operand2 float64 `json:"operand2"`
}

// SubtractResponse represents a subtraction response
type SubtractResponse struct {
	Result float64 `json:"result"`
}

// MultiplyRequest represents a multiplication request
type MultiplyRequest struct {
	Operand1 float64 `json:"operand1"`
	Operand2 float64 `json:"operand2"`
}

// MultiplyResponse represents a multiplication response
type MultiplyResponse struct {
	Result float64 `json:"result"`
}

// DivideRequest represents a division request
type DivideRequest struct {
	Operand1 float64 `json:"operand1"`
	Operand2 float64 `json:"operand2"`
}

// DivideResponse represents a division response
type DivideResponse struct {
	Result float64 `json:"result"`
}

// PowerRequest represents a power operation request
type PowerRequest struct {
	Base     float64 `json:"base"`
	Exponent float64 `json:"exponent"`
}

// PowerResponse represents a power operation response
type PowerResponse struct {
	Result float64 `json:"result"`
}

// SqrtRequest represents a square root request
type SqrtRequest struct {
	Number float64 `json:"number"`
}

// SqrtResponse represents a square root response
type SqrtResponse struct {
	Result float64 `json:"result"`
}

// RandomRequest represents a random number request
type RandomRequest struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// RandomResponse represents a random number response
type RandomResponse struct {
	Result float64 `json:"result"`
}

// HistoryRequest represents a history request
type HistoryRequest struct {
	Limit int `json:"limit"`
}

// HistoryResponse represents a history response
type HistoryResponse struct {
	History []CalculationRecord `json:"history"`
	Total   int                 `json:"total"`
}

// NewCalculatorService creates a new calculator service
func NewCalculatorService() *CalculatorService {
	return &CalculatorService{
		BaseService: rpc.NewBaseService("calculator"),
		history:     make([]CalculationRecord, 0),
	}
}

// Register registers the calculator service
func (s *CalculatorService) Register(server interface{}) error {
	s.SetMetadata("service_type", "calculator")
	s.SetMetadata("version", "1.0.0")
	s.SetMetadata("registration_time", time.Now())
	return nil
}

// Add performs addition
func (s *CalculatorService) Add(ctx context.Context, req *AddRequest) (*AddResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	result := req.Operand1 + req.Operand2
	s.addToHistory("add", req.Operand1, req.Operand2, result)
	
	return &AddResponse{Result: result}, nil
}

// Subtract performs subtraction
func (s *CalculatorService) Subtract(ctx context.Context, req *SubtractRequest) (*SubtractResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	result := req.Operand1 - req.Operand2
	s.addToHistory("subtract", req.Operand1, req.Operand2, result)
	
	return &SubtractResponse{Result: result}, nil
}

// Multiply performs multiplication
func (s *CalculatorService) Multiply(ctx context.Context, req *MultiplyRequest) (*MultiplyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	result := req.Operand1 * req.Operand2
	s.addToHistory("multiply", req.Operand1, req.Operand2, result)
	
	return &MultiplyResponse{Result: result}, nil
}

// Divide performs division
func (s *CalculatorService) Divide(ctx context.Context, req *DivideRequest) (*DivideResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Operand2 == 0 {
		return nil, status.Error(codes.InvalidArgument, "division by zero")
	}
	
	result := req.Operand1 / req.Operand2
	s.addToHistory("divide", req.Operand1, req.Operand2, result)
	
	return &DivideResponse{Result: result}, nil
}

// Power performs power operation
func (s *CalculatorService) Power(ctx context.Context, req *PowerRequest) (*PowerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	result := math.Pow(req.Base, req.Exponent)
	s.addToHistory("power", req.Base, req.Exponent, result)
	
	return &PowerResponse{Result: result}, nil
}

// Sqrt performs square root
func (s *CalculatorService) Sqrt(ctx context.Context, req *SqrtRequest) (*SqrtResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Number < 0 {
		return nil, status.Error(codes.InvalidArgument, "square root of negative number")
	}
	
	result := math.Sqrt(req.Number)
	s.addToHistory("sqrt", req.Number, 0, result)
	
	return &SqrtResponse{Result: result}, nil
}

// Random generates a random number
func (s *CalculatorService) Random(ctx context.Context, req *RandomRequest) (*RandomResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Min > req.Max {
		return nil, status.Error(codes.InvalidArgument, "min must be less than max")
	}
	
	result := req.Min + rand.Float64()*(req.Max-req.Min)
	s.addToHistory("random", req.Min, req.Max, result)
	
	return &RandomResponse{Result: result}, nil
}

// GetHistory returns calculation history
func (s *CalculatorService) GetHistory(ctx context.Context, req *HistoryRequest) (*HistoryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	limit := req.Limit
	if limit <= 0 {
		limit = 10 // default limit
	}
	
	total := len(s.history)
	start := total - limit
	if start < 0 {
		start = 0
	}
	
	history := make([]CalculationRecord, total-start)
	copy(history, s.history[start:])
	
	return &HistoryResponse{
		History: history,
		Total:   total,
	}, nil
}

// addToHistory adds a calculation to the history
func (s *CalculatorService) addToHistory(operation string, operand1, operand2, result float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	record := CalculationRecord{
		Operation: operation,
		Operand1:  operand1,
		Operand2:  operand2,
		Result:    result,
		Timestamp: time.Now(),
	}
	
	s.history = append(s.history, record)
	
	// Keep only last 1000 records
	if len(s.history) > 1000 {
		s.history = s.history[1:]
	}
}

// GetStats returns service statistics
func (s *CalculatorService) GetStats(ctx context.Context, req interface{}) (interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	operationCount := make(map[string]int)
	for _, record := range s.history {
		operationCount[record.Operation]++
	}
	
	return map[string]interface{}{
		"total_calculations": len(s.history),
		"operation_counts":   operationCount,
		"service_name":       s.Name(),
		"uptime":            s.Uptime().String(),
		"started_at":        s.StartTime(),
		"last_activity":     time.Now(),
	}, nil
}

// Health returns the health status of the calculator service
func (s *CalculatorService) Health(ctx context.Context, req interface{}) (interface{}, error) {
	health := s.BaseService.Health()
	health.Metadata["total_calculations"] = len(s.history)
	health.Metadata["service_type"] = "calculator"
	return health, nil
}
