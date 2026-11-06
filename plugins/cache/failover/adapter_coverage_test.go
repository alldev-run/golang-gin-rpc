package failover

import (
	"path/filepath"
	"testing"
	"time"
)

func TestQueryCacheAdapterOperations(t *testing.T) {
	cache, err := CreateMemoryFileORMAdapter(t.TempDir(), ".item")
	if err != nil {
		t.Fatalf("CreateMemoryFileORMAdapter() error = %v", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	cache.Set("key", "value", time.Minute)
	if value, ok := cache.Get("key"); !ok || value != "value" {
		t.Fatalf("Get() = %v, %v; want value", value, ok)
	}
	if keys := cache.Keys(); len(keys) != 1 || keys[0] != "key" {
		t.Fatalf("Keys() = %v; want [key]", keys)
	}
	adapter := cache.(*QueryCacheAdapter)
	if adapter.GetFailoverCache() == nil || len(adapter.GetStorageStatus()) == 0 {
		t.Fatal("adapter did not expose its failover cache and storage status")
	}

	cache.Delete("key")
	if _, ok := cache.Get("key"); ok {
		t.Fatal("Get() found a deleted key")
	}
	cache.Set("another", "value", time.Minute)
	cache.Clear()
	if keys := cache.Keys(); len(keys) != 0 {
		t.Fatalf("Keys() after Clear() = %v; want empty", keys)
	}
}

func TestFailoverFactoriesUseLocalStorage(t *testing.T) {
	factory := NewFactory()
	var caches []*FailoverCache
	add := func(cache *FailoverCache, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("failover factory error = %v", err)
		}
		caches = append(caches, cache)
	}

	add(factory.CreateFileFailover(filepath.Join(t.TempDir(), "file"), ".item"))
	add(factory.CreateMemoryFileFailover(filepath.Join(t.TempDir(), "memory-file"), ".item"))
	add(factory.CreateCustomFailover(StorageTypeMemory, StorageTypeMemory, StorageTypeMemory, nil))
	add(factory.CreateFromConfig(Config{FallbackOrder: []string{"memory"}}))
	add(NewFileFailover(filepath.Join(t.TempDir(), "helper-file"), ".item"))
	add(NewMemoryFileFailover(filepath.Join(t.TempDir(), "helper-memory-file"), ".item"))
	add(NewCustomFileFailover(filepath.Join(t.TempDir(), "custom-file"), ".item", 1024, 10, time.Hour))

	for _, cache := range caches {
		if err := cache.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}
}
