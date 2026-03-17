package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alldev-gin-rpc/pkg/discovery"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

func TestGRPCGovernanceUnaryInterceptor_RejectsMissingAPIKey(t *testing.T) {
	server, err := NewGRPCServer(Config{
		Type:    ServerTypeGRPC,
		Host:    "localhost",
		Port:    50051,
		Network: "tcp",
	})
	if err != nil {
		t.Fatalf("failed to create grpc server: %v", err)
	}
	server.SetAuthConfig(AuthConfig{
		Enabled:     true,
		HeaderName:  "x-api-key",
		APIKeys:     map[string]string{"valid-key": "tester"},
		SkipMethods: []string{},
	})

	_, err = server.governanceUnaryInterceptor()(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/svc.Method",
	}, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	})
	if err == nil {
		t.Fatal("expected unauthenticated error")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected unauthenticated, got %s", status.Code(err))
	}
}

func TestGRPCGovernanceUnaryInterceptor_AllowsValidAPIKeyAndPropagatesContext(t *testing.T) {
	server, err := NewGRPCServer(Config{
		Type:    ServerTypeGRPC,
		Host:    "localhost",
		Port:    50051,
		Network: "tcp",
	})
	if err != nil {
		t.Fatalf("failed to create grpc server: %v", err)
	}
	server.SetAuthConfig(AuthConfig{
		Enabled:     true,
		HeaderName:  "x-api-key",
		APIKeys:     map[string]string{"valid-key": "tester"},
		SkipMethods: []string{},
	})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-api-key", "valid-key"))
	resp, err := server.governanceUnaryInterceptor()(ctx, "request", &grpc.UnaryServerInfo{
		FullMethod: "/svc.Method",
	}, func(ctx context.Context, req interface{}) (interface{}, error) {
		if method, ok := GetAPIUserFromContext(ctx); !ok || method != "tester" {
			t.Fatalf("expected authenticated user in context, got %q, %v", method, ok)
		}
		if method, ok := GetAPIKeyFromContext(ctx); !ok || method != "valid-key" {
			t.Fatalf("expected api key in context, got %q, %v", method, ok)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected successful handler, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestGRPCGovernanceUnaryInterceptor_UsesDegradationFallback(t *testing.T) {
	server, err := NewGRPCServer(Config{
		Type:    ServerTypeGRPC,
		Host:    "localhost",
		Port:    50051,
		Network: "tcp",
	})
	if err != nil {
		t.Fatalf("failed to create grpc server: %v", err)
	}

	dm, err := NewDegradationManager(DefaultDegradationConfig())
	if err != nil {
		t.Fatalf("failed to create degradation manager: %v", err)
	}
	dm.SetLevel(DegradationLevelEmergency)
	dm.fallbacks["/svc.Method"] = func(ctx context.Context, method string, req interface{}) (interface{}, error) {
		return "fallback", nil
	}
	server.SetDegradationManager(dm)

	resp, err := server.governanceUnaryInterceptor()(context.Background(), "request", &grpc.UnaryServerInfo{
		FullMethod: "/svc.Method",
	}, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler should not be called when degradation fallback is used")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("expected fallback response, got error %v", err)
	}
	if resp != "fallback" {
		t.Fatalf("unexpected fallback response: %v", resp)
	}
}

func TestGRPCClient_ResolveTargetViaDiscovery(t *testing.T) {
	client := NewGRPCClient(ClientConfig{
		Type:        ClientTypeGRPC,
		ServiceName: "orders-grpc",
	})
	client.SetDiscoveryResolver(&mockDiscoveryResolver{
		instances: []*discovery.ServiceInstance{
			{Name: "orders-grpc", Address: "10.0.0.21", Port: 9001},
		},
	})

	target, err := client.resolveGRPCTarget(context.Background())
	if err != nil {
		t.Fatalf("expected discovery resolution to succeed, got %v", err)
	}
	if target != "10.0.0.21:9001" {
		t.Fatalf("unexpected grpc target: %s", target)
	}
}

func TestJSONRPCClient_RefreshBaseURLViaDiscovery(t *testing.T) {
	client := NewJSONRPCClient(ClientConfig{
		Type:        ClientTypeJSONRPC,
		ServiceName: "orders-jsonrpc",
	})
	client.SetDiscoveryResolver(&mockDiscoveryResolver{
		instances: []*discovery.ServiceInstance{
			{Name: "orders-jsonrpc", Address: "10.0.0.22", Port: 8088},
		},
	})

	if err := client.refreshJSONRPCBaseURL(context.Background()); err != nil {
		t.Fatalf("expected discovery resolution to succeed, got %v", err)
	}
	if client.baseURL != "http://10.0.0.22:8088" {
		t.Fatalf("unexpected jsonrpc baseURL: %s", client.baseURL)
	}
}

func TestGRPCClient_ResolveTargetViaDiscovery_RoundRobin(t *testing.T) {
	client := NewGRPCClient(ClientConfig{
		Type:        ClientTypeGRPC,
		ServiceName: "orders-grpc",
		LoadBalance: "round_robin",
	})
	client.SetDiscoveryResolver(&mockDiscoveryResolver{
		instances: []*discovery.ServiceInstance{
			{Name: "orders-grpc", Address: "10.0.0.21", Port: 9001},
			{Name: "orders-grpc", Address: "10.0.0.22", Port: 9002},
		},
	})

	first, err := client.resolveGRPCTarget(context.Background())
	if err != nil {
		t.Fatalf("expected first resolution to succeed, got %v", err)
	}
	second, err := client.resolveGRPCTarget(context.Background())
	if err != nil {
		t.Fatalf("expected second resolution to succeed, got %v", err)
	}
	if first == second {
		t.Fatalf("expected round robin to rotate targets, got %s and %s", first, second)
	}
}

func TestGRPCClient_AuthUnaryClientInterceptor_AddsMetadata(t *testing.T) {
	client := NewGRPCClient(ClientConfig{
		Type:       ClientTypeGRPC,
		APIKey:     "secret-key",
		AuthHeader: "x-api-key",
	})

	err := client.authUnaryClientInterceptor()(context.Background(), "/svc.Method", "req", nil, nil,
		func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("expected outgoing metadata")
			}
			if values := md.Get("x-api-key"); len(values) != 1 || values[0] != "secret-key" {
				t.Fatalf("unexpected metadata values: %v", values)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("expected interceptor to succeed, got %v", err)
	}
}

func TestJSONRPCClient_Call_UsesConfiguredAuthHeaderAndRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if got := r.Header.Get("X-Custom-Key"); got != "secret-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`temporary error`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ok":true},"id":1}`))
	}))
	defer server.Close()

	client := NewJSONRPCClient(ClientConfig{
		Type:         ClientTypeJSONRPC,
		Host:         strings.TrimPrefix(server.URL, "http://"),
		Port:         80,
		APIKey:       "secret-key",
		AuthHeader:   "X-Custom-Key",
		RetryCount:   2,
		RetryBackoff: time.Millisecond,
	})
	client.baseURL = server.URL

	var result map[string]bool
	if err := client.Call(context.Background(), "test.method", map[string]string{"x": "y"}, &result); err != nil {
		t.Fatalf("expected call to succeed after retry, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if !result["ok"] {
		t.Fatalf("unexpected result payload: %#v", result)
	}
}

func TestJSONRPCClient_DoJSONRPCRequestWithRetry_FailsAfterExhaustingRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`bad gateway`))
	}))
	defer server.Close()

	client := NewJSONRPCClient(ClientConfig{
		Type:         ClientTypeJSONRPC,
		RetryCount:   2,
		RetryBackoff: time.Millisecond,
	})
	req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if req.GetBody == nil {
		payload := []byte(`{}`)
		req.Body = io.NopCloser(bytes.NewReader(payload))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(payload)), nil
		}
	}

	_, err = client.doJSONRPCRequestWithRetry(req)
	if err == nil {
		t.Fatal("expected retry request to fail")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
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

type mockDiscoveryResolver struct {
	instances []*discovery.ServiceInstance
	err       error
}

func (m *mockDiscoveryResolver) GetService(ctx context.Context, serviceName string) ([]*discovery.ServiceInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

func TestNewJSONRPCServer_BindsEngineAsHTTPHandler(t *testing.T) {
	server := NewJSONRPCServer(Config{
		Type:    ServerTypeJSONRPC,
		Host:    "localhost",
		Port:    8080,
		Timeout: 1,
	})

	if server.server == nil {
		t.Fatal("expected http server to be initialized")
	}
	if server.server.Handler != server.engine {
		t.Fatal("expected gin engine to be bound as http handler")
	}
}

func TestJSONRPCServer_SetupRoutes_Idempotent(t *testing.T) {
	server := NewJSONRPCServer(Config{
		Type:    ServerTypeJSONRPC,
		Host:    "localhost",
		Port:    8080,
		Timeout: 1,
	})

	server.setupRoutes()
	firstCount := len(server.engine.Routes())
	server.setupRoutes()
	secondCount := len(server.engine.Routes())

	if firstCount != secondCount {
		t.Fatalf("expected idempotent route setup, got %d then %d", firstCount, secondCount)
	}
}

func TestJSONRPCServer_HandleJSONRPC_UsesConfiguredAuthNames(t *testing.T) {
	server := NewJSONRPCServer(Config{
		Type:    ServerTypeJSONRPC,
		Host:    "localhost",
		Port:    8080,
		Timeout: 1,
	})
	server.SetAuthConfig(AuthConfig{
		Enabled:     true,
		HeaderName:  "X-Custom-Key",
		QueryName:   "custom_key",
		APIKeys:     map[string]string{"valid-key": "tester"},
		SkipMethods: []string{},
	})

	body, err := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "svc.echo",
		Params:  map[string]interface{}{"message": "hello"},
		ID:      1,
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/rpc?custom_key=valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Key", "valid-key")
	resp := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(resp)
	ctx.Request = req

	server.handleJSONRPC(ctx)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"code":-32601`) {
		t.Fatalf("expected method-not-found response after auth passes, got %s", resp.Body.String())
	}
}

func TestJSONRPCServer_HealthAndReadyEndpoints(t *testing.T) {
	server := NewJSONRPCServer(Config{
		Type:    ServerTypeJSONRPC,
		Host:    "localhost",
		Port:    8080,
		Timeout: 1,
	})
	server.setupRoutes()

	tests := []struct {
		path           string
		expectedStatus string
	}{
		{path: "/health", expectedStatus: "healthy"},
		{path: "/ready", expectedStatus: "ready"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		resp := httptest.NewRecorder()
		server.engine.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("%s expected status 200, got %d", tt.path, resp.Code)
		}
		if !strings.Contains(resp.Body.String(), tt.expectedStatus) {
			t.Fatalf("%s expected body to contain %q, got %s", tt.path, tt.expectedStatus, resp.Body.String())
		}
	}
}

func TestManagerStart_FailsOnPreflightPortConflict(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate listener: %v", err)
	}
	defer lis.Close()

	port := lis.Addr().(*net.TCPAddr).Port
	manager := NewManager(ManagerConfig{
		Servers: map[string]Config{
			"grpc": {
				Type:    ServerTypeGRPC,
				Host:    "127.0.0.1",
				Port:    port,
				Network: "tcp",
				Timeout: 1,
			},
		},
	})

	err = manager.Start()
	if err == nil {
		t.Fatal("expected manager start to fail on occupied port")
	}
	if !strings.Contains(err.Error(), "preflight check failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
