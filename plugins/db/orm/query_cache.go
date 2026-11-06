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
		fmt.Fprintf(hasher, "%v", arg)
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
	// Cache keys are hashes of complete queries and arguments, so table-scoped
	// invalidation cannot identify matching entries and must clear the cache.
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
	CacheTypeMemory   CacheType = "memory"
	CacheTypeRedis    CacheType = "redis"
	CacheTypeMemcache CacheType = "memcache"
	CacheTypeFile     CacheType = "file"
	CacheTypeFailover CacheType = "failover"
)

// CacheConfig holds cache configuration for the backends that live in this
// package (memory/file)。redis/memcache/failover 后端见 plugins/cache/ormcache。
type CacheConfig struct {
	Type       CacheType
	FileConfig *FileCacheConfig `yaml:"file_config" json:"file_config"`
	KeyPrefix  string           `yaml:"key_prefix" json:"key_prefix"`
}

// FileCacheConfig holds file cache configuration.
type FileCacheConfig struct {
	Directory       string        `yaml:"directory" json:"directory"`               // Cache directory
	MaxSize         int64         `yaml:"max_size" json:"max_size"`                 // Maximum cache size in bytes
	MaxFiles        int           `yaml:"max_files" json:"max_files"`               // Maximum number of cache files
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"` // Cleanup interval
}

// NewCacheFromConfig creates a cache instance from configuration.
// 本函数只覆盖本包内的 memory/file 后端；redis/memcache/failover 配置化
// 构造见 plugins/cache/ormcache.NewCacheFromConfig。
func NewCacheFromConfig(config CacheConfig) (QueryCache, error) {
	switch config.Type {
	case CacheTypeMemory:
		return NewMemoryCache(), nil

	case CacheTypeFile:
		if config.FileConfig == nil {
			return nil, fmt.Errorf("file config is required for file cache")
		}
		return NewFileCache(*config.FileConfig)

	default:
		return nil, fmt.Errorf("unsupported cache type %q in orm (redis/memcache/failover moved to plugins/cache/ormcache)", config.Type)
	}
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
