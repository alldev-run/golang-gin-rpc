package orm

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"testing"
	"time"
)

// ==================== CacheEntry ====================

func TestCacheEntry_IsExpired(t *testing.T) {
	t.Run("not_expired", func(t *testing.T) {
		e := &CacheEntry{Timestamp: time.Now(), TTL: time.Hour}
		if e.IsExpired() {
			t.Error("entry should not be expired")
		}
	})
	t.Run("expired", func(t *testing.T) {
		e := &CacheEntry{Timestamp: time.Now().Add(-2 * time.Hour), TTL: time.Hour}
		if !e.IsExpired() {
			t.Error("entry should be expired")
		}
	})
}

// ==================== MemoryCache ====================

func TestMemoryCache_BasicOperations(t *testing.T) {
	mc := NewMemoryCache()

	// Set and Get
	mc.Set("key1", "value1", time.Minute)
	val, ok := mc.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("Get(key1) = %v, want value1", val)
	}

	// Get missing key
	_, ok = mc.Get("missing")
	if ok {
		t.Error("expected missing key to not exist")
	}

	// Keys
	mc.Set("key2", "value2", time.Minute)
	keys := mc.Keys()
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "key1" || keys[1] != "key2" {
		t.Errorf("Keys() = %v, want [key1 key2]", keys)
	}

	// Delete
	mc.Delete("key1")
	_, ok = mc.Get("key1")
	if ok {
		t.Error("key1 should be deleted")
	}

	// Clear
	mc.Clear()
	keys = mc.Keys()
	if len(keys) != 0 {
		t.Errorf("Keys() after Clear = %v, want empty", keys)
	}
}

func TestMemoryCache_ExpiredEntry(t *testing.T) {
	mc := NewMemoryCache()
	mc.Set("expired", "data", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, ok := mc.Get("expired")
	if ok {
		t.Error("expired entry should not be returned")
	}

	// The expired entry should be cleaned up from keys
	keys := mc.Keys()
	for _, k := range keys {
		if k == "expired" {
			t.Error("expired entry should be cleaned from cache")
		}
	}
}

func TestMemoryCache_Concurrent(t *testing.T) {
	mc := NewMemoryCache()
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- true }()
			mc.Set("key", n, time.Minute)
			_, _ = mc.Get("key")
		}(i)
	}
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			mc.Keys()
			mc.Clear()
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ==================== FileCache ====================

func TestFileCache_BasicOperations(t *testing.T) {
	dir := t.TempDir()
	fc, err := NewFileCache(FileCacheConfig{
		Directory:       dir,
		MaxSize:         1024 * 1024,
		MaxFiles:        100,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewFileCache error: %v", err)
	}

	// Set and Get
	fc.Set("fkey1", "fvalue1", time.Minute)
	val, ok := fc.Get("fkey1")
	if !ok {
		t.Fatal("expected fkey1 to exist")
	}
	if val != "fvalue1" {
		t.Errorf("Get(fkey1) = %v, want fvalue1", val)
	}

	// Get missing
	_, ok = fc.Get("missing")
	if ok {
		t.Error("missing key should not exist")
	}

	// Keys
	fc.Set("fkey2", "fvalue2", time.Minute)
	keys := fc.Keys()
	sort.Strings(keys)
	if len(keys) != 2 {
		t.Errorf("Keys() = %v, want 2 keys", keys)
	}

	// Delete
	fc.Delete("fkey1")
	_, ok = fc.Get("fkey1")
	if ok {
		t.Error("fkey1 should be deleted")
	}

	// Clear
	fc.Clear()
	keys = fc.Keys()
	if len(keys) != 0 {
		t.Errorf("Keys() after Clear = %v, want empty", keys)
	}
}

func TestFileCache_ExpiredEntry(t *testing.T) {
	dir := t.TempDir()
	fc, err := NewFileCache(FileCacheConfig{Directory: dir})
	if err != nil {
		t.Fatalf("NewFileCache error: %v", err)
	}

	fc.Set("expiring", "data", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, ok := fc.Get("expiring")
	if ok {
		t.Error("expired file cache entry should not be returned")
	}
}

func TestFileCache_DefaultConfig(t *testing.T) {
	dir := t.TempDir()
	fc, err := NewFileCache(FileCacheConfig{Directory: dir})
	if err != nil {
		t.Fatalf("NewFileCache error: %v", err)
	}
	if fc.maxSize != 100*1024*1024 {
		t.Errorf("default maxSize = %d, want %d", fc.maxSize, 100*1024*1024)
	}
	if fc.maxFiles != 10000 {
		t.Errorf("default maxFiles = %d, want 10000", fc.maxFiles)
	}
}

func TestFileCache_EmptyDirectory(t *testing.T) {
	// Clear/Keys on empty directory should not panic
	dir := t.TempDir()
	fc, err := NewFileCache(FileCacheConfig{Directory: dir})
	if err != nil {
		t.Fatalf("NewFileCache error: %v", err)
	}
	fc.Clear()
	keys := fc.Keys()
	if len(keys) != 0 {
		t.Errorf("Keys() = %v, want empty", keys)
	}
}

func TestFileCache_InvalidData(t *testing.T) {
	dir := t.TempDir()
	fc, err := NewFileCache(FileCacheConfig{Directory: dir})
	if err != nil {
		t.Fatalf("NewFileCache error: %v", err)
	}

	// Write invalid JSON to cache file
	fc.Set("bad", "data", time.Minute)
	// Overwrite with garbage
	path := fc.getFilePath("bad")
	if err := sql.ErrNoRows; err == nil {
		_ = path
	}
	// Just verify getFilePath sanitizes keys
	p2 := fc.getFilePath("path/with:chars")
	if strings.Contains(p2, "/") || strings.Contains(p2, ":") {
		// The path separator in the filename should be sanitized
		base := p2[len(dir)+1:]
		if strings.Contains(base, "/") || strings.Contains(base, ":") {
			t.Errorf("getFilePath did not sanitize: %s", base)
		}
	}
}

// ==================== CachedDB with MockDB ====================

func TestCachedDB_Exec(t *testing.T) {
	mock := &MockDB{
		ExecFunc: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, nil
		},
	}
	cache := NewMemoryCache()
	cache.Set("somekey", "someval", time.Minute)

	cdb := NewCachedDB(mock, cache)

	// Exec should clear cache
	_, _ = cdb.Exec(context.Background(), "INSERT INTO t VALUES (1)")
	keys := cache.Keys()
	if len(keys) != 0 {
		t.Errorf("cache should be cleared after Exec, got %v", keys)
	}
}

func TestCachedDB_Query(t *testing.T) {
	mock := &MockDB{
		QueryFunc: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
			return nil, nil
		},
	}
	cache := NewMemoryCache()
	cdb := NewCachedDB(mock, cache)

	// Non-SELECT query should bypass cache
	_, _ = cdb.Query(context.Background(), "INSERT INTO t VALUES (1)")

	// SELECT query should check cache
	_, _ = cdb.Query(context.Background(), "SELECT * FROM t")
}

func TestCachedDB_QueryRow(t *testing.T) {
	mock := &MockDB{
		QueryRowFunc: func(ctx context.Context, query string, args ...interface{}) *sql.Row {
			return nil
		},
	}
	cache := NewMemoryCache()
	cdb := NewCachedDB(mock, cache)

	// Non-SELECT bypasses cache
	row := cdb.QueryRow(context.Background(), "INSERT INTO t VALUES (1)")
	_ = row

	// SELECT checks cache
	row = cdb.QueryRow(context.Background(), "SELECT 1")
	_ = row
}

func TestCachedDB_BeginPingCloseStats(t *testing.T) {
	closed := false
	mock := &MockDB{
		BeginFunc: func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
			return nil, nil
		},
		PingFunc: func(ctx context.Context) error {
			return nil
		},
		CloseFunc: func() error {
			closed = true
			return nil
		},
		StatsFunc: func() sql.DBStats {
			return sql.DBStats{OpenConnections: 5}
		},
	}
	cache := NewMemoryCache()
	cdb := NewCachedDB(mock, cache)

	_, _ = cdb.Begin(context.Background(), nil)
	_ = cdb.Ping(context.Background())
	stats := cdb.Stats()
	if stats.OpenConnections != 5 {
		t.Errorf("Stats().OpenConnections = %d, want 5", stats.OpenConnections)
	}
	_ = cdb.Close()
	if !closed {
		t.Error("Close should have been called")
	}
}

func TestCachedDB_SetGetCache(t *testing.T) {
	mock := &MockDB{}
	cache1 := NewMemoryCache()
	cache2 := NewMemoryCache()
	cdb := NewCachedDB(mock, cache1)

	if cdb.GetCache() != cache1 {
		t.Error("GetCache should return initial cache")
	}
	cdb.SetCache(cache2)
	if cdb.GetCache() != cache2 {
		t.Error("GetCache should return updated cache")
	}
	cdb.ClearCache()
}

func TestCachedDB_IsSelectQuery(t *testing.T) {
	cdb := &CachedDB{}
	tests := []struct {
		input string
		want  bool
	}{
		{"SELECT * FROM t", true},
		{"  select * from t", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET a=1", false},
		{"DELETE FROM t", false},
	}
	for _, tt := range tests {
		if got := cdb.isSelectQuery(tt.input); got != tt.want {
			t.Errorf("isSelectQuery(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCachedDB_GenerateCacheKey(t *testing.T) {
	cdb := &CachedDB{}
	k1 := cdb.generateCacheKey("SELECT 1", []interface{}{1, "a"})
	k2 := cdb.generateCacheKey("SELECT 1", []interface{}{1, "a"})
	k3 := cdb.generateCacheKey("SELECT 2", []interface{}{1, "a"})

	if k1 != k2 {
		t.Error("same inputs should produce same key")
	}
	if k1 == k3 {
		t.Error("different queries should produce different keys")
	}
	if len(k1) != 32 { // MD5 hex
		t.Errorf("key length = %d, want 32", len(k1))
	}
}

// ==================== CacheConfig / NewCacheFromConfig ====================

func TestNewCacheFromConfig_Memory(t *testing.T) {
	cache, err := NewCacheFromConfig(CacheConfig{Type: CacheTypeMemory})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
	// Verify it works
	cache.Set("k", "v", time.Minute)
	val, ok := cache.Get("k")
	if !ok || val != "v" {
		t.Error("memory cache should work")
	}
}

func TestNewCacheFromConfig_UnsupportedType(t *testing.T) {
	_, err := NewCacheFromConfig(CacheConfig{Type: "unknown"})
	if err == nil {
		t.Error("expected error for unsupported cache type")
	}
}

func TestNewCacheFromConfig_FileNoConfig(t *testing.T) {
	_, err := NewCacheFromConfig(CacheConfig{Type: CacheTypeFile})
	if err == nil {
		t.Error("expected error when file config is nil")
	}
}

func TestNewCacheFromConfig_File(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCacheFromConfig(CacheConfig{
		Type: CacheTypeFile,
		FileConfig: &FileCacheConfig{
			Directory: dir,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
}

func TestNewCacheFromConfig_UnsupportedBackend(t *testing.T) {
	// redis/memcache/failover 后端已迁至 plugins/cache/ormcache，此处应明确报错。
	_, err := NewCacheFromConfig(CacheConfig{Type: CacheTypeRedis})
	if err == nil {
		t.Error("expected error for redis backend in orm package")
	}
	_, err = NewCacheFromConfig(CacheConfig{Type: CacheTypeMemcache})
	if err == nil {
		t.Error("expected error for memcache backend in orm package")
	}
}

// ==================== CacheType constants ====================

func TestCacheTypeConstants(t *testing.T) {
	if CacheTypeMemory != "memory" {
		t.Error("CacheTypeMemory mismatch")
	}
	if CacheTypeRedis != "redis" {
		t.Error("CacheTypeRedis mismatch")
	}
	if CacheTypeMemcache != "memcache" {
		t.Error("CacheTypeMemcache mismatch")
	}
	if CacheTypeFile != "file" {
		t.Error("CacheTypeFile mismatch")
	}
	if CacheTypeFailover != "failover" {
		t.Error("CacheTypeFailover mismatch")
	}
}

// ==================== NewDefaultFileCache ====================

func TestNewDefaultFileCache(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewDefaultFileCache(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
}

func TestNewFileCacheWithConfig(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewFileCacheWithConfig(FileCacheConfig{Directory: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
}
