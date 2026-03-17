package main

import (
	"context"
	"fmt"
	"time"

	"alldev-gin-rpc/pkg/cache/failover"
	"alldev-gin-rpc/pkg/db/orm"
)

func main() {
	fmt.Println("=== Failover Cache Examples ===\n")

	// Example 1: Using the new failover package directly
	fmt.Println("1. Direct Failover Package Usage:")
	
	// Create a file-based failover cache with custom suffix
	config := failover.FileStorageConfig{
		Directory:       "/tmp/my_failover_cache",
		FileSuffix:      ".my_cache",
		MaxSize:         50 * 1024 * 1024, // 50MB
		MaxFiles:        5000,
		CleanupInterval: 5 * time.Minute,
	}
	
	primary, err := failover.NewFileStorage(config)
	if err != nil {
		fmt.Printf("Error creating file storage: %v\n", err)
		return
	}
	
	secondary := failover.NewMemoryStorage()
	
	failoverConfig := failover.Config{
		MaxRetries:          3,
		RetryDelay:          500 * time.Millisecond,
		HealthCheckInterval: 15 * time.Second,
	}
	
	failoverCache := failover.NewFailoverCache(primary, secondary, nil, failoverConfig)
	defer failoverCache.Close()
	
	// Test the failover cache
	failoverCache.Set(context.Background(), "test_key", "test_value", 10*time.Minute)
	if value, found := failoverCache.Get(context.Background(), "test_key"); found {
		fmt.Printf("Retrieved from failover: %v\n", value)
	}
	
	// Check storage status
	status := failoverCache.GetStorageStatus()
	fmt.Printf("Storage status: %v\n", status)
	
	// Example 2: Using ORM-compatible adapter
	fmt.Println("\n2. ORM-Compatible Adapter:")
	
	// Create an ORM-compatible failover cache
	ormCache, err := failover.CreateFileORMAdapter("/tmp/orm_failover", ".orm_cache")
	if err != nil {
		fmt.Printf("Error creating ORM adapter: %v\n", err)
		return
	}
	defer ormCache.Close()
	
	// Use it like any other ORM cache
	ormCache.Set("orm_key", map[string]interface{}{
		"user": "john",
		"age":  30,
	}, 15*time.Minute)
	
	if value, found := ormCache.Get("orm_key"); found {
		fmt.Printf("Retrieved from ORM adapter: %v\n", value)
	}
	
	// Example 3: Using convenience functions
	fmt.Println("\n3. Convenience Functions:")
	
	// Memory-primary, file-secondary failover
	memoryFileCache, err := failover.NewMemoryFileFailover("/tmp/memory_file", ".backup")
	if err != nil {
		fmt.Printf("Error creating memory-file failover: %v\n", err)
		return
	}
	defer memoryFileCache.Close()
	
	memoryFileCache.Set("backup_key", "backup_value", 20*time.Minute)
	if value, found := memoryFileCache.Get("backup_key"); found {
		fmt.Printf("Retrieved from memory-file failover: %v\n", value)
	}
	
	// Example 4: Using with ORM package (integration)
	fmt.Println("\n4. ORM Package Integration:")
	
	// Create failover cache using ORM package
	ormFailover, err := orm.NewMemoryFileFailoverCache("/tmp/orm_integration", ".integration")
	if err != nil {
		fmt.Printf("Error creating ORM failover: %v\n", err)
		return
	}
	defer ormFailover.Close()
	
	// Use with ORM's cached database
	// Note: This would require a real database connection in practice
	// cachedDB := orm.NewCachedDB(db, ormFailover)
	
	ormFailover.Set("integration_test", []string{"item1", "item2", "item3"}, 25*time.Minute)
	if value, found := ormFailover.Get("integration_test"); found {
		fmt.Printf("Retrieved from ORM integration: %v\n", value)
	}
	
	// Example 5: Custom configuration
	fmt.Println("\n5. Custom Configuration:")
	
	// Create a custom failover with specific settings
	customConfig := failover.Config{
		StorageType:         failover.StorageTypeFile,
		StoragePath:         "/tmp/custom_failover",
		FileSuffix:          ".custom",
		MaxRetries:          5,
		RetryDelay:          2 * time.Second,
		HealthCheckInterval: 1 * time.Minute,
		FallbackOrder:       []string{"file", "memory"},
	}
	
	customFailover, err := failover.NewFailoverCacheFromConfig(customConfig)
	if err != nil {
		fmt.Printf("Error creating custom failover: %v\n", err)
		return
	}
	defer customFailover.Close()
	
	customFailover.Set(context.Background(), "custom_key", "custom_value", 30*time.Minute)
	if value, found := customFailover.Get(context.Background(), "custom_key"); found {
		fmt.Printf("Retrieved from custom failover: %v\n", value)
	}
	
	// Example 6: Factory pattern
	fmt.Println("\n6. Factory Pattern Usage:")
	
	factory := failover.NewFactory()
	
	// Create different types of failover caches
	fileFailover, err := factory.CreateFileFailover("/tmp/factory_file", ".factory")
	if err != nil {
		fmt.Printf("Error creating factory file failover: %v\n", err)
		return
	}
	defer fileFailover.Close()
	
	fileFailover.Set(context.Background(), "factory_key", "factory_value", 35*time.Minute)
	if value, found := fileFailover.Get(context.Background(), "factory_key"); found {
		fmt.Printf("Retrieved from factory failover: %v\n", value)
	}
	
	// Example 7: Advanced storage configuration
	fmt.Println("\n7. Advanced Storage Configuration:")
	
	advancedConfig := failover.FileStorageConfig{
		Directory:       "/tmp/advanced_cache",
		FileSuffix:      ".advanced",
		MaxSize:         200 * 1024 * 1024, // 200MB
		MaxFiles:        20000,
		CleanupInterval: 30 * time.Minute,
	}
	
	advancedFailover, err := failover.NewCustomFileFailover(
		advancedConfig.Directory,
		advancedConfig.FileSuffix,
		advancedConfig.MaxSize,
		advancedConfig.MaxFiles,
		advancedConfig.CleanupInterval,
	)
	if err != nil {
		fmt.Printf("Error creating advanced failover: %v\n", err)
		return
	}
	defer advancedFailover.Close()
	
	// Store large data
	largeData := make([]string, 1000)
	for i := range largeData {
		largeData[i] = fmt.Sprintf("data_item_%d", i)
	}
	
	advancedFailover.Set(context.Background(), "large_data", largeData, 40*time.Minute)
	if value, found := advancedFailover.Get(context.Background(), "large_data"); found {
		if data, ok := value.([]string); ok {
			fmt.Printf("Retrieved large data: %d items, first item: %s\n", len(data), data[0])
		}
	}
	
	fmt.Println("\n=== All Failover Examples Completed Successfully! ===")
}
