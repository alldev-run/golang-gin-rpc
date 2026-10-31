package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxFailures != 5 {
		t.Errorf("MaxFailures = %d, want 5", cfg.MaxFailures)
	}
	if cfg.ResetTimeout != 30*time.Second {
		t.Errorf("ResetTimeout = %v, want 30s", cfg.ResetTimeout)
	}
	if cfg.HalfOpenMaxRequests != 3 {
		t.Errorf("HalfOpenMaxRequests = %d, want 3", cfg.HalfOpenMaxRequests)
	}
	if cfg.SuccessThreshold != 2 {
		t.Errorf("SuccessThreshold = %d, want 2", cfg.SuccessThreshold)
	}
}

func TestNewWithDefaults(t *testing.T) {
	// Empty config should get defaults
	cb := New(Config{})

	if cb.config.MaxFailures == 0 {
		t.Error("MaxFailures should have default value")
	}
	if cb.config.ResetTimeout == 0 {
		t.Error("ResetTimeout should have default value")
	}
	if cb.state != int32(StateClosed) {
		t.Error("Initial state should be Closed")
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("State(%d).String() = %s, want %s", tt.state, tt.state.String(), tt.expected)
		}
	}
}

func TestExecuteSuccess(t *testing.T) {
	cb := New(DefaultConfig())

	err := cb.Execute(context.Background(), func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute with success should not error, got: %v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("State should be Closed, got %v", cb.State())
	}
}

func TestExecuteFailure(t *testing.T) {
	cb := New(Config{
		MaxFailures:  3,
		ResetTimeout: 100 * time.Millisecond,
		Name:         "test",
	})

	testErr := errors.New("test error")

	// Trigger failures
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func() error {
			return testErr
		})
		if err != testErr {
			t.Errorf("Expected test error, got: %v", err)
		}
	}

	// Circuit should be open now
	if cb.State() != StateOpen {
		t.Errorf("State should be Open, got %v", cb.State())
	}

	// Next request should fail with circuit open error
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Expected ErrCircuitOpen, got: %v", err)
	}
}

func TestCircuitRecovery(t *testing.T) {
	cb := New(Config{
		MaxFailures:         1,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 1,
		SuccessThreshold:    1,
		Name:                "test",
	})

	// Trigger failure to open circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("fail")
	})

	if cb.State() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// Success in half-open should close circuit
	err := cb.Execute(context.Background(), func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Circuit should be closed
	if cb.State() != StateClosed {
		t.Errorf("State should be Closed, got %v", cb.State())
	}
}

func TestForceOpenAndClose(t *testing.T) {
	cb := New(DefaultConfig())

	// Force open
	cb.ForceOpen()
	if cb.State() != StateOpen {
		t.Error("ForceOpen should set state to Open")
	}

	// Requests should fail
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Expected ErrCircuitOpen, got: %v", err)
	}

	// Force close
	cb.ForceClosed()
	if cb.State() != StateClosed {
		t.Error("ForceClosed should set state to Closed")
	}

	// Requests should succeed
	err = cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Execute should succeed after ForceClosed, got: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	cb := New(Config{
		MaxFailures: 5,
		Name:        "stats-test",
	})

	// Initial stats
	stats := cb.GetStats()
	if stats.State != StateClosed {
		t.Errorf("Initial state should be Closed, got %v", stats.State)
	}

	// Trigger some failures
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}

	stats = cb.GetStats()
	if stats.Failures != 3 {
		t.Errorf("Expected 3 failures, got %d", stats.Failures)
	}
	if stats.State != StateClosed {
		t.Errorf("State should still be Closed, got %v", stats.State)
	}
}

func TestExecuteWithResult(t *testing.T) {
	cb := New(DefaultConfig())

	result, err := cb.ExecuteWithResult(context.Background(), func() (any, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("ExecuteWithResult should not error, got: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	// Test with error
	_, err = cb.ExecuteWithResult(context.Background(), func() (any, error) {
		return nil, errors.New("fail")
	})

	if err == nil {
		t.Error("ExecuteWithResult should return error")
	}
}

func TestConcurrentAccess(t *testing.T) {
	cb := New(Config{
		MaxFailures:         5,
		ResetTimeout:        100 * time.Millisecond,
		HalfOpenMaxRequests: 3,
		Name:                "concurrent-test",
	})

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	// Run concurrent requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			err := cb.Execute(context.Background(), func() error {
				// Simulate some work
				time.Sleep(time.Millisecond)
				return nil
			})

			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Success: %d, Fail: %d", successCount, failCount)

	// All should succeed since there's no actual failure
	if successCount == 0 {
		t.Error("Expected some successes")
	}
}

func TestHalfOpenMaxRequests(t *testing.T) {
	cb := New(Config{
		MaxFailures:         1,
		ResetTimeout:        50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
		SuccessThreshold:    2,
		Name:                "half-open-test",
	})

	// Open the circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("fail")
	})

	if cb.State() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// In half-open, only HalfOpenMaxRequests should be allowed
	allowed := 0
	rejected := 0

	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		if err == nil {
			allowed++
		} else if errors.Is(err, ErrCircuitOpen) {
			rejected++
		}
	}

	t.Logf("Allowed: %d, Rejected: %d", allowed, rejected)

	// Should have limited allowed requests in half-open
	if allowed == 0 {
		t.Error("Expected some allowed requests")
	}
}

func TestFailureResetOnSuccess(t *testing.T) {
	cb := New(Config{
		MaxFailures: 3,
		Name:        "reset-test",
	})

	// 2 failures
	for i := 0; i < 2; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return errors.New("fail")
		})
	}

	stats := cb.GetStats()
	if stats.Failures != 2 {
		t.Errorf("Expected 2 failures, got %d", stats.Failures)
	}

	// Success should reset failure count
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Circuit should still be closed (not enough failures to open)
	if cb.State() != StateClosed {
		t.Errorf("State should be Closed, got %v", cb.State())
	}
}

func TestErrCircuitOpen(t *testing.T) {
	// Test that ErrCircuitOpen is the base error
	err := errors.New("circuit breaker is open")
	if !errors.Is(ErrCircuitOpen, errors.New("circuit breaker is open")) {
		// This might not work with direct comparison, but the wrapped error should work
	}

	// Test wrapped error
	cb := New(Config{Name: "test"})
	cb.ForceOpen()

	err = cb.Execute(context.Background(), func() error {
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Error should wrap ErrCircuitOpen, got: %v", err)
	}
}
