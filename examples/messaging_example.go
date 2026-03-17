package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"alldev-gin-rpc/pkg/messaging"
)

// UserEvent represents a user event message
type UserEvent struct {
	UserID    int    `json:"user_id"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
	Data      string `json:"data"`
}

// RabbitMQExample demonstrates RabbitMQ usage
func RabbitMQExample() {
	// RabbitMQ configuration using MessageType constant
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
		log.Fatalf("Invalid RabbitMQ config: %v", err)
	}

	// Create RabbitMQ client
	client, err := messaging.CreateClientByType(messaging.RabbitMQ, config)
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ client: %v", err)
	}
	defer client.Close()

	// Show capabilities
	capabilities := messaging.GetClientCapabilities(messaging.RabbitMQ)
	log.Printf("RabbitMQ capabilities: %+v", capabilities)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to user events
	err = client.Subscribe(ctx, "user.events", func(ctx context.Context, msg *messaging.Message) error {
		var event UserEvent
		if err := json.Unmarshal(msg.Payload, &event); err != nil {
			return fmt.Errorf("failed to unmarshal message: %w", err)
		}

		log.Printf("Received user event: %+v", event)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish some user events
	go func() {
		for i := 0; i < 5; i++ {
			event := UserEvent{
				UserID:    i + 1,
				Action:    "login",
				Timestamp: time.Now().Format(time.RFC3339),
				Data:      fmt.Sprintf("User %d logged in", i+1),
			}

			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("Failed to marshal event: %v", err)
				continue
			}

			msg := &messaging.Message{
				Topic:     "user.events",
				Key:       fmt.Sprintf("user_%d", i+1),
				Payload:   payload,
				Headers:   map[string]interface{}{"event_type": "user_action"},
				Timestamp: time.Now(),
			}

			if err := client.Publish(ctx, "user.events", msg); err != nil {
				log.Printf("Failed to publish message: %v", err)
			} else {
				log.Printf("Published message for user %d", i+1)
			}

			time.Sleep(1 * time.Second)
		}
	}()

	// Wait for messages
	time.Sleep(10 * time.Second)
}

// KafkaExample demonstrates Kafka usage
func KafkaExample() {
	// Kafka configuration using MessageType constant
	config := messaging.Config{
		Type:     messaging.Kafka.String(),
		Host:     "localhost",
		Port:     messaging.Kafka.DefaultPort(),
		Username: "",
		Password: "",
	}

	// Validate configuration
	if err := messaging.ValidateConfig(config); err != nil {
		log.Fatalf("Invalid Kafka config: %v", err)
	}

	// Create Kafka client
	client, err := messaging.CreateClientByType(messaging.Kafka, config)
	if err != nil {
		log.Fatalf("Failed to create Kafka client: %v", err)
	}
	defer client.Close()

	// Show capabilities
	capabilities := messaging.GetClientCapabilities(messaging.Kafka)
	log.Printf("Kafka capabilities: %+v", capabilities)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to order events
	err = client.Subscribe(ctx, "order.events", func(ctx context.Context, msg *messaging.Message) error {
		var order map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &order); err != nil {
			return fmt.Errorf("failed to unmarshal message: %w", err)
		}

		log.Printf("Received order event: %+v", order)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish some order events
	go func() {
		for i := 0; i < 5; i++ {
			order := map[string]interface{}{
				"order_id":  fmt.Sprintf("order_%d", i+1),
				"customer":  fmt.Sprintf("customer_%d", i+1),
				"amount":   (i + 1) * 100.0,
				"status":   "created",
				"timestamp": time.Now().Format(time.RFC3339),
			}

			payload, err := json.Marshal(order)
			if err != nil {
				log.Printf("Failed to marshal order: %v", err)
				continue
			}

			msg := &messaging.Message{
				Topic:     "order.events",
				Key:       fmt.Sprintf("order_%d", i+1),
				Payload:   payload,
				Headers:   map[string]interface{}{"event_type": "order_created"},
				Timestamp: time.Now(),
			}

			if err := client.PublishAsync(ctx, "order.events", msg); err != nil {
				log.Printf("Failed to publish message: %v", err)
			} else {
				log.Printf("Published order %d", i+1)
			}

			time.Sleep(1 * time.Second)
		}
	}()

	// Wait for messages
	time.Sleep(10 * time.Second)
}

// MessagingFactoryExample demonstrates using the factory pattern with type constants
func MessagingFactoryExample() {
	// RabbitMQ example using factory and type constants
	rabbitMQConfig := messaging.Config{
		Type:     messaging.RabbitMQ.String(),
		Host:     "localhost",
		Port:     messaging.RabbitMQ.DefaultPort(),
		Username: "guest",
		Password: "guest",
		Database: "/",
	}

	// Validate configuration
	if err := messaging.ValidateConfig(rabbitMQConfig); err != nil {
		log.Printf("Invalid RabbitMQ config: %v", err)
	} else {
		rabbitClient, err := messaging.CreateClientByType(messaging.RabbitMQ, rabbitMQConfig)
		if err != nil {
			log.Printf("Failed to create RabbitMQ client: %v", err)
		} else {
			log.Println("Successfully created RabbitMQ client using factory and type constants")
			rabbitClient.Close()
		}
	}

	// Kafka example using factory and type constants
	kafkaConfig := messaging.Config{
		Type: messaging.Kafka.String(),
		Host: "localhost",
		Port: messaging.Kafka.DefaultPort(),
	}

	// Validate configuration
	if err := messaging.ValidateConfig(kafkaConfig); err != nil {
		log.Printf("Invalid Kafka config: %v", err)
	} else {
		kafkaClient, err := messaging.CreateClientByType(messaging.Kafka, kafkaConfig)
		if err != nil {
			log.Printf("Failed to create Kafka client: %v", err)
		} else {
			log.Println("Successfully created Kafka client using factory and type constants")
			kafkaClient.Close()
		}
	}

	// Publisher-only example
	publisher, err := messaging.NewPublisher(rabbitMQConfig)
	if err != nil {
		log.Printf("Failed to create publisher: %v", err)
	} else {
		log.Println("Successfully created publisher")
		publisher.Close()
	}

	// Subscriber-only example
	subscriber, err := messaging.NewSubscriber(rabbitMQConfig)
	if err != nil {
		log.Printf("Failed to create subscriber: %v", err)
	} else {
		log.Println("Successfully created subscriber")
		subscriber.Close()
	}

	// Show all supported types
	log.Println("Supported messaging types:")
	for _, msgType := range messaging.GetSupportedTypes() {
		log.Printf("  - %s (%s)", msgType.DisplayName(), msgType.String())
		if msgType.IsImplemented() {
			log.Printf("    ✓ Implemented")
		} else {
			log.Printf("    ✗ Planned for future")
		}
	}

	// Show capabilities comparison
	log.Println("\nCapabilities comparison:")
	for _, msgType := range messaging.GetImplementedTypes() {
		capabilities := messaging.GetClientCapabilities(msgType)
		log.Printf("%s capabilities:", msgType.DisplayName())
		log.Printf("  - Publish: %v", capabilities.SupportsPublish)
		log.Printf("  - Subscribe: %v", capabilities.SupportsSubscribe)
		log.Printf("  - Headers: %v", capabilities.SupportsHeaders)
		log.Printf("  - Partitioning: %v", capabilities.SupportsPartitioning)
		log.Printf("  - Ordering: %v", capabilities.SupportsOrdering)
		log.Printf("  - Persistence: %v", capabilities.SupportsPersistence)
		log.Printf("  - Transactions: %v", capabilities.SupportsTransactions)
		log.Printf("  - DLQ: %v", capabilities.SupportsDLQ)
		log.Printf("  - Replay: %v", capabilities.SupportsReplay)
		log.Printf("  - Compression: %v", capabilities.SupportsCompression)
	}
}
