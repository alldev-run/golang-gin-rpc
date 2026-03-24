package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/pkg/rpc/grpc"
	"github.com/alldev-run/golang-gin-rpc/pkg/rpc/jsonrpc"
)

// GRPCProxy handles gRPC proxy requests using pkg/rpc
type GRPCProxy struct {
	gateway *Gateway
	clients map[string]*grpc.Client
}

// NewGRPCProxy creates a new gRPC proxy
func NewGRPCProxy(gateway *Gateway) *GRPCProxy {
	return &GRPCProxy{
		gateway: gateway,
		clients: make(map[string]*grpc.Client),
	}
}

// ProxyGRPC proxies gRPC requests using pkg/rpc/grpc
func (p *GRPCProxy) ProxyGRPC(c *gin.Context, route *Route) error {
	if !p.gateway.config.Protocols.GRPC {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "gRPC protocol not enabled"})
		return fmt.Errorf("gRPC protocol not enabled")
	}

	// Extract target from load balancer
	target, err := p.gateway.balancer.Select(route.targets)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available gRPC services"})
		return err
	}

	// Start tracing span for gRPC request
	if p.gateway.tracingEnabled() {
		spanName := fmt.Sprintf("gateway.grpc.%s", route.config.Service)
		ctx, span := p.gateway.tracer.StartSpan(c.Request.Context(), spanName)
		defer span.End()

		// Set gRPC specific attributes
		span.SetAttributes(
			attribute.String("grpc.target", target),
			attribute.String("grpc.service", route.config.Service),
			attribute.String("grpc.method", c.Request.Method),
		)

		c.Request = c.Request.WithContext(ctx)
	}

	// Get or create gRPC client
	client, err := p.getOrCreateGRPCClient(target)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to create gRPC client"})
		return err
	}

	// Handle gRPC-Web requests (JSON over HTTP)
	return p.handleGRPCWeb(c, client, route)
}

// handleGRPCWeb handles gRPC-Web requests using JSON-RPC format
func (p *GRPCProxy) handleGRPCWeb(c *gin.Context, client *grpc.Client, route *Route) error {
	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return err
	}

	// Parse JSON-RPC request (gRPC-Web uses JSON-RPC format)
	var rpcRequest map[string]interface{}
	if err := json.Unmarshal(body, &rpcRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON-RPC request"})
		return err
	}

	// Extract method and parameters
	method, ok := rpcRequest["method"].(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing method in JSON-RPC request"})
		return fmt.Errorf("missing method")
	}

	params, _ := rpcRequest["params"]
	id, _ := rpcRequest["id"]

	// Create response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
	}

	// For demonstration, create a simple response
	// In a real implementation, you would use gRPC reflection or protobuf definitions
	result := map[string]interface{}{
		"service": route.config.Service,
		"method":  method,
		"params":  params,
		"target":  client.Address(),
		"status":  "success",
	}

	response["result"] = result

	// Return JSON response
	c.JSON(http.StatusOK, response)
	return nil
}

// getOrCreateGRPCClient gets or creates a gRPC client using pkg/rpc/grpc
func (p *GRPCProxy) getOrCreateGRPCClient(target string) (*grpc.Client, error) {
	p.gateway.mu.Lock()
	defer p.gateway.mu.Unlock()

	if client, exists := p.clients[target]; exists {
		return client, nil
	}

	// Create gRPC client configuration
	config := grpc.DefaultClientConfig()
	config.Address = target
	config.Timeout = p.gateway.config.Protocols.GRPCConfig.Timeout
	config.EnableTLS = p.gateway.config.Protocols.GRPCConfig.EnableTLS
	if p.gateway.config.Protocols.GRPCConfig.ServerName != "" {
		config.ServerName = p.gateway.config.Protocols.GRPCConfig.ServerName
	}

	// Create gRPC client
	client, err := grpc.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	p.clients[target] = client
	return client, nil
}

// Close closes all gRPC clients
func (p *GRPCProxy) Close() error {
	logger.Info("Starting gRPC proxy close...")
	
	// 在关闭阶段，完全避免使用 Gateway 的锁
	// 直接操作 clients 映射，因为在关闭时不会有并发访问
	var lastErr error
	var clientCount int
	
	// 直接遍历并关闭客户端
	for target, client := range p.clients {
		clientCount++
		logger.Debug("Closing gRPC client", logger.String("target", target))
		if err := client.Close(); err != nil {
			logger.Debug("Failed to close gRPC client", logger.Error(err))
			lastErr = err
		}
		// 从映射中移除已关闭的客户端
		delete(p.clients, target)
	}
	
	logger.Info("Found gRPC clients to close", logger.Int("count", clientCount))
	logger.Info("gRPC proxy closed successfully")
	return lastErr
}

// JSONRPCProxy handles JSON-RPC proxy requests using pkg/rpc
type JSONRPCProxy struct {
	gateway *Gateway
	clients map[string]*jsonrpc.Client
}

// NewJSONRPCProxy creates a new JSON-RPC proxy
func NewJSONRPCProxy(gateway *Gateway) *JSONRPCProxy {
	return &JSONRPCProxy{
		gateway: gateway,
		clients: make(map[string]*jsonrpc.Client),
	}
}

// ProxyJSONRPC proxies JSON-RPC requests using pkg/rpc/jsonrpc
func (p *JSONRPCProxy) ProxyJSONRPC(c *gin.Context, route *Route) error {
	if !p.gateway.config.Protocols.JSONRPC {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "JSON-RPC protocol not enabled"})
		return fmt.Errorf("JSON-RPC protocol not enabled")
	}

	// Extract target from load balancer
	target, err := p.gateway.balancer.Select(route.targets)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available JSON-RPC services"})
		return err
	}

	// Start tracing span for JSON-RPC request
	if p.gateway.tracingEnabled() {
		spanName := fmt.Sprintf("gateway.jsonrpc.%s", route.config.Service)
		ctx, span := p.gateway.tracer.StartSpan(c.Request.Context(), spanName)
		defer span.End()

		// Set JSON-RPC specific attributes
		span.SetAttributes(
			attribute.String("jsonrpc.target", target),
			attribute.String("jsonrpc.service", route.config.Service),
			attribute.String("jsonrpc.version", p.gateway.config.Protocols.JSONRPCConfig.Version),
		)

		c.Request = c.Request.WithContext(ctx)
	}

	// Get or create JSON-RPC client
	client, err := p.getOrCreateJSONRPCClient(target)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to create JSON-RPC client"})
		return err
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return err
	}

	// Parse JSON-RPC request
	var request jsonrpc.Request
	if err := json.Unmarshal(body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON-RPC request"})
		return err
	}

	// Validate JSON-RPC request
	if request.JSONRPC != "2.0" && request.JSONRPC != "1.0" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported JSON-RPC version"})
		return fmt.Errorf("unsupported JSON-RPC version: %s", request.JSONRPC)
	}

	if request.Method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing method in JSON-RPC request"})
		return fmt.Errorf("missing method")
	}

	// Create context with timeout
	ctx := c.Request.Context()
	if p.gateway.config.Protocols.JSONRPCConfig.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.gateway.config.Protocols.JSONRPCConfig.Timeout)
		defer cancel()
	}

	// Call JSON-RPC method
	var result interface{}
	err = client.Call(ctx, request.Method, request.Params, &result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"jsonrpc": request.JSONRPC,
			"id":      request.ID,
			"error": map[string]interface{}{
				"code":    -32603,
				"message": "Internal error",
				"data":    err.Error(),
			},
		})
		return err
	}

	// Create JSON-RPC response
	jsonResponse := jsonrpc.Response{
		JSONRPC: request.JSONRPC,
		ID:      request.ID,
		Result:  result,
	}

	// Return response
	c.JSON(http.StatusOK, jsonResponse)
	return nil
}

// getOrCreateJSONRPCClient gets or creates a JSON-RPC client using pkg/rpc/jsonrpc
func (p *JSONRPCProxy) getOrCreateJSONRPCClient(target string) (*jsonrpc.Client, error) {
	p.gateway.mu.Lock()
	defer p.gateway.mu.Unlock()

	if client, exists := p.clients[target]; exists {
		return client, nil
	}

	// Create JSON-RPC client configuration
	config := jsonrpc.DefaultClientConfig()
	config.URL = target
	config.Timeout = p.gateway.config.Protocols.JSONRPCConfig.Timeout
	config.Headers = p.gateway.config.Protocols.JSONRPCConfig.Headers

	// Create JSON-RPC client
	client := jsonrpc.NewClient(config)

	p.clients[target] = client
	return client, nil
}

// Close closes all JSON-RPC clients
func (p *JSONRPCProxy) Close() error {
	logger.Info("Starting JSON-RPC proxy close...")
	
	// 在关闭阶段，完全避免使用 Gateway 的锁
	// 直接操作 clients 映射，因为在关闭时不会有并发访问
	clientCount := len(p.clients)
	
	// JSON-RPC clients don't need explicit closing in pkg/rpc/jsonrpc
	// 直接清空映射
	p.clients = make(map[string]*jsonrpc.Client)
	
	logger.Info("Found JSON-RPC clients to close", logger.Int("count", clientCount))
	logger.Info("JSON-RPC proxy closed successfully")
	return nil
}
