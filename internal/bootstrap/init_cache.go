package bootstrap

import (
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
)

// InitializeCache initializes cache
func (b *Bootstrap) InitializeCache() error {
	if b.pluginConfig.Redis.Host == "" {
		logger.Info("No cache configuration found, skipping cache initialization")
		b.markComponentSkipped("cache")
		return nil
	}

	cacheCfg := redis.DefaultConfig()
	cacheCfg.Host = b.pluginConfig.Redis.Host
	cacheCfg.Port = b.pluginConfig.Redis.Port
	cacheCfg.Password = b.pluginConfig.Redis.Password
	cacheCfg.Database = b.pluginConfig.Redis.Database
	cacheCfg.PoolSize = b.pluginConfig.Redis.PoolSize
	cacheCfg.MinIdleConns = b.pluginConfig.Redis.MinIdleConns

	cacheInstance, err := cache.NewRedisCache(cacheCfg)
	if err != nil {
		return fmt.Errorf("failed to create redis cache: %w", err)
	}

	b.cache = cacheInstance
	b.setDependency(DepCache, cacheInstance)
	logger.Info("Redis cache initialized")

	return nil
}
