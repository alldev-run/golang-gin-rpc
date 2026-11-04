package ormcache

import (
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/plugins/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/orm"
)

// Offline configs for constructor error-path tests: unreachable endpoints
// with tiny timeouts so failures surface in milliseconds.
var (
	redisUnreachableConfig = redis.Config{
		Host:         "127.0.0.1",
		Port:         1,
		DialTimeout:  100 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
	}
	memcacheOfflineConfig = memcache.Config{
		Hosts:   []string{"127.0.0.1:1"},
		Timeout: 100 * time.Millisecond,
	}
)

// ==================== NewCacheFromConfig ====================

func TestNewCacheFromConfig_Memory(t *testing.T) {
	cache, err := NewCacheFromConfig(CacheConfig{Type: orm.CacheTypeMemory})
	if err != nil {
		t.Fatalf("NewCacheFromConfig(memory) error = %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
}

func TestNewCacheFromConfig_RedisNoConfig(t *testing.T) {
	if _, err := NewCacheFromConfig(CacheConfig{Type: CacheTypeRedis}); err == nil {
		t.Error("expected error when redis config is nil")
	}
}

func TestNewCacheFromConfig_RedisUnreachable(t *testing.T) {
	// redis.New pings the server; an unreachable address must surface as a
	// constructor error without hanging.
	cfg := CacheConfig{
		Type:        CacheTypeRedis,
		RedisConfig: &redisUnreachableConfig,
	}
	if _, err := NewCacheFromConfig(cfg); err == nil {
		t.Error("expected error for unreachable redis server")
	}
}

func TestNewCacheFromConfig_MemcacheNoConfig(t *testing.T) {
	if _, err := NewCacheFromConfig(CacheConfig{Type: CacheTypeMemcache}); err == nil {
		t.Error("expected error when memcache config is nil")
	}
}

func TestNewCacheFromConfig_MemcacheOffline(t *testing.T) {
	// memcache.New is lazy: construction succeeds offline, operations fail.
	cfg := CacheConfig{
		Type:           CacheTypeMemcache,
		MemcacheConfig: &memcacheOfflineConfig,
	}
	cache, err := NewCacheFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewCacheFromConfig(memcache) error = %v", err)
	}
	cache.Set("k", "v", time.Minute) // must not panic against unreachable server
	if _, found := cache.Get("k"); found {
		t.Error("Get against unreachable memcache should miss")
	}
}

func TestNewCacheFromConfig_FileNoConfig(t *testing.T) {
	if _, err := NewCacheFromConfig(CacheConfig{Type: orm.CacheTypeFile}); err == nil {
		t.Error("expected error when file config is nil")
	}
}

func TestNewCacheFromConfig_File(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewCacheFromConfig(CacheConfig{
		Type:       orm.CacheTypeFile,
		FileConfig: &orm.FileCacheConfig{Directory: dir},
	})
	if err != nil {
		t.Fatalf("NewCacheFromConfig(file) error = %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
}

func TestNewCacheFromConfig_FailoverNoPrimary(t *testing.T) {
	_, err := NewCacheFromConfig(CacheConfig{
		Type:          CacheTypeFailover,
		FallbackOrder: []CacheType{},
	})
	if err == nil {
		t.Error("expected error when no primary cache for failover")
	}
}

func TestNewCacheFromConfig_FailoverWithMemory(t *testing.T) {
	cache, err := NewCacheFromConfig(CacheConfig{
		Type:          CacheTypeFailover,
		FallbackOrder: []CacheType{orm.CacheTypeMemory, orm.CacheTypeMemory},
	})
	if err != nil {
		t.Fatalf("NewCacheFromConfig(failover) error = %v", err)
	}

	cache.Set("k", "v", time.Minute)
	if got, found := cache.Get("k"); !found || got != "v" {
		t.Errorf("Get = (%v, %v), want (v, true)", got, found)
	}
	if keys := cache.Keys(); len(keys) != 1 {
		t.Errorf("Keys = %v, want 1 key", keys)
	}
	cache.Delete("k")
	if _, found := cache.Get("k"); found {
		t.Error("deleted key should be gone")
	}
	cache.Clear()
}

func TestNewCacheFromConfig_Unsupported(t *testing.T) {
	if _, err := NewCacheFromConfig(CacheConfig{Type: "bogus"}); err == nil {
		t.Error("expected error for unsupported cache type")
	}
}

// ==================== FailoverCache composition ====================

func TestNewFailoverCache_MemoryBackends(t *testing.T) {
	fc := NewFailoverCache(orm.NewMemoryCache(), orm.NewMemoryCache(), nil)
	if fc == nil {
		t.Fatal("FailoverCache should not be nil")
	}

	fc.Set("a", 1, time.Minute)
	if got, found := fc.Get("a"); !found || got != 1 {
		t.Errorf("Get = (%v, %v), want (1, true)", got, found)
	}
	if status := fc.GetStorageStatus(); status == nil {
		t.Error("GetStorageStatus should not return nil")
	}
	if err := fc.Close(); err != nil {
		t.Errorf("Close error = %v, want nil", err)
	}
}

// ==================== CacheType constants ====================

func TestCacheTypeConstants(t *testing.T) {
	if CacheTypeRedis != "redis" {
		t.Error("CacheTypeRedis mismatch")
	}
	if CacheTypeMemcache != "memcache" {
		t.Error("CacheTypeMemcache mismatch")
	}
	if CacheTypeFailover != "failover" {
		t.Error("CacheTypeFailover mismatch")
	}
}
