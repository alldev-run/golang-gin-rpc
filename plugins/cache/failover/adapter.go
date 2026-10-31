// Package failover provides failover cache functionality with configurable storage.
package failover

import (
	"context"
	"time"
)

// QueryCacheAdapter adapts the failover cache to the ORM QueryCache interface.
type QueryCacheAdapter struct {
	failoverCache *FailoverCache
}

// NewQueryCacheAdapter creates a new adapter for the failover cache.
func NewQueryCacheAdapter(failoverCache *FailoverCache) *QueryCacheAdapter {
	return &QueryCacheAdapter{
		failoverCache: failoverCache,
	}
}

// Get retrieves a value from the cache (implements ORM QueryCache interface).
func (a *QueryCacheAdapter) Get(key string) (interface{}, bool) {
	return a.failoverCache.Get(context.Background(), key)
}

// Set stores a value in the cache (implements ORM QueryCache interface).
func (a *QueryCacheAdapter) Set(key string, value interface{}, ttl time.Duration) {
	a.failoverCache.Set(context.Background(), key, value, ttl)
}

// Delete removes a value from the cache (implements ORM QueryCache interface).
func (a *QueryCacheAdapter) Delete(key string) {
	a.failoverCache.Delete(context.Background(), key)
}

// Clear removes all values from the cache (implements ORM QueryCache interface).
func (a *QueryCacheAdapter) Clear() {
	a.failoverCache.Clear(context.Background())
}

// Keys returns all cache keys (implements ORM QueryCache interface).
func (a *QueryCacheAdapter) Keys() []string {
	return a.failoverCache.Keys(context.Background())
}

// GetFailoverCache returns the underlying failover cache for advanced operations.
func (a *QueryCacheAdapter) GetFailoverCache() *FailoverCache {
	return a.failoverCache
}

// Close closes the underlying failover cache.
func (a *QueryCacheAdapter) Close() error {
	return a.failoverCache.Close()
}

// GetStorageStatus returns the health status of all storage backends.
func (a *QueryCacheAdapter) GetStorageStatus() map[string]bool {
	return a.failoverCache.GetStorageStatus()
}

// ORM-compatible interface (matches the existing QueryCache interface)
type ORMQueryCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	Keys() []string
	Close() error
}

// Ensure QueryCacheAdapter implements ORMQueryCache
var _ ORMQueryCache = (*QueryCacheAdapter)(nil)

// CreateORMAdapter creates a failover cache that's compatible with ORM QueryCache interface.
func CreateORMAdapter(config Config) (ORMQueryCache, error) {
	failoverCache, err := NewFailoverCacheFromConfig(config)
	if err != nil {
		return nil, err
	}
	return NewQueryCacheAdapter(failoverCache), nil
}

// CreateFileORMAdapter creates a file-based failover cache compatible with ORM.
func CreateFileORMAdapter(directory, fileSuffix string) (ORMQueryCache, error) {
	failoverCache, err := NewFileFailover(directory, fileSuffix)
	if err != nil {
		return nil, err
	}
	return NewQueryCacheAdapter(failoverCache), nil
}

// CreateMemoryFileORMAdapter creates a memory-primary, file-secondary failover cache compatible with ORM.
func CreateMemoryFileORMAdapter(directory, fileSuffix string) (ORMQueryCache, error) {
	failoverCache, err := NewMemoryFileFailover(directory, fileSuffix)
	if err != nil {
		return nil, err
	}
	return NewQueryCacheAdapter(failoverCache), nil
}

// CreateDefaultORMAdapter creates a default failover cache compatible with ORM.
func CreateDefaultORMAdapter() (ORMQueryCache, error) {
	failoverCache, err := NewDefaultFailover()
	if err != nil {
		return nil, err
	}
	return NewQueryCacheAdapter(failoverCache), nil
}
