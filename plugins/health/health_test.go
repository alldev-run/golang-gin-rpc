package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// CacheStats for testing
type CacheStats struct {
	Hits       uint64
	Misses     uint64
	Sets       uint64
	Deletes    uint64
	Errors     uint64
	LastAccess time.Time
}

// MockHealthChecker implements HealthChecker for testing
type MockHealthChecker struct {
	name      string
	result    *CheckResult
	shouldErr bool
	delay     time.Duration
}

func NewMockHealthChecker(name string, status HealthStatus) *MockHealthChecker {
	return &MockHealthChecker{
		name: name,
		result: &CheckResult{
			Name:      name,
			Status:    status,
			Message:   "Mock check result",
			Timestamp: time.Now(),
		},
	}
}

func (m *MockHealthChecker) Name() string {
	return m.name
}

func (m *MockHealthChecker) Check(ctx context.Context) *CheckResult {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return &CheckResult{
				Name:      m.name,
				Status:    StatusUnknown,
				Message:   "Check timed out",
				Timestamp: time.Now(),
			}
		case <-time.After(m.delay):
			// Continue with check
		}
	}
	
	if m.shouldErr {
		return &CheckResult{
			Name:      m.name,
			Status:    StatusUnhealthy,
			Message:   "Mock error",
			Timestamp: time.Now(),
		}
	}
	
	result := *m.result
	result.Timestamp = time.Now()
	result.Duration = time.Since(result.Timestamp)
	result.LastChecked = time.Now()
	return &result
}

func (m *MockHealthChecker) SetShouldErr(shouldErr bool) {
	m.shouldErr = shouldErr
}

func (m *MockHealthChecker) SetDelay(delay time.Duration) {
	m.delay = delay
}

func TestNewHealthManager(t *testing.T) {
	hm := NewHealthManager()
	
	if hm.checkers == nil {
		t.Error("NewHealthManager() should initialize checkers map")
	}
	
	if hm.configs == nil {
		t.Error("NewHealthManager() should initialize configs map")
	}
	
	if hm.results == nil {
		t.Error("NewHealthManager() should initialize results map")
	}
	
	if hm.stopCh == nil {
		t.Error("NewHealthManager() should initialize stop channel")
	}
	
	if hm.running {
		t.Error("NewHealthManager() should not be running initially")
	}
}

func TestHealthManager_RegisterChecker(t *testing.T) {
	hm := NewHealthManager()
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	
	hm.RegisterChecker(checker, config)
	
	// Verify checker was registered
	if _, exists := hm.checkers["test"]; !exists {
		t.Error("RegisterChecker() should add checker to map")
	}
	
	if _, exists := hm.configs["test"]; !exists {
		t.Error("RegisterChecker() should add config to map")
	}
}

func TestHealthManager_UnregisterChecker(t *testing.T) {
	hm := NewHealthManager()
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	
	// Register first
	hm.RegisterChecker(checker, config)
	
	// Then unregister
	hm.UnregisterChecker("test")
	
	// Verify checker was removed
	if _, exists := hm.checkers["test"]; exists {
		t.Error("UnregisterChecker() should remove checker from map")
	}
	
	if _, exists := hm.configs["test"]; exists {
		t.Error("UnregisterChecker() should remove config from map")
	}
	
	if _, exists := hm.results["test"]; exists {
		t.Error("UnregisterChecker() should remove result from map")
	}
}

func TestHealthManager_StartStop(t *testing.T) {
	hm := NewHealthManager()
	
	// Start
	hm.Start()
	
	if !hm.running {
		t.Error("Start() should set running to true")
	}
	
	if hm.ticker == nil {
		t.Error("Start() should initialize ticker")
	}
	
	// Stop
	hm.Stop()
	
	if hm.running {
		t.Error("Stop() should set running to false")
	}
}

func TestHealthManager_Start_Idempotent(t *testing.T) {
	hm := NewHealthManager()
	
	// Start multiple times
	hm.Start()
	hm.Start()
	
	if !hm.running {
		t.Error("Start() should be idempotent")
	}
	
	// Stop
	hm.Stop()
}

func TestHealthManager_CheckHealth(t *testing.T) {
	hm := NewHealthManager()
	
	// Register checkers
	healthyChecker := NewMockHealthChecker("healthy", StatusHealthy)
	unhealthyChecker := NewMockHealthChecker("unhealthy", StatusUnhealthy)
	
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(healthyChecker, config)
	hm.RegisterChecker(unhealthyChecker, config)
	
	// Perform health check
	ctx := context.Background()
	report := hm.CheckHealth(ctx)
	
	// Verify report
	if report.Timestamp.IsZero() {
		t.Error("CheckHealth() should set timestamp")
	}
	
	if len(report.Checks) != 2 {
		t.Errorf("CheckHealth() should return 2 checks, got %d", len(report.Checks))
	}
	
	if report.Summary.Total != 2 {
		t.Errorf("CheckHealth() summary total should be 2, got %d", report.Summary.Total)
	}
	
	if report.Summary.Healthy != 1 {
		t.Errorf("CheckHealth() summary healthy should be 1, got %d", report.Summary.Healthy)
	}
	
	if report.Summary.Unhealthy != 1 {
		t.Errorf("CheckHealth() summary unhealthy should be 1, got %d", report.Summary.Unhealthy)
	}
	
	if report.Status != StatusUnhealthy {
		t.Errorf("CheckHealth() status should be unhealthy when any check is unhealthy, got %v", report.Status)
	}
}

func TestHealthManager_CheckHealth_DisabledChecker(t *testing.T) {
	hm := NewHealthManager()
	
	// Register disabled checker
	disabledChecker := NewMockHealthChecker("disabled", StatusUnhealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = false
	
	hm.RegisterChecker(disabledChecker, config)
	
	// Perform health check
	ctx := context.Background()
	report := hm.CheckHealth(ctx)
	
	// Disabled checker should not be included
	if len(report.Checks) != 0 {
		t.Errorf("CheckHealth() should not include disabled checkers, got %d", len(report.Checks))
	}
	
	if report.Status != StatusHealthy {
		t.Errorf("CheckHealth() status should be healthy when no checks are enabled, got %v", report.Status)
	}
}

func TestHealthManager_CheckHealth_Timeout(t *testing.T) {
	hm := NewHealthManager()
	
	// Register checker with delay
	slowChecker := NewMockHealthChecker("slow", StatusHealthy)
	slowChecker.SetDelay(100 * time.Millisecond)
	
	config := DefaultHealthCheckConfig()
	config.Timeout = 10 * time.Millisecond // Short timeout
	config.Enabled = true
	
	hm.RegisterChecker(slowChecker, config)
	
	// Perform health check
	ctx := context.Background()
	report := hm.CheckHealth(ctx)
	
	// Should have timeout result
	if len(report.Checks) != 1 {
		t.Errorf("CheckHealth() should return 1 check, got %d", len(report.Checks))
	}
	
	result := report.Checks["slow"]
	if result.Status != StatusUnknown {
		t.Errorf("CheckHealth() should return unknown status for timeout, got %v", result.Status)
	}
	
	if result.Message != "Check timed out" {
		t.Errorf("CheckHealth() should return timeout message, got %v", result.Message)
	}
}

func TestHealthManager_CheckHealthByName(t *testing.T) {
	hm := NewHealthManager()
	
	// Register checker
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(checker, config)
	
	// Check specific checker
	ctx := context.Background()
	result := hm.CheckHealthByName(ctx, "test")
	
	if result.Name != "test" {
		t.Errorf("CheckHealthByName() should return result for specified checker, got %v", result.Name)
	}
	
	if result.Status != StatusHealthy {
		t.Errorf("CheckHealthByName() should return correct status, got %v", result.Status)
	}
}

func TestHealthManager_CheckHealthByName_NotFound(t *testing.T) {
	hm := NewHealthManager()
	
	ctx := context.Background()
	result := hm.CheckHealthByName(ctx, "nonexistent")
	
	if result.Name != "nonexistent" {
		t.Errorf("CheckHealthByName() should return result with requested name, got %v", result.Name)
	}
	
	if result.Status != StatusUnknown {
		t.Errorf("CheckHealthByName() should return unknown status for non-existent checker, got %v", result.Status)
	}
	
	if result.Message != "Checker not found" {
		t.Errorf("CheckHealthByName() should return not found message, got %v", result.Message)
	}
}

func TestHealthManager_CheckHealthByName_Disabled(t *testing.T) {
	hm := NewHealthManager()
	
	// Register disabled checker
	checker := NewMockHealthChecker("disabled", StatusHealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = false
	
	hm.RegisterChecker(checker, config)
	
	ctx := context.Background()
	result := hm.CheckHealthByName(ctx, "disabled")
	
	if result.Status != StatusUnknown {
		t.Errorf("CheckHealthByName() should return unknown status for disabled checker, got %v", result.Status)
	}
	
	if result.Message != "Checker disabled" {
		t.Errorf("CheckHealthByName() should return disabled message, got %v", result.Message)
	}
}

func TestHealthManager_GetCachedResults(t *testing.T) {
	hm := NewHealthManager()
	
	// Register checker
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(checker, config)
	
	// Perform health check to populate cache
	ctx := context.Background()
	hm.CheckHealth(ctx)
	
	// Get cached results
	report := hm.GetCachedResults()
	
	if len(report.Checks) != 1 {
		t.Errorf("GetCachedResults() should return cached results, got %d", len(report.Checks))
	}
	
	// Should not perform new checks
	if checker.Check(ctx) == report.Checks["test"] {
		t.Error("GetCachedResults() should return cached results, not perform new checks")
	}
}

func TestHealthManager_HTTPHandler(t *testing.T) {
	hm := NewHealthManager()
	
	// Register checker
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(checker, config)
	
	// Create HTTP request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Call handler
	handler := hm.HTTPHandler()
	handler.ServeHTTP(w, req)
	
	// Check response
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTPHandler() should return 200 for healthy status, got %d", resp.StatusCode)
	}
	
	// Parse response
	var report HealthReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Errorf("HTTPHandler() should return valid JSON, error: %v", err)
	}
	
	if len(report.Checks) != 1 {
		t.Errorf("HTTPHandler() should return health report with checks, got %d", len(report.Checks))
	}
}

func TestHealthManager_HTTPHandler_SpecificCheck(t *testing.T) {
	hm := NewHealthManager()
	
	// Register checker
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(checker, config)
	
	// Create HTTP request for specific check
	req := httptest.NewRequest("GET", "/health?check=test", nil)
	w := httptest.NewRecorder()
	
	// Call handler
	handler := hm.HTTPHandler()
	handler.ServeHTTP(w, req)
	
	// Check response
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTPHandler() should return 200 for healthy specific check, got %d", resp.StatusCode)
	}
	
	// Parse response
	var result CheckResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("HTTPHandler() should return valid JSON for specific check, error: %v", err)
	}
	
	if result.Name != "test" {
		t.Errorf("HTTPHandler() should return result for specific check, got %v", result.Name)
	}
}

func TestHealthManager_HTTPHandler_Unhealthy(t *testing.T) {
	hm := NewHealthManager()
	
	// Register unhealthy checker
	checker := NewMockHealthChecker("test", StatusUnhealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(checker, config)
	
	// Create HTTP request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Call handler
	handler := hm.HTTPHandler()
	handler.ServeHTTP(w, req)
	
	// Check response
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("HTTPHandler() should return 503 for unhealthy status, got %d", resp.StatusCode)
	}
}

func TestHealthManager_HTTPHandler_Degraded(t *testing.T) {
	hm := NewHealthManager()
	
	// Register degraded checker
	checker := NewMockHealthChecker("test", StatusDegraded)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	
	hm.RegisterChecker(checker, config)
	
	// Create HTTP request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Call handler
	handler := hm.HTTPHandler()
	handler.ServeHTTP(w, req)
	
	// Check response
	resp := w.Result()
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTPHandler() should return 200 for degraded status, got %d", resp.StatusCode)
	}
}

func TestDatabaseHealthChecker(t *testing.T) {
	// Mock database
	mockDB := &MockDB{pingErr: nil}
	checker := NewDatabaseHealthChecker("test_db", mockDB)
	
	if checker.Name() != "test_db" {
		t.Errorf("DatabaseHealthChecker.Name() = %v, want %v", checker.Name(), "test_db")
	}
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Name != "test_db" {
		t.Errorf("DatabaseHealthChecker.Check() name = %v, want %v", result.Name, "test_db")
	}
	
	if result.Status != StatusHealthy {
		t.Errorf("DatabaseHealthChecker.Check() status = %v, want %v", result.Status, StatusHealthy)
	}
	
	if result.Message != "Database is healthy" {
		t.Errorf("DatabaseHealthChecker.Check() message = %v, want %v", result.Message, "Database is healthy")
	}
}

func TestDatabaseHealthChecker_Error(t *testing.T) {
	// Mock database with error
	mockDB := &MockDB{pingErr: errors.New("connection failed")}
	checker := NewDatabaseHealthChecker("test_db", mockDB)
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Status != StatusUnhealthy {
		t.Errorf("DatabaseHealthChecker.Check() status = %v, want %v", result.Status, StatusUnhealthy)
	}
	
	if result.Message != "Database ping failed: connection failed" {
		t.Errorf("DatabaseHealthChecker.Check() message = %v, want %v", result.Message, "Database ping failed: connection failed")
	}
}

func TestDatabaseHealthChecker_NilDB(t *testing.T) {
	checker := NewDatabaseHealthChecker("test_db", nil)
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Status != StatusUnhealthy {
		t.Errorf("DatabaseHealthChecker.Check() status = %v, want %v", result.Status, StatusUnhealthy)
	}
	
	if result.Message != "Database connection is nil" {
		t.Errorf("DatabaseHealthChecker.Check() message = %v, want %v", result.Message, "Database connection is nil")
	}
}

func TestHTTPHealthChecker(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	checker := NewHTTPHealthChecker("test_http", server.URL)
	
	if checker.Name() != "test_http" {
		t.Errorf("HTTPHealthChecker.Name() = %v, want %v", checker.Name(), "test_http")
	}
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Name != "test_http" {
		t.Errorf("HTTPHealthChecker.Check() name = %v, want %v", result.Name, "test_http")
	}
	
	if result.Status != StatusHealthy {
		t.Errorf("HTTPHealthChecker.Check() status = %v, want %v", result.Status, StatusHealthy)
	}
	
	if result.Message != "HTTP endpoint is healthy" {
		t.Errorf("HTTPHealthChecker.Check() message = %v, want %v", result.Message, "HTTP endpoint is healthy")
	}
}

func TestHTTPHealthChecker_Error(t *testing.T) {
	// Use a more clearly invalid URL
	checker := NewHTTPHealthChecker("test_http", "http://127.0.0.1:99999") // Invalid port
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	// Should be unhealthy due to connection error
	if result.Status == StatusHealthy {
		t.Errorf("HTTPHealthChecker.Check() status should not be healthy for invalid URL")
	}
	
	if result.Message == "" {
		t.Errorf("HTTPHealthChecker.Check() message should not be empty for invalid URL")
	}
}

func TestHTTPHealthChecker_StatusCodes(t *testing.T) {
	tests := []struct {
		statusCode int
		expected  HealthStatus
	}{
		{200, StatusHealthy},
		{201, StatusHealthy},
		{299, StatusHealthy},
		{400, StatusDegraded},
		{401, StatusDegraded},
		{404, StatusDegraded},
		{499, StatusDegraded},
		{500, StatusUnhealthy},
		{502, StatusUnhealthy},
		{599, StatusUnhealthy},
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			// Mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()
			
			checker := NewHTTPHealthChecker("test_http", server.URL)
			ctx := context.Background()
			result := checker.Check(ctx)
			
			if result.Status != tt.expected {
				t.Errorf("HTTPHealthChecker.Check() status = %v, want %v for status code %d", result.Status, tt.expected, tt.statusCode)
			}
			
			if result.Details["status_code"] != tt.statusCode {
				t.Errorf("HTTPHealthChecker.Check() status_code = %v, want %v", result.Details["status_code"], tt.statusCode)
			}
		})
	}
}

func TestCustomHealthChecker(t *testing.T) {
	checkFunc := func(ctx context.Context) *CheckResult {
		return &CheckResult{
			Name:      "custom",
			Status:    StatusHealthy,
			Message:   "Custom check",
			Timestamp: time.Now(),
		}
	}
	
	checker := NewCustomHealthChecker("custom", checkFunc)
	
	if checker.Name() != "custom" {
		t.Errorf("CustomHealthChecker.Name() = %v, want %v", checker.Name(), "custom")
	}
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Name != "custom" {
		t.Errorf("CustomHealthChecker.Check() name = %v, want %v", result.Name, "custom")
	}
	
	if result.Status != StatusHealthy {
		t.Errorf("CustomHealthChecker.Check() status = %v, want %v", result.Status, StatusHealthy)
	}
	
	if result.Message != "Custom check" {
		t.Errorf("CustomHealthChecker.Check() message = %v, want %v", result.Message, "Custom check")
	}
}

func TestCustomHealthChecker_NilFunc(t *testing.T) {
	checker := NewCustomHealthChecker("custom", nil)
	
	ctx := context.Background()
	result := checker.Check(ctx)
	
	if result.Status != StatusUnhealthy {
		t.Errorf("CustomHealthChecker.Check() status = %v, want %v", result.Status, StatusUnhealthy)
	}
	
	if result.Message != "Check function is nil" {
		t.Errorf("CustomHealthChecker.Check() message = %v, want %v", result.Message, "Check function is nil")
	}
}

func TestGenerateReport(t *testing.T) {
	hm := NewHealthManager()
	
	results := map[string]*CheckResult{
		"healthy": {
			Name:      "healthy",
			Status:    StatusHealthy,
			Timestamp: time.Now(),
		},
		"unhealthy": {
			Name:      "unhealthy",
			Status:    StatusUnhealthy,
			Timestamp: time.Now(),
		},
		"degraded": {
			Name:      "degraded",
			Status:    StatusDegraded,
			Timestamp: time.Now(),
		},
	}
	
	report := hm.generateReport(results)
	
	// Check summary
	if report.Summary.Total != 3 {
		t.Errorf("generateReport() total = %v, want %v", report.Summary.Total, 3)
	}
	
	if report.Summary.Healthy != 1 {
		t.Errorf("generateReport() healthy = %v, want %v", report.Summary.Healthy, 1)
	}
	
	if report.Summary.Unhealthy != 1 {
		t.Errorf("generateReport() unhealthy = %v, want %v", report.Summary.Unhealthy, 1)
	}
	
	if report.Summary.Degraded != 1 {
		t.Errorf("generateReport() degraded = %v, want %v", report.Summary.Degraded, 1)
	}
	
	// Check overall status
	if report.Status != StatusUnhealthy {
		t.Errorf("generateReport() status = %v, want %v", report.Status, StatusUnhealthy)
	}
}

func TestGetGlobalHealthManager(t *testing.T) {
	// First call should create new instance
	hm1 := GetGlobalHealthManager()
	if hm1 == nil {
		t.Error("GetGlobalHealthManager() should not return nil")
	}
	
	// Second call should return same instance
	hm2 := GetGlobalHealthManager()
	if hm1 != hm2 {
		t.Error("GetGlobalHealthManager() should return singleton instance")
	}
}

// Mock implementations for testing
type MockDB struct {
	pingErr error
}

func (m *MockDB) Ping(ctx context.Context) error {
	return m.pingErr
}

type mockCache struct {
	setErr error
	data   map[string]interface{}
}

func (m *mockCache) Get(ctx context.Context, key string) (interface{}, error) {
	if m.data == nil {
		return nil, errors.New("cache not initialized")
	}
	return m.data[key], nil
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.setErr != nil {
		return m.setErr
	}
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.data == nil {
		return nil
	}
	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	if m.data == nil {
		return false, errors.New("cache not initialized")
	}
	_, exists := m.data[key]
	return exists, nil
}

func (m *mockCache) Clear(ctx context.Context) error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCache) Close() error {
	return nil
}

func (m *mockCache) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	return m.Get(ctx, key)
}

func (m *mockCache) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	return m.Set(ctx, key, value, baseTTL)
}

func (m *mockCache) GetStats() CacheStats {
	return CacheStats{}
}

// Benchmark tests
func BenchmarkHealthManager_CheckHealth(b *testing.B) {
	hm := NewHealthManager()
	
	// Register checkers
	for i := 0; i < 10; i++ {
		checker := NewMockHealthChecker(fmt.Sprintf("checker_%d", i), StatusHealthy)
		config := DefaultHealthCheckConfig()
		config.Enabled = true
		hm.RegisterChecker(checker, config)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hm.CheckHealth(ctx)
	}
}

func BenchmarkHealthManager_HTTPHandler(b *testing.B) {
	hm := NewHealthManager()
	
	// Register checker
	checker := NewMockHealthChecker("test", StatusHealthy)
	config := DefaultHealthCheckConfig()
	config.Enabled = true
	hm.RegisterChecker(checker, config)
	
	handler := hm.HTTPHandler()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
