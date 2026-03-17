package rpc

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewClient_ReturnsErrorForUnsupportedType(t *testing.T) {
	client, err := NewClient(ClientConfig{
		Type: ClientType("unsupported"),
	})

	if err == nil {
		t.Fatal("expected error for unsupported client type")
	}
	if client != nil {
		t.Fatal("expected nil client for unsupported client type")
	}
	if !strings.Contains(err.Error(), "unsupported client type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClient_ReturnsConcreteTypes(t *testing.T) {
	tests := []struct {
		name       string
		config     ClientConfig
		wantType   ClientType
		wantGRPC   bool
		wantJSONRP bool
	}{
		{
			name: "grpc",
			config: ClientConfig{
				Type: ClientTypeGRPC,
				Host: "localhost",
				Port: 50051,
			},
			wantType: ClientTypeGRPC,
			wantGRPC: true,
		},
		{
			name: "jsonrpc",
			config: ClientConfig{
				Type: ClientTypeJSONRPC,
				Host: "localhost",
				Port: 8080,
			},
			wantType:   ClientTypeJSONRPC,
			wantJSONRP: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if client == nil {
				t.Fatal("expected non-nil client")
			}
			if client.Type() != tt.wantType {
				t.Fatalf("expected client type %s, got %s", tt.wantType, client.Type())
			}
			if _, ok := client.(*GRPCClient); ok != tt.wantGRPC {
				t.Fatalf("grpc client assertion mismatch: got %v want %v", ok, tt.wantGRPC)
			}
			if _, ok := client.(*JSONRPCClient); ok != tt.wantJSONRP {
				t.Fatalf("jsonrpc client assertion mismatch: got %v want %v", ok, tt.wantJSONRP)
			}
		})
	}
}

func TestNewServer_ReturnsErrorForUnsupportedType(t *testing.T) {
	server, err := NewServer(Config{
		Type: ServerType("unsupported"),
	})

	if err == nil {
		t.Fatal("expected error for unsupported server type")
	}
	if server != nil {
		t.Fatal("expected nil server for unsupported server type")
	}
	if !strings.Contains(err.Error(), "unsupported server type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewGRPCServer_TLSValidation(t *testing.T) {
	_, err := NewGRPCServer(Config{
		Type:      ServerTypeGRPC,
		Host:      "localhost",
		Port:      50051,
		EnableTLS: true,
	})
	if err == nil {
		t.Fatal("expected error when tls is enabled without cert/key")
	}
	if !strings.Contains(err.Error(), "cert_file/key_file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServer_ReturnsConcreteTypes(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		wantGRPC   bool
		wantJSONRP bool
	}{
		{
			name: "grpc",
			config: Config{
				Type:    ServerTypeGRPC,
				Host:    "localhost",
				Port:    50051,
				Network: "tcp",
			},
			wantGRPC: true,
		},
		{
			name: "jsonrpc",
			config: Config{
				Type:    ServerTypeJSONRPC,
				Host:    "localhost",
				Port:    8080,
				Network: "tcp",
			},
			wantJSONRP: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if server == nil {
				t.Fatal("expected non-nil server")
			}
			if _, ok := server.(*GRPCServer); ok != tt.wantGRPC {
				t.Fatalf("grpc server assertion mismatch: got %v want %v", ok, tt.wantGRPC)
			}
			if _, ok := server.(*JSONRPCServer); ok != tt.wantJSONRP {
				t.Fatalf("jsonrpc server assertion mismatch: got %v want %v", ok, tt.wantJSONRP)
			}
		})
	}
}

func TestNewJSONRPCClient_UsesHTTPSSchemeWhenTLSEnabled(t *testing.T) {
	client := NewJSONRPCClient(ClientConfig{
		Type:      ClientTypeJSONRPC,
		Host:      "localhost",
		Port:      8443,
		EnableTLS: true,
		Timeout:   5 * time.Second,
	})

	if client.baseURL != "https://localhost:8443" {
		t.Fatalf("expected https baseURL, got %s", client.baseURL)
	}
	if client.httpClient == nil {
		t.Fatal("expected non-nil http client")
	}
	if client.httpClient.Timeout != 5*time.Second {
		t.Fatalf("unexpected timeout: %v", client.httpClient.Timeout)
	}
}

func TestNewJSONRPCClient_UsesHTTPSchemeByDefault(t *testing.T) {
	client := NewJSONRPCClient(ClientConfig{
		Type:    ClientTypeJSONRPC,
		Host:    "localhost",
		Port:    8080,
		Timeout: 5 * time.Second,
	})

	if client.baseURL != "http://localhost:8080" {
		t.Fatalf("expected http baseURL, got %s", client.baseURL)
	}
}

func TestAPIKeyTransport_UsesDefaultBaseAndSetsHeader(t *testing.T) {
	transport := &apiKeyTransport{
		APIKey: "test-api-key",
	}

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, _ = transport.RoundTrip(req)

	if got := req.Header.Get("X-API-Key"); got != "test-api-key" {
		t.Fatalf("expected api key header to be set, got %q", got)
	}
	if transport.Base == nil {
		t.Fatal("expected default base transport to be assigned")
	}
}

func TestManagerAddServer_PropagatesServerCreationError(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Servers:                 map[string]Config{},
		GracefulShutdownTimeout: time.Second,
	})

	err := manager.AddServer("grpc", Config{
		Type:      ServerTypeGRPC,
		Host:      "localhost",
		Port:      50051,
		EnableTLS: true,
	})
	if err == nil {
		t.Fatal("expected add server to fail for invalid tls config")
	}
	if !strings.Contains(err.Error(), "failed to create server grpc") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := manager.GetServer("grpc"); exists {
		t.Fatal("server should not have been added on error")
	}
}

func TestManagerStop_UsesDefaultGracefulTimeoutWhenUnset(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Servers: map[string]Config{},
	})

	if err := manager.Start(); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}
	if err := manager.Stop(); err != nil {
		t.Fatalf("expected stop to succeed with default timeout, got: %v", err)
	}
}
