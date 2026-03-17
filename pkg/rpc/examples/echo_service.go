// Package examples provides example RPC services
package examples

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"alldev-gin-rpc/pkg/rpc"
)

// EchoService represents an echo service
type EchoService struct {
	*rpc.BaseService
}

// EchoRequest represents an echo request
type EchoRequest struct {
	Message string `json:"message"`
	Times   int    `json:"times"`
}

// EchoResponse represents an echo response
type EchoResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// ReverseRequest represents a reverse request
type ReverseRequest struct {
	Text string `json:"text"`
}

// ReverseResponse represents a reverse response
type ReverseResponse struct {
	Reversed string `json:"reversed"`
}

// UpperRequest represents an uppercase request
type UpperRequest struct {
	Text string `json:"text"`
}

// UpperResponse represents an uppercase response
type UpperResponse struct {
	Result string `json:"result"`
}

// LowerRequest represents a lowercase request
type LowerRequest struct {
	Text string `json:"text"`
}

// LowerResponse represents a lowercase response
type LowerResponse struct {
	Result string `json:"result"`
}

// LengthRequest represents a length request
type LengthRequest struct {
	Text string `json:"text"`
}

// LengthResponse represents a length response
type LengthResponse struct {
	Length int `json:"length"`
}

// NewEchoService creates a new echo service
func NewEchoService() *EchoService {
	return &EchoService{
		BaseService: rpc.NewBaseService("echo"),
	}
}

// Register registers the echo service
func (s *EchoService) Register(server interface{}) error {
	s.SetMetadata("service_type", "echo")
	s.SetMetadata("version", "1.0.0")
	s.SetMetadata("registration_time", time.Now())
	return nil
}

// Echo repeats the message specified number of times
func (s *EchoService) Echo(ctx context.Context, req *EchoRequest) (*EchoResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Message == "" {
		return nil, status.Error(codes.InvalidArgument, "message is required")
	}
	
	if req.Times <= 0 {
		req.Times = 1 // default to 1 time
	}
	
	if req.Times > 100 {
		return nil, status.Error(codes.InvalidArgument, "times cannot exceed 100")
	}
	
	var result string
	if req.Times == 1 {
		result = req.Message
	} else {
		result = strings.Repeat(req.Message+" ", req.Times)
		result = strings.TrimSpace(result)
	}
	
	return &EchoResponse{
		Message: result,
		Count:   req.Times,
	}, nil
}

// Reverse reverses the input text
func (s *EchoService) Reverse(ctx context.Context, req *ReverseRequest) (*ReverseResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	
	// Reverse the string
	runes := []rune(req.Text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	
	reversed := string(runes)
	
	return &ReverseResponse{
		Reversed: reversed,
	}, nil
}

// Upper converts text to uppercase
func (s *EchoService) Upper(ctx context.Context, req *UpperRequest) (*UpperResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	
	result := strings.ToUpper(req.Text)
	
	return &UpperResponse{
		Result: result,
	}, nil
}

// Lower converts text to lowercase
func (s *EchoService) Lower(ctx context.Context, req *LowerRequest) (*LowerResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	
	result := strings.ToLower(req.Text)
	
	return &LowerResponse{
		Result: result,
	}, nil
}

// Length returns the length of the text
func (s *EchoService) Length(ctx context.Context, req *LengthRequest) (*LengthResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	
	length := len([]rune(req.Text)) // Count runes, not bytes, for proper Unicode support
	
	return &LengthResponse{
		Length: length,
	}, nil
}

// GetStats returns service statistics
func (s *EchoService) GetStats(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"service_name": s.Name(),
		"uptime":       s.Uptime().String(),
		"started_at":   s.StartTime(),
		"last_activity": time.Now(),
		"methods": []string{
			"echo",
			"reverse", 
			"upper",
			"lower",
			"length",
		},
	}, nil
}

// Health returns the health status of the echo service
func (s *EchoService) Health(ctx context.Context, req interface{}) (interface{}, error) {
	health := s.BaseService.Health()
	health.Metadata["service_type"] = "echo"
	health.Metadata["available_methods"] = []string{"echo", "reverse", "upper", "lower", "length"}
	return health, nil
}
