package messaging

import (
	"fmt"
	"time"
)

// MessageType represents the type of messaging system
type MessageType string

const (
	MessageTypeRabbitMQ MessageType = "rabbitmq"
	MessageTypeKafka    MessageType = "kafka"
	MessageTypeNATS     MessageType = "nats"
	MessageTypeRedis    MessageType = "redis"
	MessageTypeMemory   MessageType = "memory"
)

// DeliveryMode represents the message delivery mode
type DeliveryMode string

const (
	DeliveryModeTransient DeliveryMode = "transient"
	DeliveryModePersistent DeliveryMode = "persistent"
)

// Config represents the configuration for a messaging client
type Config struct {
	// Type is the messaging system type
	Type MessageType `yaml:"type" json:"type"`
	
	// Host is the messaging server host
	Host string `yaml:"host" json:"host"`
	
	// Port is the messaging server port
	Port int `yaml:"port" json:"port"`
	
	// Username for authentication
	Username string `yaml:"username" json:"username"`
	
	// Password for authentication
	Password string `yaml:"password" json:"password"`
	
	// Database/VHost for the messaging system
	Database string `yaml:"database" json:"database"`
	
	// Timeout for connection and operations
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// ConnectionTimeout for establishing connections
	ConnectionTimeout time.Duration `yaml:"connection_timeout" json:"connection_timeout"`
	
	// ReadTimeout for read operations
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout"`
	
	// WriteTimeout for write operations
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
	
	// MaxRetries for failed operations
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
	
	// RetryDelay between retries
	RetryDelay time.Duration `yaml:"retry_delay" json:"retry_delay"`
	
	// PoolSize for connection pool
	PoolSize int `yaml:"pool_size" json:"pool_size"`
	
	// IdleTimeout for idle connections
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	
	// Heartbeat interval for keeping connections alive
	Heartbeat time.Duration `yaml:"heartbeat" json:"heartbeat"`
	
	// PrefetchCount for message prefetching
	PrefetchCount int `yaml:"prefetch_count" json:"prefetch_count"`
	
	// DefaultExchange for publishing messages
	DefaultExchange string `yaml:"default_exchange" json:"default_exchange"`
	
	// DefaultQueue for consuming messages
	DefaultQueue string `yaml:"default_queue" json:"default_queue"`
	
	// DefaultRoutingKey for routing messages
	DefaultRoutingKey string `yaml:"default_routing_key" json:"default_routing_key"`
	
	// DeliveryMode for messages
	DeliveryMode DeliveryMode `yaml:"delivery_mode" json:"delivery_mode"`
	
	// Priority for message priority
	Priority int `yaml:"priority" json:"priority"`
	
	// TTL for message time-to-live
	TTL time.Duration `yaml:"ttl" json:"ttl"`
	
	// Expiration for message expiration
	Expiration time.Duration `yaml:"expiration" json:"expiration"`
	
	// Compression for message compression
	Compression bool `yaml:"compression" json:"compression"`
	
	// Persistent connections
	Persistent bool `yaml:"persistent" json:"persistent"`
	
	// AutoReconnect for automatic reconnection
	AutoReconnect bool `yaml:"auto_reconnect" json:"auto_reconnect"`
	
	// ReconnectDelay for reconnection attempts
	ReconnectDelay time.Duration `yaml:"reconnect_delay" json:"reconnect_delay"`
	
	// MaxReconnectAttempts for reconnection
	MaxReconnectAttempts int `yaml:"max_reconnect_attempts" json:"max_reconnect_attempts"`
	
	// Additional options for the messaging system
	Options map[string]interface{} `yaml:"options" json:"options"`
	
	// Enabled indicates if messaging is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns default messaging configuration
func DefaultConfig() Config {
	return Config{
		Type:                MessageTypeRabbitMQ,
		Host:                "localhost",
		Port:                5672,
		Username:            "guest",
		Password:            "guest",
		Database:            "/",
		Timeout:             30 * time.Second,
		ConnectionTimeout:   10 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		PoolSize:            10,
		IdleTimeout:         60 * time.Second,
		Heartbeat:           30 * time.Second,
		PrefetchCount:       10,
		DefaultExchange:     "",
		DefaultQueue:        "default",
		DefaultRoutingKey:   "default",
		DeliveryMode:        DeliveryModePersistent,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         false,
		Persistent:          true,
		AutoReconnect:       true,
		ReconnectDelay:      5 * time.Second,
		MaxReconnectAttempts: 5,
		Options:             make(map[string]interface{}),
		Enabled:             true,
	}
}

// RabbitMQConfig returns RabbitMQ-specific configuration
func RabbitMQConfig(host string, port int) Config {
	return Config{
		Type:                MessageTypeRabbitMQ,
		Host:                host,
		Port:                port,
		Username:            "guest",
		Password:            "guest",
		Database:            "/",
		Timeout:             30 * time.Second,
		ConnectionTimeout:   10 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		PoolSize:            10,
		IdleTimeout:         60 * time.Second,
		Heartbeat:           30 * time.Second,
		PrefetchCount:       10,
		DefaultExchange:     "amq.direct",
		DefaultQueue:        "default",
		DefaultRoutingKey:   "default",
		DeliveryMode:        DeliveryModePersistent,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         false,
		Persistent:          true,
		AutoReconnect:       true,
		ReconnectDelay:      5 * time.Second,
		MaxReconnectAttempts: 5,
		Options: map[string]interface{}{
			"vhost":           "/",
			"exchange_durable": true,
			"queue_durable":   true,
		},
		Enabled: true,
	}
}

// KafkaConfig returns Kafka-specific configuration
func KafkaConfig(hosts []string) Config {
	return Config{
		Type:                MessageTypeKafka,
		Host:                hosts[0],
		Port:                9092,
		Username:            "",
		Password:            "",
		Database:            "",
		Timeout:             30 * time.Second,
		ConnectionTimeout:   10 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		MaxRetries:          3,
		RetryDelay:          5 * time.Second,
		PoolSize:            10,
		IdleTimeout:         60 * time.Second,
		Heartbeat:           3 * time.Second,
		PrefetchCount:       100,
		DefaultExchange:     "",
		DefaultQueue:        "default-topic",
		DefaultRoutingKey:   "default",
		DeliveryMode:        DeliveryModePersistent,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         true,
		Persistent:          true,
		AutoReconnect:       true,
		ReconnectDelay:      5 * time.Second,
		MaxReconnectAttempts: 5,
		Options: map[string]interface{}{
			"brokers":           hosts,
			"client_id":         "github.com/alldev-run/golang-gin-rpc",
			"compression_type":  "gzip",
			"acks":              "all",
			"retries":           3,
			"batch_size":        16384,
			"linger_ms":         10,
			"buffer_memory":     33554432, // 32MB
		},
		Enabled: true,
	}
}

// NATSConfig returns NATS-specific configuration
func NATSConfig(host string, port int) Config {
	return Config{
		Type:                MessageTypeNATS,
		Host:                host,
		Port:                port,
		Username:            "",
		Password:            "",
		Database:            "",
		Timeout:             10 * time.Second,
		ConnectionTimeout:   5 * time.Second,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxRetries:          3,
		RetryDelay:          2 * time.Second,
		PoolSize:            5,
		IdleTimeout:         30 * time.Second,
		Heartbeat:           0, // NATS doesn't use heartbeat
		PrefetchCount:       0, // NATS doesn't use prefetch
		DefaultExchange:     "",
		DefaultQueue:        "default",
		DefaultRoutingKey:   "default",
		DeliveryMode:        DeliveryModeTransient,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         false,
		Persistent:          false,
		AutoReconnect:       true,
		ReconnectDelay:      2 * time.Second,
		MaxReconnectAttempts: 10,
		Options: map[string]interface{}{
			"client_name": "github.com/alldev-run/golang-gin-rpc",
			"no_echo":    true,
		},
		Enabled: true,
	}
}

// RedisConfig returns Redis-specific configuration
func RedisConfig(host string, port int) Config {
	return Config{
		Type:                MessageTypeRedis,
		Host:                host,
		Port:                port,
		Username:            "",
		Password:            "",
		Database:            "0",
		Timeout:             5 * time.Second,
		ConnectionTimeout:   5 * time.Second,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
		MaxRetries:          3,
		RetryDelay:          1 * time.Second,
		PoolSize:            10,
		IdleTimeout:         30 * time.Second,
		Heartbeat:           0,
		PrefetchCount:       0,
		DefaultExchange:     "",
		DefaultQueue:        "default",
		DefaultRoutingKey:   "default",
		DeliveryMode:        DeliveryModeTransient,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         false,
		Persistent:          false,
		AutoReconnect:       true,
		ReconnectDelay:      1 * time.Second,
		MaxReconnectAttempts: 5,
		Options: map[string]interface{}{
			"db":           0,
			"key_prefix":   "messaging:",
			"max_retries":  3,
		},
		Enabled: true,
	}
}

// MemoryConfig returns in-memory configuration
func MemoryConfig() Config {
	return Config{
		Type:                MessageTypeMemory,
		Host:                "",
		Port:                0,
		Username:            "",
		Password:            "",
		Database:            "",
		Timeout:             1 * time.Second,
		ConnectionTimeout:   100 * time.Millisecond,
		ReadTimeout:         1 * time.Second,
		WriteTimeout:        1 * time.Second,
		MaxRetries:          0,
		RetryDelay:          0,
		PoolSize:            1,
		IdleTimeout:         0,
		Heartbeat:           0,
		PrefetchCount:       0,
		DefaultExchange:     "",
		DefaultQueue:        "default",
		DefaultRoutingKey:   "default",
		DeliveryMode:        DeliveryModeTransient,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         false,
		Persistent:          false,
		AutoReconnect:       false,
		ReconnectDelay:      0,
		MaxReconnectAttempts: 0,
		Options:             make(map[string]interface{}),
		Enabled:             true,
	}
}

// DevelopmentConfig returns development-friendly configuration
func DevelopmentConfig() Config {
	return Config{
		Type:                MessageTypeMemory,
		Host:                "",
		Port:                0,
		Username:            "",
		Password:            "",
		Database:            "",
		Timeout:             1 * time.Second,
		ConnectionTimeout:   100 * time.Millisecond,
		ReadTimeout:         1 * time.Second,
		WriteTimeout:        1 * time.Second,
		MaxRetries:          0,
		RetryDelay:          0,
		PoolSize:            1,
		IdleTimeout:         0,
		Heartbeat:           0,
		PrefetchCount:       0,
		DefaultExchange:     "",
		DefaultQueue:        "dev-queue",
		DefaultRoutingKey:   "dev",
		DeliveryMode:        DeliveryModeTransient,
		Priority:            0,
		TTL:                 0,
		Expiration:          0,
		Compression:         false,
		Persistent:          false,
		AutoReconnect:       false,
		ReconnectDelay:      0,
		MaxReconnectAttempts: 0,
		Options: map[string]interface{}{
			"env": "development",
		},
		Enabled: true,
	}
}

// ProductionConfig returns production-friendly configuration
func ProductionConfig(msgType MessageType, host string, port int) Config {
	switch msgType {
	case MessageTypeRabbitMQ:
		return RabbitMQConfig(host, port)
	case MessageTypeKafka:
		return KafkaConfig([]string{host})
	case MessageTypeNATS:
		return NATSConfig(host, port)
	case MessageTypeRedis:
		return RedisConfig(host, port)
	default:
		return RabbitMQConfig(host, port)
	}
}

// Validate validates the messaging configuration
func (c Config) Validate() error {
	if c.Type == "" {
		c.Type = MessageTypeRabbitMQ
	}
	if c.Host == "" && c.Type != MessageTypeMemory {
		switch c.Type {
		case MessageTypeRabbitMQ:
			c.Host = "localhost"
			c.Port = 5672
		case MessageTypeKafka:
			c.Host = "localhost"
			c.Port = 9092
		case MessageTypeNATS:
			c.Host = "localhost"
			c.Port = 4222
		case MessageTypeRedis:
			c.Host = "localhost"
			c.Port = 6379
		}
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.ConnectionTimeout == 0 {
		c.ConnectionTimeout = 10 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = 5 * time.Second
	}
	if c.PoolSize == 0 {
		c.PoolSize = 10
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
	}
	if c.ReconnectDelay == 0 {
		c.ReconnectDelay = 5 * time.Second
	}
	if c.MaxReconnectAttempts == 0 {
		c.MaxReconnectAttempts = 5
	}
	if c.DefaultQueue == "" {
		c.DefaultQueue = "default"
	}
	if c.DefaultRoutingKey == "" {
		c.DefaultRoutingKey = "default"
	}
	if c.Options == nil {
		c.Options = make(map[string]interface{})
	}
	return nil
}

// GetConnectionString returns the connection string for the messaging system
func (c Config) GetConnectionString() string {
	switch c.Type {
	case MessageTypeRabbitMQ:
		return fmt.Sprintf("amqp://%s:%s@%s:%d%s", c.Username, c.Password, c.Host, c.Port, c.Database)
	case MessageTypeKafka:
		return fmt.Sprintf("%s:%d", c.Host, c.Port)
	case MessageTypeNATS:
		return fmt.Sprintf("nats://%s:%d", c.Host, c.Port)
	case MessageTypeRedis:
		return fmt.Sprintf("redis://%s:%d/%s", c.Host, c.Port, c.Database)
	default:
		return ""
	}
}
