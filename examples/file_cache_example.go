package main

import (
	"fmt"
	"time"

	"golang-gin-rpc/pkg/db/orm"
)

func main() {
	// Example 1: Create a default file cache
	fmt.Println("=== Default File Cache ===")
	cache, err := orm.NewDefaultFileCache("/tmp/my_cache")
	if err != nil {
		fmt.Printf("Error creating file cache: %v\n", err)
		return
	}

	// Test basic operations
	cache.Set("key1", "value1", 5*time.Minute)
	cache.Set("key2", map[string]interface{}{"name": "test", "age": 25}, 10*time.Minute)

	if value, found := cache.Get("key1"); found {
		fmt.Printf("Retrieved key1: %v\n", value)
	}

	if value, found := cache.Get("key2"); found {
		fmt.Printf("Retrieved key2: %v\n", value)
	}

	fmt.Println("All keys:", cache.Keys())

	// Example 2: Create file cache with custom configuration
	fmt.Println("\n=== Custom File Cache ===")
	config := orm.FileCacheConfig{
		Directory:       "/tmp/custom_cache",
		MaxSize:         50 * 1024 * 1024, // 50MB
		MaxFiles:        5000,
		CleanupInterval: 5 * time.Minute,
	}

	customCache, err := orm.NewFileCache(config)
	if err != nil {
		fmt.Printf("Error creating custom file cache: %v\n", err)
		return
	}

	customCache.Set("custom_key", []int{1, 2, 3, 4, 5}, 15*time.Minute)
	if value, found := customCache.Get("custom_key"); found {
		fmt.Printf("Retrieved custom_key: %v\n", value)
	}

	// Example 3: Use file cache in configuration
	fmt.Println("\n=== File Cache in Configuration ===")
	cacheConfig := orm.CacheConfig{
		Type: orm.CacheTypeFile,
		FileConfig: &orm.FileCacheConfig{
			Directory:       "/tmp/config_cache",
			MaxSize:         20 * 1024 * 1024, // 20MB
			MaxFiles:        1000,
			CleanupInterval: 3 * time.Minute,
		},
		KeyPrefix: "myapp:",
	}

	configCache, err := orm.NewCacheFromConfig(cacheConfig)
	if err != nil {
		fmt.Printf("Error creating cache from config: %v\n", err)
		return
	}

	configCache.Set("config_key", "config_value", 8*time.Minute)
	if value, found := configCache.Get("config_key"); found {
		fmt.Printf("Retrieved config_key: %v\n", value)
	}

	// Example 4: Failover cache with file cache
	fmt.Println("\n=== Failover Cache with File Cache ===")
	failoverConfig := orm.CacheConfig{
		Type: orm.CacheTypeFailover,
		FallbackOrder: []orm.CacheType{
			orm.CacheTypeFile,    // Primary: File cache
			orm.CacheTypeMemory,   // Secondary: Memory cache
		},
		FileConfig: &orm.FileCacheConfig{
			Directory:       "/tmp/failover_cache",
			MaxSize:         10 * 1024 * 1024, // 10MB
			MaxFiles:        1000,
			CleanupInterval: 2 * time.Minute,
		},
	}

	failoverCache, err := orm.NewCacheFromConfig(failoverConfig)
	if err != nil {
		fmt.Printf("Error creating failover cache: %v\n", err)
		return
	}

	failoverCache.Set("failover_key", "failover_value", 12*time.Minute)
	if value, found := failoverCache.Get("failover_key"); found {
		fmt.Printf("Retrieved failover_key: %v\n", value)
	}

	fmt.Println("\nFile cache examples completed successfully!")
}
