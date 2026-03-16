package cache

import (
	"context"
	"fmt"
	"time"

	"golang-gin-rpc/pkg/cache/redis"
	"golang-gin-rpc/pkg/cache/memcache"
	"golang-gin-rpc/pkg/cache/failover"
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

// Cache defines the cache interface
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context) error
	Close() error
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
