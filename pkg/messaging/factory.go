package messaging

import (
	"fmt"
)

// NewClient creates a new messaging client based on the configuration
func NewClient(config Config) (Client, error) {
	// Parse message type from config string
	msgType, err := ParseMessageType(config.Type)
	if err != nil {
		return nil, fmt.Errorf("invalid messaging type '%s': %w", config.Type, err)
	}

	// Check if the messaging type is implemented
	if !msgType.IsImplemented() {
		return nil, fmt.Errorf("messaging type '%s' is not yet implemented. Supported types: %v", 
			msgType.DisplayName(), GetImplementedTypes())
	}

	// Create client based on type
	switch msgType {
	case RabbitMQ:
		return NewRabbitMQClient(config)
	case Kafka:
		return NewKafkaClient(config)
	default:
		return nil, fmt.Errorf("unsupported messaging type: %s", msgType.DisplayName())
	}
}

// NewPublisher creates a new publisher client
func NewPublisher(config Config) (Publisher, error) {
	return NewClient(config)
}

// NewSubscriber creates a new subscriber client
func NewSubscriber(config Config) (Subscriber, error) {
	return NewClient(config)
}

// CreateClientByType creates a client using MessageType enum
func CreateClientByType(msgType MessageType, config Config) (Client, error) {
	// Validate message type
	if !msgType.IsValid() {
		return nil, fmt.Errorf("invalid message type: %s", msgType)
	}

	// Check if implemented
	if !msgType.IsImplemented() {
		return nil, fmt.Errorf("messaging type '%s' is not yet implemented", msgType.DisplayName())
	}

	// Update config type
	config.Type = msgType.String()

	return NewClient(config)
}

// GetClientCapabilities returns the capabilities for a messaging type
func GetClientCapabilities(msgType MessageType) MessagingCapabilities {
	return GetCapabilities(msgType)
}

// ValidateConfig validates the messaging configuration
func ValidateConfig(config Config) error {
	// Parse message type
	msgType, err := ParseMessageType(config.Type)
	if err != nil {
		return fmt.Errorf("invalid messaging type: %w", err)
	}

	// Check if implemented
	if !msgType.IsImplemented() {
		return fmt.Errorf("messaging type '%s' is not implemented", msgType.DisplayName())
	}

	// Validate required fields based on type
	switch msgType {
	case RabbitMQ:
		if config.Host == "" {
			return fmt.Errorf("RabbitMQ requires host")
		}
		if config.Port == 0 {
			config.Port = msgType.DefaultPort()
		}
		if config.Username == "" {
			return fmt.Errorf("RabbitMQ requires username")
		}
		if config.Password == "" {
			return fmt.Errorf("RabbitMQ requires password")
		}
	case Kafka:
		if config.Host == "" {
			return fmt.Errorf("Kafka requires host")
		}
		if config.Port == 0 {
			config.Port = msgType.DefaultPort()
		}
	}

	return nil
}
