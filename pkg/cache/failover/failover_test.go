package failover

import (
	"context"
	"testing"
	"time"
)

// MockStorage implements Storage interface for testing
type MockStorage struct {
	data       map[string]interface{}
	isHealthy  bool
	shouldFail bool
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		data:      make(map[string]interface{}),
		isHealthy: true,
	}
}

func (m *MockStorage) Get(ctx context.Context, key string) (interface{}, bool) {
	if m.shouldFail {
		return nil, false
	}
	val, exists := m.data[key]
	return val, exists
}

func (m *MockStorage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.shouldFail {
		return &StorageError{Type: "set", Message: "mock set failure"}
	}
	m.data[key] = value
	return nil
}

func (m *MockStorage) Delete(ctx context.Context, key string) error {
	if m.shouldFail {
		return &StorageError{Type: "delete", Message: "mock delete failure"}
	}
	delete(m.data, key)
	return nil
}

func (m *MockStorage) Clear(ctx context.Context) error {
	if m.shouldFail {
		return &StorageError{Type: "clear", Message: "mock clear failure"}
	}
	m.data = make(map[string]interface{})
	return nil
}

func (m *MockStorage) HealthCheck(ctx context.Context) error {
	if !m.isHealthy {
		return &StorageError{Type: "health", Message: "mock unhealthy"}
	}
	return nil
}

func (m *MockStorage) Close() error {
	return nil
}

func (m *MockStorage) SetHealthy(healthy bool) {
	m.isHealthy = healthy
}

func (m *MockStorage) SetShouldFail(fail bool) {
	m.shouldFail = fail
}

func TestNewFailoverCache(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{
		MaxRetries:          3,
		RetryDelay:          time.Millisecond * 100,
		HealthCheckInterval: time.Millisecond * 500,
	}

	fc := NewFailoverCache(primary, secondary, tertiary, config)

	if fc.primary != primary {
		t.Errorf("NewFailoverCache().primary = %v, want %v", fc.primary, primary)
	}
	if fc.secondary != secondary {
		t.Errorf("NewFailoverCache().secondary = %v, want %v", fc.secondary, secondary)
	}
	if fc.tertiary != tertiary {
		t.Errorf("NewFailoverCache().tertiary = %v, want %v", fc.tertiary, tertiary)
	}
	if fc.maxRetries != 3 {
		t.Errorf("NewFailoverCache().maxRetries = %v, want %v", fc.maxRetries, 3)
	}
	if fc.retryDelay != time.Millisecond*100 {
		t.Errorf("NewFailoverCache().retryDelay = %v, want %v", fc.retryDelay, time.Millisecond*100)
	}
	if fc.healthCheckInterval != time.Millisecond*500 {
		t.Errorf("NewFailoverCache().healthCheckInterval = %v, want %v", fc.healthCheckInterval, time.Millisecond*500)
	}

	// Clean up
	if fc.healthTicker != nil {
		fc.healthTicker.Stop()
	}
}

func TestFailoverCache_Get_PrimarySuccess(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Set value in primary
	primary.Set(context.Background(), "test-key", "test-value", 0)

	// Get should return from primary
	val, found := fc.Get(context.Background(), "test-key")
	if !found {
		t.Error("Get() found = false, want true")
	}
	if val != "test-value" {
		t.Errorf("Get() val = %v, want %v", val, "test-value")
	}
}

func TestFailoverCache_Get_SecondaryFallback(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Make primary fail
	primary.SetShouldFail(true)
	
	// Set value in secondary
	secondary.Set(context.Background(), "test-key", "test-value", 0)

	// Get should fallback to secondary
	val, found := fc.Get(context.Background(), "test-key")
	if !found {
		t.Error("Get() found = false, want true")
	}
	if val != "test-value" {
		t.Errorf("Get() val = %v, want %v", val, "test-value")
	}
}

func TestFailoverCache_Get_TertiaryFallback(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Make primary and secondary fail
	primary.SetShouldFail(true)
	secondary.SetShouldFail(true)
	
	// Set value in tertiary
	tertiary.Set(context.Background(), "test-key", "test-value", 0)

	// Get should fallback to tertiary
	val, found := fc.Get(context.Background(), "test-key")
	if !found {
		t.Error("Get() found = false, want true")
	}
	if val != "test-value" {
		t.Errorf("Get() val = %v, want %v", val, "test-value")
	}
}

func TestFailoverCache_Get_AllFail(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Make all fail
	primary.SetShouldFail(true)
	secondary.SetShouldFail(true)
	tertiary.SetShouldFail(true)

	// Get should return not found
	val, found := fc.Get(context.Background(), "test-key")
	if found {
		t.Error("Get() found = true, want false")
	}
	if val != nil {
		t.Errorf("Get() val = %v, want nil", val)
	}
}

func TestFailoverCache_Set_PrimarySuccess(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Set should work on primary
	err := fc.Set(context.Background(), "test-key", "test-value", 0)
	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}

	// Verify it's in primary
	val, found := primary.Get(context.Background(), "test-key")
	if !found {
		t.Error("Value not found in primary storage")
	}
	if val != "test-value" {
		t.Errorf("Primary value = %v, want %v", val, "test-value")
	}
}

func TestFailoverCache_Set_SecondaryFallback(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Make primary fail
	primary.SetShouldFail(true)

	// Set should fallback to secondary
	err := fc.Set(context.Background(), "test-key", "test-value", 0)
	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}

	// Verify it's in secondary
	val, found := secondary.Get(context.Background(), "test-key")
	if !found {
		t.Error("Value not found in secondary storage")
	}
	if val != "test-value" {
		t.Errorf("Secondary value = %v, want %v", val, "test-value")
	}
}

func TestFailoverCache_Delete_PrimarySuccess(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Set value first
	primary.Set(context.Background(), "test-key", "test-value", 0)

	// Delete should work on primary
	err := fc.Delete(context.Background(), "test-key")
	if err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}

	// Verify it's deleted from primary
	_, found := primary.Get(context.Background(), "test-key")
	if found {
		t.Error("Value still found in primary storage after delete")
	}
}

func TestFailoverCache_Clear_AllStorages(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)
	defer func() {
		if fc.healthTicker != nil {
			fc.healthTicker.Stop()
		}
	}()

	// Set values in all storages
	primary.Set(context.Background(), "key1", "value1", 0)
	secondary.Set(context.Background(), "key2", "value2", 0)
	tertiary.Set(context.Background(), "key3", "value3", 0)

	// Clear should work on all storages
	err := fc.Clear(context.Background())
	if err != nil {
		t.Errorf("Clear() error = %v, want nil", err)
	}

	// Verify all are cleared
	if _, found := primary.Get(context.Background(), "key1"); found {
		t.Error("Value still found in primary storage after clear")
	}
	if _, found := secondary.Get(context.Background(), "key2"); found {
		t.Error("Value still found in secondary storage after clear")
	}
	if _, found := tertiary.Get(context.Background(), "key3"); found {
		t.Error("Value still found in tertiary storage after clear")
	}
}

func TestFailoverCache_Close(t *testing.T) {
	primary := NewMockStorage()
	secondary := NewMockStorage()
	tertiary := NewMockStorage()
	
	config := Config{MaxRetries: 3}
	fc := NewFailoverCache(primary, secondary, tertiary, config)

	// Close should stop health ticker and close all storages
	err := fc.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	if fc.healthTicker != nil {
		t.Error("Health ticker should be nil after close")
	}
}
