package messaging

import (
	"fmt"
	"strings"
)

// MessageType represents the type of messaging system
type MessageType string

// Supported messaging types
const (
	// RabbitMQ messaging system
	RabbitMQ MessageType = "rabbitmq"
	
	// Apache Kafka messaging system  
	Kafka MessageType = "kafka"
	
	// Redis Streams (future implementation)
	RedisStreams MessageType = "redis_streams"
	
	// NATS messaging system (future implementation)
	NATS MessageType = "nats"
	
	// ActiveMQ messaging system (future implementation)
	ActiveMQ MessageType = "activemq"
	
	// Amazon SQS (future implementation)
	AmazonSQS MessageType = "sqs"
	
	// Google Pub/Sub (future implementation)
	GooglePubSub MessageType = "pubsub"
	
	// Azure Service Bus (future implementation)
	AzureServiceBus MessageType = "servicebus"
	
	// Pulsar messaging system (future implementation)
	Pulsar MessageType = "pulsar"
)

// IsValid checks if the message type is supported
func (mt MessageType) IsValid() bool {
	supportedTypes := []MessageType{
		RabbitMQ,
		Kafka,
		RedisStreams,
		NATS,
		ActiveMQ,
		AmazonSQS,
		GooglePubSub,
		AzureServiceBus,
		Pulsar,
	}
	
	for _, supportedType := range supportedTypes {
		if mt == supportedType {
			return true
		}
	}
	return false
}

// String returns the string representation of MessageType
func (mt MessageType) String() string {
	return string(mt)
}

// DisplayName returns a human-readable display name
func (mt MessageType) DisplayName() string {
	switch mt {
	case RabbitMQ:
		return "RabbitMQ"
	case Kafka:
		return "Apache Kafka"
	case RedisStreams:
		return "Redis Streams"
	case NATS:
		return "NATS"
	case ActiveMQ:
		return "ActiveMQ"
	case AmazonSQS:
		return "Amazon SQS"
	case GooglePubSub:
		return "Google Pub/Sub"
	case AzureServiceBus:
		return "Azure Service Bus"
	case Pulsar:
		return "Apache Pulsar"
	default:
		return "Unknown"
	}
}

// DefaultPort returns the default port for the messaging type
func (mt MessageType) DefaultPort() int {
	switch mt {
	case RabbitMQ:
		return 5672
	case Kafka:
		return 9092
	case RedisStreams:
		return 6379
	case NATS:
		return 4222
	case ActiveMQ:
		return 61616
	case AmazonSQS:
		return 443 // HTTPS
	case GooglePubSub:
		return 443 // HTTPS
	case AzureServiceBus:
		return 5671 // AMQPS
	case Pulsar:
		return 6650 // Binary protocol
	default:
		return 0
	}
}

// IsCloudBased returns true if the messaging type is a cloud service
func (mt MessageType) IsCloudBased() bool {
	switch mt {
	case AmazonSQS, GooglePubSub, AzureServiceBus:
		return true
	default:
		return false
	}
}

// IsOpenSource returns true if the messaging type is open source
func (mt MessageType) IsOpenSource() bool {
	switch mt {
	case RabbitMQ, Kafka, RedisStreams, NATS, ActiveMQ, Pulsar:
		return true
	default:
		return false
	}
}

// GetSupportedTypes returns all supported messaging types
func GetSupportedTypes() []MessageType {
	return []MessageType{
		RabbitMQ,
		Kafka,
		RedisStreams,
		NATS,
		ActiveMQ,
		AmazonSQS,
		GooglePubSub,
		AzureServiceBus,
		Pulsar,
	}
}

// GetImplementedTypes returns messaging types that are currently implemented
func GetImplementedTypes() []MessageType {
	return []MessageType{
		RabbitMQ,
		Kafka,
	}
}

// GetFutureTypes returns messaging types planned for future implementation
func GetFutureTypes() []MessageType {
	return []MessageType{
		RedisStreams,
		NATS,
		ActiveMQ,
		AmazonSQS,
		GooglePubSub,
		AzureServiceBus,
		Pulsar,
	}
}

// ParseMessageType parses a string into MessageType
func ParseMessageType(s string) (MessageType, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	
	switch s {
	case "rabbitmq", "amqp":
		return RabbitMQ, nil
	case "kafka":
		return Kafka, nil
	case "redis_streams", "redis-streams", "redisstreams":
		return RedisStreams, nil
	case "nats":
		return NATS, nil
	case "activemq", "active-mq":
		return ActiveMQ, nil
	case "sqs", "amazon-sqs", "amazonsqs":
		return AmazonSQS, nil
	case "pubsub", "google-pubsub", "gcp-pubsub":
		return GooglePubSub, nil
	case "servicebus", "azure-servicebus", "azure-service-bus":
		return AzureServiceBus, nil
	case "pulsar", "apache-pulsar":
		return Pulsar, nil
	default:
		return "", fmt.Errorf("unsupported messaging type: %s", s)
	}
}

// MessagingCapabilities represents the capabilities of a messaging system
type MessagingCapabilities struct {
	SupportsPublish       bool `json:"supports_publish"`
	SupportsSubscribe     bool `json:"supports_subscribe"`
	SupportsHeaders       bool `json:"supports_headers"`
	SupportsPartitioning  bool `json:"supports_partitioning"`
	SupportsOrdering      bool `json:"supports_ordering"`
	SupportsPersistence   bool `json:"supports_persistence"`
	SupportsTransactions  bool `json:"supports_transactions"`
	SupportsDLQ           bool `json:"supports_dlq"` // Dead Letter Queue
	SupportsReplay        bool `json:"supports_replay"`
	SupportsCompression   bool `json:"supports_compression"`
}

// GetCapabilities returns the capabilities for a given messaging type
func GetCapabilities(mt MessageType) MessagingCapabilities {
	switch mt {
	case RabbitMQ:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: false,
			SupportsOrdering:     false,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          true,
			SupportsReplay:       false,
			SupportsCompression:  false,
		}
	case Kafka:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: true,
			SupportsOrdering:     true,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          true,
			SupportsReplay:       true,
			SupportsCompression:  true,
		}
	case RedisStreams:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      false,
			SupportsPartitioning: false,
			SupportsOrdering:     true,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          false,
			SupportsReplay:       true,
			SupportsCompression:  false,
		}
	case NATS:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: false,
			SupportsOrdering:     false,
			SupportsPersistence:  true,
			SupportsTransactions: false,
			SupportsDLQ:          false,
			SupportsReplay:       false,
			SupportsCompression:  false,
		}
	case ActiveMQ:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: false,
			SupportsOrdering:     true,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          true,
			SupportsReplay:       true,
			SupportsCompression:  false,
		}
	case AmazonSQS:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: false,
			SupportsOrdering:     false,
			SupportsPersistence:  true,
			SupportsTransactions: false,
			SupportsDLQ:          true,
			SupportsReplay:       false,
			SupportsCompression:  false,
		}
	case GooglePubSub:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: false,
			SupportsOrdering:     false,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          true,
			SupportsReplay:       true,
			SupportsCompression:  false,
		}
	case AzureServiceBus:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: true,
			SupportsOrdering:     true,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          true,
			SupportsReplay:       true,
			SupportsCompression:  false,
		}
	case Pulsar:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      true,
			SupportsPartitioning: true,
			SupportsOrdering:     true,
			SupportsPersistence:  true,
			SupportsTransactions: true,
			SupportsDLQ:          true,
			SupportsReplay:       true,
			SupportsCompression:  true,
		}
	default:
		return MessagingCapabilities{}
	}
}

// IsImplemented returns true if the messaging type is currently implemented
func (mt MessageType) IsImplemented() bool {
	implementedTypes := GetImplementedTypes()
	for _, implementedType := range implementedTypes {
		if mt == implementedType {
			return true
		}
	}
	return false
}

// RequiresAuthentication returns true if the messaging type typically requires authentication
func (mt MessageType) RequiresAuthentication() bool {
	switch mt {
	case RabbitMQ, Kafka, ActiveMQ, AzureServiceBus, Pulsar:
		return true
	case RedisStreams, NATS:
		return false // Optional
	case AmazonSQS, GooglePubSub:
		return true // Required for cloud services
	default:
		return false
	}
}
