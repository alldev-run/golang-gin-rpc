// Package ormcache provides redis/memcache/failover backends for orm's
// QueryCache. 这些后端放在 cache 模块而非 plugins/db/orm，使 plugins/db
// 不依赖 plugins/cache（依赖方向：cache -> db，见各 register.go）。
package ormcache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/failover"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/orm"
)

// QueryCache 与 CacheType 为 orm 对应类型的别名，迁移自 orm 包，保持易用性。
type QueryCache = orm.QueryCache

// CacheType is an alias of orm.CacheType.
type CacheType = orm.CacheType

const (
	// CacheTypeRedis selects the redis backend.
	CacheTypeRedis = orm.CacheTypeRedis
	// CacheTypeMemcache selects the memcache backend.
	CacheTypeMemcache = orm.CacheTypeMemcache
	// CacheTypeFailover selects the failover chain backend.
	CacheTypeFailover = orm.CacheTypeFailover
)

// RedisCache implements QueryCache using Redis.
type RedisCache struct {
	client    *redis.Client
	keyPrefix string
}

// NewRedisCache creates a new Redis-based cache.
func NewRedisCache(client *redis.Client, keyPrefix ...string) *RedisCache {
	prefix := "orm_cache:"
	if len(keyPrefix) > 0 {
		prefix = keyPrefix[0]
	}
	return &RedisCache{
		client:    client,
		keyPrefix: prefix,
	}
}

// Get retrieves a value from Redis cache.
func (rc *RedisCache) Get(key string) (interface{}, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := rc.client.Get(ctx, rc.keyPrefix+key)
	if err != nil {
		return nil, false
	}

	var result interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		logger.Errorf("Failed to unmarshal cache data", logger.String("key", key), logger.Error(err))
		return nil, false
	}

	return result, true
}

// Set stores a value in Redis cache.
func (rc *RedisCache) Set(key string, value interface{}, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := json.Marshal(value)
	if err != nil {
		logger.Errorf("Failed to marshal cache data", logger.String("key", key), logger.Error(err))
		return
	}

	if err := rc.client.Set(ctx, rc.keyPrefix+key, string(data), ttl); err != nil {
		logger.Errorf("Failed to set cache", logger.String("key", key), logger.Error(err))
	}
}

// Delete removes a value from Redis cache.
func (rc *RedisCache) Delete(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rc.client.Del(ctx, rc.keyPrefix+key); err != nil {
		logger.Errorf("Failed to delete cache", logger.String("key", key), logger.Error(err))
	}
}

// Clear removes all values from Redis cache with the key prefix.
func (rc *RedisCache) Clear() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all keys with prefix
	rdb := rc.client.RDB()
	keys, err := rdb.Keys(ctx, rc.keyPrefix+"*").Result()
	if err != nil {
		logger.Errorf("Failed to get cache keys for clear", logger.Error(err))
		return
	}

	if len(keys) > 0 {
		err := rdb.Del(ctx, keys...).Err()
		if err != nil {
			logger.Errorf("Failed to clear cache", logger.Error(err))
		}
	}
}

// Keys returns all cache keys with the prefix removed.
func (rc *RedisCache) Keys() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rdb := rc.client.RDB()
	keys, err := rdb.Keys(ctx, rc.keyPrefix+"*").Result()
	if err != nil {
		logger.Errorf("Failed to get cache keys", logger.Error(err))
		return nil
	}

	// Remove prefix from keys
	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = strings.TrimPrefix(key, rc.keyPrefix)
	}

	return result
}

// MemcacheCache implements QueryCache using Memcached.
type MemcacheCache struct {
	client    *memcache.Client
	keyPrefix string
}

// NewMemcacheCache creates a new Memcached-based cache.
func NewMemcacheCache(client *memcache.Client, keyPrefix ...string) *MemcacheCache {
	prefix := "orm_cache:"
	if len(keyPrefix) > 0 {
		prefix = keyPrefix[0]
	}
	return &MemcacheCache{
		client:    client,
		keyPrefix: prefix,
	}
}

// Get retrieves a value from Memcached cache.
func (mc *MemcacheCache) Get(key string) (interface{}, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	item, err := mc.client.Get(ctx, mc.keyPrefix+key)
	if err != nil {
		return nil, false
	}

	var result interface{}
	if err := json.Unmarshal(item.Value, &result); err != nil {
		logger.Errorf("Failed to unmarshal cache data", logger.String("key", key), logger.Error(err))
		return nil, false
	}

	return result, true
}

// Set stores a value in Memcached cache.
func (mc *MemcacheCache) Set(key string, value interface{}, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := json.Marshal(value)
	if err != nil {
		logger.Errorf("Failed to marshal cache data", logger.String("key", key), logger.Error(err))
		return
	}

	item := &memcache.Item{
		Key:        mc.keyPrefix + key,
		Value:      data,
		Expiration: int32(ttl.Seconds()),
	}

	if err := mc.client.Set(ctx, item); err != nil {
		logger.Errorf("Failed to set cache", logger.String("key", key), logger.Error(err))
	}
}

// Delete removes a value from Memcached cache.
func (mc *MemcacheCache) Delete(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := mc.client.Delete(ctx, mc.keyPrefix+key); err != nil {
		logger.Errorf("Failed to delete cache", logger.String("key", key), logger.Error(err))
	}
}

// Clear removes all values from Memcached cache with the key prefix.
func (mc *MemcacheCache) Clear() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Memcached has no pattern-delete API, so this clears the entire cache.
	if err := mc.client.DeleteAll(ctx); err != nil {
		logger.Errorf("Failed to clear cache", logger.Error(err))
	}
}

// Keys returns all cache keys (limited implementation for Memcached).
func (mc *MemcacheCache) Keys() []string {
	// Memcached doesn't have a way to list all keys
	// This is a limitation of the protocol
	logger.Warn("Memcached doesn't support listing all keys")
	return nil
}

// FailoverCache provides failover support between multiple cache backends.
// This is a wrapper around the dedicated failover package.
type FailoverCache struct {
	adapter *failover.QueryCacheAdapter
}

// NewFailoverCache creates a new failover cache with multiple backends.
func NewFailoverCache(primary, secondary, tertiary QueryCache) *FailoverCache {
	// Convert existing QueryCache implementations to failover storage
	// This is a compatibility wrapper
	storages := make([]failover.Storage, 0)

	if primary != nil {
		storages = append(storages, &queryCacheStorage{cache: primary})
	}
	if secondary != nil {
		storages = append(storages, &queryCacheStorage{cache: secondary})
	}
	if tertiary != nil {
		storages = append(storages, &queryCacheStorage{cache: tertiary})
	}

	var primaryStorage, secondaryStorage, tertiaryStorage failover.Storage
	if len(storages) > 0 {
		primaryStorage = storages[0]
	}
	if len(storages) > 1 {
		secondaryStorage = storages[1]
	}
	if len(storages) > 2 {
		tertiaryStorage = storages[2]
	}

	config := failover.Config{
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}

	failoverCache := failover.NewFailoverCache(primaryStorage, secondaryStorage, tertiaryStorage, config)
	adapter := failover.NewQueryCacheAdapter(failoverCache)

	return &FailoverCache{
		adapter: adapter,
	}
}

// queryCacheStorage adapts existing QueryCache to failover Storage interface.
type queryCacheStorage struct {
	cache QueryCache
}

// Get implements failover.Storage interface.
func (q *queryCacheStorage) Get(ctx context.Context, key string) (interface{}, bool) {
	return q.cache.Get(key)
}

// Set implements failover.Storage interface.
func (q *queryCacheStorage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	q.cache.Set(key, value, ttl)
	return nil
}

// Delete implements failover.Storage interface.
func (q *queryCacheStorage) Delete(ctx context.Context, key string) error {
	q.cache.Delete(key)
	return nil
}

// Clear implements failover.Storage interface.
func (q *queryCacheStorage) Clear(ctx context.Context) error {
	q.cache.Clear()
	return nil
}

// Keys implements failover.Storage interface.
func (q *queryCacheStorage) Keys(ctx context.Context) []string {
	return q.cache.Keys()
}

// HealthCheck implements failover.Storage interface.
func (q *queryCacheStorage) HealthCheck(ctx context.Context) error {
	// Simple health check - try to get a non-existent key
	_, _ = q.cache.Get("health_check_key")
	return nil
}

// Close implements failover.Storage interface.
func (q *queryCacheStorage) Close() error {
	return nil
}

// Get retrieves a value trying primary, then secondary, then tertiary.
func (fc *FailoverCache) Get(key string) (interface{}, bool) {
	return fc.adapter.Get(key)
}

// Set stores a value in all available backends.
func (fc *FailoverCache) Set(key string, value interface{}, ttl time.Duration) {
	fc.adapter.Set(key, value, ttl)
}

// Delete removes a value from all backends.
func (fc *FailoverCache) Delete(key string) {
	fc.adapter.Delete(key)
}

// Clear removes all values from all backends.
func (fc *FailoverCache) Clear() {
	fc.adapter.Clear()
}

// Keys returns keys from the primary backend.
func (fc *FailoverCache) Keys() []string {
	return fc.adapter.Keys()
}

// GetStorageStatus returns the health status of all storage backends.
func (fc *FailoverCache) GetStorageStatus() map[string]bool {
	return fc.adapter.GetStorageStatus()
}

// Close closes all storage backends.
func (fc *FailoverCache) Close() error {
	return fc.adapter.Close()
}

// CacheConfig holds cache configuration for the backends in this package.
// memory/file 后端的纯 orm 配置见 orm.CacheConfig。
type CacheConfig struct {
	Type           CacheType
	RedisConfig    *redis.Config        `yaml:"redis_config" json:"redis_config"`
	MemcacheConfig *memcache.Config     `yaml:"memcache_config" json:"memcache_config"`
	FileConfig     *orm.FileCacheConfig `yaml:"file_config" json:"file_config"`
	KeyPrefix      string               `yaml:"key_prefix" json:"key_prefix"`
	FallbackOrder  []CacheType          `yaml:"fallback_order" json:"fallback_order"`
}

// NewCacheFromConfig creates a cache instance from configuration.
func NewCacheFromConfig(config CacheConfig) (QueryCache, error) {
	switch config.Type {
	case orm.CacheTypeMemory:
		return orm.NewMemoryCache(), nil

	case CacheTypeRedis:
		if config.RedisConfig == nil {
			return nil, fmt.Errorf("redis config is required for redis cache")
		}
		client, err := redis.New(*config.RedisConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create redis client: %w", err)
		}
		return NewRedisCache(client, config.KeyPrefix), nil

	case CacheTypeMemcache:
		if config.MemcacheConfig == nil {
			return nil, fmt.Errorf("memcache config is required for memcache cache")
		}
		client, err := memcache.New(*config.MemcacheConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create memcache client: %w", err)
		}
		return NewMemcacheCache(client, config.KeyPrefix), nil

	case orm.CacheTypeFile:
		if config.FileConfig == nil {
			return nil, fmt.Errorf("file config is required for file cache")
		}
		return orm.NewFileCache(*config.FileConfig)

	case CacheTypeFailover:
		var primary, secondary, tertiary QueryCache

		// Create fallback caches based on order
		for i, cacheType := range config.FallbackOrder {
			var cache QueryCache

			switch cacheType {
			case orm.CacheTypeMemory:
				cache = orm.NewMemoryCache()
			case CacheTypeRedis:
				if config.RedisConfig != nil {
					client, err := redis.New(*config.RedisConfig)
					if err == nil {
						cache = NewRedisCache(client, config.KeyPrefix)
					}
				}
			case CacheTypeMemcache:
				if config.MemcacheConfig != nil {
					client, err := memcache.New(*config.MemcacheConfig)
					if err == nil {
						cache = NewMemcacheCache(client, config.KeyPrefix)
					}
				}
			case orm.CacheTypeFile:
				if config.FileConfig != nil {
					fileCache, err := orm.NewFileCache(*config.FileConfig)
					if err == nil {
						cache = fileCache
					}
				}
			}

			if cache != nil {
				switch i {
				case 0:
					primary = cache
				case 1:
					secondary = cache
				case 2:
					tertiary = cache
				}
			}
		}

		if primary == nil {
			return nil, fmt.Errorf("at least one primary cache is required for failover")
		}

		return NewFailoverCache(primary, secondary, tertiary), nil

	default:
		return nil, fmt.Errorf("unsupported cache type: %s", config.Type)
	}
}

// NewRedisCacheWithConfig creates a Redis cache with configuration.
func NewRedisCacheWithConfig(config redis.Config, keyPrefix ...string) (QueryCache, error) {
	client, err := redis.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}
	return NewRedisCache(client, keyPrefix...), nil
}

// NewMemcacheCacheWithConfig creates a Memcache cache with configuration.
func NewMemcacheCacheWithConfig(config memcache.Config, keyPrefix ...string) (QueryCache, error) {
	client, err := memcache.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create memcache client: %w", err)
	}
	return NewMemcacheCache(client, keyPrefix...), nil
}

// NewDefaultFailoverCache creates a failover cache with default configuration.
// Order: Redis -> Memcache -> Memory
func NewDefaultFailoverCache(redisConfig *redis.Config, memcacheConfig *memcache.Config, keyPrefix ...string) (QueryCache, error) {
	var caches []QueryCache

	// Try Redis first
	if redisConfig != nil {
		if client, err := redis.New(*redisConfig); err == nil {
			caches = append(caches, NewRedisCache(client, keyPrefix...))
		}
	}

	// Try Memcache second
	if memcacheConfig != nil {
		if client, err := memcache.New(*memcacheConfig); err == nil {
			caches = append(caches, NewMemcacheCache(client, keyPrefix...))
		}
	}

	// Memory cache as last resort
	caches = append(caches, orm.NewMemoryCache())

	if len(caches) < 2 {
		return nil, fmt.Errorf("need at least 2 cache backends for failover")
	}

	var primary, secondary, tertiary QueryCache
	primary = caches[0]
	if len(caches) > 1 {
		secondary = caches[1]
	}
	if len(caches) > 2 {
		tertiary = caches[2]
	}

	return NewFailoverCache(primary, secondary, tertiary), nil
}

// NewFailoverCacheWithConfig creates a failover cache using the failover package.
func NewFailoverCacheWithConfig(directory, fileSuffix string) (*FailoverCache, error) {
	adapter, err := failover.CreateFileORMAdapter(directory, fileSuffix)
	if err != nil {
		return nil, err
	}

	return &FailoverCache{
		adapter: adapter.(*failover.QueryCacheAdapter),
	}, nil
}

// NewMemoryFileFailoverCache creates a memory-primary, file-secondary failover cache.
func NewMemoryFileFailoverCache(directory, fileSuffix string) (*FailoverCache, error) {
	adapter, err := failover.CreateMemoryFileORMAdapter(directory, fileSuffix)
	if err != nil {
		return nil, err
	}

	return &FailoverCache{
		adapter: adapter.(*failover.QueryCacheAdapter),
	}, nil
}

// NewFailoverCacheFromPackage creates a default failover cache using the failover package.
func NewFailoverCacheFromPackage() (*FailoverCache, error) {
	adapter, err := failover.CreateDefaultORMAdapter()
	if err != nil {
		return nil, err
	}

	return &FailoverCache{
		adapter: adapter.(*failover.QueryCacheAdapter),
	}, nil
}
