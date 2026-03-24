// Package rpc provides a unified RPC framework for gRPC and JSON-RPC
package rpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/alldev-run/golang-gin-rpc/pkg/ratelimiter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
)

// ServerType represents the type of RPC server
type ServerType string

const (
	ServerTypeGRPC    ServerType = "grpc"
	ServerTypeJSONRPC ServerType = "jsonrpc"
)

// Config holds RPC server configuration
type Config struct {
	Type        ServerType `yaml:"type" json:"type"`
	Host        string     `yaml:"host" json:"host"`
	Port        int        `yaml:"port" json:"port"`
	Network     string     `yaml:"network" json:"network"` // tcp, unix
	Timeout     int        `yaml:"timeout" json:"timeout"`   // seconds
	MaxMsgSize  int        `yaml:"max_msg_size" json:"max_msg_size"`
	EnableTLS   bool       `yaml:"enable_tls" json:"enable_tls"`
	CertFile    string     `yaml:"cert_file" json:"cert_file"`
	KeyFile     string     `yaml:"key_file" json:"key_file"`
	Reflection  bool       `yaml:"reflection" json:"reflection"` // gRPC reflection
}

// DefaultConfig returns default RPC configuration
func DefaultConfig() Config {
	return Config{
		Type:       ServerTypeGRPC,
		Host:       "localhost",
		Port:       50051,
		Network:    "tcp",
		Timeout:    30,
		MaxMsgSize: 4 * 1024 * 1024, // 4MB
		EnableTLS:  false,
		Reflection: true,
	}
}

// Service represents an RPC service
type Service interface {
	Name() string
	Register(server interface{}) error
}

// Server represents the RPC server interface
type Server interface {
	Start() error
	Stop() error
	Addr() string
	Type() ServerType
	RegisterService(service Service) error
}

type startPreflightChecker interface {
	PreflightCheck() error
}

// GRPCServer wraps gRPC server
type GRPCServer struct {
	config      *Config
	server      *grpc.Server
	services    []Service
	tracing     *tracing.GRPCInterceptor
	auth        *RPCAuth
	degradation *DegradationManager
	rateLimiter *ratelimiter.Manager
	observer    clientObserver
}

// JSONRPCServer wraps JSON-RPC server
type JSONRPCServer struct {
	config      *Config
	server      *http.Server
	engine      *gin.Engine
	services    map[string]interface{}
	tracing     *tracing.JSONRPCInterceptor
	auth        *RPCAuth
	degradation *DegradationManager
	rateLimiter *ratelimiter.Manager
	observer    clientObserver
	routesSetup bool
}

// NewServer creates a new RPC server based on configuration
func NewServer(config Config) (Server, error) {
	switch config.Type {
	case ServerTypeGRPC:
		return NewGRPCServer(config)
	case ServerTypeJSONRPC:
		return NewJSONRPCServer(config), nil
	default:
		return nil, fmt.Errorf("unsupported server type: %s", config.Type)
	}
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(config Config) (*GRPCServer, error) {
	tracingInterceptor := tracing.NewGRPCInterceptor(tracing.GlobalTracer())
	auth := NewRPCAuth(DefaultAuthConfig())
	grpcServer := &GRPCServer{
		config:      &config,
		services:    []Service{},
		tracing:     tracingInterceptor,
		auth:        auth,
		rateLimiter: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
	}
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
		grpc.ChainUnaryInterceptor(
			tracingInterceptor.UnaryServerInterceptor(),
			grpcServer.governanceUnaryInterceptor(),
		),
	}
	if config.EnableTLS {
		if config.CertFile == "" || config.KeyFile == "" {
			return nil, fmt.Errorf("grpc TLS enabled but cert_file/key_file not configured")
		}
		creds, err := credentials.NewServerTLSFromFile(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load grpc TLS certs: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	grpcServer.server = grpc.NewServer(opts...)
	return grpcServer, nil
}

// NewJSONRPCServer creates a new JSON-RPC server
func NewJSONRPCServer(config Config) *JSONRPCServer {
	if !strings.HasPrefix(config.Host, ":") {
		config.Host = ":" + fmt.Sprintf("%d", config.Port)
	}

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	// Initialize tracing interceptor
	tracingInterceptor := tracing.NewJSONRPCInterceptor(tracing.GlobalTracer())
	
	// Add tracing middleware
	engine.Use(tracingInterceptor.Middleware())

	// Initialize authentication
	authConfig := DefaultAuthConfig()
	auth := NewRPCAuth(authConfig)

	return &JSONRPCServer{
		config:     &config,
		server:     &http.Server{Addr: config.Host, Handler: engine},
		engine:     engine,
		services:   make(map[string]interface{}),
		tracing:    tracingInterceptor,
		auth:       auth,
		rateLimiter: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
	}
}

// RegisterService registers a service with the server
func (s *GRPCServer) RegisterService(service Service) error {
	if err := service.Register(s.server); err != nil {
		return fmt.Errorf("failed to register gRPC service %s: %w", service.Name(), err)
	}
	s.services = append(s.services, service)
	return nil
}

// RegisterService registers a service with the JSON-RPC server
func (s *JSONRPCServer) RegisterService(service Service) error {
	s.services[service.Name()] = service
	s.setupRoutes()
	return nil
}

// Start starts the gRPC server
func (s *GRPCServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	lis, err := net.Listen(s.config.Network, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Enable reflection for development
	if s.config.Reflection {
		reflection.Register(s.server)
	}

	return s.server.Serve(lis)
}

func (s *GRPCServer) PreflightCheck() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	lis, err := net.Listen(s.config.Network, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	_ = lis.Close()
	return nil
}

func (s *GRPCServer) governanceUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		statusText := "ok"
		ctx = SetRPCMethodInContext(ctx, info.FullMethod)
		defer func() {
			if s.observer != nil {
				s.observer.RecordRequest(ClientTypeGRPC, info.FullMethod, s.Addr(), statusText, time.Since(start))
			}
		}()
		if gov, ok := s.observer.(governanceObserver); ok {
			gov.RecordGovernance(ClientTypeGRPC, info.FullMethod, "method_latency")
		}

		headerName := "x-api-key"
		if s.auth != nil && s.auth.config.HeaderName != "" {
			headerName = strings.ToLower(s.auth.config.HeaderName)
		}
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(headerName); len(values) > 0 && values[0] != "" {
				apiKey := values[0]
				ctx = SetAPIKeyInContext(ctx, apiKey)
				if s.auth != nil && s.auth.HasAPIKey(apiKey) {
					if user, exists := s.auth.config.APIKeys[apiKey]; exists {
						ctx = SetAPIUserInContext(ctx, user)
					}
				}
			}
		}

		if s.auth != nil && s.auth.config.Enabled && !s.auth.ShouldSkipAuth(info.FullMethod) {
			if gov, ok := s.observer.(governanceObserver); ok {
				gov.RecordGovernance(ClientTypeGRPC, info.FullMethod, "auth_attempt")
			}
			if !s.auth.IsAuthenticated(ctx) {
				statusText = "unauthenticated"
				if gov, ok := s.observer.(governanceObserver); ok {
					gov.RecordGovernance(ClientTypeGRPC, info.FullMethod, "auth_reject")
				}
				return nil, status.Error(codes.Unauthenticated, "API key is required")
			}
		}

		if s.rateLimiter != nil && !s.rateLimiter.Allow(info.FullMethod) {
			statusText = "rate_limited"
			if gov, ok := s.observer.(governanceObserver); ok {
				gov.RecordGovernance(ClientTypeGRPC, info.FullMethod, "rate_limit_reject")
			}
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		var err error
		if s.degradation != nil {
			if !s.degradation.ShouldAllowMethod(info.FullMethod) {
				if fallback, ok := s.degradation.GetFallback(info.FullMethod); ok {
					result, fallbackErr := fallback(ctx, info.FullMethod, req)
					if fallbackErr != nil {
						statusText = "degraded_error"
						return nil, status.Error(codes.Unavailable, fallbackErr.Error())
					}
					statusText = "degraded_fallback"
					if gov, ok := s.observer.(governanceObserver); ok {
						gov.RecordGovernance(ClientTypeGRPC, info.FullMethod, "degrade_fallback")
					}
					return result, nil
				}
				statusText = "degraded_reject"
				if gov, ok := s.observer.(governanceObserver); ok {
					gov.RecordGovernance(ClientTypeGRPC, info.FullMethod, "degrade_reject")
				}
				return nil, status.Error(codes.Unavailable, "service temporarily unavailable due to degradation")
			}

			start := time.Now()
			defer func() {
				s.degradation.RecordMetrics(time.Since(start), err)
			}()
		}

		resp, handlerErr := handler(ctx, req)
		err = handlerErr
		if handlerErr != nil {
			statusText = "handler_error"
		}
		return resp, handlerErr
	}
}

// Start starts the JSON-RPC server
func (s *JSONRPCServer) Start() error {
	s.setupRoutes()
	if s.config.EnableTLS {
		if s.config.CertFile == "" || s.config.KeyFile == "" {
			return fmt.Errorf("jsonrpc TLS enabled but cert_file/key_file not configured")
		}
		return s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}
	return s.server.ListenAndServe()
}

func (s *JSONRPCServer) PreflightCheck() error {
	lis, err := net.Listen(s.config.Network, s.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.server.Addr, err)
	}
	_ = lis.Close()
	return nil
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() error {
	s.server.GracefulStop()
	return nil
}

// Stop stops the JSON-RPC server
func (s *JSONRPCServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.Timeout)*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Addr returns the server address
func (s *GRPCServer) Addr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// Addr returns the server address
func (s *JSONRPCServer) Addr() string {
	return s.config.Host
}

// Type returns the server type
func (s *GRPCServer) Type() ServerType {
	return ServerTypeGRPC
}

// Type returns the server type
func (s *JSONRPCServer) Type() ServerType {
	return ServerTypeJSONRPC
}

// setupRoutes sets up JSON-RPC routes
func (s *JSONRPCServer) setupRoutes() {
	if s.routesSetup {
		return
	}
	// Wrap the handler with tracing
	s.engine.GET("/health", s.healthCheck)
	s.engine.GET("/ready", s.readyCheck)
	s.engine.POST("/rpc", s.tracing.WrapHandler("jsonrpc.request", s.handleJSONRPC))
	s.engine.GET("/rpc", s.tracing.WrapHandler("jsonrpc.request", s.handleJSONRPC)) // Support GET for simple requests
	s.routesSetup = true
}

func (s *JSONRPCServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"type":   string(s.Type()),
		"addr":   s.Addr(),
	})
}

func (s *JSONRPCServer) readyCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"type":   string(s.Type()),
		"addr":   s.Addr(),
	})
}

// handleJSONRPC handles JSON-RPC requests
func (s *JSONRPCServer) handleJSONRPC(c *gin.Context) {
	start := time.Now()
	statusText := "ok"
	methodName := ""
	defer func() {
		if s.observer != nil {
			s.observer.RecordRequest(ClientTypeJSONRPC, methodName, c.FullPath(), statusText, time.Since(start))
		}
	}()
	if gov, ok := s.observer.(governanceObserver); ok {
		gov.RecordGovernance(ClientTypeJSONRPC, methodName, "method_latency")
	}

	// Extract API key from request
	headerName := s.auth.config.HeaderName
	if headerName == "" {
		headerName = "X-API-Key"
	}
	queryName := s.auth.config.QueryName
	if queryName == "" {
		queryName = "api_key"
	}
	apiKey := c.GetHeader(headerName)
	if apiKey == "" {
		apiKey = c.Query(queryName)
	}

	// Set API key in context for RPC authentication
	ctx := c.Request.Context()
	if apiKey != "" {
		ctx = SetAPIKeyInContext(ctx, apiKey)
		// Set user info if key is valid
		if s.auth.HasAPIKey(apiKey) {
			if user, exists := s.auth.config.APIKeys[apiKey]; exists {
				ctx = SetAPIUserInContext(ctx, user)
			}
		}
	}

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		statusText = "parse_error"
		c.JSON(http.StatusOK, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
				Data:    err.Error(),
			},
			ID: nil,
		})
		return
	}

	// Update request context after the method has been parsed
	methodName = req.Method
	if gov, ok := s.observer.(governanceObserver); ok {
		gov.RecordGovernance(ClientTypeJSONRPC, req.Method, "method_latency")
	}
	ctx = SetRPCMethodInContext(ctx, req.Method)
	c.Request = c.Request.WithContext(ctx)

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		statusText = "invalid_request"
		c.JSON(http.StatusOK, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "jsonrpc version must be 2.0",
			},
			ID: req.ID,
		})
		return
	}

	// Validate method
	if req.Method == "" {
		statusText = "invalid_request"
		c.JSON(http.StatusOK, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
				Data:    "method is required",
			},
			ID: req.ID,
		})
		return
	}

	// Check authentication (skip for system methods)
	if s.auth.config.Enabled && !s.auth.ShouldSkipAuth(req.Method) {
		if gov, ok := s.observer.(governanceObserver); ok {
			gov.RecordGovernance(ClientTypeJSONRPC, req.Method, "auth_attempt")
		}
		if !s.auth.IsAuthenticated(ctx) {
			statusText = "unauthorized"
			if gov, ok := s.observer.(governanceObserver); ok {
				gov.RecordGovernance(ClientTypeJSONRPC, req.Method, "auth_reject")
			}
			c.JSON(http.StatusOK, JSONRPCResponse{
				JSONRPC: "2.0",
				Error: &JSONRPCError{
					Code:    -32601,
					Message: "Unauthorized",
					Data:    "API key is required",
				},
				ID: req.ID,
			})
			return
		}
	}

	// Check degradation
	var result interface{}
	var err error
	if s.rateLimiter != nil && !s.rateLimiter.Allow(req.Method) {
		statusText = "rate_limited"
		if gov, ok := s.observer.(governanceObserver); ok {
			gov.RecordGovernance(ClientTypeJSONRPC, req.Method, "rate_limit_reject")
		}
		c.JSON(http.StatusTooManyRequests, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    ServerError,
				Message: "Rate limit exceeded",
			},
			ID: req.ID,
		})
		return
	}

	if s.degradation != nil {
		// Check if method should be allowed
		if !s.degradation.ShouldAllowMethod(req.Method) {
			// Try to get fallback
			if fallback, ok := s.degradation.GetFallback(req.Method); ok {
				result, err = fallback(ctx, req.Method, req.Params)
				if err != nil {
					statusText = "degraded_error"
					c.JSON(http.StatusOK, JSONRPCResponse{
						JSONRPC: "2.0",
						Error: &JSONRPCError{
							Code:    -32603,
							Message: "Internal error",
							Data:    err.Error(),
						},
						ID: req.ID,
					})
					return
				}
				statusText = "degraded_fallback"
				if gov, ok := s.observer.(governanceObserver); ok {
					gov.RecordGovernance(ClientTypeJSONRPC, req.Method, "degrade_fallback")
				}
				c.JSON(http.StatusOK, JSONRPCResponse{
					JSONRPC: "2.0",
					Result:  result,
					ID:      req.ID,
				})
				return
			}
			statusText = "degraded_reject"
			if gov, ok := s.observer.(governanceObserver); ok {
				gov.RecordGovernance(ClientTypeJSONRPC, req.Method, "degrade_reject")
			}
			c.JSON(http.StatusOK, JSONRPCResponse{
				JSONRPC: "2.0",
				Error: &JSONRPCError{
					Code:    -32603,
					Message: "Service temporarily unavailable due to degradation",
				},
				ID: req.ID,
			})
			return
		}
		
		// Record metrics
		start := time.Now()
		defer func() {
			s.degradation.RecordMetrics(time.Since(start), err)
		}()
	}

	// Execute the method
	result, err = s.executeMethod(ctx, req.Method, req.Params)
	if err != nil {
		// Check if it's a method not found error
		if err == ErrMethodNotFound {
			statusText = "method_not_found"
			c.JSON(http.StatusOK, JSONRPCResponse{
				JSONRPC: "2.0",
				Error: &JSONRPCError{
					Code:    -32601,
					Message: "Method not found",
					Data:    req.Method,
				},
				ID: req.ID,
			})
			return
		}

		// Internal error
		statusText = "internal_error"
		c.JSON(http.StatusOK, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			},
			ID: req.ID,
		})
		return
	}

	// Return success response
	c.JSON(http.StatusOK, JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	})
}

// JSON-RPC 2.0 types
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	Result  interface{}    `json:"result,omitempty"`
	Error   *JSONRPCError  `json:"error,omitempty"`
	ID      interface{}    `json:"id,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error codes for JSON-RPC 2.0
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	ServerError    = -32000
)

// ErrMethodNotFound is returned when a method is not found
var ErrMethodNotFound = fmt.Errorf("method not found")

// executeMethod executes a registered method
func (s *JSONRPCServer) executeMethod(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// Parse method name (format: "service.method")
	parts := strings.Split(method, ".")
	if len(parts) != 2 {
		return nil, ErrMethodNotFound
	}

	serviceName := parts[0]
	methodName := parts[1]

	// Find the service
	service, exists := s.services[serviceName]
	if !exists {
		return nil, ErrMethodNotFound
	}

	// Call the method using reflection
	return s.callMethod(ctx, service, methodName, params)
}

// callMethod calls a method on a service using reflection
func (s *JSONRPCServer) callMethod(ctx context.Context, service interface{}, methodName string, params interface{}) (interface{}, error) {
	// Get service type
	v := reflect.ValueOf(service)
	method := v.MethodByName(methodName)

	if !method.IsValid() {
		return nil, ErrMethodNotFound
	}

	// Prepare arguments
	// Most JSON-RPC methods expect (context, params) or just (params)
	methodType := method.Type()
	numArgs := methodType.NumIn()
	args := make([]reflect.Value, numArgs)

	for i := 0; i < numArgs; i++ {
		argType := methodType.In(i)

		// Check if first argument is context.Context
		if i == 0 && argType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			args[i] = reflect.ValueOf(ctx)
		} else {
			// Assume params
			if params != nil {
				// Convert params to the expected type
				paramValue := reflect.ValueOf(params)
				if paramValue.Type().ConvertibleTo(argType) {
					args[i] = paramValue.Convert(argType)
				} else {
					args[i] = reflect.Zero(argType)
				}
			} else {
				args[i] = reflect.Zero(argType)
			}
		}
	}

	// Call the method
	results := method.Call(args)

	// Handle results
	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		// Single return value - could be error or result
		if results[0].Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			if !results[0].IsNil() {
				return nil, results[0].Interface().(error)
			}
			return nil, nil
		}
		return results[0].Interface(), nil
	}

	// Two return values: (result, error)
	result := results[0].Interface()
	if !results[1].IsNil() {
		return result, results[1].Interface().(error)
	}

	return result, nil
}

// GetServices returns all registered services
func (s *GRPCServer) GetServices() []Service {
	return s.services
}

// GetServices returns all registered services
func (s *JSONRPCServer) GetServices() map[string]interface{} {
	return s.services
}

// SetAuthConfig sets the authentication configuration
func (s *GRPCServer) SetAuthConfig(config AuthConfig) {
	s.auth = NewRPCAuth(config)
}

// SetAuthConfig sets the authentication configuration
func (s *JSONRPCServer) SetAuthConfig(config AuthConfig) {
	s.auth = NewRPCAuth(config)
}

// GetAuthConfig returns the current authentication configuration
func (s *GRPCServer) GetAuthConfig() *RPCAuth {
	return s.auth
}

// GetAuthConfig returns the current authentication configuration
func (s *JSONRPCServer) GetAuthConfig() *RPCAuth {
	return s.auth
}

// AddAPIKey adds an API key for authentication
func (s *JSONRPCServer) AddAPIKey(key, description string) {
	s.auth.AddAPIKey(key, description)
}

// RemoveAPIKey removes an API key from authentication
func (s *JSONRPCServer) RemoveAPIKey(key string) {
	s.auth.RemoveAPIKey(key)
}

// EnableAuth enables authentication with default configuration
func (s *JSONRPCServer) EnableAuth() {
	config := DefaultAuthConfig()
	config.Enabled = true
	s.auth = NewRPCAuth(config)
}

// DisableAuth disables authentication
func (s *JSONRPCServer) DisableAuth() {
	s.auth.config.Enabled = false
}

// SetDegradationManager sets the degradation manager for the server
func (s *GRPCServer) SetDegradationManager(dm *DegradationManager) {
	s.degradation = dm
}

// SetDegradationManager sets the degradation manager for the server
func (s *JSONRPCServer) SetDegradationManager(dm *DegradationManager) {
	s.degradation = dm
}

// SetRateLimiterManager sets the rate limiter manager for JSON-RPC server
func (s *GRPCServer) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	s.rateLimiter = rlm
}

func (s *GRPCServer) SetObserver(observer clientObserver) {
	s.observer = observer
}

// SetRateLimiterManager sets the rate limiter manager for JSON-RPC server
func (s *JSONRPCServer) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	s.rateLimiter = rlm
}

func (s *JSONRPCServer) SetObserver(observer clientObserver) {
	s.observer = observer
}

// ServiceInfo provides information about registered services
type ServiceInfo struct {
	Name    string      `json:"name"`
	Type    ServerType  `json:"type"`
	Methods []string    `json:"methods"`
}

// GetServiceInfo returns information about all services
func GetServiceInfo(server Server) []ServiceInfo {
	var infos []ServiceInfo
	
	switch server.Type() {
	case ServerTypeGRPC:
		if grpcServer, ok := server.(*GRPCServer); ok {
			for _, service := range grpcServer.GetServices() {
				infos = append(infos, ServiceInfo{
					Name:    service.Name(),
					Type:    ServerTypeGRPC,
					Methods: getServiceMethods(service),
				})
			}
		}
	case ServerTypeJSONRPC:
		if jsonServer, ok := server.(*JSONRPCServer); ok {
			for name, service := range jsonServer.GetServices() {
				infos = append(infos, ServiceInfo{
					Name:    name,
					Type:    ServerTypeJSONRPC,
					Methods: getServiceMethods(service),
				})
			}
		}
	}
	
	return infos
}

// getServiceMethods extracts method names from a service using reflection
func getServiceMethods(service interface{}) []string {
	var methods []string
	serviceType := reflect.TypeOf(service)
	
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		// Only include exported methods
		if method.PkgPath == "" {
			methods = append(methods, method.Name)
		}
	}
	
	return methods
}
