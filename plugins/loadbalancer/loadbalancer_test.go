package loadbalancer

import (
	"context"
	"fmt"
	"testing"
)

// TestLogger is a simple logger for testing
type TestLogger struct {
	logs []string
}

func (l *TestLogger) Debug(msg string, fields ...Field) {
	l.logs = append(l.logs, fmt.Sprintf("DEBUG: %s %v", msg, fields))
}

func (l *TestLogger) Info(msg string, fields ...Field) {
	l.logs = append(l.logs, fmt.Sprintf("INFO: %s %v", msg, fields))
}

func (l *TestLogger) Warn(msg string, fields ...Field) {
	l.logs = append(l.logs, fmt.Sprintf("WARN: %s %v", msg, fields))
}

func (l *TestLogger) Error(msg string, fields ...Field) {
	l.logs = append(l.logs, fmt.Sprintf("ERROR: %s %v", msg, fields))
}

func (l *TestLogger) GetLogs() []string {
	return l.logs
}

func (l *TestLogger) Clear() {
	l.logs = nil
}

// createTestTarget creates a test target
func createTestTarget(address string, weight int) *Target {
	target := NewTarget(address)
	target.Weight = weight
	return target
}

// createTestTargetUnhealthy creates an unhealthy test target
func createTestTargetUnhealthy(address string, weight int) *Target {
	target := createTestTarget(address, weight)
	target.Healthy = false
	return target
}

func TestLoadBalancerFactory(t *testing.T) {
	logger := &TestLogger{}
	factory := NewLoadBalancerFactory(WithLogger(logger))
	
	tests := []struct {
		name     string
		strategy Strategy
		wantErr  bool
	}{
		{"RoundRobin", StrategyRoundRobin, false},
		{"Random", StrategyRandom, false},
		{"Weighted", StrategyWeighted, false},
		{"LeastConnections", StrategyLeastConnections, false},
		{"Unknown", Strategy("unknown"), true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb, err := factory.Create(tt.strategy)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && lb == nil {
				t.Error("Create() returned nil load balancer")
			}
		})
	}
}

func TestRoundRobinLoadBalancer(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewRoundRobinLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 1),
		createTestTarget("target2", 1),
		createTestTarget("target3", 1),
	}
	
	err := lb.UpdateTargets(targets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	// Test round-robin selection
	selected := make(map[string]int)
	for i := 0; i < 9; i++ {
		target, err := lb.Select(context.Background(), nil)
		if err != nil {
			t.Errorf("Select() error = %v", err)
			continue
		}
		selected[target.Address]++
	}
	
	// Each target should be selected 3 times
	for addr, count := range selected {
		if count != 3 {
			t.Errorf("Target %s selected %d times, want 3", addr, count)
		}
	}
}

func TestRandomLoadBalancer(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewRandomLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 1),
		createTestTarget("target2", 1),
		createTestTarget("target3", 1),
	}
	
	err := lb.UpdateTargets(targets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	// Test random selection
	selected := make(map[string]int)
	for i := 0; i < 30; i++ {
		target, err := lb.Select(context.Background(), nil)
		if err != nil {
			t.Errorf("Select() error = %v", err)
			continue
		}
		selected[target.Address]++
	}
	
	// Each target should be selected at least once
	for _, target := range targets {
		if selected[target.Address] == 0 {
			t.Errorf("Target %s was never selected", target.Address)
		}
	}
}

func TestWeightedLoadBalancer(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewWeightedLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 5), // 50%
		createTestTarget("target2", 3), // 30%
		createTestTarget("target3", 2), // 20%
	}
	
	err := lb.UpdateTargets(targets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	// Test weighted selection
	selected := make(map[string]int)
	for i := 0; i < 100; i++ {
		target, err := lb.Select(context.Background(), nil)
		if err != nil {
			t.Errorf("Select() error = %v", err)
			continue
		}
		selected[target.Address]++
	}
	
	// Check approximate distribution (allow some variance)
	target1Ratio := float64(selected["target1"]) / 100.0
	target2Ratio := float64(selected["target2"]) / 100.0
	target3Ratio := float64(selected["target3"]) / 100.0
	
	if target1Ratio < 0.4 || target1Ratio > 0.6 {
		t.Errorf("Target1 ratio %.2f, want ~0.5", target1Ratio)
	}
	if target2Ratio < 0.2 || target2Ratio > 0.4 {
		t.Errorf("Target2 ratio %.2f, want ~0.3", target2Ratio)
	}
	if target3Ratio < 0.1 || target3Ratio > 0.3 {
		t.Errorf("Target3 ratio %.2f, want ~0.2", target3Ratio)
	}
}

func TestLeastConnectionsLoadBalancer(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewLeastConnectionsLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 1),
		createTestTarget("target2", 1),
		createTestTarget("target3", 1),
	}
	
	err := lb.UpdateTargets(targets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	// Test least connections selection
	// First selection should be any target
	target, err := lb.Select(context.Background(), nil)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	
	firstSelected := target.Address
	
	// Second selection should be a different target (since first has 1 connection)
	target, err = lb.Select(context.Background(), nil)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	
	if target.Address == firstSelected {
		t.Error("Expected different target to be selected")
	}
	
	// Test connection release
	err = lb.ReleaseConnection(firstSelected)
	if err != nil {
		t.Errorf("ReleaseConnection() error = %v", err)
	}
	
	// Check connection count
	count := lb.GetConnectionCount(firstSelected)
	if count != 0 {
		t.Errorf("Expected connection count 0, got %d", count)
	}
}

func TestHealthFiltering(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	opts.EnableHealthCheck = true
	
	lb := NewRoundRobinLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 1),
		createTestTargetUnhealthy("target2", 1),
		createTestTarget("target3", 1),
	}
	
	err := lb.UpdateTargets(targets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	// Test that only healthy targets are selected
	selected := make(map[string]bool)
	for i := 0; i < 10; i++ {
		target, err := lb.Select(context.Background(), nil)
		if err != nil {
			t.Errorf("Select() error = %v", err)
			continue
		}
		selected[target.Address] = true
	}
	
	// Should only select healthy targets
	if selected["target2"] {
		t.Error("Unhealthy target was selected")
	}
	
	if !selected["target1"] || !selected["target3"] {
		t.Error("Healthy targets were not selected")
	}
}

func TestNoTargetsAvailable(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewRoundRobinLoadBalancer(opts)
	defer lb.Close()
	
	// Test with empty targets
	target, err := lb.Select(context.Background(), nil)
	if err == nil {
		t.Error("Expected error when no targets available")
	}
	if target != nil {
		t.Error("Expected nil target when no targets available")
	}
	
	// Test with only unhealthy targets
	unhealthyTargets := []*Target{
		createTestTargetUnhealthy("target1", 1),
	}
	
	err = lb.UpdateTargets(unhealthyTargets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	target, err = lb.Select(context.Background(), nil)
	if err == nil {
		t.Error("Expected error when no healthy targets available")
	}
	if target != nil {
		t.Error("Expected nil target when no healthy targets available")
	}
}

func TestClose(t *testing.T) {
	logger := &TestLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewRoundRobinLoadBalancer(opts)
	
	targets := []*Target{
		createTestTarget("target1", 1),
	}
	
	err := lb.UpdateTargets(targets)
	if err != nil {
		t.Fatalf("UpdateTargets() error = %v", err)
	}
	
	// Close the load balancer
	err = lb.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
	
	// Try to update targets after close
	err = lb.UpdateTargets(targets)
	if err == nil {
		t.Error("Expected error when updating targets after close")
	}
}

func BenchmarkRoundRobin(b *testing.B) {
	logger := &NoopLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewRoundRobinLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 1),
		createTestTarget("target2", 1),
		createTestTarget("target3", 1),
	}
	
	lb.UpdateTargets(targets)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = lb.Select(context.Background(), nil)
	}
}

func BenchmarkRandom(b *testing.B) {
	logger := &NoopLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewRandomLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 1),
		createTestTarget("target2", 1),
		createTestTarget("target3", 1),
	}
	
	lb.UpdateTargets(targets)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = lb.Select(context.Background(), nil)
	}
}

func BenchmarkWeighted(b *testing.B) {
	logger := &NoopLogger{}
	opts := DefaultOptions()
	opts.Logger = logger
	
	lb := NewWeightedLoadBalancer(opts)
	defer lb.Close()
	
	targets := []*Target{
		createTestTarget("target1", 5),
		createTestTarget("target2", 3),
		createTestTarget("target3", 2),
	}
	
	lb.UpdateTargets(targets)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = lb.Select(context.Background(), nil)
	}
}
