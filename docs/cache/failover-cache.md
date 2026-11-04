# Failover Cache Guide

## Overview

The failover cache package provides a robust, configurable failover mechanism for caching with support for multiple storage backends. It offers automatic health checking, retry logic, and seamless failover between different storage systems.

## Key Features

- **Multiple Storage Backends**: Memory, File, Redis, Memcache support
- **Configurable File Storage**: Customizable paths and file suffixes
- **Health Monitoring**: Automatic health checks with status reporting
- **Retry Logic**: Configurable retry attempts and delays
- **ORM Integration**: Seamless integration with existing ORM cache interface
- **Thread Safety**: Concurrent access protection
- **Graceful Degradation**: Automatic fallback to secondary/tertiary storage

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Primary       │    │   Secondary     │    │   Tertiary      │
│   Storage       │───▶│   Storage       │───▶│   Storage       │
│                 │    │                 │    │                 │
│ (Fastest)       │    │ (Persistent)    │    │ (Last Resort)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Failover       │
                    │  Cache          │
                    │  Manager        │
                    └─────────────────┘
```

## Storage Types

### 1. Memory Storage
- **Fastest**: In-memory caching
- **Volatile**: Data lost on restart
- **Best for**: Hot data, temporary caching

### 2. File Storage
- **Persistent**: Survives restarts
- **Configurable**: Custom paths and suffixes
- **Best for**: Warm data, development environments

### 3. Redis Storage
- **Networked**: Distributed caching
- **Fast**: Network latency but still fast
- **Best for**: Production, distributed systems

### 4. Memcache Storage
- **Networked**: Simple key-value store
- **Limited**: No complex data structures
- **Best for**: Simple caching needs

## Configuration

### Basic Configuration

```go
config := failover.Config{
    StorageType:         failover.StorageTypeFile,
    StoragePath:         "/var/cache/myapp",
    FileSuffix:          ".cache",
    MaxRetries:          3,
    RetryDelay:          1 * time.Second,
    HealthCheckInterval: 30 * time.Second,
    FallbackOrder:       []string{"file", "memory"},
}
```

### File Storage Configuration

```go
fileConfig := failover.FileStorageConfig{
    Directory:       "/app/cache",
    FileSuffix:      ".myapp",
    MaxSize:         500 * 1024 * 1024, // 500MB
    MaxFiles:        50000,
    CleanupInterval: 15 * time.Minute,
}
```

## Usage Examples

### 1. Basic File Failover

```go
// Create file-based failover with custom suffix
cache, err := failover.NewFileFailover("/tmp/my_cache", ".my_cache")
if err != nil {
    log.Fatal(err)
}
defer cache.Close()

// Use the cache
cache.Set(context.Background(), "key", "value", 10*time.Minute)
value, found := cache.Get(context.Background(), "key")
```

### 2. Memory + File Failover

```go
// Memory primary, file secondary
cache, err := failover.NewMemoryFileFailover("/tmp/backup", ".backup")
if err != nil {
    log.Fatal(err)
}
defer cache.Close()

// Data is stored in both memory and file
cache.Set(context.Background(), "user:123", userData, 30*time.Minute)
```

### 3. Custom Configuration

```go
config := failover.Config{
    StorageType:         failover.StorageTypeFile,
    StoragePath:         "/app/failover",
    FileSuffix:          ".fail",
    MaxRetries:          5,
    RetryDelay:          2 * time.Second,
    HealthCheckInterval: 1 * time.Minute,
    FallbackOrder:       []string{"file", "memory"},
}

cache, err := failover.NewFailoverCacheFromConfig(config)
if err != nil {
    log.Fatal(err)
}
defer cache.Close()
```

### 4. ORM Integration

```go
// Create ORM-compatible failover cache
ormCache, err := failover.CreateFileORMAdapter("/tmp/orm_cache", ".orm")
if err != nil {
    log.Fatal(err)
}
defer ormCache.Close()

// Use with ORM's cached database
cachedDB := orm.NewCachedDB(db, ormCache)

// Or use ORM package convenience functions
ormFailover, err := orm.NewMemoryFileFailoverCache("/tmp/orm", ".orm")
if err != nil {
    log.Fatal(err)
}
defer ormFailover.Close()
```

### 5. Factory Pattern

```go
factory := failover.NewFactory()

// Create different types of failover caches
fileCache, _ := factory.CreateFileFailover("/tmp/cache", ".cache")
memoryFileCache, _ := factory.CreateMemoryFileFailover("/tmp/backup", ".backup")

// Custom configuration
configs := map[string]interface{}{
    "primary": failover.FileStorageConfig{
        Directory: "/app/primary",
        FileSuffix: ".primary",
    },
}

customCache, _ := factory.CreateCustomFailover(
    failover.StorageTypeFile,
    failover.StorageTypeMemory,
    "", // no tertiary
    configs,
)
```

## File Storage Customization

### Custom Paths and Suffixes

```go
// Custom directory and file suffix
cache, err := failover.NewCustomFileFailover(
    "/app/custom_cache",    // Directory
    ".custom",              // File suffix
    100*1024*1024,         // Max size: 100MB
    10000,                  // Max files
    5*time.Minute,          // Cleanup interval
)
```

### File Structure

```
/app/custom_cache/
├── user_123.custom
├── session_abc.custom
├── cache_key_xyz.custom
└── ...
```

Each file contains JSON-encoded cache entry:
```json
{
  "data": "...",
  "timestamp": "2026-03-16T13:45:00Z",
  "ttl": 1800000000000
}
```

## Health Monitoring

### Check Storage Status

```go
cache, _ := failover.NewFileFailover("/tmp/cache", ".cache")

// Get health status of all storage backends
status := cache.GetStorageStatus()
fmt.Printf("Primary healthy: %v\n", status["primary"])
fmt.Printf("Secondary healthy: %v\n", status["secondary"])
```

### Health Check Configuration

```go
config := failover.Config{
    HealthCheckInterval: 15 * time.Second, // Check every 15 seconds
    MaxRetries:          3,                // Retry 3 times
    RetryDelay:          500 * time.Millisecond, // Wait 500ms between retries
}
```

## Advanced Features

### 1. Retry Logic

```go
// Automatic retry with exponential backoff
config := failover.Config{
    MaxRetries: 5,
    RetryDelay: 1 * time.Second,
}

// When primary fails, automatically retry with secondary
value, found := cache.Get(context.Background(), "key")
```

### 2. Data Restoration

```go
// When data is found in secondary storage,
// it's automatically restored to primary for faster future access
cache.Set(context.Background(), "key", "value", 10*time.Minute)

// If primary fails, secondary returns data
// and restores it to primary automatically
```

### 3. Concurrent Access

```go
// Thread-safe operations
go func() {
    cache.Set(context.Background(), "key1", "value1", 5*time.Minute)
}()

go func() {
    cache.Set(context.Background(), "key2", "value2", 5*time.Minute)
}()

// Both operations are safe and atomic
```

## Best Practices

### 1. Storage Selection

```go
// Production: Redis primary, file secondary
config := failover.Config{
    FallbackOrder: []string{"redis", "file", "memory"},
}

// Development: File primary, memory secondary  
config := failover.Config{
    FallbackOrder: []string{"file", "memory"},
}

// Testing: Memory only
config := failover.Config{
    FallbackOrder: []string{"memory"},
}
```

### 2. File Storage Optimization

```go
// Optimize for your use case
config := failover.FileStorageConfig{
    Directory:       "/app/cache",           // Use fast storage (SSD)
    FileSuffix:      ".cache",               // Consistent suffix
    MaxSize:         1024 * 1024 * 1024,     // 1GB limit
    MaxFiles:        100000,                 // 100k files max
    CleanupInterval: 10 * time.Minute,      // Regular cleanup
}
```

### 3. Health Monitoring

```go
// Regular health checks
config := failover.Config{
    HealthCheckInterval: 30 * time.Second, // Frequent checks
    MaxRetries:          3,                // Reasonable retries
    RetryDelay:          1 * time.Second,  // Quick retry
}
```

### 4. Error Handling

```go
cache, err := failover.NewFileFailover("/tmp/cache", ".cache")
if err != nil {
    // Handle creation error
    log.Printf("Failed to create cache: %v", err)
    return
}
defer cache.Close()

// Operations are safe even if storage fails
value, found := cache.Get(context.Background(), "key")
if !found {
    // Handle cache miss
    log.Printf("Cache miss for key: %s", "key")
}
```

## Migration from Old Failover

### Old Way
```go
// Before: Simple failover in ORM package
failover := orm.NewFailoverCache(primary, secondary, tertiary)
```

### New Way
```go
// After: Rich failover with configuration
cache, err := failover.NewMemoryFileFailoverCache("/tmp/cache", ".cache")
if err != nil {
    log.Fatal(err)
}
defer cache.Close()

// Or use the adapter for ORM compatibility
ormCache := failover.NewQueryCacheAdapter(cache)
```

## Performance Considerations

### Memory Usage
- **Primary Storage**: Always in memory for fastest access
- **File Storage**: On-demand loading with automatic cleanup
- **Network Storage**: Connection pooling and timeout management

### Disk Usage
- **Size Limits**: Configurable maximum size and file count
- **Cleanup**: Automatic expired file removal
- **Compression**: Consider compression for large data

### Network Latency
- **Redis/Memcache**: Connection reuse and pipelining
- **Timeouts**: Configurable timeouts for network operations
- **Retry Logic**: Automatic retry with backoff

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```go
   // Check directory permissions
   config := failover.FileStorageConfig{
       Directory: "/tmp/cache", // Use writable directory
   }
   ```

2. **Disk Full**
   ```go
   // Monitor disk usage and set limits
   config := failover.FileStorageConfig{
       MaxSize:  100 * 1024 * 1024, // 100MB limit
       MaxFiles: 10000,              // File count limit
   }
   ```

3. **Slow Performance**
   ```go
   // Use SSD storage and optimize cleanup
   config := failover.FileStorageConfig{
       Directory:       "/ssd/cache",      // Fast storage
       CleanupInterval: 5 * time.Minute,  // Frequent cleanup
   }
   ```

### Debug Logging

```go
// Enable debug logging (if available)
cache, err := failover.NewFileFailover("/tmp/cache", ".cache")
if err != nil {
    log.Printf("Cache creation failed: %v", err)
}

// Check health status
status := cache.GetStorageStatus()
for name, healthy := range status {
    log.Printf("Storage %s: healthy=%v", name, healthy)
}
```

## Integration Examples

### With Web Applications

```go
// In your main.go
func main() {
    // Create failover cache
    cache, err := failover.NewMemoryFileFailover("/tmp/web_cache", ".web")
    if err != nil {
        log.Fatal(err)
    }
    defer cache.Close()

    // Use in HTTP handlers
    http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
        cacheKey := fmt.Sprintf("data:%s", r.URL.Query().Get("id"))
        
        if data, found := cache.Get(context.Background(), cacheKey); found {
            json.NewEncoder(w).Encode(data)
            return
        }
        
        // Fetch from database
        data := fetchDataFromDB(r.URL.Query().Get("id"))
        cache.Set(context.Background(), cacheKey, data, 10*time.Minute)
        json.NewEncoder(w).Encode(data)
    })
}
```

### With Microservices

```go
// In each microservice
func NewService() *Service {
    // Create service-specific cache
    cache, err := failover.NewFileFailover(
        fmt.Sprintf("/tmp/%s_cache", serviceName),
        fmt.Sprintf(".%s", serviceName),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    return &Service{
        cache: cache,
    }
}

type Service struct {
    cache *failover.FailoverCache
}

func (s *Service) GetUserData(userID string) (*User, error) {
    cacheKey := fmt.Sprintf("user:%s", userID)
    
    if data, found := s.cache.Get(context.Background(), cacheKey); found {
        return data.(*User), nil
    }
    
    user, err := fetchUserFromAPI(userID)
    if err != nil {
        return nil, err
    }
    
    s.cache.Set(context.Background(), cacheKey, user, 30*time.Minute)
    return user, nil
}
```

This comprehensive failover cache system provides robust, configurable caching with multiple storage backends, health monitoring, and seamless integration with existing applications.
