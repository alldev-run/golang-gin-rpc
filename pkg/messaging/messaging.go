package messaging

import (
	"context"
	"time"
)

// Message represents a message that can be sent to a messaging system
type Message struct {
	Topic     string                 `json:"topic"`
	Key       string                 `json:"key,omitempty"`
	Payload   []byte                 `json:"payload"`
	Headers   map[string]interface{} `json:"headers,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// MessageHandler is a function that handles incoming messages
type MessageHandler func(ctx context.Context, msg *Message) error

// Publisher interface for publishing messages
type Publisher interface {
	Publish(ctx context.Context, topic string, msg *Message) error
	PublishAsync(ctx context.Context, topic string, msg *Message) error
	Close() error
}

// Subscriber interface for subscribing to messages
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	Unsubscribe(topic string) error
	Close() error
}

// Client interface that combines both publisher and subscriber
type Client interface {
	Publisher
	Subscriber
}


// MessageError represents an error in messaging operations
type MessageError struct {
	Operation string
	Topic     string
	Err       error
}

func (e *MessageError) Error() string {
	return e.Operation + " failed for topic " + e.Topic + ": " + e.Err.Error()
}

func (e *MessageError) Unwrap() error {
	return e.Err
}
