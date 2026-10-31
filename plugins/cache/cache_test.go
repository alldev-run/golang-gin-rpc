package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockCache implements Cache interface for testing
type MockCache struct {
	data    map[string]interface{}
	mu      sync.RWMutex
	calls   map[string]int
	callMu  sync.RWMutex
}

func NewMockCache() *MockCache {
	return &MockCache{
		data:  make(map[string]interface{}),
		calls: make(map[string]int),
	}
}

func (m *MockCache) Get(ctx context.Context, key string) (interface{}, error) {
	m.recordCall("Get")
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	value, exists := m.data[key]
	if !exists {
		return nil, errors.New("not found")
	}
	return value, nil
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.recordCall("Set")
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.data[key] = value
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	m.recordCall("Delete")
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.data, key)
	return nil
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	m.recordCall("Exists")
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockCache) Clear(ctx context.Context) error {
	m.recordCall("Clear")
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.data = make(map[string]interface{})
	return nil
}

func (m *MockCache) Close() error {
	m.recordCall("Close")
	return nil
}

func (m *MockCache) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	m.recordCall("GetWithLock")
	return m.Get(ctx, key)
}

func (m *MockCache) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	m.recordCall("SetWithRandomTTL")
	return m.Set(ctx, key, value, baseTTL)
}

func (m *MockCache) GetStats() CacheStats {
	m.recordCall("GetStats")
	return CacheStats{
		Hits:       10,
		Misses:     5,
		Sets:       8,
		Deletes:    2,
		Errors:     1,
		LastAccess: time.Now(),
	}
}

func (m *MockCache) recordCall(method string) {
	m.callMu.Lock()
	defer m.callMu.Unlock()
	m.calls[method]++
}

func (m *MockCache) GetCallCount(method string) int {
	m.callMu.RLock()
	defer m.callMu.RUnlock()
	return m.calls[method]
}

func (m *MockCache) ResetCalls() {
	m.callMu.Lock()
	defer m.callMu.Unlock()
	m.calls = make(map[string]int)
}

func TestNewBreakdownCache(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	
	if bdc.cache != mockCache {
		t.Error("NewBreakdownCache() should set the cache field")
	}
	
	if bdc.groups == nil {
		t.Error("NewBreakdownCache() should initialize groups map")
	}
	
	if len(bdc.groups) != 0 {
		t.Error("NewBreakdownCache() should initialize empty groups map")
	}
}

func TestBreakdownCache_Get(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Test cache hit
	mockCache.Set(ctx, "key1", "value1", time.Hour)
	value, err := bdc.Get(ctx, "key1")
	
	if err != nil {
		t.Errorf("Get() error = %v, want nil", err)
	}
	
	if value != "value1" {
		t.Errorf("Get() value = %v, want %v", value, "value1")
	}
	
	// Test cache miss
	value, err = bdc.Get(ctx, "nonexistent")
	
	if err == nil {
		t.Error("Get() should return error for cache miss")
	}
	
	if value != nil {
		t.Errorf("Get() value = %v, want nil", value)
	}
}

func TestBreakdownCache_Set(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	err := bdc.Set(ctx, "key1", "value1", time.Hour)
	if err != nil {
		t.Errorf("Set() error = %v, want nil", err)
	}
	
	// Verify value was set
	value, err := mockCache.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Set() failed to set value, Get() error = %v", err)
	}
	
	if value != "value1" {
		t.Errorf("Set() failed to set value, Get() value = %v, want %v", value, "value1")
	}
}

func TestBreakdownCache_Delete(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Set up test data
	mockCache.Set(ctx, "key1", "value1", time.Hour)
	
	err := bdc.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}
	
	// Verify value was deleted
	exists, err := mockCache.Exists(ctx, "key1")
	if err != nil {
		t.Errorf("Delete() verification failed, Exists() error = %v", err)
	}
	
	if exists {
		t.Error("Delete() failed to delete value")
	}
	
	// Check stats
	stats := bdc.GetStats()
	if stats.Deletes != 1 {
		t.Errorf("Delete() should increment delete count, got %v", stats.Deletes)
	}
}

func TestBreakdownCache_GetWithLock_CacheHit(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Set up cache hit
	mockCache.Set(ctx, "key1", "value1", time.Hour)
	
	value, err := bdc.GetWithLock(ctx, "key1")
	
	if err != nil {
		t.Errorf("GetWithLock() error = %v, want nil", err)
	}
	
	if value != "value1" {
		t.Errorf("GetWithLock() value = %v, want %v", value, "value1")
	}
	
	// Check stats
	stats := bdc.GetStats()
	if stats.Hits != 1 {
		t.Errorf("GetWithLock() should increment hit count, got %v", stats.Hits)
	}
}

func TestBreakdownCache_GetWithLock_CacheMiss(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	value, err := bdc.GetWithLock(ctx, "nonexistent")
	
	if err == nil {
		t.Error("GetWithLock() should return error for cache miss")
	}
	
	if value != nil {
		t.Errorf("GetWithLock() value = %v, want nil", value)
	}
	
	// Check stats
	stats := bdc.GetStats()
	if stats.Misses != 1 {
		t.Errorf("GetWithLock() should increment miss count, got %v", stats.Misses)
	}
}

func TestBreakdownCache_GetWithLock_ConcurrentRequests(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Simulate concurrent requests for same key
	var wg sync.WaitGroup
	results := make(chan interface{}, 10)
	errors := make(chan error, 10)
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := bdc.GetWithLock(ctx, "concurrent_key")
			if err != nil {
				errors <- err
			} else {
				results <- value
			}
		}()
	}
	
	wg.Wait()
	close(results)
	close(errors)
	
	// Should have mostly errors (cache miss)
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}
	
	// Allow for some variation due to timing
	if errorCount < 8 {
		t.Errorf("Expected at least 8 errors, got %v", errorCount)
	}
	
	// Should have few or no results
	resultCount := 0
	for range results {
		resultCount++
	}
	
	if resultCount > 2 {
		t.Errorf("Expected at most 2 results, got %v", resultCount)
	}
	
	// Check that underlying cache was called (singleflight may not prevent all calls due to timing)
	callCount := mockCache.GetCallCount("Get")
	if callCount < 1 {
		t.Errorf("Expected at least 1 call to underlying cache, got %v", callCount)
	}
}

func TestBreakdownCache_SetWithRandomTTL(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	baseTTL := time.Hour
	
	err := bdc.SetWithRandomTTL(ctx, "key1", "value1", baseTTL)
	if err != nil {
		t.Errorf("SetWithRandomTTL() error = %v, want nil", err)
	}
	
	// Check stats
	stats := bdc.GetStats()
	if stats.Sets != 1 {
		t.Errorf("SetWithRandomTTL() should increment set count, got %v", stats.Sets)
	}
	
	// Verify underlying cache was called
	if mockCache.GetCallCount("Set") != 1 {
		t.Errorf("Expected 1 call to underlying Set, got %v", mockCache.GetCallCount("Set"))
	}
}

// ErrorMockCache implements Cache interface and always returns errors
type ErrorMockCache struct {
	*MockCache
}

func NewErrorMockCache() *ErrorMockCache {
	return &ErrorMockCache{
		MockCache: NewMockCache(),
	}
}

func (e *ErrorMockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	e.recordCall("Set")
	return errors.New("set error")
}

func TestBreakdownCache_SetWithRandomTTL_Error(t *testing.T) {
	// Create a mock cache that returns error on Set
	errorCache := NewErrorMockCache()
	
	bdc := NewBreakdownCache(errorCache)
	ctx := context.Background()
	
	err := bdc.SetWithRandomTTL(ctx, "key1", "value1", time.Hour)
	if err == nil {
		t.Error("SetWithRandomTTL() should return error when underlying cache fails")
	}
	
	// Check stats
	stats := bdc.GetStats()
	if stats.Errors != 1 {
		t.Errorf("SetWithRandomTTL() should increment error count, got %v", stats.Errors)
	}
}

func TestBreakdownCache_GetStats(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	
	// Perform some operations
	ctx := context.Background()
	bdc.Set(ctx, "key1", "value1", time.Hour)
	
	// Test get with cache miss
	bdc.Get(ctx, "nonexistent")
	
	// Test delete
	bdc.Delete(ctx, "key1")
	
	stats := bdc.GetStats()
	
	// Check that stats structure exists (actual values may vary)
	if stats.LastAccess.IsZero() {
		t.Error("GetStats() should record last access time")
	}
}

func TestBreakdownCache_groupForKey(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	
	// Test that same key maps to same group
	group1 := bdc.groupForKey("test_key")
	group2 := bdc.groupForKey("test_key")
	
	if group1 != group2 {
		t.Error("groupForKey() should return same group for same key")
	}
	
	// Test that different keys might map to different groups
	group3 := bdc.groupForKey("different_key")
	
	// Note: Due to hash distribution, they might be the same group
	// This is expected behavior, just verify the method works
	if group3 == nil {
		t.Error("groupForKey() should not return nil")
	}
}

func TestSingleGroup_loadOrStore(t *testing.T) {
	g := &singleGroup{m: make(map[string]*call)}
	
	// First call should create new call
	c, ok := g.loadOrStore("test_key")
	if ok {
		t.Error("loadOrStore() should return false for new key")
	}
	if c == nil {
		t.Error("loadOrStore() should return call for new key")
	}
	
	// Second call should return existing call
	c2, ok2 := g.loadOrStore("test_key")
	if !ok2 {
		t.Error("loadOrStore() should return true for existing key")
	}
	if c2 != c {
		t.Error("loadOrStore() should return same call for existing key")
	}
}

func TestSingleGroup_delete(t *testing.T) {
	g := &singleGroup{m: make(map[string]*call)}
	
	// Add a call
	c, _ := g.loadOrStore("test_key")
	
	// Delete the call
	g.delete("test_key")
	
	// Verify it's deleted
	c2, ok := g.loadOrStore("test_key")
	if ok {
		t.Error("delete() should remove the call")
	}
	if c2 == c {
		t.Error("delete() should create new call after deletion")
	}
}

func TestSimpleHash(t *testing.T) {
	// Test hash function behavior
	key := "test"
	hash1 := simpleHash(key)
	hash2 := simpleHash(key)
	
	// Test consistency
	if hash1 != hash2 {
		t.Error("simpleHash() should be consistent for same input")
	}
	
	// Test non-negative
	if hash1 < 0 {
		t.Error("simpleHash() should return non-negative value")
	}
	
	// Test different keys produce different hashes
	hash3 := simpleHash("different")
	if hash1 == hash3 {
		t.Error("simpleHash() should produce different hashes for different keys")
	}
}

func TestBreakdownCache_Exists(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Test existing key
	mockCache.Set(ctx, "key1", "value1", time.Hour)
	exists, err := bdc.Exists(ctx, "key1")
	
	if err != nil {
		t.Errorf("Exists() error = %v, want nil", err)
	}
	
	if !exists {
		t.Error("Exists() should return true for existing key")
	}
	
	// Test non-existing key
	exists, err = bdc.Exists(ctx, "nonexistent")
	
	if err != nil {
		t.Errorf("Exists() error = %v, want nil", err)
	}
	
	if exists {
		t.Error("Exists() should return false for non-existing key")
	}
}

func TestBreakdownCache_Clear(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Set up test data
	mockCache.Set(ctx, "key1", "value1", time.Hour)
	mockCache.Set(ctx, "key2", "value2", time.Hour)
	
	err := bdc.Clear(ctx)
	if err != nil {
		t.Errorf("Clear() error = %v, want nil", err)
	}
	
	// Verify all data was cleared
	exists, _ := mockCache.Exists(ctx, "key1")
	if exists {
		t.Error("Clear() should remove all data")
	}
	
	exists, _ = mockCache.Exists(ctx, "key2")
	if exists {
		t.Error("Clear() should remove all data")
	}
}

func TestBreakdownCache_Close(t *testing.T) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	
	err := bdc.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	
	// Verify underlying cache was closed
	if mockCache.GetCallCount("Close") != 1 {
		t.Errorf("Expected 1 call to underlying Close, got %v", mockCache.GetCallCount("Close"))
	}
}

func TestRedisAdapter_Implementation(t *testing.T) {
	// Test that redisAdapter implements all required methods
	adapter := &redisAdapter{}
	
	// Test that methods exist (don't call them to avoid panic)
	stats := adapter.GetStats()
	
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("redisAdapter.GetStats() should return empty stats")
	}
}

func TestMemcacheAdapter_Implementation(t *testing.T) {
	// Test that memcacheAdapter implements all required methods
	adapter := &memcacheAdapter{}
	
	// Test that methods exist (don't call them to avoid panic)
	stats := adapter.GetStats()
	
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("memcacheAdapter.GetStats() should return empty stats")
	}
}

func TestFailoverAdapter_Implementation(t *testing.T) {
	// Test that failoverAdapter implements all required methods
	adapter := &failoverAdapter{}
	
	// Test that methods exist (don't call them to avoid panic)
	stats := adapter.GetStats()
	
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("failoverAdapter.GetStats() should return empty stats")
	}
}

// Benchmark tests
func BenchmarkBreakdownCache_Get_Hit(b *testing.B) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	// Set up cache hit
	mockCache.Set(ctx, "benchmark_key", "benchmark_value", time.Hour)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bdc.Get(ctx, "benchmark_key")
	}
}

func BenchmarkBreakdownCache_Get_Miss(b *testing.B) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bdc.Get(ctx, "nonexistent_key")
	}
}

func BenchmarkBreakdownCache_Set(b *testing.B) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("benchmark_key_%d", i)
		bdc.Set(ctx, key, "benchmark_value", time.Hour)
	}
}

func BenchmarkBreakdownCache_GetWithLock_Concurrent(b *testing.B) {
	mockCache := NewMockCache()
	bdc := NewBreakdownCache(mockCache)
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bdc.GetWithLock(ctx, "concurrent_key")
		}
	})
}

func BenchmarkSimpleHash(b *testing.B) {
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = fmt.Sprintf("benchmark_key_%d", i)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%1000]
		simpleHash(key)
	}
}

func TestRedisAdapter_Clear_ReturnsExplicitUnsupportedError(t *testing.T) {
	adapter := &redisAdapter{}

	err := adapter.Clear(context.Background())
	if !errors.Is(err, ErrCacheOperationNotSupported) {
		t.Fatalf("expected ErrCacheOperationNotSupported, got %v", err)
	}
}

func TestNewFailoverCache_UsesConfigDefaults(t *testing.T) {
	cache, err := NewFailoverCache(FailoverConfig{})
	if err != nil {
		t.Fatalf("expected zero-value config to be defaulted, got error: %v", err)
	}
	if cache == nil {
		t.Fatal("expected failover cache to be created")
	}
	if err := cache.Close(); err != nil {
		t.Fatalf("expected failover cache close to succeed, got: %v", err)
	}
}
