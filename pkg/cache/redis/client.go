// Package redis provides a Redis client with connection pooling,
// common operations, and distributed locking support.
package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)


// Client wraps redis.Client with additional functionality and multi-instance support.
type Client struct {
	config  Config
	rdb     *redis.Client        // single instance or sentinel client
	cluster *redis.ClusterClient // cluster mode client
	nodes   []*redis.Client      // master-slave nodes
	mu      sync.RWMutex         // protects nodes and config
}

// New creates a new Redis client based on config mode.
func New(config Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch config.Mode {
	case ModeSingle, "":
		return newSingleClient(ctx, config)
	case ModeCluster:
		return newClusterClient(ctx, config)
	case ModeSentinel:
		return newSentinelClient(ctx, config)
	case ModeMasterSlave:
		return newMasterSlaveClient(ctx, config)
	case ModeMulti:
		return newMultiClient(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported redis mode: %s", config.Mode)
	}
}

// newSingleClient creates a single Redis instance client.
func newSingleClient(ctx context.Context, config Config) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     config.Password,
		DB:           config.Database,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Client{
		rdb:    rdb,
		config: config,
	}, nil
}

// newClusterClient creates a Redis Cluster client.
func newClusterClient(ctx context.Context, config Config) (*Client, error) {
	addrs := make([]string, len(config.Nodes))
	for i, node := range config.Nodes {
		addrs[i] = fmt.Sprintf("%s:%d", node.Host, node.Port)
	}

	cluster := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:          addrs,
		Password:       config.Password,
		MaxRedirects:   config.Cluster.MaxRedirects,
		ReadOnly:       config.Cluster.ReadOnly,
		RouteByLatency: config.Cluster.RouteByLatency,
		RouteRandomly:  config.Cluster.RouteRandomly,
		PoolSize:       config.PoolSize,
		MinIdleConns:   config.MinIdleConns,
		MaxRetries:     config.MaxRetries,
		DialTimeout:    config.DialTimeout,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		PoolTimeout:    config.PoolTimeout,
	})

	if err := cluster.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis cluster: %w", err)
	}

	return &Client{
		cluster: cluster,
		config:  config,
	}, nil
}

// newSentinelClient creates a Redis Sentinel client for HA.
func newSentinelClient(ctx context.Context, config Config) (*Client, error) {
	rdb := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:       config.Sentinel.MasterName,
		SentinelAddrs:    config.Sentinel.SentinelAddrs,
		SentinelPassword: config.Sentinel.SentinelPassword,
		Password:         config.Password,
		DB:               config.Database,
		PoolSize:         config.PoolSize,
		MinIdleConns:     config.MinIdleConns,
		MaxRetries:       config.MaxRetries,
		DialTimeout:      config.DialTimeout,
		ReadTimeout:      config.ReadTimeout,
		WriteTimeout:     config.WriteTimeout,
		PoolTimeout:      config.PoolTimeout,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis sentinel: %w", err)
	}

	return &Client{
		rdb:    rdb,
		config: config,
	}, nil
}

// newMasterSlaveClient creates a master-slave client with read/write splitting.
func newMasterSlaveClient(ctx context.Context, config Config) (*Client, error) {
	nodes := make([]*redis.Client, len(config.Nodes))
	for i, node := range config.Nodes {
		addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
		client := redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     node.Password,
			DB:           node.Database,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxRetries:   config.MaxRetries,
			DialTimeout:  config.DialTimeout,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolTimeout:  config.PoolTimeout,
		})
		nodes[i] = client
	}

	// Test all connections
	for i, client := range nodes {
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to ping redis node %d: %w", i, err)
		}
	}

	return &Client{
		nodes:  nodes,
		config: config,
	}, nil
}

// newMultiClient creates a multi-instance client with sharding support.
func newMultiClient(ctx context.Context, config Config) (*Client, error) {
	nodes := make([]*redis.Client, len(config.Nodes))
	for i, node := range config.Nodes {
		addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
		client := redis.NewClient(&redis.Options{
			Addr:         addr,
			Password:     node.Password,
			DB:           node.Database,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			MaxRetries:   config.MaxRetries,
			DialTimeout:  config.DialTimeout,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolTimeout:  config.PoolTimeout,
		})
		nodes[i] = client
	}

	// Test all connections
	for i, client := range nodes {
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to ping redis node %d: %w", i, err)
		}
	}

	return &Client{
		nodes:  nodes,
		config: config,
	}, nil
}

// getNodeForKey returns the appropriate node index for a given key using sharding strategy.
// This method is not thread-safe; caller must hold c.mu.RLock.
func (c *Client) getNodeForKey(key string) int {
	if c.config.Mode != ModeMulti || len(c.nodes) == 0 {
		return 0
	}

	strategy := c.config.Multi.ShardingStrategy
	
	// Check key prefix routes first
	if len(c.config.Multi.KeyPrefixRoutes) > 0 {
		for prefix, nodeIdx := range c.config.Multi.KeyPrefixRoutes {
			if strings.HasPrefix(key, prefix) {
				if nodeIdx >= 0 && nodeIdx < len(c.nodes) {
					return nodeIdx
				}
				// Fall back to default if index out of bounds
				if defaultIdx := c.config.Multi.DefaultNode; defaultIdx >= 0 && defaultIdx < len(c.nodes) {
					return defaultIdx
				}
				return 0
			}
		}
	}
	
	// Use sharding strategy
	switch strategy {
	case "hash":
		// Simple hash-based sharding
		hash := 0
		for _, ch := range key {
			hash = (hash*31 + int(ch)) % len(c.nodes)
		}
		return hash
	case "range":
		// For range-based, use default (should be configured with specific logic)
		return c.getSafeDefaultNode()
	default:
		// Default to configured default node
		return c.getSafeDefaultNode()
	}
}

// getSafeDefaultNode returns the default node index with bounds checking.
// This method is not thread-safe; caller must hold c.mu.RLock.
func (c *Client) getSafeDefaultNode() int {
	if len(c.nodes) == 0 {
		return 0
	}
	defaultIdx := c.config.Multi.DefaultNode
	if defaultIdx >= 0 && defaultIdx < len(c.nodes) {
		return defaultIdx
	}
	return 0
}

// getClientForKey returns the appropriate client for a given key.
// Returns the first available node if no valid client is found for the key.
func (c *Client) getClientForKey(key string) redis.Cmdable {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.rdb != nil {
		return c.rdb
	}
	if c.cluster != nil {
		return c.cluster
	}
	if len(c.nodes) > 0 {
		if c.config.Mode == ModeMulti {
			idx := c.getNodeForKey(key)
			// Bounds check to prevent panic
			if idx >= 0 && idx < len(c.nodes) && c.nodes[idx] != nil {
				return c.nodes[idx]
			}
			// Fall back to first available node
			return c.getFirstAvailableNode()
		}
		if c.nodes[0] != nil {
			return c.nodes[0]
		}
		return c.getFirstAvailableNode()
	}
	// Return nil if no clients available - caller should handle this
	return nil
}

// getFirstAvailableNode returns the first non-nil node client.
// This method is not thread-safe; caller must hold c.mu.RLock.
func (c *Client) getFirstAvailableNode() redis.Cmdable {
	for _, node := range c.nodes {
		if node != nil {
			return node
		}
	}
	return nil
}

// RDB returns the underlying redis.Client instance (for single/sentinel mode).
func (c *Client) RDB() *redis.Client {
	if c.rdb != nil {
		return c.rdb
	}
	// For cluster mode, return nil or create a proxy
	return nil
}

// Cluster returns the underlying redis.ClusterClient instance (for cluster mode).
func (c *Client) Cluster() *redis.ClusterClient {
	return c.cluster
}

// Close closes all Redis connections.
func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	if c.cluster != nil {
		return c.cluster.Close()
	}
	for _, node := range c.nodes {
		if err := node.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Ping checks the Redis connection health.
func (c *Client) Ping(ctx context.Context) error {
	if c.rdb != nil {
		return c.rdb.Ping(ctx).Err()
	}
	if c.cluster != nil {
		return c.cluster.Ping(ctx).Err()
	}
	// For master-slave, ping all nodes
	for i, node := range c.nodes {
		if err := node.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("failed to ping node %d: %w", i, err)
		}
	}
	return nil
}

// getClient returns the appropriate client for the operation.
func (c *Client) getClient(write bool) redis.Cmdable {
	if c.rdb != nil {
		return c.rdb
	}
	if c.cluster != nil {
		return c.cluster
	}
	// For master-slave, select based on operation type
	if len(c.nodes) > 0 {
		if write {
			// Find master node
			for _, node := range c.nodes {
				if node != nil {
					return node
				}
			}
		}
		// For reads, use first available node (could implement load balancing)
		return c.nodes[0]
	}
	return nil
}

// ==================== String Operations ====================

// Get retrieves a string value by key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.getClientForKey(key).Get(ctx, key).Result()
}

// Set stores a string value with optional expiration.
func (c *Client) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return c.getClientForKey(key).Set(ctx, key, value, expiration).Err()
}

// SetNX sets value only if key doesn't exist (SET if Not eXists).
func (c *Client) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	return c.getClientForKey(key).SetNX(ctx, key, value, expiration).Result()
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	// For multiple keys, use the client for the first key
	if len(keys) > 0 {
		return c.getClientForKey(keys[0]).Del(ctx, keys...).Err()
	}
	return nil
}

// Exists checks if keys exist.
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) > 0 {
		return c.getClientForKey(keys[0]).Exists(ctx, keys...).Result()
	}
	return 0, nil
}

// Expire sets expiration on a key.
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return c.getClientForKey(key).Expire(ctx, key, expiration).Result()
}

// TTL returns remaining time to live of a key.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.getClientForKey(key).TTL(ctx, key).Result()
}

// ==================== Hash Operations ====================

// HGet retrieves a field value from a hash.
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.getClientForKey(key).HGet(ctx, key, field).Result()
}

// HSet sets field-value pairs in a hash.
func (c *Client) HSet(ctx context.Context, key string, values ...any) error {
	return c.getClientForKey(key).HSet(ctx, key, values...).Err()
}

// HGetAll retrieves all fields and values from a hash.
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.getClientForKey(key).HGetAll(ctx, key).Result()
}

// HDel deletes fields from a hash.
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	return c.getClientForKey(key).HDel(ctx, key, fields...).Err()
}

// ==================== List Operations ====================

// LPush pushes values to the left of a list.
func (c *Client) LPush(ctx context.Context, key string, values ...any) error {
	return c.getClientForKey(key).LPush(ctx, key, values...).Err()
}

// RPush pushes values to the right of a list.
func (c *Client) RPush(ctx context.Context, key string, values ...any) error {
	return c.getClientForKey(key).RPush(ctx, key, values...).Err()
}

// LPop pops a value from the left of a list.
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	return c.getClientForKey(key).LPop(ctx, key).Result()
}

// RPop pops a value from the right of a list.
func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	return c.getClientForKey(key).RPop(ctx, key).Result()
}

// LRange returns a range of elements from a list.
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.getClientForKey(key).LRange(ctx, key, start, stop).Result()
}

// LLen returns the length of a list.
func (c *Client) LLen(ctx context.Context, key string) (int64, error) {
	return c.getClientForKey(key).LLen(ctx, key).Result()
}

// ==================== Set Operations ====================

// SAdd adds members to a set.
func (c *Client) SAdd(ctx context.Context, key string, members ...any) error {
	return c.getClientForKey(key).SAdd(ctx, key, members...).Err()
}

// SRem removes members from a set.
func (c *Client) SRem(ctx context.Context, key string, members ...any) error {
	return c.getClientForKey(key).SRem(ctx, key, members...).Err()
}

// SIsMember checks if a member exists in a set.
func (c *Client) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return c.getClientForKey(key).SIsMember(ctx, key, member).Result()
}

// SMembers returns all members of a set.
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.getClientForKey(key).SMembers(ctx, key).Result()
}

// ==================== Distributed Lock ====================

// Lock attempts to acquire a distributed lock.
func (c *Client) Lock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return c.getClientForKey(key).SetNX(ctx, key, "1", expiration).Result()
}

// Unlock releases a distributed lock.
func (c *Client) Unlock(ctx context.Context, key string) error {
	return c.getClientForKey(key).Del(ctx, key).Err()
}

// ==================== Pub/Sub ====================

// Publish publishes a message to a channel.
func (c *Client) Publish(ctx context.Context, channel string, message any) error {
	return c.getClientForKey(channel).Publish(ctx, channel, message).Err()
}

// Subscribe subscribes to one or more channels.
// Note: For cluster mode, this uses the first available node.
func (c *Client) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	if c.rdb != nil {
		return c.rdb.Subscribe(ctx, channels...)
	}
	if c.cluster != nil {
		// For cluster, subscribe to the first node
		return c.cluster.Subscribe(ctx, channels...)
	}
	if len(c.nodes) > 0 {
		return c.nodes[0].Subscribe(ctx, channels...)
	}
	return nil
}

// ==================== Pipeline ====================

// Pipeline returns a new pipeline for batch operations.
// Note: For cluster/master-slave mode, this uses the write client.
func (c *Client) Pipeline() redis.Pipeliner {
	return c.getClient(true).Pipeline()
}

// TxPipeline returns a new transactional pipeline.
// Note: For cluster/master-slave mode, this uses the write client.
func (c *Client) TxPipeline() redis.Pipeliner {
	return c.getClient(true).TxPipeline()
}

// Pipelined executes a function within a pipeline and returns results.
// This is useful for batch operations where you need the results.
func (c *Client) Pipelined(ctx context.Context, fn func(p redis.Pipeliner) error) ([]redis.Cmder, error) {
	pipe := c.Pipeline()
	if err := fn(pipe); err != nil {
		return nil, err
	}
	return pipe.Exec(ctx)
}

// TxPipelined executes a function within a transactional pipeline and returns results.
// All commands in the function are executed atomically (MULTI/EXEC).
func (c *Client) TxPipelined(ctx context.Context, fn func(p redis.Pipeliner) error) ([]redis.Cmder, error) {
	pipe := c.TxPipeline()
	if err := fn(pipe); err != nil {
		return nil, err
	}
	return pipe.Exec(ctx)
}

// ==================== Lua Scripting ====================

// Eval executes a Lua script with keys and arguments.
func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return c.getClientForKey(keys[0]).Eval(ctx, script, keys, args...).Result()
}

// EvalSha executes a cached Lua script by its SHA1 digest.
func (c *Client) EvalSha(ctx context.Context, sha1 string, keys []string, args ...any) (any, error) {
	return c.getClientForKey(keys[0]).EvalSha(ctx, sha1, keys, args...).Result()
}

// ScriptLoad loads a script into the script cache.
func (c *Client) ScriptLoad(ctx context.Context, script string, key string) (string, error) {
	return c.getClientForKey(key).ScriptLoad(ctx, script).Result()
}

// ScriptExists checks if scripts exist in the script cache.
func (c *Client) ScriptExists(ctx context.Context, key string, hashes ...string) ([]bool, error) {
	return c.getClientForKey(key).ScriptExists(ctx, hashes...).Result()
}

// ==================== Watch Transactions ====================

// Watch watches keys for changes and executes a transaction function.
// This uses optimistic locking - if any watched key changes, txFunc will be retried.
func (c *Client) Watch(ctx context.Context, key string, txFunc func(tx *redis.Tx) error) error {
	if c.rdb != nil {
		return c.rdb.Watch(ctx, txFunc, key)
	}
	// For other modes, use the first node for the key
	client := c.getClientForKey(key)
	if rdb, ok := client.(*redis.Client); ok {
		return rdb.Watch(ctx, txFunc, key)
	}
	return fmt.Errorf("watch not supported in cluster mode")
}

// Transaction executes a function within an atomic transaction with automatic retries.
// It combines Watch + TxPipeline for optimistic locking scenarios.
// Example: decrementing a counter atomically
//
//	err := c.Transaction(ctx, "counter", 3, func(tx *redis.Tx, get func(key string) *redis.StringCmd) error {
//	    val, _ := get("counter").Int()
//	    tx.Set(ctx, "counter", val-1, 0)
//	    return nil
//	})
func (c *Client) Transaction(ctx context.Context, key string, maxRetries int, txFunc func(tx *redis.Tx, get func(string) *redis.StringCmd) error) error {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	watchFunc := func(tx *redis.Tx) error {
		// Create a getter function for reading current values
		getFunc := func(k string) *redis.StringCmd {
			return tx.Get(ctx, k)
		}

		// Execute user transaction function
		if err := txFunc(tx, getFunc); err != nil {
			return err
		}

		return nil
	}

	lastErr := fmt.Errorf("transaction failed after %d retries", maxRetries)
	for i := 0; i < maxRetries; i++ {
		err := c.Watch(ctx, key, watchFunc)
		if err == nil {
			return nil // Success
		}
		// Check if it's a watch error (optimistic lock conflict)
		if err == redis.TxFailedErr {
			lastErr = err
			continue // Retry
		}
		return err // Other errors, return immediately
	}

	return lastErr
}
