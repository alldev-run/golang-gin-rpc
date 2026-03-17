// Package grpc provides gRPC client utilities
package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ClientConfig holds gRPC client configuration
type ClientConfig struct {
	Address    string        `yaml:"address" json:"address"`
	Timeout    time.Duration `yaml:"timeout" json:"timeout"`
	EnableTLS  bool          `yaml:"enable_tls" json:"enable_tls"`
	CertFile   string        `yaml:"cert_file" json:"cert_file"`
	ServerName string        `yaml:"server_name" json:"server_name"`
	MaxMsgSize int           `yaml:"max_msg_size" json:"max_msg_size"`
	KeepAlive  *KeepAliveConfig `yaml:"keep_alive" json:"keep_alive"`
}

// KeepAliveConfig holds keepalive configuration
type KeepAliveConfig struct {
	Time                time.Duration `yaml:"time" json:"time"`
	Timeout             time.Duration `yaml:"timeout" json:"timeout"`
	PermitWithoutStream bool          `yaml:"permit_without_stream" json:"permit_without_stream"`
}

// DefaultClientConfig returns default gRPC client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Address:    "localhost:50051",
		Timeout:    30 * time.Second,
		EnableTLS:  false,
		MaxMsgSize: 4 * 1024 * 1024, // 4MB
		KeepAlive: &KeepAliveConfig{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		},
	}
}

// Client wraps gRPC client connection
type Client struct {
	conn   *grpc.ClientConn
	config ClientConfig
}

// NewClient creates a new gRPC client
func NewClient(config ClientConfig) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTimeout(config.Timeout),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(config.MaxMsgSize),
			grpc.MaxCallSendMsgSize(config.MaxMsgSize),
		),
	}

	// Configure TLS
	if config.EnableTLS {
		tlsConfig := &tls.Config{
			ServerName: config.ServerName,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Configure keepalive
	if config.KeepAlive != nil {
		kp := keepalive.ClientParameters{
			Time:                config.KeepAlive.Time,
			Timeout:             config.KeepAlive.Timeout,
			PermitWithoutStream: config.KeepAlive.PermitWithoutStream,
		}
		opts = append(opts, grpc.WithKeepaliveParams(kp))
	}

	conn, err := grpc.Dial(config.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server at %s: %w", config.Address, err)
	}

	return &Client{
		conn:   conn,
		config: config,
	}, nil
}

// NewClientWithConn creates a client with an existing connection
func NewClientWithConn(conn *grpc.ClientConn, config ClientConfig) *Client {
	return &Client{
		conn:   conn,
		config: config,
	}
}

// Conn returns the underlying gRPC connection
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// Address returns the server address
func (c *Client) Address() string {
	return c.config.Address
}

// IsSecure returns true if TLS is enabled
func (c *Client) IsSecure() bool {
	return c.config.EnableTLS
}

// GetState returns the connection state
func (c *Client) GetState() connectivity.State {
	return c.conn.GetState()
}

// WaitForReady waits for the connection to be ready
func (c *Client) WaitForReady(ctx context.Context) error {
	currentState := c.conn.GetState()
	if currentState == connectivity.Ready {
		return nil
	}
	
	changed := c.conn.WaitForStateChange(ctx, currentState)
	if !changed {
		return ctx.Err()
	}
	
	finalState := c.conn.GetState()
	if finalState == connectivity.Ready {
		return nil
	}
	return fmt.Errorf("connection not ready, current state: %v", finalState)
}

// ClientPool manages a pool of gRPC clients
type ClientPool struct {
	clients map[string]*Client
	config  ClientConfig
}

// NewClientPool creates a new gRPC client pool
func NewClientPool(config ClientConfig) *ClientPool {
	return &ClientPool{
		clients: make(map[string]*Client),
		config:  config,
	}
}

// Get returns a client for the given address, creating one if necessary
func (p *ClientPool) Get(address string) (*Client, error) {
	if client, exists := p.clients[address]; exists {
		return client, nil
	}

	config := p.config
	config.Address = address
	
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	p.clients[address] = client
	return client, nil
}

// Close closes all clients in the pool
func (p *ClientPool) Close() error {
	var errors []error
	
	for addr, client := range p.clients {
		if err := client.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close client %s: %w", addr, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors closing clients: %v", errors)
	}
	
	return nil
}

// Remove removes and closes a client from the pool
func (p *ClientPool) Remove(address string) error {
	if client, exists := p.clients[address]; exists {
		delete(p.clients, address)
		return client.Close()
	}
	return nil
}

// Size returns the number of clients in the pool
func (p *ClientPool) Size() int {
	return len(p.clients)
}

// Addresses returns all addresses in the pool
func (p *ClientPool) Addresses() []string {
	addrs := make([]string, 0, len(p.clients))
	for addr := range p.clients {
		addrs = append(addrs, addr)
	}
	return addrs
}

// HealthChecker provides health checking for gRPC services
type HealthChecker struct {
	client *Client
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(client *Client) *HealthChecker {
	return &HealthChecker{client: client}
}

// CheckHealth checks if the gRPC service is healthy
func (h *HealthChecker) CheckHealth(ctx context.Context) error {
	// Simple health check - try to get connection state
	state := h.client.GetState()
	
	switch state {
	case connectivity.Ready, connectivity.Idle:
		return nil
	case connectivity.Connecting:
		return fmt.Errorf("service is connecting")
	case connectivity.TransientFailure:
		return fmt.Errorf("service is in transient failure")
	case connectivity.Shutdown:
		return fmt.Errorf("service is shutdown")
	default:
		return fmt.Errorf("unknown service state: %v", state)
	}
}

// WaitForHealthy waits for the service to become healthy
func (h *HealthChecker) WaitForHealthy(ctx context.Context) error {
	return h.client.WaitForReady(ctx)
}
