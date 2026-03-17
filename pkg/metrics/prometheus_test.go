package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	
	if collector == nil {
		t.Error("NewMetricsCollector() should not return nil")
	}
	
	// Test that metrics are initialized
	if collector.httpRequestsTotal == nil {
		t.Error("NewMetricsCollector() should initialize httpRequestsTotal")
	}
	
	if collector.httpRequestDuration == nil {
		t.Error("NewMetricsCollector() should initialize httpRequestDuration")
	}
	
	if collector.rpcRequestsTotal == nil {
		t.Error("NewMetricsCollector() should initialize rpcRequestsTotal")
	}
	
	if collector.dbConnectionsActive == nil {
		t.Error("NewMetricsCollector() should initialize dbConnectionsActive")
	}
	
	if collector.cacheOperationsTotal == nil {
		t.Error("NewMetricsCollector() should initialize cacheOperationsTotal")
	}
	
	if collector.activeConnections == nil {
		t.Error("NewMetricsCollector() should initialize activeConnections")
	}
}

func TestMetricsCollector_RecordHTTPRequest(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record HTTP request
	collector.RecordHTTPRequest("GET", "/api/users", "200", 100*time.Millisecond)
	
	// Test that the method doesn't panic
	// Note: We can't easily test the actual metric values without Prometheus client
}

func TestMetricsCollector_RecordRPCRequest(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record RPC request
	collector.RecordRPCRequest("UserService", "GetUser", "success", 50*time.Millisecond)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_RecordRPCError(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record RPC error
	collector.RecordRPCError("UserService", "GetUser", "timeout")
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_RecordDBConnection(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record DB connection
	collector.RecordDBConnection("mysql", "primary", 10.0)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_RecordDBQuery(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record DB query
	collector.RecordDBQuery("mysql", "select", 25*time.Millisecond)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_RecordDBError(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record DB error
	collector.RecordDBError("mysql", "select", "connection_lost")
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_RecordCacheOperation(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record cache operation
	collector.RecordCacheOperation("redis", "get", "hit")
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_UpdateCacheHitRatio(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Update cache hit ratio
	collector.UpdateCacheHitRatio("redis", 0.85)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_UpdateDiscoveryServices(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Update discovery services
	collector.UpdateDiscoveryServices("consul", "user-service", 3.0)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_RecordDiscoveryOperation(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Record discovery operation
	collector.RecordDiscoveryOperation("consul", "register", "success")
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_UpdateActiveConnections(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Update active connections
	collector.UpdateActiveConnections("http", 150.0)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_UpdateMemoryUsage(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Update memory usage
	collector.UpdateMemoryUsage("heap", 1024*1024*100) // 100MB
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_UpdateGoroutineCount(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Update goroutine count
	collector.UpdateGoroutineCount("total", 50.0)
	
	// Test that the method doesn't panic
}

func TestMetricsCollector_MetricsHandler(t *testing.T) {
	NewMetricsCollector()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("MetricsHandler() should return 200, got %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("MetricsHandler() should return text/plain content type, got %s", contentType)
	}
}

func TestMetricsCollector_StartMetricsServer(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Test starting server (this will fail, but we can test the logic)
	go func() {
		// This will likely fail due to port being in use, but we can catch it
		defer func() {
			if r := recover(); r != nil {
				// Expected behavior
			}
		}()
		collector.StartMetricsServer(":9999")
	}()
	
	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)
}

func TestMetricsCollector_StartMetricsServerWithContext(t *testing.T) {
	collector := NewMetricsCollector()
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Test starting server with context
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected behavior
			}
		}()
		collector.StartMetricsServerWithContext(ctx, ":9998")
	}()
	
	// Wait for context to timeout
	<-ctx.Done()
	time.Sleep(10 * time.Millisecond)
}

func TestMetricsCollector_MetricsMiddleware(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})
	
	// Wrap with metrics middleware
	wrappedHandler := collector.MetricsMiddleware(testHandler)
	
	// Test with HTTP request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(w, req)
	
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("MetricsMiddleware() should pass through status code, got %d", resp.StatusCode)
	}
	
	body := w.Body.String()
	if body != "test" {
		t.Errorf("MetricsMiddleware() should pass through response body, got %s", body)
	}
}

func TestResponseWriter(t *testing.T) {
	// Test responseWriter wrapper
	originalWriter := httptest.NewRecorder()
	wrapper := &responseWriter{
		ResponseWriter: originalWriter,
		statusCode:     200,
	}
	
	// Test WriteHeader
	wrapper.WriteHeader(404)
	if wrapper.statusCode != 404 {
		t.Errorf("responseWriter.WriteHeader() should set status code, got %d", wrapper.statusCode)
	}
	
	// Test that original writer receives the header
	resp := originalWriter.Result()
	if resp.StatusCode != 404 {
		t.Errorf("responseWriter.WriteHeader() should call original WriteHeader, got %d", resp.StatusCode)
	}
}

func TestSystemMetricsCollector_NewSystemMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	systemCollector := NewSystemMetricsCollector(collector, 100*time.Millisecond)
	
	if systemCollector == nil {
		t.Error("NewSystemMetricsCollector() should not return nil")
	}
	
	if systemCollector.collector != collector {
		t.Error("NewSystemMetricsCollector() should set collector field")
	}
	
	if systemCollector.interval != 100*time.Millisecond {
		t.Errorf("NewSystemMetricsCollector() should set interval, got %v", systemCollector.interval)
	}
	
	if systemCollector.stopCh == nil {
		t.Error("NewSystemMetricsCollector() should initialize stop channel")
	}
}

func TestSystemMetricsCollector_collectSystemMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	systemCollector := NewSystemMetricsCollector(collector, 100*time.Millisecond)
	
	// Test collectSystemMetrics - should not panic
	systemCollector.collectSystemMetrics()
}

func TestSystemMetricsCollector_StartStop(t *testing.T) {
	collector := NewMetricsCollector()
	systemCollector := NewSystemMetricsCollector(collector, 10*time.Millisecond)
	
	// Start collection
	go systemCollector.Start()
	
	// Let it run for a short time
	time.Sleep(50 * time.Millisecond)
	
	// Stop collection
	systemCollector.Stop()
	
	// Should stop without panicking
	time.Sleep(10 * time.Millisecond)
}

func TestDefaultMetricsExporter_NewDefaultMetricsExporter(t *testing.T) {
	exporter := NewDefaultMetricsExporter()
	
	if exporter == nil {
		t.Error("NewDefaultMetricsExporter() should not return nil")
	}
	
	if exporter.collector == nil {
		t.Error("NewDefaultMetricsExporter() should initialize collector")
	}
	
	if exporter.systemCollector == nil {
		t.Error("NewDefaultMetricsExporter() should initialize system collector")
	}
}

func TestDefaultMetricsExporter_GetCollector(t *testing.T) {
	exporter := NewDefaultMetricsExporter()
	
	collector := exporter.GetCollector()
	if collector != exporter.collector {
		t.Error("GetCollector() should return the internal collector")
	}
}

func TestDefaultMetricsExporter_StartStop(t *testing.T) {
	exporter := NewDefaultMetricsExporter()
	
	// Test Start (will likely fail due to port, but we can test the logic)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected behavior
			}
		}()
		exporter.Start(":9997")
	}()
	
	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)
	
	// Stop the exporter
	exporter.Stop()
	
	// Should stop without panicking
	time.Sleep(10 * time.Millisecond)
}

func TestDefaultMetricsExporter_StartWithContext(t *testing.T) {
	exporter := NewDefaultMetricsExporter()
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Test Start with context
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Expected behavior
			}
		}()
		exporter.StartWithContext(ctx, ":9996")
	}()
	
	// Wait for context to timeout
	<-ctx.Done()
	
	// Stop the exporter
	exporter.Stop()
}

// Test enhanced metrics (if they exist)
func TestEnhancedMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Test business metrics
	if collector.businessRevenue != nil {
		collector.businessRevenue.WithLabelValues("online", "success").Add(100.0)
	}
	
	if collector.businessOrdersTotal != nil {
		collector.businessOrdersTotal.WithLabelValues("online", "success").Inc()
	}
	
	if collector.businessUsersActive != nil {
		collector.businessUsersActive.WithLabelValues("active").Set(1000.0)
	}
	
	// Test error metrics
	if collector.errorTotal != nil {
		collector.errorTotal.WithLabelValues("validation", "invalid_input").Inc()
	}
	
	if collector.errorRate != nil {
		collector.errorRate.WithLabelValues("http").Set(0.01)
	}
	
	if collector.panicTotal != nil {
		collector.panicTotal.WithLabelValues("api").Inc()
	}
	
	// Test security metrics
	if collector.authAttemptsTotal != nil {
		collector.authAttemptsTotal.WithLabelValues("jwt").Inc()
	}
	
	if collector.authFailuresTotal != nil {
		collector.authFailuresTotal.WithLabelValues("jwt").Inc()
	}
	
	if collector.rateLimitHitsTotal != nil {
		collector.rateLimitHitsTotal.WithLabelValues("ip").Inc()
	}
	
	// Test cache metrics
	if collector.cacheSize != nil {
		collector.cacheSize.WithLabelValues("redis").Set(1000.0)
	}
	
	if collector.cacheEvictions != nil {
		collector.cacheEvictions.WithLabelValues("redis").Inc()
	}
	
	// Test database metrics
	if collector.dbConnectionsIdle != nil {
		collector.dbConnectionsIdle.WithLabelValues("mysql", "primary").Set(5.0)
	}
	
	if collector.dbTransactionCount != nil {
		collector.dbTransactionCount.WithLabelValues("mysql", "commit").Inc()
	}
	
	// Test HTTP metrics
	if collector.httpRequestSize != nil {
		collector.httpRequestSize.WithLabelValues("POST", "/api/users").Observe(1024.0)
	}
	
	if collector.httpResponseSize != nil {
		collector.httpResponseSize.WithLabelValues("GET", "/api/users").Observe(2048.0)
	}
}

// Test custom metrics functionality
func TestCustomMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Test that custom metrics map is initialized
	if collector.customMetrics == nil {
		t.Error("NewMetricsCollector() should initialize custom metrics map")
	}
	
	// Test registering custom metric (if method exists)
	// This would require the enhanced metrics implementation
}

// Benchmark tests
func BenchmarkMetricsCollector_RecordHTTPRequest(b *testing.B) {
	collector := NewMetricsCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordHTTPRequest("GET", "/api/users", "200", 100*time.Millisecond)
	}
}

func BenchmarkMetricsCollector_RecordRPCRequest(b *testing.B) {
	collector := NewMetricsCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordRPCRequest("UserService", "GetUser", "success", 50*time.Millisecond)
	}
}

func BenchmarkMetricsCollector_RecordDBQuery(b *testing.B) {
	collector := NewMetricsCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordDBQuery("mysql", "select", 25*time.Millisecond)
	}
}

func BenchmarkMetricsCollector_RecordCacheOperation(b *testing.B) {
	collector := NewMetricsCollector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordCacheOperation("redis", "get", "hit")
	}
}

func BenchmarkMetricsCollector_MetricsMiddleware(b *testing.B) {
	collector := NewMetricsCollector()
	
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := collector.MetricsMiddleware(testHandler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
	}
}

func BenchmarkSystemMetricsCollector_collectSystemMetrics(b *testing.B) {
	collector := NewMetricsCollector()
	systemCollector := NewSystemMetricsCollector(collector, 100*time.Millisecond)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		systemCollector.collectSystemMetrics()
	}
}

// Integration test for metrics flow
func TestMetricsFlow(t *testing.T) {
	collector := NewMetricsCollector()

	// Simulate application activity
	for i := 0; i < 100; i++ {
		collector.RecordHTTPRequest("GET", "/api/users", "200", time.Duration(i)*time.Millisecond)
		collector.RecordDBQuery("mysql", "select", time.Duration(i)*time.Millisecond)
		collector.RecordCacheOperation("redis", "get", "hit")
		collector.UpdateActiveConnections("http", float64(i))
	}
	
	// Test metrics handler
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Metrics flow test: handler should return 200, got %d", resp.StatusCode)
	}
	
	// Check that metrics are present in output
	body := w.Body.String()
	
	// Check for some basic metric names
	expectedMetrics := []string{
		"http_requests_total",
		"db_query_duration_seconds",
		"cache_operations_total",
		"active_connections",
	}
	
	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Metrics flow test: output should contain metric %s", metric)
		}
	}
}

// Test error handling
func TestMetricsErrorHandling(t *testing.T) {
	collector := NewMetricsCollector()
	
	// Test with nil values (should not panic)
	collector.RecordHTTPRequest("", "", "", 0)
	collector.RecordRPCRequest("", "", "", 0)
	collector.RecordDBConnection("", "", 0)
	collector.RecordCacheOperation("", "", "")
	collector.UpdateActiveConnections("", 0)
	collector.UpdateMemoryUsage("", 0)
	collector.UpdateGoroutineCount("", 0)
	
	// Test metrics middleware with nil request
	handler := collector.MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, nil)
	
	// Should not panic
}

// Test concurrent access
func TestMetricsConcurrentAccess(t *testing.T) {
	collector := NewMetricsCollector()
	
	var wg sync.WaitGroup
	concurrency := 100
	operations := 1000
	
	// Concurrent HTTP requests
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				collector.RecordHTTPRequest("GET", "/test", "200", time.Millisecond)
			}
		}(i)
	}
	
	// Concurrent DB operations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				collector.RecordDBQuery("mysql", "select", time.Millisecond)
			}
		}(i)
	}
	
	// Concurrent cache operations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				collector.RecordCacheOperation("redis", "get", "hit")
			}
		}(i)
	}
	
	wg.Wait()
	
	// Should complete without race conditions
}
