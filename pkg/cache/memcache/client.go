// Package memcache provides a Memcached client with connection pooling
// and common caching operations.
package memcache

import (
	"context"
	"fmt"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

// Config holds Memcached connection configuration.
type Config struct {
	Hosts        []string      `yaml:"hosts" json:"hosts"`             // List of memcached servers
	MaxIdleConns int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`         // Connection timeout
}

// DefaultConfig returns default Memcached configuration.
func DefaultConfig() Config {
	return Config{
		Hosts:        []string{"localhost:11211"},
		MaxIdleConns: 2,
		Timeout:      5 * time.Second,
	}
}

// Client wraps memcache.Client with additional functionality.
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
	client.MaxIdleConns = config.MaxIdleConns
	client.Timeout = config.Timeout

	// Test connection
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping memcached: %w", err)
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

// Get retrieves an item from the cache.
func (c *Client) Get(ctx context.Context, key string) (*memcache.Item, error) {
	return c.client.Get(key)
}

// GetMulti retrieves multiple items from the cache.
func (c *Client) GetMulti(ctx context.Context, keys []string) (map[string]*memcache.Item, error) {
	return c.client.GetMulti(keys)
}

// Set stores an item in the cache.
func (c *Client) Set(ctx context.Context, item *memcache.Item) error {
	return c.client.Set(item)
}

// Add adds an item to the cache only if it doesn't exist.
func (c *Client) Add(ctx context.Context, item *memcache.Item) error {
	return c.client.Add(item)
}

// Replace replaces an item in the cache only if it exists.
func (c *Client) Replace(ctx context.Context, item *memcache.Item) error {
	return c.client.Replace(item)
}

// Delete removes an item from the cache.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.client.Delete(key)
}

// DeleteAll clears all items from the cache.
func (c *Client) DeleteAll(ctx context.Context) error {
	return c.client.DeleteAll()
}

// Increment atomically increments a numeric value.
func (c *Client) Increment(ctx context.Context, key string, delta uint64) (uint64, error) {
	return c.client.Increment(key, delta)
}

// Decrement atomically decrements a numeric value.
func (c *Client) Decrement(ctx context.Context, key string, delta uint64) (uint64, error) {
	return c.client.Decrement(key, delta)
}

// Touch updates the expiration time of an item.
func (c *Client) Touch(ctx context.Context, key string, seconds int32) error {
	return c.client.Touch(key, seconds)
}

// Ping checks the connection health.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping()
}

// Close closes the client connection.
func (c *Client) Close() error {
	// memcache.Client doesn't have explicit close
	return nil
}

// GetClient returns the underlying memcache client.
func (c *Client) GetClient() *memcache.Client {
	return c.client
}
