# File Cache Guide

## Overview

The ORM package now supports filesystem-based caching, providing persistent storage for query results that survives application restarts. File cache is particularly useful for:

- Applications that need persistent caching
- Environments without Redis or Memcached
- Development and testing environments
- Cost-effective caching solution

## Features

- **Persistent Storage**: Cache data stored on filesystem survives restarts
- **TTL Support**: Automatic expiration of cache entries
- **Size Limits**: Configurable maximum cache size and file count
- **Automatic Cleanup**: Background cleanup of expired and old files
- **Thread Safe**: Concurrent access protection with mutex locks
- **Failover Support**: Can be used in failover cache configurations

## Configuration

### FileCacheConfig

```go
type FileCacheConfig struct {
    Directory       string        // Cache directory path
    MaxSize         int64         // Maximum cache size in bytes
    MaxFiles        int           // Maximum number of cache files
    CleanupInterval time.Duration // Cleanup interval for expired files
}
```

### Default Values

- **Directory**: `/tmp/orm_cache`
- **MaxSize**: 100MB (100 * 1024 * 1024 bytes)
- **MaxFiles**: 10,000 files
- **CleanupInterval**: 10 minutes

## Usage Examples

### 1. Basic File Cache

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/db/orm"

// Create default file cache
cache, err := orm.NewDefaultFileCache("/tmp/my_cache")
if err != nil {
    log.Fatal(err)
}

// Store data
cache.Set("user:123", userData, 30*time.Minute)

// Retrieve data
if value, found := cache.Get("user:123"); found {
    fmt.Printf("User data: %v", value)
}
```

### 2. Custom Configuration

```go
config := orm.FileCacheConfig{
    Directory:       "/var/cache/myapp",
    MaxSize:         500 * 1024 * 1024, // 500MB
    MaxFiles:        50000,
    CleanupInterval: 15 * time.Minute,
}

cache, err := orm.NewFileCache(config)
if err != nil {
    log.Fatal(err)
}
```

### 3. Configuration-Based Setup

```go
cacheConfig := orm.CacheConfig{
    Type: orm.CacheTypeFile,
    FileConfig: &orm.FileCacheConfig{
        Directory:       "/app/cache",
        MaxSize:         200 * 1024 * 1024, // 200MB
        MaxFiles:        20000,
        CleanupInterval: 5 * time.Minute,
    },
    KeyPrefix: "myapp:",
}

cache, err := orm.NewCacheFromConfig(cacheConfig)
```

### 4. Failover Configuration

```go
// File cache as primary, memory as fallback
failoverConfig := orm.CacheConfig{
    Type: orm.CacheTypeFailover,
    FallbackOrder: []orm.CacheType{
        orm.CacheTypeFile,    // Primary: File cache
        orm.CacheTypeMemory,  // Secondary: Memory cache
    },
    FileConfig: &orm.FileCacheConfig{
        Directory: "/app/cache",
        MaxSize:   100 * 1024 * 1024, // 100MB
        MaxFiles:  10000,
    },
}

cache, err := orm.NewCacheFromConfig(failoverConfig)
```

### 5. Database Integration

```go
// Create cached database with file cache
fileCache, _ := orm.NewDefaultFileCache("/tmp/db_cache")
cachedDB := orm.NewCachedDB(db, fileCache)

// Execute cached queries
result, err := cachedDB.CachedQuery(ctx, "SELECT * FROM users WHERE active = ?", true)
if err != nil {
    log.Fatal(err)
}
```

## File Structure

Cache files are stored with the following naming convention:

```
<directory>/<sanitized_key>.cache
```

- Keys are sanitized for filesystem safety (replacing `/` and `:` with `_`)
- Each file contains a JSON-encoded `CacheEntry` with data, timestamp, and TTL
- Files are automatically cleaned up when expired

## Performance Considerations

### Advantages
- **Persistent**: Survives application restarts
- **Cost Effective**: No additional infrastructure required
- **Simple Setup**: Just specify a directory path

### Limitations
- **Slower than Memory**: File I/O is slower than in-memory caching
- **Disk Space**: Requires sufficient disk space
- **Concurrent Access**: File system locking can limit concurrency

### Best Practices
1. Use SSD storage for better performance
2. Monitor disk space usage
3. Set appropriate cleanup intervals
4. Consider using file cache as secondary/fallback cache
5. Use separate directories for different applications

## Monitoring and Maintenance

### Cache Statistics
```go
// Get all cache keys
keys := cache.Keys()
fmt.Printf("Total cached items: %d\n", len(keys))

// Monitor cache directory size
// (Implementation depends on your monitoring system)
```

### Manual Cleanup
```go
// Clear all cache
cache.Clear()

// Delete specific key
cache.Delete("specific_key")
```

## Security Considerations

1. **Directory Permissions**: Ensure proper file system permissions
2. **Sensitive Data**: Be cautious with sensitive data in cache files
3. **Cache Directory**: Use dedicated directories to avoid conflicts
4. **Cleanup**: Regular cleanup to prevent disk space issues

## Troubleshooting

### Common Issues

1. **Permission Denied**: Check directory permissions
2. **Disk Full**: Monitor disk space usage
3. **Slow Performance**: Consider faster storage or memory cache
4. **Cache Misses**: Check TTL settings and cleanup intervals

### Debug Logging

Enable debug logging to troubleshoot issues:

```go
// Cache operations are logged through the logger package
// Check logs for cache operation details
```

## Integration with Other Caches

File cache works seamlessly with other cache types in failover configurations:

```go
// Recommended production setup
failoverConfig := orm.CacheConfig{
    Type: orm.CacheTypeFailover,
    FallbackOrder: []orm.CacheType{
        orm.CacheTypeRedis,     // Primary: Redis (fast)
        orm.CacheTypeFile,      // Secondary: File (persistent)
        orm.CacheTypeMemory,    // Tertiary: Memory (last resort)
    },
    RedisConfig: &redis.Config{...},
    FileConfig: &orm.FileCacheConfig{...},
}
```

This provides both performance and persistence benefits.
