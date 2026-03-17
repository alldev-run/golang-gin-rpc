// Package rpc provides RPC client implementations for gRPC and JSON-RPC
package rpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"alldev-gin-rpc/pkg/ratelimiter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientType represents the type of RPC client
type ClientType string

const (
	ClientTypeGRPC    ClientType = "grpc"
	ClientTypeJSONRPC ClientType = "jsonrpc"
)

// ClientConfig holds client configuration
type ClientConfig struct {
	Type       ClientType
	Host       string
	Port       int
	Timeout    time.Duration
	EnableTLS  bool
	MaxMsgSize int
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Type:       ClientTypeGRPC,
		Host:       "localhost",
		Port:       50051,
		Timeout:    30 * time.Second,
		MaxMsgSize: 4 * 1024 * 1024,
	}
}

// Client represents the RPC client interface
type Client interface {
	Connect() error
	Close() error
	IsConnected() bool
	Type() ClientType
	Call(ctx context.Context, method string, params interface{}, result interface{}) error
}

// GRPCClient wraps gRPC client
type GRPCClient struct {
	config      ClientConfig
	conn        *grpc.ClientConn
	degradation *DegradationManager
	rateLimiter *ratelimiter.Manager
}

// JSONRPCClient wraps JSON-RPC client
type JSONRPCClient struct {
	config      ClientConfig
	httpClient  *http.Client
	baseURL     string
	degradation *DegradationManager
	rateLimiter *ratelimiter.Manager
}

// NewClient creates a new RPC client based on configuration
func NewClient(config ClientConfig) (Client, error) {
	switch config.Type {
	case ClientTypeGRPC:
		return NewGRPCClient(config), nil
	case ClientTypeJSONRPC:
		return NewJSONRPCClient(config), nil
	default:
		return nil, fmt.Errorf("unsupported client type: %s", config.Type)
	}
}

// NewGRPCClient creates a new gRPC client
func NewGRPCClient(config ClientConfig) *GRPCClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &GRPCClient{
		config:      config,
		rateLimiter: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
	}
}

// Connect establishes connection to gRPC server
func (c *GRPCClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if c.config.EnableTLS {
		opts[0] = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			ServerName: c.config.Host,
		}))
	}

	if c.config.MaxMsgSize > 0 {
		opts = append(opts,
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(c.config.MaxMsgSize),
				grpc.MaxCallSendMsgSize(c.config.MaxMsgSize),
			),
		)
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	c.conn = conn
	return nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected returns true if connected
func (c *GRPCClient) IsConnected() bool {
	return c.conn != nil
}

// Type returns the client type
func (c *GRPCClient) Type() ClientType {
	return ClientTypeGRPC
}

// Call makes a gRPC call
func (c *GRPCClient) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.rateLimiter != nil && !c.rateLimiter.Allow(method) {
		return fmt.Errorf("rate limit exceeded for method: %s", method)
	}

	if c.degradation != nil && !c.degradation.ShouldAllowMethod(method) {
		return fmt.Errorf("method %s blocked by degradation policy", method)
	}

	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// For gRPC, we need to use the connection to invoke methods
	// This is a simplified version - in production, you'd use generated stubs
	return fmt.Errorf("direct gRPC call not supported, use generated client stubs")
}

// Connection returns the underlying gRPC connection for use with generated stubs
func (c *GRPCClient) Connection() *grpc.ClientConn {
	return c.conn
}

// SetDegradationManager sets the degradation manager for the client
func (c *GRPCClient) SetDegradationManager(dm *DegradationManager) {
	c.degradation = dm
}

// SetRateLimiterManager sets the rate limiter manager for the client
func (c *GRPCClient) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	c.rateLimiter = rlm
}

// NewJSONRPCClient creates a new JSON-RPC client
func NewJSONRPCClient(config ClientConfig) *JSONRPCClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	baseURL := fmt.Sprintf("http://%s:%d", config.Host, config.Port)
	if config.EnableTLS {
		baseURL = fmt.Sprintf("https://%s:%d", config.Host, config.Port)
	}

	return &JSONRPCClient{
		config:      config,
		baseURL:     baseURL,
		rateLimiter: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Connect tests connection to JSON-RPC server (no-op for HTTP)
func (c *JSONRPCClient) Connect() error {
	// For HTTP client, we don't maintain persistent connection
	// Just verify the server is reachable
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/rpc?ping=true", nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to JSON-RPC server: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// Close closes the client (no-op for HTTP)
func (c *JSONRPCClient) Close() error {
	return nil
}

// IsConnected always returns true for HTTP client
func (c *JSONRPCClient) IsConnected() bool {
	return true
}

// Type returns the client type
func (c *JSONRPCClient) Type() ClientType {
	return ClientTypeJSONRPC
}

// SetDegradationManager sets the degradation manager for the client
func (c *JSONRPCClient) SetDegradationManager(dm *DegradationManager) {
	c.degradation = dm
}

// SetRateLimiterManager sets the rate limiter manager for the client
func (c *JSONRPCClient) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	c.rateLimiter = rlm
}

// Call makes a JSON-RPC call
func (c *JSONRPCClient) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.rateLimiter != nil && !c.rateLimiter.Allow(method) {
		return fmt.Errorf("rate limit exceeded for method: %s", method)
	}

	if c.degradation != nil && !c.degradation.ShouldAllowMethod(method) {
		if fallback, ok := c.degradation.GetFallback(method); ok {
			fallbackResult, err := fallback(ctx, method, params)
			if err != nil {
				return err
			}
			if result != nil && fallbackResult != nil {
				resultBytes, err := json.Marshal(fallbackResult)
				if err != nil {
					return fmt.Errorf("failed to marshal fallback result: %w", err)
				}
				if err := json.Unmarshal(resultBytes, result); err != nil {
					return fmt.Errorf("failed to unmarshal fallback result: %w", err)
				}
			}
			return nil
		}
		return fmt.Errorf("method %s blocked by degradation policy", method)
	}

	start := time.Now()
	defer func() {
		if c.degradation != nil {
			c.degradation.RecordMetrics(time.Since(start), nil)
		}
	}()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if jsonResp.Error != nil {
		return fmt.Errorf("JSON-RPC error [%d]: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	if result != nil && jsonResp.Result != nil {
		resultBytes, err := json.Marshal(jsonResp.Result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		if err := json.Unmarshal(resultBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// SetAPIKey sets the API key for authentication
func (c *JSONRPCClient) SetAPIKey(apiKey string) {
	c.httpClient.Transport = &apiKeyTransport{
		Base:   c.httpClient.Transport,
		APIKey: apiKey,
	}
}

// apiKeyTransport adds API key to requests
type apiKeyTransport struct {
	Base   http.RoundTripper
	APIKey string
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-API-Key", t.APIKey)
	if t.Base == nil {
		t.Base = http.DefaultTransport
	}
	return t.Base.RoundTrip(req)
}
