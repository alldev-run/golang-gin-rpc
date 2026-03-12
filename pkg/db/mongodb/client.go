// Package mongodb provides a MongoDB client with connection configuration
// and common database operations.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Config holds MongoDB connection configuration.
type Config struct {
	URI             string        `yaml:"uri" json:"uri"`                           // MongoDB connection URI
	Database        string        `yaml:"database" json:"database"`                 // Default database name
	ConnectTimeout  time.Duration `yaml:"connect_timeout" json:"connect_timeout"`   // Connection timeout
	MaxPoolSize     uint64        `yaml:"max_pool_size" json:"max_pool_size"`       // Max connection pool size
	MinPoolSize     uint64        `yaml:"min_pool_size" json:"min_pool_size"`       // Min connection pool size
	MaxConnIdleTime time.Duration `yaml:"max_conn_idle_time" json:"max_conn_idle_time"` // Max connection idle time
}

// DefaultConfig returns default MongoDB configuration.
func DefaultConfig() Config {
	return Config{
		URI:             "mongodb://localhost:27017",
		Database:        "default",
		ConnectTimeout:  10 * time.Second,
		MaxPoolSize:     100,
		MinPoolSize:     10,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// Client wraps mongo.Client with additional functionality.
type Client struct {
	client   *mongo.Client
	database *mongo.Database
	config   Config
}

// New creates a new MongoDB client from config.
func New(config Config) (*Client, error) {
	if config.URI == "" {
		config.URI = DefaultConfig().URI
	}
	if config.Database == "" {
		config.Database = DefaultConfig().Database
	}

	clientOptions := options.Client().ApplyURI(config.URI)

	if config.MaxPoolSize > 0 {
		clientOptions.SetMaxPoolSize(config.MaxPoolSize)
	}
	if config.MinPoolSize > 0 {
		clientOptions.SetMinPoolSize(config.MinPoolSize)
	}
	if config.MaxConnIdleTime > 0 {
		clientOptions.SetMaxConnIdleTime(config.MaxConnIdleTime)
	}

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	// Test connection with context
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	return &Client{
		client:   client,
		database: client.Database(config.Database),
		config:   config,
	}, nil
}

// Database returns the default database.
func (c *Client) Database() *mongo.Database {
	return c.database
}

// Collection returns a collection from the default database.
func (c *Client) Collection(name string) *mongo.Collection {
	return c.database.Collection(name)
}

// CollectionFromDB returns a collection from a specific database.
func (c *Client) CollectionFromDB(dbName, collName string) *mongo.Collection {
	return c.client.Database(dbName).Collection(collName)
}

// Ping checks the connection health.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, readpref.Primary())
}

// Close closes the client connection.
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// ListDatabases returns a list of databases.
func (c *Client) ListDatabases(ctx context.Context) (mongo.ListDatabasesResult, error) {
	return c.client.ListDatabases(ctx, struct{}{})
}

// ListCollections returns a list of collections in the default database.
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	return c.database.ListCollectionNames(ctx, struct{}{})
}

// CreateCollection creates a new collection.
func (c *Client) CreateCollection(ctx context.Context, name string) error {
	return c.database.CreateCollection(ctx, name)
}

// DropCollection drops a collection.
func (c *Client) DropCollection(ctx context.Context, name string) error {
	return c.database.Collection(name).Drop(ctx)
}

// Client returns the underlying mongo.Client.
func (c *Client) Client() *mongo.Client {
	return c.client
}
