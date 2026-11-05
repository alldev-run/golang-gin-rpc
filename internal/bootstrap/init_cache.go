package bootstrap

import (
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
)

// InitializeCache initializes cache
func (b *Bootstrap) InitializeCache() error {
	if b.config.Redis.Host == "" {
		logger.Info("No cache configuration found, skipping cache initialization")
		return nil
	}

	cacheCfg := redis.DefaultConfig()
	cacheCfg.Host = b.config.Redis.Host
	cacheCfg.Port = b.config.Redis.Port
	cacheCfg.Password = b.config.Redis.Password
	cacheCfg.Database = b.config.Redis.Database
	cacheCfg.PoolSize = b.config.Redis.PoolSize
	cacheCfg.MinIdleConns = b.config.Redis.MinIdleConns

	cacheInstance, err := cache.NewRedisCache(cacheCfg)
	if err != nil {
		return fmt.Errorf("failed to create redis cache: %w", err)
	}

	b.cache = cacheInstance
	b.setDependency(DepCache, cacheInstance)
	logger.Info("Redis cache initialized")

	return nil
}
