package gateway

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTracingConfiguration(t *testing.T) {
	config := DefaultConfig()
	
	// Test default tracing configuration
	assert.NotNil(t, config.Tracing)
	assert.Equal(t, "jaeger", config.Tracing.Type)
	assert.Equal(t, "gateway", config.Tracing.ServiceName)
	assert.False(t, config.Tracing.Enabled) // Default disabled
	assert.Equal(t, 6831, config.Tracing.Port)
	assert.Equal(t, 1.0, config.Tracing.SampleRate)
}

func TestProtocolConfiguration(t *testing.T) {
	config := DefaultConfig()
	
	// Test default protocol configuration
	assert.True(t, config.Protocols.HTTP)
	assert.True(t, config.Protocols.HTTP2)
	assert.False(t, config.Protocols.GRPC)
	assert.False(t, config.Protocols.JSONRPC)
	
	// Test gRPC configuration
	assert.False(t, config.Protocols.GRPCConfig.EnableTLS)
	assert.Equal(t, 30*time.Second, config.Protocols.GRPCConfig.Timeout)
	
	// Test JSON-RPC configuration
	assert.Equal(t, "2.0", config.Protocols.JSONRPCConfig.Version)
	assert.False(t, config.Protocols.JSONRPCConfig.EnableBatch)
	assert.Equal(t, 30*time.Second, config.Protocols.JSONRPCConfig.Timeout)
}

func TestGatewayWithTracing(t *testing.T) {
	config := DefaultConfig()
	config.Tracing.Enabled = true
	config.Tracing.Type = "zipkin" // Use implemented tracer
	config.Tracing.Port = 9411     // Zipkin default port
	
	gw := NewGateway(config)
	assert.NotNil(t, gw)
	
	// Initialize gateway to setup tracing
	err := gw.Initialize()
	assert.NoError(t, err)
	assert.NotNil(t, gw.tracer)
	assert.True(t, gw.tracingEnabled())
}

func TestGatewayWithProtocols(t *testing.T) {
	config := DefaultConfig()
	config.Protocols.GRPC = true
	config.Protocols.JSONRPC = true
	
	gw := NewGateway(config)
	assert.NotNil(t, gw)
	
	// Initialize gateway to setup protocol proxies
	err := gw.Initialize()
	assert.NoError(t, err)
	assert.NotNil(t, gw.grpcProxy)
	assert.NotNil(t, gw.jsonProxy)
}

func TestRouteWithProtocol(t *testing.T) {
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:     "/api/test",
			Method:   "GET",
			Service:  "test-service",
			Protocol: "http",
			Targets:  []string{"http://localhost:8080"},
		},
		{
			Path:     "/grpc/test",
			Method:   "POST",
			Service:  "grpc-service",
			Protocol: "grpc",
			Targets:  []string{"grpc://localhost:50051"},
		},
		{
			Path:     "/rpc/test",
			Method:   "POST",
			Service:  "jsonrpc-service",
			Protocol: "jsonrpc",
			Targets:  []string{"http://localhost:8080/rpc"},
		},
	}
	
	gw := NewGateway(config)
	assert.NotNil(t, gw)
	
	// Test route initialization
	err := gw.initRoutes()
	assert.NoError(t, err)
	
	// Verify routes were created
	router := gw.GetRouter()
	assert.NotNil(t, router)
}

func TestTracingMiddleware(t *testing.T) {
	config := DefaultConfig()
	config.Tracing.Enabled = true
	
	gw := NewGateway(config)
	assert.NotNil(t, gw)
	
	// Test tracing middleware creation
	middleware := gw.TracingMiddleware()
	assert.NotNil(t, middleware)
}

func TestInjectTracingHeaders(t *testing.T) {
	config := DefaultConfig()
	config.Tracing.Enabled = true
	
	gw := NewGateway(config)
	assert.NotNil(t, gw)
	
	// Test tracing headers injection
	req, err := http.NewRequest("GET", "http://localhost:8080/test", nil)
	assert.NoError(t, err)
	
	gw.InjectTracingHeaders(req, req.Context())
	// Should not panic and headers should be added if tracing is enabled
}
