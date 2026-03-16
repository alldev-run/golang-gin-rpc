package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	assert.NoError(t, err)
	assert.NotEmpty(t, endpoints)
	assert.Equal(t, "http://localhost:8001", endpoints[0])
	
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
