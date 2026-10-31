// Package memcache provides a Memcached client with connection pooling
// and common caching operations.
package memcache

import (
	"context"
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"
)


// Item represents an item to be stored in memcache.
type Item struct {
	Key        string
	Value      []byte
	Flags      uint32
	Expiration int32 // seconds
}

// Client provides a memcache client implementation using gomemcache.
type Client struct {
	client *memcache.Client
	config Config
}

// New creates a new Memcached client from config.
func New(config Config) (*Client, error) {
	if len(config.Hosts) == 0 {
		config.Hosts = DefaultConfig().Hosts
	}

	client := memcache.New(config.Hosts...)
	client.Timeout = config.Timeout
	client.MaxIdleConns = config.MaxIdleConns

	return &Client{
		client: client,
		config: config,
	}, nil
}

// Get retrieves an item from the cache.
func (c *Client) Get(ctx context.Context, key string) (*Item, error) {
	item, err := c.client.Get(key)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return nil, fmt.Errorf("key not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return &Item{
		Key:        item.Key,
		Value:      item.Value,
		Flags:      item.Flags,
		Expiration: item.Expiration,
	}, nil
}

// GetMulti retrieves multiple items from the cache.
func (c *Client) GetMulti(ctx context.Context, keys []string) (map[string]*Item, error) {
	items, err := c.client.GetMulti(keys)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple keys: %w", err)
	}

	result := make(map[string]*Item, len(items))
	for key, item := range items {
		result[key] = &Item{
			Key:        item.Key,
			Value:      item.Value,
			Flags:      item.Flags,
			Expiration: item.Expiration,
		}
	}

	return result, nil
}

// Set stores an item in the cache.
func (c *Client) Set(ctx context.Context, item *Item) error {
	mcItem := &memcache.Item{
		Key:        item.Key,
		Value:      item.Value,
		Flags:      item.Flags,
		Expiration: item.Expiration,
	}

	if err := c.client.Set(mcItem); err != nil {
		return fmt.Errorf("failed to set key %s: %w", item.Key, err)
	}

	return nil
}

// Add adds an item to the cache only if it doesn't exist.
func (c *Client) Add(ctx context.Context, item *Item) error {
	mcItem := &memcache.Item{
		Key:        item.Key,
		Value:      item.Value,
		Flags:      item.Flags,
		Expiration: item.Expiration,
	}

	if err := c.client.Add(mcItem); err != nil {
		if err == memcache.ErrNotStored {
			return fmt.Errorf("key %s already exists", item.Key)
		}
		return fmt.Errorf("failed to add key %s: %w", item.Key, err)
	}

	return nil
}

// Replace replaces an item in the cache only if it exists.
func (c *Client) Replace(ctx context.Context, item *Item) error {
	mcItem := &memcache.Item{
		Key:        item.Key,
		Value:      item.Value,
		Flags:      item.Flags,
		Expiration: item.Expiration,
	}

	if err := c.client.Replace(mcItem); err != nil {
		if err == memcache.ErrNotStored {
			return fmt.Errorf("key %s does not exist", item.Key)
		}
		return fmt.Errorf("failed to replace key %s: %w", item.Key, err)
	}

	return nil
}

// Delete removes an item from the cache.
func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.client.Delete(key); err != nil {
		if err == memcache.ErrCacheMiss {
			return fmt.Errorf("key %s not found", key)
		}
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	return nil
}

// DeleteAll clears all items from the cache (flush_all).
func (c *Client) DeleteAll(ctx context.Context) error {
	if err := c.client.DeleteAll(); err != nil {
		return fmt.Errorf("failed to flush all: %w", err)
	}

	return nil
}

// Increment atomically increments a numeric value.
func (c *Client) Increment(ctx context.Context, key string, delta uint64) (uint64, error) {
	newValue, err := c.client.Increment(key, delta)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return 0, fmt.Errorf("key %s not found", key)
		}
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}

	return newValue, nil
}

// Decrement atomically decrements a numeric value.
func (c *Client) Decrement(ctx context.Context, key string, delta uint64) (uint64, error) {
	newValue, err := c.client.Decrement(key, delta)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return 0, fmt.Errorf("key %s not found", key)
		}
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}

	return newValue, nil
}

// Touch updates the expiration time of an item.
func (c *Client) Touch(ctx context.Context, key string, seconds int32) error {
	// gomemcache Touch expects int32 seconds directly
	if err := c.client.Touch(key, seconds); err != nil {
		if err == memcache.ErrCacheMiss {
			return fmt.Errorf("key %s not found", key)
		}
		return fmt.Errorf("failed to touch key %s: %w", key, err)
	}

	return nil
}

// Ping checks the connection health.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.client.Ping(); err != nil {
		return fmt.Errorf("failed to ping memcache: %w", err)
	}

	return nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	// No explicit cleanup needed for this simple implementation
	return nil
}
