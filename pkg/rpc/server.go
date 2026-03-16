// Package rpc provides a unified RPC framework for gRPC and JSON-RPC
package rpc

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

// GRPCServer wraps gRPC server
type GRPCServer struct {
	config *Config
	server *grpc.Server
	services []Service
}

// JSONRPCServer wraps JSON-RPC server
type JSONRPCServer struct {
	config *Config
	server *http.Server
	engine *gin.Engine
	services map[string]interface{}
}

// NewServer creates a new RPC server based on configuration
func NewServer(config Config) Server {
	switch config.Type {
	case ServerTypeGRPC:
		return NewGRPCServer(config)
	case ServerTypeJSONRPC:
		return NewJSONRPCServer(config)
	default:
		panic(fmt.Sprintf("unsupported server type: %s", config.Type))
	}
}

// NewGRPCServer creates a new gRPC server
func NewGRPCServer(config Config) *GRPCServer {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
	}

	return &GRPCServer{
		config:    &config,
		server:    grpc.NewServer(opts...),
		services: []Service{},
	}
}

// NewJSONRPCServer creates a new JSON-RPC server
func NewJSONRPCServer(config Config) *JSONRPCServer {
	if !strings.HasPrefix(config.Host, ":") {
		config.Host = ":" + fmt.Sprintf("%d", config.Port)
	}

	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	return &JSONRPCServer{
		config:    &config,
		server:    &http.Server{Addr: config.Host},
		engine:    engine,
		services: make(map[string]interface{}),
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

// Start starts the JSON-RPC server
func (s *JSONRPCServer) Start() error {
	s.setupRoutes()
	return s.server.ListenAndServe()
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() error {
	s.server.GracefulStop()
	return nil
}

// Stop stops the JSON-RPC server
func (s *JSONRPCServer) Stop() error {
	return s.server.Close()
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
	s.engine.POST("/rpc", s.handleJSONRPC)
	s.engine.GET("/rpc", s.handleJSONRPC) // Support GET for simple requests
}

// handleJSONRPC handles JSON-RPC requests
func (s *JSONRPCServer) handleJSONRPC(c *gin.Context) {
	// TODO: Implement JSON-RPC 2.0 protocol handler
	c.JSON(http.StatusOK, gin.H{
		"jsonrpc": "2.0",
		"result":  "JSON-RPC server is running",
		"id":      nil,
	})
}

// GetServices returns all registered services
func (s *GRPCServer) GetServices() []Service {
	return s.services
}

// GetServices returns all registered services
func (s *JSONRPCServer) GetServices() map[string]interface{} {
	return s.services
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
