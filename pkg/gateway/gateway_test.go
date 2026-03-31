package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
	auditpkg "github.com/alldev-run/golang-gin-rpc/pkg/audit"
	middlewarepkg "github.com/alldev-run/golang-gin-rpc/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureGatewayAuditSink struct {
	events []auditpkg.Event
}

func (s *captureGatewayAuditSink) Write(ctx context.Context, event auditpkg.Event) error {
	s.events = append(s.events, event)
	return nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	assert.Equal(t, "0.0.0.0", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.Equal(t, 30*time.Second, config.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.WriteTimeout)
	assert.Equal(t, 60*time.Second, config.IdleTimeout)
	assert.Equal(t, "round_robin", config.LoadBalancer.Strategy)
	assert.Equal(t, "static", config.Discovery.Type)
	assert.False(t, config.RateLimit.Enabled)
}

func TestNewGateway(t *testing.T) {
	config := DefaultConfig()
	gw := NewGateway(config)
	
	assert.NotNil(t, gw)
	assert.Equal(t, config, gw.GetConfig())
	assert.NotNil(t, gw.GetRouter())
}

func TestNewGateway_AuditConfigFromGatewayConfig(t *testing.T) {
	config := DefaultConfig()
	config.Audit.Enabled = false
	config.Audit.SkipPaths = []string{"/internal/health"}
	config.Audit.SensitiveKeys = []string{"authorization", "secret_key"}

	gw := NewGateway(config)
	auditCfg := gw.GetAuditConfig()

	assert.False(t, auditCfg.Enabled)
	assert.Equal(t, []string{"/internal/health"}, auditCfg.SkipPaths)
	assert.Equal(t, []string{"authorization", "secret_key"}, auditCfg.SensitiveKeys)
}

func TestNewGateway_RBACPolicyFromGatewayConfig(t *testing.T) {
	config := DefaultConfig()
	config.RBAC.Enabled = true
	config.RBAC.RolePermissions = map[string][]string{
		"admin": {"user.read", "user.write"},
	}

	gw := NewGateway(config)
	policy := gw.GetRBACPolicy()
	require.NotNil(t, policy)
	assert.True(t, policy.HasPermission([]string{"admin"}, "user.write"))
}

func TestRouteKey(t *testing.T) {
	config := DefaultConfig()
	gw := NewGateway(config)
	
	key := gw.routeKey("/api/user", "GET")
	assert.Equal(t, "GET:/api/user", key)
	
	key2 := gw.routeKey("/api/order/*", "POST")
	assert.Equal(t, "POST:/api/order/*", key2)
}

func TestLoadBalancerFactory(t *testing.T) {
	factory := NewLoadBalancerFactory()
	
	// Test different strategies
	rr := factory.Create("round_robin")
	assert.NotNil(t, rr)
	
	random := factory.Create("random")
	assert.NotNil(t, random)
	
	weighted := factory.Create("weighted")
	assert.NotNil(t, weighted)
	
	leastConn := factory.Create("least_connections")
	assert.NotNil(t, leastConn)
	
	// Test unknown strategy (should default to round_robin)
	unknown := factory.Create("unknown")
	assert.NotNil(t, unknown)
}

func TestRoundRobinLoadBalancer(t *testing.T) {
	lb := NewRoundRobinLoadBalancer()
	
	targets := []string{"http://localhost:8001", "http://localhost:8002"}
	lb.UpdateTargets(targets)
	
	// Test selection
	selected, err := lb.Select(nil)
	assert.NoError(t, err)
	assert.Contains(t, targets, selected)
	
	// Test multiple selections
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		selected, _ := lb.Select(nil)
		selections[selected]++
	}
	
	// Should have selected both targets
	assert.Equal(t, 2, len(selections))
}

func TestRandomLoadBalancer(t *testing.T) {
	lb := NewRandomLoadBalancer()
	
	targets := []string{"http://localhost:8001", "http://localhost:8002"}
	lb.UpdateTargets(targets)
	
	// Test selection
	selected, err := lb.Select(nil)
	assert.NoError(t, err)
	assert.Contains(t, targets, selected)
}

func TestServiceDiscovery(t *testing.T) {
	config := DiscoveryConfig{
		Type:      "static",
		Endpoints: []string{},
		Namespace: "test",
		Timeout:   5 * time.Second,
	}
	
	sd, err := NewServiceDiscovery(config)
	assert.NoError(t, err)
	assert.NotNil(t, sd)
	
	// Test static initialization
	err = sd.Initialize()
	assert.NoError(t, err)
	
	// Test getting endpoints
	endpoints, err := sd.GetServiceEndpoints("user-service")
	assert.Error(t, err)
	assert.Nil(t, endpoints)
	assert.Contains(t, err.Error(), "static discovery should use route targets directly")
	
	// Test unknown service
	_, err = sd.GetServiceEndpoints("unknown-service")
	assert.Error(t, err)
	
	// Test consul discovery (if available)
	consulConfig := DiscoveryConfig{
		Type:      "consul",
		Endpoints: []string{"localhost:8500"},
		Namespace: "test",
		Timeout:   5 * time.Second,
	}
	
	consulSD, err := NewServiceDiscovery(consulConfig)
	// This might fail if consul is not running, which is expected
	if err == nil {
		assert.NotNil(t, consulSD)
		consulSD.Stop()
	}
}

func TestParseDuration(t *testing.T) {
	// Test valid duration
	d := parseDuration("1m")
	assert.Equal(t, time.Minute, d)
	
	// Test invalid duration (should default to 1 minute)
	d = parseDuration("invalid")
	assert.Equal(t, time.Minute, d)
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()
	
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}

func TestIsTimeout(t *testing.T) {
	// Test timeout error
	timeoutErr := IsTimeout(assert.AnError)
	assert.False(t, timeoutErr)
	
	// Test nil error
	nilErr := IsTimeout(nil)
	assert.False(t, nilErr)
}

func TestIsTooManyRetries(t *testing.T) {
	// Test retry error
	retryErr := IsTooManyRetries(assert.AnError)
	assert.False(t, retryErr)
	
	// Test nil error
	nilErr := IsTooManyRetries(nil)
	assert.False(t, nilErr)
}

// TestProxy tests the proxy functionality
func TestProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Hello from backend",
			"path":    r.URL.Path,
			"method":  r.Method,
		})
	}))
	defer backendServer.Close()

	// Create gateway config
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:     "/api/test",
			Method:   "GET",
			Service:  "test-service",
			Targets:  []string{backendServer.URL},
			StripPrefix: false,
		},
	}

	// Create gateway
	gw := NewGateway(config)
	require.NotNil(t, gw)

	// Setup routes
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request
	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Hello from backend", response["message"])
	assert.Equal(t, "/api/test", response["path"])
	assert.Equal(t, "GET", response["method"])
}

func TestSetupRoutes_AuditMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backendServer.Close()

	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:        "/api/test",
			Method:      "GET",
			Service:     "audit-service",
			Targets:     []string{backendServer.URL},
			StripPrefix: false,
		},
	}

	gw := NewGateway(config)
	sink := &captureGatewayAuditSink{}
	auditCfg := middlewarepkg.DefaultAuditConfig()
	auditCfg.Sink = sink
	gw.SetAuditConfig(auditCfg)

	engine := gin.New()
	gw.SetupRoutes(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/test?from=gateway", nil)
	req.Header.Set("Authorization", "Bearer sample-token")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	require.Len(t, sink.events, 1)
	assert.Equal(t, auditpkg.ActionRead, sink.events[0].Action)
	assert.Equal(t, "/api/test", sink.events[0].Path)
	headers, ok := sink.events[0].Metadata["headers"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "***", headers["Authorization"])
}

func TestSetupRoutes_RBACPermissionDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backendServer.Close()

	config := DefaultConfig()
	config.RBAC.Enabled = true
	config.RBAC.RolePermissions = map[string][]string{
		"user": {"profile.read"},
	}
	config.Routes = []RouteConfig{
		{
			Path:                "/api/secure",
			Method:              "GET",
			Service:             "secure-service",
			Targets:             []string{backendServer.URL},
			RequiredPermissions: []string{"profile.write"},
			PermissionMode:      "any",
		},
	}

	gw := NewGateway(config)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("claims", &jwtx.Claims{Payload: map[string]string{"roles": "user"}})
		c.Next()
	})
	gw.SetupRoutes(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/secure", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusForbidden, resp.Code)
}

func TestSetupRoutes_RBACPermissionAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backendServer.Close()

	config := DefaultConfig()
	config.RBAC.Enabled = true
	config.RBAC.RolePermissions = map[string][]string{
		"editor": {"doc.read", "doc.write"},
	}
	config.Routes = []RouteConfig{
		{
			Path:                "/api/doc",
			Method:              "GET",
			Service:             "doc-service",
			Targets:             []string{backendServer.URL},
			RequiredPermissions: []string{"doc.write"},
			PermissionMode:      "any",
		},
	}

	gw := NewGateway(config)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("claims", &jwtx.Claims{Payload: map[string]string{"roles": "editor"}})
		c.Next()
	})
	gw.SetupRoutes(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/doc", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// TestProxyWithStripPrefix tests proxy with prefix stripping
func TestProxyWithStripPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path": r.URL.Path,
		})
	}))
	defer backendServer.Close()

	// Create gateway config with strip prefix
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:        "/api/v1",
			Method:      "GET",
			Service:     "test-service",
			Targets:     []string{backendServer.URL},
			StripPrefix: true,
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request with prefix stripping on the exact configured route path
	req := httptest.NewRequest("GET", "/api/v1", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "/", response["path"]) // Prefix should be stripped from the exact route path
}

// TestProxyWithHeaders tests proxy with custom headers
func TestProxyWithHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a mock backend server that captures headers
	var receivedHeaders map[string][]string
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"headers_received": true,
		})
	}))
	defer backendServer.Close()

	// Create gateway config with custom headers
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:   "/api/test",
			Method: "GET",
			Service: "test-service",
			Targets: []string{backendServer.URL},
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"X-Service":      "gateway",
			},
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request with custom headers
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Client-Header", "client-value")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotNil(t, receivedHeaders)
	assert.Equal(t, "custom-value", receivedHeaders["X-Custom-Header"][0])
	assert.Equal(t, "gateway", receivedHeaders["X-Service"][0])
	assert.Equal(t, "client-value", receivedHeaders["X-Client-Header"][0])
}

// TestProxyWithQueryParams tests proxy with custom query parameters
func TestProxyWithQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a mock backend server that captures query params
	var receivedQuery string
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": receivedQuery,
		})
	}))
	defer backendServer.Close()

	// Create gateway config with custom query parameters
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:   "/api/test",
			Method: "GET",
			Service: "test-service",
			Targets: []string{backendServer.URL},
			Query: map[string]string{
				"version": "v1",
				"source":  "gateway",
			},
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request with query parameters
	req := httptest.NewRequest("GET", "/api/test?client=web", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	
	// Should contain both original and added query parameters
	assert.Contains(t, receivedQuery, "client=web")
	assert.Contains(t, receivedQuery, "version=v1")
	assert.Contains(t, receivedQuery, "source=gateway")
}

// TestProxyErrorHandling tests proxy error handling
func TestProxyErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create gateway config with invalid target
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:    "/api/test",
			Method:  "GET",
			Service: "test-service",
			Targets: []string{"http://invalid-host:9999"}, // Invalid target
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request to invalid target
	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	// Current proxy implementation maps generic upstream execution failures to 500
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

// TestProxyNotFound tests proxy when route is not found
func TestProxyNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create gateway config without the requested route
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:    "/api/other",
			Method:  "GET",
			Service: "other-service",
			Targets: []string{"http://localhost:8001"},
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request to non-existent route
	req := httptest.NewRequest("GET", "/api/notfound", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
	
	assert.Contains(t, resp.Body.String(), "404 page not found")
}

// TestHealthCheck tests the health check endpoint
func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := DefaultConfig()
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test health check
	req := httptest.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.Contains(t, response, "timestamp")
}

// TestReadinessCheck tests the readiness check endpoint
func TestReadinessCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := DefaultConfig()
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test readiness check when not started
	req := httptest.NewRequest("GET", "/ready", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "not ready", response["status"])
	assert.Equal(t, "gateway not started", response["reason"])
}

// TestGatewayInfo tests the gateway info endpoint
func TestGatewayInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:    "/api/test",
			Method:  "GET",
			Service: "test-service",
			Targets: []string{"http://localhost:8001"},
		},
	}

	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test gateway info
	req := httptest.NewRequest("GET", "/info", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", response["version"])
	assert.Equal(t, config.Host, response["host"])
	assert.Equal(t, float64(config.Port), response["port"])
	assert.Contains(t, response, "routes")
	assert.Contains(t, response, "timestamp")
}

// TestProxyRequestBody tests proxy with request body
func TestProxyRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create a mock backend server that captures request body
	var receivedBody []byte
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"body_received": string(receivedBody),
		})
	}))
	defer backendServer.Close()

	// Create gateway config
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:    "/api/test",
			Method:  "POST",
			Service: "test-service",
			Targets: []string{backendServer.URL},
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request with body
	requestBody := `{"message": "Hello from client"}`
	req := httptest.NewRequest("POST", "/api/test", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, requestBody, response["body_received"])
}

// TestProxyRetry tests proxy retry functionality
func TestProxyRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	attemptCount := 0
	var backendServer *httptest.Server
	backendServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			hj, ok := w.(http.Hijacker)
			require.True(t, ok)
			conn, _, err := hj.Hijack()
			require.NoError(t, err)
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"attempt": attemptCount,
		})
	}))
	defer backendServer.Close()

	// Create gateway config with retries
	config := DefaultConfig()
	config.Routes = []RouteConfig{
		{
			Path:    "/api/test",
			Method:  "GET",
			Service: "test-service",
			Targets: []string{backendServer.URL},
			Retries: 3,
		},
	}

	// Create gateway and setup routes
	gw := NewGateway(config)
	engine := gin.New()
	gw.SetupRoutes(engine)

	// Test proxy request with retries
	req := httptest.NewRequest("GET", "/api/test", nil)
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(3), response["attempt"])
}
