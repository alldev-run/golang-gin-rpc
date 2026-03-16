// Package failover provides failover cache functionality with configurable storage.
package failover

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Storage interface defines the storage backend for failover cache.
type Storage interface {
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Keys(ctx context.Context) []string
	HealthCheck(ctx context.Context) error
	Close() error
}

// CacheEntry represents a cached item with metadata.
type CacheEntry struct {
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	TTL       time.Duration `json:"ttl"`
}

// IsExpired checks if the cache entry has expired.
func (ce *CacheEntry) IsExpired() bool {
	return time.Since(ce.Timestamp) > ce.TTL
}

// MemoryStorage provides in-memory storage implementation.
type MemoryStorage struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
}

// NewMemoryStorage creates a new memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		cache: make(map[string]*CacheEntry),
	}
}

// Get retrieves a value from memory storage.
func (m *MemoryStorage) Get(ctx context.Context, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.cache[key]
	if !exists || entry.IsExpired() {
		if exists {
			delete(m.cache, key)
		}
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in memory storage.
func (m *MemoryStorage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache[key] = &CacheEntry{
		Data:      value,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	return nil
}

// Delete removes a value from memory storage.
func (m *MemoryStorage) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cache, key)
	return nil
}

// Clear removes all values from memory storage.
func (m *MemoryStorage) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]*CacheEntry)
	return nil
}

// Keys returns all keys in memory storage.
func (m *MemoryStorage) Keys(ctx context.Context) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.cache))
	for key := range m.cache {
		keys = append(keys, key)
	}

	return keys
}

// HealthCheck checks the health of memory storage.
func (m *MemoryStorage) HealthCheck(ctx context.Context) error {
	// Memory storage is always healthy
	return nil
}

// Close closes the memory storage.
func (m *MemoryStorage) Close() error {
	// No cleanup needed for memory storage
	return nil
}

// FileStorage provides file-based storage implementation.
type FileStorage struct {
	directory       string
	fileSuffix      string
	maxSize         int64
	maxFiles        int
	cleanupInterval time.Duration
	mu              sync.RWMutex
	cleanupTicker   *time.Ticker
}

// NewFileStorage creates a new file storage.
func NewFileStorage(config FileStorageConfig) (*FileStorage, error) {
	if config.Directory == "" {
		config.Directory = "/tmp/failover_cache"
	}
	if config.FileSuffix == "" {
		config.FileSuffix = ".failover"
	}
	if config.MaxSize == 0 {
		config.MaxSize = 100 * 1024 * 1024 // 100MB
	}
	if config.MaxFiles == 0 {
		config.MaxFiles = 10000
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 10 * time.Minute
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(config.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	storage := &FileStorage{
		directory:       config.Directory,
		fileSuffix:      config.FileSuffix,
		maxSize:         config.MaxSize,
		maxFiles:        config.MaxFiles,
		cleanupInterval: config.CleanupInterval,
	}

	// Start cleanup goroutine
	storage.cleanupTicker = time.NewTicker(storage.cleanupInterval)
	go storage.startCleanup()

	return storage, nil
}

// Get retrieves a value from file storage.
func (f *FileStorage) Get(ctx context.Context, key string) (interface{}, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	filePath := f.getFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	if entry.IsExpired() {
		os.Remove(filePath)
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in file storage.
func (f *FileStorage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	entry := CacheEntry{
		Data:      value,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	filePath := f.getFilePath(key)
	return os.WriteFile(filePath, data, 0644)
}

// Delete removes a value from file storage.
func (f *FileStorage) Delete(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	filePath := f.getFilePath(key)
	return os.Remove(filePath)
}

// Clear removes all values from file storage.
func (f *FileStorage) Clear(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	files, err := os.ReadDir(f.directory)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), f.fileSuffix) {
			os.Remove(filepath.Join(f.directory, file.Name()))
		}
	}

	return nil
}

// Keys returns all keys in file storage.
func (f *FileStorage) Keys(ctx context.Context) []string {
	f.mu.RLock()
	defer f.mu.Unlock()

	files, err := os.ReadDir(f.directory)
	if err != nil {
		return nil
	}

	var keys []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), f.fileSuffix) {
			key := strings.TrimSuffix(file.Name(), f.fileSuffix)
			keys = append(keys, key)
		}
	}

	return keys
}

// HealthCheck checks the health of file storage.
func (f *FileStorage) HealthCheck(ctx context.Context) error {
	// Check if directory is accessible
	_, err := os.Stat(f.directory)
	return err
}

// Close closes the file storage.
func (f *FileStorage) Close() error {
	if f.cleanupTicker != nil {
		f.cleanupTicker.Stop()
	}
	return nil
}

// getFilePath returns the file path for a given key.
func (f *FileStorage) getFilePath(key string) string {
	// Sanitize key for filesystem
	safeKey := strings.ReplaceAll(key, "/", "_")
	safeKey = strings.ReplaceAll(safeKey, ":", "_")
	return filepath.Join(f.directory, safeKey+f.fileSuffix)
}

// startCleanup starts the cleanup goroutine.
func (f *FileStorage) startCleanup() {
	for range f.cleanupTicker.C {
		f.cleanup()
	}
}

// cleanup removes expired files and enforces size limits.
func (f *FileStorage) cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	files, err := os.ReadDir(f.directory)
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
		if file.IsDir() || !strings.HasSuffix(file.Name(), f.fileSuffix) {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		filePath := filepath.Join(f.directory, file.Name())
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
			os.Remove(filepath.Join(f.directory, fileInfo.name))
			totalSize -= fileInfo.size
		}
	}

	// If still over limits, remove oldest files
	if totalSize > f.maxSize || len(fileInfos) > f.maxFiles {
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].modTime.Before(fileInfos[j].modTime)
		})

		for _, fileInfo := range fileInfos {
			if totalSize <= f.maxSize && len(fileInfos) <= f.maxFiles {
				break
			}
			os.Remove(filepath.Join(f.directory, fileInfo.name))
			totalSize -= fileInfo.size
			fileInfos = fileInfos[1:]
		}
	}
}

// RedisStorage provides Redis-based storage implementation.
type RedisStorage struct {
	// This is a placeholder for Redis storage
	// Implementation would depend on Redis client library
	host      string
	port      int
	password  string
	database  int
	keyPrefix string
	timeout   time.Duration
}

// NewRedisStorage creates a new Redis storage.
func NewRedisStorage(config RedisStorageConfig) (*RedisStorage, error) {
	return &RedisStorage{
		host:      config.Host,
		port:      config.Port,
		password:  config.Password,
		database:  config.Database,
		keyPrefix: config.KeyPrefix,
		timeout:   config.Timeout,
	}, nil
}

// Get retrieves a value from Redis storage.
func (r *RedisStorage) Get(ctx context.Context, key string) (interface{}, bool) {
	// Placeholder implementation
	return nil, false
}

// Set stores a value in Redis storage.
func (r *RedisStorage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Placeholder implementation
	return fmt.Errorf("redis storage not implemented")
}

// Delete removes a value from Redis storage.
func (r *RedisStorage) Delete(ctx context.Context, key string) error {
	// Placeholder implementation
	return fmt.Errorf("redis storage not implemented")
}

// Clear removes all values from Redis storage.
func (r *RedisStorage) Clear(ctx context.Context) error {
	// Placeholder implementation
	return fmt.Errorf("redis storage not implemented")
}

// Keys returns all keys in Redis storage.
func (r *RedisStorage) Keys(ctx context.Context) []string {
	// Placeholder implementation
	return nil
}

// HealthCheck checks the health of Redis storage.
func (r *RedisStorage) HealthCheck(ctx context.Context) error {
	// Placeholder implementation
	return fmt.Errorf("redis storage not implemented")
}

// Close closes the Redis storage.
func (r *RedisStorage) Close() error {
	// Placeholder implementation
	return nil
}

// MemcacheStorage provides Memcache-based storage implementation.
type MemcacheStorage struct {
	// This is a placeholder for Memcache storage
	// Implementation would depend on Memcache client library
	hosts     []string
	keyPrefix string
	timeout   time.Duration
}

// NewMemcacheStorage creates a new Memcache storage.
func NewMemcacheStorage(config MemcacheStorageConfig) (*MemcacheStorage, error) {
	return &MemcacheStorage{
		hosts:     config.Hosts,
		keyPrefix: config.KeyPrefix,
		timeout:   config.Timeout,
	}, nil
}

// Get retrieves a value from Memcache storage.
func (m *MemcacheStorage) Get(ctx context.Context, key string) (interface{}, bool) {
	// Placeholder implementation
	return nil, false
}

// Set stores a value in Memcache storage.
func (m *MemcacheStorage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Placeholder implementation
	return fmt.Errorf("memcache storage not implemented")
}

// Delete removes a value from Memcache storage.
func (m *MemcacheStorage) Delete(ctx context.Context, key string) error {
	// Placeholder implementation
	return fmt.Errorf("memcache storage not implemented")
}

// Clear removes all values from Memcache storage.
func (m *MemcacheStorage) Clear(ctx context.Context) error {
	// Placeholder implementation
	return fmt.Errorf("memcache storage not implemented")
}

// Keys returns all keys in Memcache storage.
func (m *MemcacheStorage) Keys(ctx context.Context) []string {
	// Placeholder implementation
	return nil
}

// HealthCheck checks the health of Memcache storage.
func (m *MemcacheStorage) HealthCheck(ctx context.Context) error {
	// Placeholder implementation
	return fmt.Errorf("memcache storage not implemented")
}

// Close closes the Memcache storage.
func (m *MemcacheStorage) Close() error {
	// Placeholder implementation
	return nil
}
