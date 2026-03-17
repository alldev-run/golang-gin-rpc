package messaging

import (
	"fmt"
	"strings"
)


// IsValid checks if the message type is supported
func (mt MessageType) IsValid() bool {
	supportedTypes := []MessageType{
		MessageTypeRabbitMQ,
		MessageTypeKafka,
		MessageTypeNATS,
		MessageTypeRedis,
		MessageTypeMemory,
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
	case MessageTypeRabbitMQ:
		return "RabbitMQ"
	case MessageTypeKafka:
		return "Apache Kafka"
	case MessageTypeNATS:
		return "NATS"
	case MessageTypeRedis:
		return "Redis"
	case MessageTypeMemory:
		return "Memory"
	default:
		return "Unknown"
	}
}

// DefaultPort returns the default port for the messaging type
func (mt MessageType) DefaultPort() int {
	switch mt {
	case MessageTypeRabbitMQ:
		return 5672
	case MessageTypeKafka:
		return 9092
	case MessageTypeNATS:
		return 4222
	case MessageTypeRedis:
		return 6379
	default:
		return 0
	}
}

// IsCloudBased returns true if the messaging type is a cloud service
func (mt MessageType) IsCloudBased() bool {
	// No cloud-based types in current implementation
	return false
}

// IsOpenSource returns true if the messaging type is open source
func (mt MessageType) IsOpenSource() bool {
	switch mt {
	case MessageTypeRabbitMQ, MessageTypeKafka, MessageTypeNATS, MessageTypeRedis:
		return true
	default:
		return false
	}
}

// GetSupportedTypes returns all supported messaging types
func GetSupportedTypes() []MessageType {
	return []MessageType{
		MessageTypeRabbitMQ,
		MessageTypeKafka,
		MessageTypeNATS,
		MessageTypeRedis,
		MessageTypeMemory,
	}
}

// GetImplementedTypes returns messaging types that are currently implemented
func GetImplementedTypes() []MessageType {
	return []MessageType{
		MessageTypeRabbitMQ,
		MessageTypeKafka,
	}
}

// GetFutureTypes returns messaging types planned for future implementation
func GetFutureTypes() []MessageType {
	return []MessageType{
		MessageTypeNATS,
		MessageTypeRedis,
	}
}

// ParseMessageType parses a string into MessageType
func ParseMessageType(s string) (MessageType, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	
	switch s {
	case "rabbitmq", "amqp":
		return MessageTypeRabbitMQ, nil
	case "kafka":
		return MessageTypeKafka, nil
	case "nats":
		return MessageTypeNATS, nil
	case "redis":
		return MessageTypeRedis, nil
	case "memory":
		return MessageTypeMemory, nil
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
	case MessageTypeRabbitMQ:
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
	case MessageTypeKafka:
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
	case MessageTypeNATS:
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
	case MessageTypeRedis:
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
	case MessageTypeMemory:
		return MessagingCapabilities{
			SupportsPublish:      true,
			SupportsSubscribe:    true,
			SupportsHeaders:      false,
			SupportsPartitioning: false,
			SupportsOrdering:     false,
			SupportsPersistence:  false,
			SupportsTransactions: false,
			SupportsDLQ:          false,
			SupportsReplay:       false,
			SupportsCompression:  false,
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
	case MessageTypeRabbitMQ, MessageTypeKafka:
		return true
	case MessageTypeNATS, MessageTypeRedis, MessageTypeMemory:
		return false
	default:
		return false
	}
}
