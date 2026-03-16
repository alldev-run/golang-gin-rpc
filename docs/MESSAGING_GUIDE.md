# 📨 Messaging System Guide

## 🎯 Overview

This guide covers the messaging system that provides unified support for multiple messaging systems in the golang-gin-rpc framework, including **RabbitMQ**, **Apache Kafka**, and future support for additional systems.

## 🏗️ Architecture

### Core Components

- **Interface Layer**: `messaging.go` - Defines unified interfaces for messaging operations
- **Type System**: `types.go` - Comprehensive type system with enums and capabilities
- **RabbitMQ Implementation**: `rabbitmq.go` - RabbitMQ client and operations
- **Kafka Implementation**: `kafka.go` - Kafka client and operations  
- **Factory Pattern**: `factory.go` - Creates appropriate client based on configuration

### Supported Messaging Types

The framework supports multiple messaging types through a comprehensive type system:

#### **Currently Implemented**
- **RabbitMQ** (`messaging.RabbitMQ`) - Traditional message broker with exchange/queue patterns
- **Apache Kafka** (`messaging.Kafka`) - Distributed streaming platform

#### **Planned for Future Implementation**
- **Redis Streams** (`messaging.RedisStreams`) - Redis-based streaming
- **NATS** (`messaging.NATS`) - Lightweight messaging system
- **ActiveMQ** (`messaging.ActiveMQ`) - Enterprise message broker
- **Amazon SQS** (`messaging.AmazonSQS`) - Cloud-based queue service
- **Google Pub/Sub** (`messaging.GooglePubSub`) - Cloud messaging service
- **Azure Service Bus** (`messaging.AzureServiceBus`) - Cloud enterprise messaging
- **Apache Pulsar** (`messaging.Pulsar`) - Cloud-native messaging platform

### Type System Features

```go
// MessageType enum with comprehensive functionality
type MessageType string

const (
    RabbitMQ        MessageType = "rabbitmq"
    Kafka           MessageType = "kafka"
    RedisStreams    MessageType = "redis_streams"
    NATS            MessageType = "nats"
    ActiveMQ        MessageType = "activemq"
    AmazonSQS       MessageType = "sqs"
    GooglePubSub    MessageType = "pubsub"
    AzureServiceBus MessageType = "servicebus"
    Pulsar          MessageType = "pulsar"
)

// Type capabilities and metadata
msgType.DefaultPort()        // Get default port
msgType.DisplayName()        // Human-readable name
msgType.IsImplemented()       // Check if implemented
msgType.IsCloudBased()        // Check if cloud service
msgType.RequiresAuthentication() // Check auth requirements
```

### Key Interfaces

```go
type Publisher interface {
    Publish(ctx context.Context, topic string, msg *Message) error
    PublishAsync(ctx context.Context, topic string, msg *Message) error
    Close() error
}

type Subscriber interface {
    Subscribe(ctx context.Context, topic string, handler MessageHandler) error
    Unsubscribe(topic string) error
    Close() error
}

type Client interface {
    Publisher
    Subscriber
}
```

## 🔧 Configuration

### RabbitMQ Configuration

```yaml
messaging:
  rabbitmq:
    type: "rabbitmq"
    host: "localhost"
    port: 5672
    username: "guest"
    password: "guest"
    database: "/"
    options:
      exchange_type: "fanout"
      durable: true
      auto_delete: false
```

### Kafka Configuration

```yaml
messaging:
  kafka:
    type: "kafka"
    host: "localhost"
    port: 9092
    username: ""
    password: ""
    database: ""
    options:
      batch_size: 100
      linger_ms: 10
      compression_type: "gzip"
      acks: "all"
      retries: 3
```

## 🚀 Usage Examples

### Basic RabbitMQ Usage with Type Constants

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "time"
    
    "golang-gin-rpc/pkg/messaging"
)

func main() {
    // Create RabbitMQ configuration using type constants
    config := messaging.Config{
        Type:     messaging.RabbitMQ.String(),
        Host:     "localhost",
        Port:     messaging.RabbitMQ.DefaultPort(),
        Username: "guest",
        Password: "guest",
        Database: "/",
    }
    
    // Validate configuration
    if err := messaging.ValidateConfig(config); err != nil {
        log.Fatal(err)
    }
    
    // Create client using factory pattern
    client, err := messaging.CreateClientByType(messaging.RabbitMQ, config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Show capabilities
    capabilities := messaging.GetClientCapabilities(messaging.RabbitMQ)
    log.Printf("RabbitMQ capabilities: %+v", capabilities)
    
    ctx := context.Background()
    
    // Subscribe to messages
    err = client.Subscribe(ctx, "user.events", func(ctx context.Context, msg *messaging.Message) error {
        log.Printf("Received: %s", string(msg.Payload))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Publish message
    msg := &messaging.Message{
        Topic:     "user.events",
        Payload:   []byte(`{"user_id": 123, "action": "login"}`),
        Headers:   map[string]interface{}{"event_type": "user_action"},
        Timestamp: time.Now(),
    }
    
    err = client.Publish(ctx, "user.events", msg)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Basic Kafka Usage with Type Constants

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "time"
    
    "golang-gin-rpc/pkg/messaging"
)

func main() {
    // Create Kafka configuration using type constants
    config := messaging.Config{
        Type: messaging.Kafka.String(),
        Host: "localhost",
        Port: messaging.Kafka.DefaultPort(),
    }
    
    // Validate configuration
    if err := messaging.ValidateConfig(config); err != nil {
        log.Fatal(err)
    }
    
    // Create client using factory pattern
    client, err := messaging.CreateClientByType(messaging.Kafka, config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Show capabilities
    capabilities := messaging.GetClientCapabilities(messaging.Kafka)
    log.Printf("Kafka capabilities: %+v", capabilities)
    
    ctx := context.Background()
    
    // Subscribe to messages
    err = client.Subscribe(ctx, "order.events", func(ctx context.Context, msg *messaging.Message) error {
        log.Printf("Received: %s", string(msg.Payload))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Publish message
    msg := &messaging.Message{
        Topic:     "order.events",
        Key:       "order_123",
        Payload:   []byte(`{"order_id": "123", "amount": 99.99}`),
        Headers:   map[string]interface{}{"event_type": "order_created"},
        Timestamp: time.Now(),
    }
    
    err = client.Publish(ctx, "order.events", msg)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Factory Pattern with Type Validation

```go
// Create client using factory with type validation
func createMessagingClient(msgType messaging.MessagingType) (messaging.Client, error) {
    // Check if type is valid
    if !msgType.IsValid() {
        return nil, fmt.Errorf("invalid messaging type: %s", msgType)
    }
    
    // Check if implemented
    if !msgType.IsImplemented() {
        return nil, fmt.Errorf("messaging type '%s' is not yet implemented", 
            msgType.DisplayName())
    }
    
    // Create configuration with default port
    config := messaging.Config{
        Type: msgType.String(),
        Host: "localhost",
        Port: msgType.DefaultPort(),
    }
    
    // Validate configuration
    if err := messaging.ValidateConfig(config); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }
    
    // Create client
    return messaging.CreateClientByType(msgType, config)
}

// Usage
client, err := createMessagingClient(messaging.RabbitMQ)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### Dynamic Type Selection

```go
// Dynamic messaging type selection based on configuration
func setupMessaging(configStr string) (messaging.Client, error) {
    // Parse message type from string
    msgType, err := messaging.ParseMessageType(configStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse message type: %w", err)
    }
    
    // Show type information
    log.Printf("Setting up %s messaging", msgType.DisplayName())
    log.Printf("Cloud based: %v", msgType.IsCloudBased())
    log.Printf("Requires auth: %v", msgType.RequiresAuthentication())
    
    // Create configuration
    config := messaging.Config{
        Type: msgType.String(),
        Host: "localhost",
        Port: msgType.DefaultPort(),
    }
    
    // Add authentication if required
    if msgType.RequiresAuthentication() {
        config.Username = os.Getenv("MESSAGING_USERNAME")
        config.Password = os.Getenv("MESSAGING_PASSWORD")
    }
    
    return messaging.CreateClientByType(msgType, config)
}
```

## 📋 Message Structure

```go
type Message struct {
    Topic     string                 `json:"topic"`
    Key       string                 `json:"key,omitempty"`        // Kafka only
    Payload   []byte                 `json:"payload"`
    Headers   map[string]interface{} `json:"headers,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
}
```

## 🔍 Advanced Features

### Async Publishing

```go
// Async publishing (non-blocking)
err := client.PublishAsync(ctx, "topic", msg)
```

### Message Headers

```go
msg := &messaging.Message{
    Topic:   "events",
    Payload: []byte(`{"data": "value"}`),
    Headers: map[string]interface{}{
        "event_type": "user_action",
        "version":    "1.0",
        "source":     "user_service",
    },
    Timestamp: time.Now(),
}
```

### Error Handling

```go
type MessageError struct {
    Operation string
    Topic     string
    Err       error
}

// Check for specific message errors
if msgErr, ok := err.(*messaging.MessageError); ok {
    log.Printf("Message operation '%s' failed for topic '%s': %v", 
        msgErr.Operation, msgErr.Topic, msgErr.Err)
}
```

## 🛠️ Best Practices

### 1. Connection Management

```go
// Always close connections when done
defer client.Close()

// Handle connection errors gracefully
if err := client.Publish(ctx, "topic", msg); err != nil {
    // Check if it's a connection error
    if isConnectionError(err) {
        // Implement retry logic
    }
}
```

### 2. Context Usage

```go
// Use context with timeout for operations
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := client.Publish(ctx, "topic", msg)
```

### 3. Message Handling

```go
// Always handle messages asynchronously
handler := func(ctx context.Context, msg *messaging.Message) error {
    go func() {
        // Process message
        if err := processMessage(msg); err != nil {
            log.Printf("Error processing message: %v", err)
        }
    }()
    return nil
}
```

### 4. Configuration Management

```go
// Use environment variables for sensitive data
config := messaging.Config{
    Type:     os.Getenv("MESSAGING_TYPE"),
    Host:     os.Getenv("MESSAGING_HOST"),
    Port:     mustGetInt(os.Getenv("MESSAGING_PORT")),
    Username: os.Getenv("MESSAGING_USERNAME"),
    Password: os.Getenv("MESSAGING_PASSWORD"),
}
```

## 🐳 Docker Setup

### RabbitMQ Docker Compose

```yaml
version: '3.8'
services:
  rabbitmq:
    image: rabbitmq:3-management
    ports:
      - "5672:5672"
      - "15672:15672"
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq

volumes:
  rabbitmq_data:
```

### Kafka Docker Compose

```yaml
version: '3.8'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000

  kafka:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
```

## 🔄 Migration Guide

### From RabbitMQ to Kafka

1. **Change Configuration**:
   ```yaml
   # Before
   type: "rabbitmq"
   port: 5672
   
   # After  
   type: "kafka"
   port: 9092
   ```

2. **Update Code**:
   ```go
   // No code changes needed if using factory pattern
   client, err := messaging.NewClient(config)
   ```

3. **Consider Differences**:
   - Kafka supports message keys for partitioning
   - RabbitMQ uses exchanges, Kafka uses topics
   - Kafka provides message ordering within partitions

## 📊 Performance Considerations

### RabbitMQ

- **Throughput**: ~10K-50K messages/second
- **Latency**: ~1-5ms
- **Use Cases**: Task queues, RPC patterns, routing

### Kafka

- **Throughput**: ~100K+ messages/second  
- **Latency**: ~5-10ms
- **Use Cases**: Event streaming, log aggregation, analytics

## 🔍 Monitoring & Debugging

### RabbitMQ

```bash
# Check connection
rabbitmqctl status

# List queues
rabbitmqctl list_queues

# Monitor exchanges
rabbitmqctl list_exchanges
```

### Kafka

```bash
# List topics
kafka-topics.sh --list --bootstrap-server localhost:9092

# Describe topic
kafka-topics.sh --describe --topic my-topic --bootstrap-server localhost:9092

# Consume messages
kafka-console-consumer.sh --topic my-topic --bootstrap-server localhost:9092
```

## 🆘 Troubleshooting

### Common Issues

1. **Connection Refused**
   - Check if broker is running
   - Verify host/port configuration
   - Check firewall settings

2. **Authentication Failed**
   - Verify username/password
   - Check user permissions

3. **Topic/Exchange Not Found**
   - Ensure topic/exchange exists
   - Check auto-creation settings

4. **Message Not Delivered**
   - Check broker logs
   - Verify message format
   - Check network connectivity

## 📚 Additional Resources

- [RabbitMQ Documentation](https://www.rabbitmq.com/documentation.html)
- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [Go RabbitMQ Client](https://github.com/rabbitmq/amqp091-go)
- [Go Kafka Client](https://github.com/IBM/sarama)
