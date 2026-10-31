// Package rpc provides base service implementations for RPC servers
package rpc

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BaseService provides common functionality for RPC services
type BaseService struct {
	name     string
	metadata map[string]interface{}
	mu       sync.RWMutex
	started  time.Time
}

// NewBaseService creates a new base service
func NewBaseService(name string) *BaseService {
	return &BaseService{
		name:     name,
		metadata: make(map[string]interface{}),
		started:  time.Now(),
	}
}

// Name returns the service name
func (s *BaseService) Name() string {
	return s.name
}

// SetMetadata sets metadata for the service
func (s *BaseService) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
}

// GetMetadata gets metadata from the service
func (s *BaseService) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.metadata[key]
	return value, exists
}

// GetAllMetadata returns all metadata
func (s *BaseService) GetAllMetadata() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	metadata := make(map[string]interface{})
	for k, v := range s.metadata {
		metadata[k] = v
	}
	return metadata
}

// StartTime returns when the service was created
func (s *BaseService) StartTime() time.Time {
	return s.started
}

// Uptime returns the service uptime
func (s *BaseService) Uptime() time.Duration {
	return time.Since(s.started)
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Status    string                 `json:"status"`
	Uptime    time.Duration          `json:"uptime"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Health returns the health status of the service
func (s *BaseService) Health() HealthStatus {
	return HealthStatus{
		Status:    "healthy",
		Uptime:    s.Uptime(),
		Timestamp: time.Now(),
		Metadata:  s.GetAllMetadata(),
	}
}

// MethodInfo represents information about a service method
type MethodInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputType    string      `json:"input_type"`
	OutputType   string      `json:"output_type"`
	Parameters  []Parameter `json:"parameters"`
}

// Parameter represents a method parameter
type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// GetMethods returns information about all methods in the service
func (s *BaseService) GetMethods() []MethodInfo {
	serviceType := reflect.TypeOf(s)
	var methods []MethodInfo
	
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		
		// Only include exported methods that start with capital letters
		if !method.IsExported() || strings.HasPrefix(method.Name, "Get") && 
		   (strings.HasSuffix(method.Name, "Metadata") || strings.HasSuffix(method.Name, "Methods")) {
			continue
		}
		
		methodInfo := MethodInfo{
			Name:        method.Name,
			Description: fmt.Sprintf("Method %s of service %s", method.Name, s.name),
			InputType:    getMethodInputType(method),
			OutputType:   getMethodOutputType(method),
		}
		
		methods = append(methods, methodInfo)
	}
	
	return methods
}

// getMethodInputType extracts input type information from a method
func getMethodInputType(method reflect.Method) string {
	if method.Type.NumIn() > 1 {
		return method.Type.In(1).String()
	}
	return "void"
}

// getMethodOutputType extracts output type information from a method
func getMethodOutputType(method reflect.Method) string {
	if method.Type.NumOut() > 0 {
		return method.Type.Out(0).String()
	}
	return "void"
}

// SystemService provides system-level RPC methods
type SystemService struct {
	*BaseService
}

// NewSystemService creates a new system service
func NewSystemService() *SystemService {
	return &SystemService{
		BaseService: NewBaseService("system"),
	}
}

// Register registers the system service with a gRPC server
func (s *SystemService) Register(server interface{}) error {
	// This would be implemented by specific gRPC service registration
	// For now, we'll just return nil as a placeholder
	return nil
}

// Health returns the health status of the service
func (s *SystemService) Health(ctx context.Context, req interface{}) (interface{}, error) {
	return s.BaseService.Health(), nil
}

// Ping returns a simple pong response
func (s *SystemService) Ping(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"message": "pong",
		"service": s.Name(),
		"time":    time.Now().Unix(),
	}, nil
}

// Info returns information about the service
func (s *SystemService) Info(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"name":    s.Name(),
		"uptime":  s.Uptime().String(),
		"methods": s.GetMethods(),
		"metadata": s.GetAllMetadata(),
	}, nil
}

// ListMethods returns a list of available methods
func (s *SystemService) ListMethods(ctx context.Context, req interface{}) (interface{}, error) {
	return s.GetMethods(), nil
}

// JSONRPCHandler provides a base handler for JSON-RPC methods
type JSONRPCHandler struct {
	service interface{}
}

// NewJSONRPCHandler creates a new JSON-RPC handler
func NewJSONRPCHandler(service interface{}) *JSONRPCHandler {
	return &JSONRPCHandler{service: service}
}

// Handle handles a JSON-RPC method call
func (h *JSONRPCHandler) Handle(c *gin.Context, method string, params interface{}) (interface{}, error) {
	ctx := c.Request.Context()
	
	// Use reflection to call the method
	serviceValue := reflect.ValueOf(h.service)
	methodValue := serviceValue.MethodByName(method)
	
	if !methodValue.IsValid() {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("method not found: %s", method))
	}
	
	// Prepare arguments
	var args []reflect.Value
	args = append(args, reflect.ValueOf(ctx))
	
	if params != nil {
		args = append(args, reflect.ValueOf(params))
	} else {
		args = append(args, reflect.ValueOf(nil))
	}
	
	// Call the method
	results := methodValue.Call(args)
	
	// Handle results
	if len(results) == 0 {
		return nil, nil
	}
	
	if len(results) == 1 {
		if err, ok := results[0].Interface().(error); ok && err != nil {
			return nil, err
		}
		return results[0].Interface(), nil
	}
	
	if len(results) == 2 {
		if err, ok := results[1].Interface().(error); ok && err != nil {
			return nil, err
		}
		return results[0].Interface(), nil
	}
	
	return nil, fmt.Errorf("unexpected number of return values: %d", len(results))
}

// ServiceRegistry manages multiple RPC services
type ServiceRegistry struct {
	services map[string]interface{}
	mu       sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]interface{}),
	}
}

// Register registers a service
func (r *ServiceRegistry) Register(name string, service interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[name] = service
}

// Get gets a service by name
func (r *ServiceRegistry) Get(name string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	service, exists := r.services[name]
	return service, exists
}

// List returns all registered service names
func (r *ServiceRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.services))
	for name := range r.services {
		names = append(names, name)
	}
	return names
}

// Unregister removes a service
func (r *ServiceRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.services, name)
}

// Size returns the number of registered services
func (r *ServiceRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.services)
}

// Clear removes all services
func (r *ServiceRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services = make(map[string]interface{})
}

// GetAll returns all registered services
func (r *ServiceRegistry) GetAll() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	services := make(map[string]interface{})
	for name, service := range r.services {
		services[name] = service
	}
	return services
}

// Middleware provides middleware functionality for RPC services
type Middleware struct {
	name string
	fn   func(ctx context.Context, req interface{}) (interface{}, error)
}

// NewMiddleware creates a new middleware
func NewMiddleware(name string, fn func(ctx context.Context, req interface{}) (interface{}, error)) *Middleware {
	return &Middleware{
		name: name,
		fn:   fn,
	}
}

// Name returns the middleware name
func (m *Middleware) Name() string {
	return m.name
}

// Execute executes the middleware
func (m *Middleware) Execute(ctx context.Context, req interface{}) (interface{}, error) {
	return m.fn(ctx, req)
}

// MiddlewareChain manages a chain of middleware
type MiddlewareChain struct {
	middleware []*Middleware
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middleware: make([]*Middleware, 0),
	}
}

// Add adds middleware to the chain
func (c *MiddlewareChain) Add(middleware *Middleware) {
	c.middleware = append(c.middleware, middleware)
}

// Execute executes all middleware in the chain
func (c *MiddlewareChain) Execute(ctx context.Context, req interface{}) (interface{}, error) {
	currentReq := req
	
	for _, middleware := range c.middleware {
		result, err := middleware.Execute(ctx, currentReq)
		if err != nil {
			return nil, err
		}
		currentReq = result
	}
	
	return currentReq, nil
}

// Size returns the number of middleware in the chain
func (c *MiddlewareChain) Size() int {
	return len(c.middleware)
}

// Clear removes all middleware
func (c *MiddlewareChain) Clear() {
	c.middleware = make([]*Middleware, 0)
}
