package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		name string
		s    State
		want string
	}{
		{"closed", StateClosed, "closed"},
		{"open", StateOpen, "open"},
		{"half-open", StateHalfOpen, "half-open"},
		{"unknown", State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("State.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.MaxRequests != 1 {
		t.Errorf("DefaultConfig().MaxRequests = %v, want %v", config.MaxRequests, 1)
	}
	if config.Interval != time.Minute {
		t.Errorf("DefaultConfig().Interval = %v, want %v", config.Interval, time.Minute)
	}
	if config.Timeout != time.Minute {
		t.Errorf("DefaultConfig().Timeout = %v, want %v", config.Timeout, time.Minute)
	}
	if config.ReadyToTrip == nil {
		t.Error("DefaultConfig().ReadyToTrip is nil")
	}
	if config.OnStateChange == nil {
		t.Error("DefaultConfig().OnStateChange is nil")
	}
	if config.IsSuccessful == nil {
		t.Error("DefaultConfig().IsSuccessful is nil")
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	name := "test-cb"
	config := Config{
		MaxRequests: 5,
		Interval:    time.Second * 30,
		Timeout:     time.Second * 60,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to State) {
			// Test callback
		},
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}

	cb := NewCircuitBreaker(name, config)

	if cb.name != name {
		t.Errorf("NewCircuitBreaker().name = %v, want %v", cb.name, name)
	}
	if cb.maxRequests != 5 {
		t.Errorf("NewCircuitBreaker().maxRequests = %v, want %v", cb.maxRequests, 5)
	}
	if cb.interval != time.Second*30 {
		t.Errorf("NewCircuitBreaker().interval = %v, want %v", cb.interval, time.Second*30)
	}
	if cb.timeout != time.Second*60 {
		t.Errorf("NewCircuitBreaker().timeout = %v, want %v", cb.timeout, time.Second*60)
	}
	if cb.state != StateClosed {
		t.Errorf("NewCircuitBreaker().state = %v, want %v", cb.state, StateClosed)
	}
}

func TestNewCircuitBreaker_DefaultValues(t *testing.T) {
	config := Config{
		MaxRequests: 0,        // Should default to 1
		Interval:    0,        // Should default to time.Minute
		Timeout:     0,        // Should default to time.Minute
	}

	cb := NewCircuitBreaker("test", config)

	if cb.maxRequests != 1 {
		t.Errorf("Expected maxRequests to default to 1, got %v", cb.maxRequests)
	}
	if cb.interval != time.Minute {
		t.Errorf("Expected interval to default to time.Minute, got %v", cb.interval)
	}
	if cb.timeout != time.Minute {
		t.Errorf("Expected timeout to default to time.Minute, got %v", cb.timeout)
	}
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultConfig())
	
	successFn := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFn)
	
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if result != "success" {
		t.Errorf("Execute() result = %v, want %v", result, "success")
	}
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultConfig())
	
	failFn := func() (interface{}, error) {
		return nil, errors.New("test error")
	}

	result, err := cb.Execute(failFn)
	
	if err == nil {
		t.Error("Execute() error = nil, want error")
	}
	if result != nil {
		t.Errorf("Execute() result = %v, want nil", result)
	}
}

func TestCircuitBreaker_Execute_Panic(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultConfig())
	
	panicFn := func() (interface{}, error) {
		panic("test panic")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, but none occurred")
		}
	}()

	_, _ = cb.Execute(panicFn)
}

func TestCircuitBreaker_Execute_Fallback(t *testing.T) {
	fallbackCalled := false
	config := DefaultConfig()
	config.Fallback = func(err error) error {
		fallbackCalled = true
		return errors.New("fallback error")
	}
	
	cb := NewCircuitBreaker("test", config)
	
	failFn := func() (interface{}, error) {
		return nil, errors.New("original error")
	}

	_, err := cb.Execute(failFn)
	
	if err == nil {
		t.Error("Execute() error = nil, want error")
	}
	if !fallbackCalled {
		t.Error("Fallback was not called")
	}
	if err.Error() != "fallback error" {
		t.Errorf("Execute() error = %v, want %v", err.Error(), "fallback error")
	}
}

func TestCircuitBreaker_MultipleFailures(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultConfig())
	
	failFn := func() (interface{}, error) {
		return nil, errors.New("test error")
	}

	// Execute multiple failures to trigger circuit opening
	for i := 0; i < 10; i++ {
		_, err := cb.Execute(failFn)
		if i < 6 && err == nil {
			t.Errorf("Expected error for failure %d, got nil", i+1)
		}
	}
}

// Mock test for state transitions
func TestCircuitBreaker_StateTransitions(t *testing.T) {
	stateChanges := make(map[string]int)
	config := DefaultConfig()
	config.OnStateChange = func(name string, from, to State) {
		key := from.String() + "->" + to.String()
		stateChanges[key]++
	}
	
	cb := NewCircuitBreaker("test", config)
	
	failFn := func() (interface{}, error) {
		return nil, errors.New("test error")
	}

	// Trigger failures to change state
	for i := 0; i < 10; i++ {
		cb.Execute(failFn)
	}

	// Should have at least one state change from closed to open
	if stateChanges["closed->open"] == 0 {
		t.Error("Expected state change from closed to open")
	}
}
