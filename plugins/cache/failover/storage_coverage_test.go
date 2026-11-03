package failover

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestMemoryStorageOperations(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	if _, ok := storage.Get(ctx, "missing"); ok {
		t.Fatal("Get() found a missing key")
	}
	if err := storage.Set(ctx, "key", "value", time.Minute); err != nil {
		t.Fatal(err)
	}
	if value, ok := storage.Get(ctx, "key"); !ok || value != "value" {
		t.Fatalf("Get() = %v, %v", value, ok)
	}
	if keys := storage.Keys(ctx); len(keys) != 1 || keys[0] != "key" {
		t.Fatalf("Keys() = %v", keys)
	}
	if err := storage.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if err := storage.Delete(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	if err := storage.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFileStorageOperations(t *testing.T) {
	ctx := context.Background()
	storage, err := NewFileStorage(FileStorageConfig{
		Directory:       filepath.Join(t.TempDir(), "cache"),
		FileSuffix:      ".item",
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}
	defer storage.Close()

	if _, ok := storage.Get(ctx, "missing"); ok {
		t.Fatal("Get() found a missing key")
	}
	if err := storage.Set(ctx, "a/b", map[string]string{"status": "ready"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if value, ok := storage.Get(ctx, "a/b"); !ok || value == nil {
		t.Fatalf("Get() = %v, %v", value, ok)
	}
	if keys := storage.Keys(ctx); len(keys) != 1 || keys[0] != "a_b" {
		t.Fatalf("Keys() = %v", keys)
	}
	if err := storage.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if err := storage.Delete(ctx, "a/b"); err != nil {
		t.Fatal(err)
	}
	if err := storage.Clear(ctx); err != nil {
		t.Fatal(err)
	}
}
