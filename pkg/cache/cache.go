package cache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"alldev-gin-rpc/pkg/cache/redis"
	"alldev-gin-rpc/pkg/cache/memcache"
	"alldev-gin-rpc/pkg/cache/failover"
)

// redisAdapter adapts redis.Client to Cache interface
type redisAdapter struct {
	client *redis.Client
}

func (r *redisAdapter) Get(ctx context.Context, key string) (interface{}, error) {
	return r.client.Get(ctx, key)
}

func (r *redisAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl)
}

func (r *redisAdapter) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key)
}

func (r *redisAdapter) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key)
	return count > 0, err
}

func (r *redisAdapter) Clear(ctx context.Context) error {
	// Redis doesn't have a clear all command, this would need implementation
	return nil
}

func (r *redisAdapter) Close() error {
	return r.client.Close()
}

func (r *redisAdapter) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	return r.Get(ctx, key)
}

func (r *redisAdapter) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	return r.Set(ctx, key, value, baseTTL)
}

func (r *redisAdapter) GetStats() CacheStats {
	return CacheStats{} // Basic implementation
}

// memcacheAdapter adapts memcache.Client to Cache interface
type memcacheAdapter struct {
	client *memcache.Client
}

func (m *memcacheAdapter) Get(ctx context.Context, key string) (interface{}, error) {
	item, err := m.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	return item.Value, nil
}

func (m *memcacheAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var expiration int32
	if ttl > 0 {
		expiration = int32(ttl.Seconds())
	}
	
	var valueBytes []byte
	switch v := value.(type) {
	case []byte:
		valueBytes = v
	case string:
		valueBytes = []byte(v)
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}
	
	item := &memcache.Item{
		Key:        key,
		Value:      valueBytes,
		Expiration: expiration,
	}
	return m.client.Set(ctx, item)
}

func (m *memcacheAdapter) Delete(ctx context.Context, key string) error {
	return m.client.Delete(ctx, key)
}

func (m *memcacheAdapter) Exists(ctx context.Context, key string) (bool, error) {
	item, err := m.client.Get(ctx, key)
	if err != nil {
		return false, err
	}
	return item != nil, nil
}

func (m *memcacheAdapter) Clear(ctx context.Context) error {
	return m.client.DeleteAll(ctx)
}

func (m *memcacheAdapter) Close() error {
	return m.client.Close()
}

func (m *memcacheAdapter) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	return m.Get(ctx, key)
}

func (m *memcacheAdapter) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	return m.Set(ctx, key, value, baseTTL)
}

func (m *memcacheAdapter) GetStats() CacheStats {
	return CacheStats{} // Basic implementation
}

// Cache defines the cache interface with enterprise features
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	Close() error
	// Enterprise features
	GetWithLock(ctx context.Context, key string) (interface{}, error)
	SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error
	GetStats() CacheStats
}

// CacheStats provides cache statistics
type CacheStats struct {
	Hits        uint64
	Misses      uint64
	Sets        uint64
	Deletes     uint64
	Errors      uint64	
	LastAccess  time.Time
}

// BreakdownCache prevents cache breakdown with singleflight
type BreakdownCache struct {
	cache    Cache
	groups   map[string]*singleGroup
	mu       sync.RWMutex
	stats    CacheStats
	statsMu  sync.RWMutex
}

// singleGroup prevents duplicate requests for the same key
type singleGroup struct {
	mu sync.Mutex
	m  map[string]*call // key -> in-flight call
}

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// NewBreakdownCache creates a cache with breakdown protection
func NewBreakdownCache(baseCache Cache) *BreakdownCache {
	return &BreakdownCache{
		cache:  baseCache,
		groups: make(map[string]*singleGroup),
	}
}

// GetWithLock prevents cache breakdown using singleflight pattern
func (bc *BreakdownCache) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	// Try to get from cache first
	val, err := bc.cache.Get(ctx, key)
	if err == nil && val != nil {
		bc.recordHit()
		return val, nil
	}
	
	bc.recordMiss()
	
	// Use singleflight to prevent duplicate requests
	g := bc.groupForKey(key)
	c, ok := g.loadOrStore(key)
	if ok {
		// Wait for the in-flight request to complete
		c.wg.Wait()
		return c.val, c.err
	}
	
	// This goroutine is responsible for loading the value
	c.wg.Add(1)
	defer func() {
		c.wg.Done()
		g.delete(key)
	}()
	
	// Try cache again in case it was populated while we were waiting
	val, err = bc.cache.Get(ctx, key)
	if err == nil && val != nil {
		c.val = val
		return val, nil
	}
	
	// Return nil to let caller handle cache miss
	c.err = fmt.Errorf("cache miss")
	return nil, c.err
}

// SetWithRandomTTL prevents cache avalanche by adding random jitter
func (bc *BreakdownCache) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	// Add random jitter to prevent avalanche (±25% of base TTL)
	jitter := time.Duration(rand.Float64() * 0.5 - 0.25) * baseTTL
	finalTTL := baseTTL + jitter
	
	err := bc.cache.Set(ctx, key, value, finalTTL)
	if err != nil {
		bc.recordError()
		return err
	}
	
	bc.recordSet()
	return nil
}

// groupForKey gets or creates a singleGroup for a key
func (bc *BreakdownCache) groupForKey(key string) *singleGroup {
	hash := simpleHash(key) % 32 // Use 32 groups to reduce contention
	groupKey := fmt.Sprintf("group_%d", hash)
	
	bc.mu.RLock()
	g, exists := bc.groups[groupKey]
	bc.mu.RUnlock()
	
	if !exists {
		bc.mu.Lock()
		defer bc.mu.Unlock()
		
		// Double-check after acquiring write lock
		if g, exists = bc.groups[groupKey]; !exists {
			g = &singleGroup{m: make(map[string]*call)}
			bc.groups[groupKey] = g
		}
	}
	
	return g
}

// loadOrStore loads or stores a call for the given key
func (g *singleGroup) loadOrStore(key string) (*call, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if c, ok := g.m[key]; ok {
		return c, true
	}
	
	c := &call{}
	g.m[key] = c
	return c, false
}

// delete removes a call from the group
func (g *singleGroup) delete(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.m, key)
}

// Stats recording methods
func (bc *BreakdownCache) recordHit() {
	bc.statsMu.Lock()
	bc.stats.Hits++
	bc.stats.LastAccess = time.Now()
	bc.statsMu.Unlock()
}

func (bc *BreakdownCache) recordMiss() {
	bc.statsMu.Lock()
	bc.stats.Misses++
	bc.stats.LastAccess = time.Now()
	bc.statsMu.Unlock()
}

func (bc *BreakdownCache) recordSet() {
	bc.statsMu.Lock()
	bc.stats.Sets++
	bc.stats.LastAccess = time.Now()
	bc.statsMu.Unlock()
}

func (bc *BreakdownCache) recordError() {
	bc.statsMu.Lock()
	bc.stats.Errors++
	bc.stats.LastAccess = time.Now()
	bc.statsMu.Unlock()
}

// GetStats returns cache statistics
func (bc *BreakdownCache) GetStats() CacheStats {
	bc.statsMu.RLock()
	defer bc.statsMu.RUnlock()
	return bc.stats
}

// Forward standard cache methods
func (bc *BreakdownCache) Get(ctx context.Context, key string) (interface{}, error) {
	return bc.cache.Get(ctx, key)
}

func (bc *BreakdownCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return bc.cache.Set(ctx, key, value, ttl)
}

func (bc *BreakdownCache) Delete(ctx context.Context, key string) error {
	err := bc.cache.Delete(ctx, key)
	if err == nil {
		bc.statsMu.Lock()
		bc.stats.Deletes++
		bc.stats.LastAccess = time.Now()
		bc.statsMu.Unlock()
	}
	return err
}

func (bc *BreakdownCache) Exists(ctx context.Context, key string) (bool, error) {
	return bc.cache.Exists(ctx, key)
}

func (bc *BreakdownCache) Clear(ctx context.Context) error {
	return bc.cache.Clear(ctx)
}

func (bc *BreakdownCache) Close() error {
	return bc.cache.Close()
}

// simpleHash creates a simple hash for key distribution
func simpleHash(key string) int {
	hash := 0
	for _, c := range key {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// RedisConfig holds Redis configuration
type RedisConfig = redis.Config

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(config RedisConfig) (Cache, error) {
	client, err := redis.New(config)
	if err != nil {
		return nil, err
	}
	return &redisAdapter{client: client}, nil
}

// MemcacheConfig holds Memcache configuration  
type MemcacheConfig = memcache.Config

// NewMemcacheCache creates a new Memcache cache instance
func NewMemcacheCache(config MemcacheConfig) (Cache, error) {
	client, err := memcache.New(config)
	if err != nil {
		return nil, err
	}
	return &memcacheAdapter{client: client}, nil
}

// failoverAdapter adapts failover.FailoverCache to Cache interface
type failoverAdapter struct {
	cache *failover.FailoverCache
}

func (f *failoverAdapter) Get(ctx context.Context, key string) (interface{}, error) {
	value, exists := f.cache.Get(ctx, key)
	if !exists {
		return nil, nil
	}
	return value, nil
}

func (f *failoverAdapter) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return f.cache.Set(ctx, key, value, ttl)
}

func (f *failoverAdapter) Delete(ctx context.Context, key string) error {
	return f.cache.Delete(ctx, key)
}

func (f *failoverAdapter) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := f.cache.Get(ctx, key)
	return exists, nil
}

func (f *failoverAdapter) Clear(ctx context.Context) error {
	return f.cache.Clear(ctx)
}

func (f *failoverAdapter) Close() error {
	return f.cache.Close()
}

func (f *failoverAdapter) GetWithLock(ctx context.Context, key string) (interface{}, error) {
	return f.Get(ctx, key)
}

func (f *failoverAdapter) SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
	return f.Set(ctx, key, value, baseTTL)
}

func (f *failoverAdapter) GetStats() CacheStats {
	return CacheStats{} // Basic implementation
}

// FailoverConfig holds failover cache configuration
type FailoverConfig = failover.Config

// NewFailoverCache creates a new failover cache instance
func NewFailoverCache(config FailoverConfig) (Cache, error) {
	// For now, return a default failover cache
	// In a real implementation, you would use the config to create the appropriate storage backends
	fc, err := failover.NewDefaultFailover()
	if err != nil {
		return nil, err
	}
	return &failoverAdapter{cache: fc}, nil
}
