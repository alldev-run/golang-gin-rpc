package orm

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
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
