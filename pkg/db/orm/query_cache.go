package orm

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/cache/failover"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// CacheEntry represents a cached query result.
type CacheEntry struct {
	Data      interface{}   // Cached data
	Timestamp time.Time     // When the entry was cached
	TTL       time.Duration // Time to live
}

// IsExpired checks if the cache entry has expired.
func (ce *CacheEntry) IsExpired() bool {
	return time.Since(ce.Timestamp) > ce.TTL
}

// QueryCache provides caching functionality for query results.
type QueryCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	Keys() []string
}

// RedisCache implements QueryCache using Redis.
type RedisCache struct {
	client *redis.Client
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

	// Memcached doesn't support pattern-based deletion like Redis
	// We'll need to track keys or use a different approach
	// For now, we'll delete all cache
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
// This is now a wrapper around the dedicated failover package.
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

// MemoryCache is an in-memory implementation of QueryCache.
type MemoryCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		cache: make(map[string]*CacheEntry),
	}
}

// Get retrieves a value from the cache.
func (mc *MemoryCache) Get(key string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.cache[key]
	if !exists || entry.IsExpired() {
		if exists {
			delete(mc.cache, key) // Clean up expired entry
		}
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in the cache.
func (mc *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache[key] = &CacheEntry{
		Data:      value,
		Timestamp: time.Now(),
		TTL:       ttl,
	}
}

// Delete removes a value from the cache.
func (mc *MemoryCache) Delete(key string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.cache, key)
}

// Clear removes all values from the cache.
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.cache = make(map[string]*CacheEntry)
}

// Keys returns all cache keys.
func (mc *MemoryCache) Keys() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	keys := make([]string, 0, len(mc.cache))
	for key := range mc.cache {
		keys = append(keys, key)
	}

	return keys
}

// FileCache implements QueryCache using filesystem storage.
type FileCache struct {
	directory       string
	maxSize         int64
	maxFiles        int
	cleanupInterval time.Duration
	mu              sync.RWMutex
}

// NewFileCache creates a new file-based cache.
func NewFileCache(config FileCacheConfig) (*FileCache, error) {
	if config.Directory == "" {
		config.Directory = "/tmp/orm_cache"
	}
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024 // 100MB default
	}
	if config.MaxFiles == 0 {
		config.MaxFiles = 10000
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 10 * time.Minute
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &FileCache{
		directory:       config.Directory,
		maxSize:         config.MaxSize,
		maxFiles:        config.MaxFiles,
		cleanupInterval: config.CleanupInterval,
	}

	// Start cleanup goroutine
	go cache.startCleanup()

	return cache, nil
}

// Get retrieves a value from file cache.
func (fc *FileCache) Get(key string) (interface{}, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	filePath := fc.getFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	if entry.IsExpired() {
		os.Remove(filePath) // Clean up expired entry
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in file cache.
func (fc *FileCache) Set(key string, value interface{}, ttl time.Duration) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	entry := CacheEntry{
		Data:      value,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		logger.Errorf("Failed to marshal cache data", logger.String("key", key), logger.Error(err))
		return
	}

	filePath := fc.getFilePath(key)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		logger.Errorf("Failed to write cache file | key: " + key + " | error: " + err.Error())
	}
}

// Delete removes a value from file cache.
func (fc *FileCache) Delete(key string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	filePath := fc.getFilePath(key)
	os.Remove(filePath)
}

// Clear removes all values from file cache.
func (fc *FileCache) Clear() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	files, err := os.ReadDir(fc.directory)
	if err != nil {
		return
	}

	for _, file := range files {
		if !file.IsDir() {
			os.Remove(filepath.Join(fc.directory, file.Name()))
		}
	}
}

// Keys returns all cache keys.
func (fc *FileCache) Keys() []string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	files, err := os.ReadDir(fc.directory)
	if err != nil {
		return nil
	}

	var keys []string
	for _, file := range files {
		if !file.IsDir() {
			// Remove file extension to get key
			key := strings.TrimSuffix(file.Name(), ".cache")
			keys = append(keys, key)
		}
	}

	return keys
}

// getFilePath returns the file path for a given key.
func (fc *FileCache) getFilePath(key string) string {
	// Sanitize key for filesystem
	safeKey := strings.ReplaceAll(key, "/", "_")
	safeKey = strings.ReplaceAll(safeKey, ":", "_")
	return filepath.Join(fc.directory, safeKey+".cache")
}

// startCleanup starts the cleanup goroutine.
func (fc *FileCache) startCleanup() {
	ticker := time.NewTicker(fc.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		fc.cleanup()
	}
}

// cleanup removes expired files and enforces size limits.
func (fc *FileCache) cleanup() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	files, err := os.ReadDir(fc.directory)
	if err != nil {
		return
	}

	var totalSize int64
	var fileInfos []struct {
		name    string
		size    int64
		modTime time.Time
		expired bool
	}

	// Collect file information
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(fc.directory, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var entry CacheEntry
		expired := false
		if err := json.Unmarshal(data, &entry); err == nil {
			expired = entry.IsExpired()
		}

		totalSize += info.Size()
		fileInfos = append(fileInfos, struct {
			name    string
			size    int64
			modTime time.Time
			expired bool
		}{
			name:    file.Name(),
			size:    info.Size(),
			modTime: info.ModTime(),
			expired: expired,
		})
	}

	// Remove expired files
	for _, fileInfo := range fileInfos {
		if fileInfo.expired {
			os.Remove(filepath.Join(fc.directory, fileInfo.name))
			totalSize -= fileInfo.size
		}
	}

	// If still over size limit, remove oldest files
	if totalSize > fc.maxSize || len(fileInfos) > fc.maxFiles {
		// Sort by modification time (oldest first)
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].modTime.Before(fileInfos[j].modTime)
		})

		for _, fileInfo := range fileInfos {
			if totalSize <= fc.maxSize && len(fileInfos) <= fc.maxFiles {
				break
			}
			os.Remove(filepath.Join(fc.directory, fileInfo.name))
			totalSize -= fileInfo.size
			fileInfos = fileInfos[1:]
		}
	}
}

// CachedDB wraps a DB interface to provide caching.
type CachedDB struct {
	db    DB
	cache QueryCache
}

// NewCachedDB creates a new cached database wrapper.
func NewCachedDB(db DB, cache QueryCache) *CachedDB {
	return &CachedDB{
		db:    db,
		cache: cache,
	}
}

// Exec executes a query without caching (modifications shouldn't be cached).
func (cdb *CachedDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	// Invalidate related cache entries on modifications
	cdb.invalidateCacheForQuery(query, args)

	return cdb.db.Exec(ctx, query, args...)
}

// Query executes a SELECT query with caching.
func (cdb *CachedDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// Only cache SELECT queries
	if !cdb.isSelectQuery(query) {
		return cdb.db.Query(ctx, query, args...)
	}

	cacheKey := cdb.generateCacheKey(query, args)

	// Try to get from cache first
	if cached, found := cdb.cache.Get(cacheKey); found {
		if rows, ok := cached.(*sql.Rows); ok {
			return rows, nil
		}
	}

	// Execute query
	rows, err := cdb.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// Note: We can't easily cache *sql.Rows as they need to be consumed immediately
	// For caching, we'd need to read all data and cache the result
	// This is a simplified implementation - in practice, you'd want to cache the actual data

	return rows, nil
}

// QueryRow executes a query that returns a single row with caching.
func (cdb *CachedDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// Only cache SELECT queries
	if !cdb.isSelectQuery(query) {
		return cdb.db.QueryRow(ctx, query, args...)
	}

	cacheKey := cdb.generateCacheKey(query, args)

	// Try to get from cache first
	if cached, found := cdb.cache.Get(cacheKey); found {
		if row, ok := cached.(*sql.Row); ok {
			return row
		}
	}

	return cdb.db.QueryRow(ctx, query, args...)
}

// CachedQuery executes a query and caches the result.
func (cdb *CachedDB) CachedQuery(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	if !cdb.isSelectQuery(query) {
		return nil, fmt.Errorf("only SELECT queries can be cached")
	}

	cacheKey := cdb.generateCacheKey(query, args)

	// Try to get from cache first
	if cached, found := cdb.cache.Get(cacheKey); found {
		return cached, nil
	}

	// Execute query and cache result
	rows, err := cdb.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Read all data into memory for caching
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Cache the result
	cdb.cache.Set(cacheKey, result, 5*time.Minute) // Default 5 minute TTL

	return result, nil
}

// CachedQueryRow executes a query that returns a single row and caches the result.
func (cdb *CachedDB) CachedQueryRow(ctx context.Context, query string, args ...interface{}) (map[string]interface{}, error) {
	result, err := cdb.CachedQuery(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	if rows, ok := result.([]map[string]interface{}); ok && len(rows) > 0 {
		return rows[0], nil
	}

	return nil, sql.ErrNoRows
}

// Begin starts a transaction.
func (cdb *CachedDB) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return cdb.db.Begin(ctx, opts)
}

// Ping checks the database connection.
func (cdb *CachedDB) Ping(ctx context.Context) error {
	return cdb.db.Ping(ctx)
}

// Close closes the database connection.
func (cdb *CachedDB) Close() error {
	return cdb.db.Close()
}

// Stats returns database statistics.
func (cdb *CachedDB) Stats() sql.DBStats {
	return cdb.db.Stats()
}

// generateCacheKey generates a cache key for a query.
func (cdb *CachedDB) generateCacheKey(query string, args []interface{}) string {
	hasher := md5.New()
	hasher.Write([]byte(query))

	for _, arg := range args {
		hasher.Write([]byte(fmt.Sprintf("%v", arg)))
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// isSelectQuery checks if a query is a SELECT query.
func (cdb *CachedDB) isSelectQuery(query string) bool {
	query = strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(query, "SELECT")
}

// invalidateCacheForQuery invalidates cache entries related to a query.
func (cdb *CachedDB) invalidateCacheForQuery(query string, args []interface{}) {
	// Simple implementation: clear all cache on any modification
	// In a real implementation, you'd want more sophisticated cache invalidation
	cdb.cache.Clear()
}

// SetCache sets the cache implementation.
func (cdb *CachedDB) SetCache(cache QueryCache) {
	cdb.cache = cache
}

// GetCache returns the current cache implementation.
func (cdb *CachedDB) GetCache() QueryCache {
	return cdb.cache
}

// ClearCache clears all cached entries.
func (cdb *CachedDB) ClearCache() {
	cdb.cache.Clear()
}

// CacheType represents the type of cache backend.
type CacheType string

const (
	CacheTypeMemory    CacheType = "memory"
	CacheTypeRedis     CacheType = "redis"
	CacheTypeMemcache  CacheType = "memcache"
	CacheTypeFile      CacheType = "file"
	CacheTypeFailover  CacheType = "failover"
)

// CacheConfig holds cache configuration.
type CacheConfig struct {
	Type           CacheType
	RedisConfig    *redis.Config     `yaml:"redis_config" json:"redis_config"`
	MemcacheConfig *memcache.Config  `yaml:"memcache_config" json:"memcache_config"`
	FileConfig     *FileCacheConfig  `yaml:"file_config" json:"file_config"`
	KeyPrefix      string            `yaml:"key_prefix" json:"key_prefix"`
	FallbackOrder  []CacheType       `yaml:"fallback_order" json:"fallback_order"`
}

// FileCacheConfig holds file cache configuration.
type FileCacheConfig struct {
	Directory    string        `yaml:"directory" json:"directory"`         // Cache directory
	MaxSize      int64         `yaml:"max_size" json:"max_size"`           // Maximum cache size in bytes
	MaxFiles     int           `yaml:"max_files" json:"max_files"`         // Maximum number of cache files
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"` // Cleanup interval
}

// NewCacheFromConfig creates a cache instance from configuration.
func NewCacheFromConfig(config CacheConfig) (QueryCache, error) {
	switch config.Type {
	case CacheTypeMemory:
		return NewMemoryCache(), nil
		
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
		
	case CacheTypeFile:
		if config.FileConfig == nil {
			return nil, fmt.Errorf("file config is required for file cache")
		}
		return NewFileCache(*config.FileConfig)
		
	case CacheTypeFailover:
		var primary, secondary, tertiary QueryCache
		
		// Create fallback caches based on order
		for i, cacheType := range config.FallbackOrder {
			var cache QueryCache
			
			switch cacheType {
			case CacheTypeMemory:
				cache = NewMemoryCache()
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
			case CacheTypeFile:
				if config.FileConfig != nil {
					fileCache, err := NewFileCache(*config.FileConfig)
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
	caches = append(caches, NewMemoryCache())
	
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

// NewFileCacheWithConfig creates a file cache with configuration.
func NewFileCacheWithConfig(config FileCacheConfig) (QueryCache, error) {
	return NewFileCache(config)
}

// NewDefaultFileCache creates a file cache with default configuration.
func NewDefaultFileCache(directory ...string) (QueryCache, error) {
	config := FileCacheConfig{
		Directory:       "/tmp/orm_cache",
		MaxSize:         100 * 1024 * 1024, // 100MB
		MaxFiles:        10000,
		CleanupInterval: 10 * time.Minute,
	}
	
	if len(directory) > 0 {
		config.Directory = directory[0]
	}
	
	return NewFileCache(config)
}

// NewFailoverCacheWithConfig creates a failover cache using the new failover package.
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

// NewFailoverCacheFromPackage creates a default failover cache using the new failover package.
func NewFailoverCacheFromPackage() (*FailoverCache, error) {
	adapter, err := failover.CreateDefaultORMAdapter()
	if err != nil {
		return nil, err
	}
	
	return &FailoverCache{
		adapter: adapter.(*failover.QueryCacheAdapter),
	}, nil
}
