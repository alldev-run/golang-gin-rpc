// Package jsonrpc provides JSON-RPC client utilities
package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientConfig holds JSON-RPC client configuration
type ClientConfig struct {
	URL        string        `yaml:"url" json:"url"`
	Timeout    time.Duration `yaml:"timeout" json:"timeout"`
	Headers    map[string]string `yaml:"headers" json:"headers"`
	UserAgent  string        `yaml:"user_agent" json:"user_agent"`
	MaxRetries int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay time.Duration `yaml:"retry_delay" json:"retry_delay"`
}

// DefaultClientConfig returns default JSON-RPC client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		URL:        "http://localhost:8080/rpc",
		Timeout:    30 * time.Second,
		Headers:    make(map[string]string),
		UserAgent:  "alldev-gin-rpc-jsonrpc-client/1.0",
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *RPCError) Error() string {
	return fmt.Sprintf("JSON-RPC Error %d: %s", e.Code, e.Message)
}

// BatchRequest represents a batch of JSON-RPC requests
type BatchRequest []*Request

// BatchResponse represents a batch of JSON-RPC responses
type BatchResponse []*Response

// Client wraps JSON-RPC client functionality
type Client struct {
	config ClientConfig
	http   *http.Client
}

// NewClient creates a new JSON-RPC client
func NewClient(config ClientConfig) *Client {
	return &Client{
		config: config,
		http: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Call makes a single JSON-RPC method call
func (c *Client) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	return c.CallWithID(ctx, method, params, result, nil)
}

// CallWithID makes a JSON-RPC method call with a specific ID
func (c *Client) CallWithID(ctx context.Context, method string, params interface{}, result interface{}, id interface{}) error {
	request := &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	response, err := c.doRequest(ctx, request)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return response.Error
	}

	if result != nil && response.Result != nil {
		return json.Unmarshal(response.Result.(json.RawMessage), result)
	}

	return nil
}

// CallBatch makes a batch of JSON-RPC method calls
func (c *Client) CallBatch(ctx context.Context, requests BatchRequest) (BatchResponse, error) {
	var batchRequests []*Request
	for _, req := range requests {
		batchRequests = append(batchRequests, req)
	}

	return c.doBatchRequest(ctx, batchRequests)
}

// Notify makes a JSON-RPC notification (no response expected)
func (c *Client) Notify(ctx context.Context, method string, params interface{}) error {
	request := &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		// No ID for notifications
	}

	_, err := c.doRequest(ctx, request)
	return err
}

// doRequest performs a single JSON-RPC request
func (c *Client) doRequest(ctx context.Context, request *Request) (*Response, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.URL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Add custom headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// doBatchRequest performs a batch JSON-RPC request
func (c *Client) doBatchRequest(ctx context.Context, requests []*Request) (BatchResponse, error) {
	requestBody, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.URL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create batch request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Add custom headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send batch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch response: %w", err)
	}

	var responses BatchResponse
	if err := json.Unmarshal(body, &responses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch response: %w", err)
	}

	return responses, nil
}

// CallWithRetry makes a JSON-RPC call with retry logic
func (c *Client) CallWithRetry(ctx context.Context, method string, params interface{}, result interface{}) error {
	var lastErr error
	
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		err := c.Call(ctx, method, params, result)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Don't retry on certain errors
		if rpcErr, ok := err.(*RPCError); ok {
			if rpcErr.Code == -32600 || rpcErr.Code == -32601 || rpcErr.Code == -32602 {
				// Invalid request, method not found, invalid params - don't retry
				return err
			}
		}
		
		if attempt < c.config.MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay):
				continue
			}
		}
	}
	
	return fmt.Errorf("max retries exceeded, last error: %w", lastErr)
}

// HealthChecker provides health checking for JSON-RPC services
type HealthChecker struct {
	client *Client
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(client *Client) *HealthChecker {
	return &HealthChecker{client: client}
}

// CheckHealth checks if the JSON-RPC service is healthy
func (h *HealthChecker) CheckHealth(ctx context.Context) error {
	// Try to call a simple health check method
	var result interface{}
	err := h.client.Call(ctx, "system.health", nil, &result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// Ping sends a simple ping to the service
func (h *HealthChecker) Ping(ctx context.Context) (interface{}, error) {
	var result interface{}
	err := h.client.Call(ctx, "system.ping", nil, &result)
	if err != nil {
		return nil, fmt.Errorf("ping failed: %w", err)
	}
	return result, nil
}

// ServiceInfo represents information about a JSON-RPC service
type ServiceInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Methods []string `json:"methods"`
}

// GetServiceInfo retrieves information about the service
func (h *HealthChecker) GetServiceInfo(ctx context.Context) (*ServiceInfo, error) {
	var info ServiceInfo
	err := h.client.Call(ctx, "system.info", nil, &info)
	if err != nil {
		return nil, fmt.Errorf("failed to get service info: %w", err)
	}
	return &info, nil
}

// ClientPool manages a pool of JSON-RPC clients
type ClientPool struct {
	clients map[string]*Client
	config  ClientConfig
}

// NewClientPool creates a new JSON-RPC client pool
func NewClientPool(config ClientConfig) *ClientPool {
	return &ClientPool{
		clients: make(map[string]*Client),
		config:  config,
	}
}

// Get returns a client for the given URL, creating one if necessary
func (p *ClientPool) Get(url string) *Client {
	if client, exists := p.clients[url]; exists {
		return client
	}

	config := p.config
	config.URL = url
	
	client := NewClient(config)
	p.clients[url] = client
	return client
}

// Remove removes a client from the pool
func (p *ClientPool) Remove(url string) {
	delete(p.clients, url)
}

// Size returns the number of clients in the pool
func (p *ClientPool) Size() int {
	return len(p.clients)
}

// URLs returns all URLs in the pool
func (p *ClientPool) URLs() []string {
	urls := make([]string, 0, len(p.clients))
	for url := range p.clients {
		urls = append(urls, url)
	}
	return urls
}
