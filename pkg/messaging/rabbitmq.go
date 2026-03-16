package messaging

import (
	"context"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQClient implements the Client interface for RabbitMQ
type RabbitMQClient struct {
	config     Config
	connection *amqp.Connection
	channel    *amqp.Channel
	mu         sync.RWMutex
	consumers  map[string]*amqp.Channel
}

// NewRabbitMQClient creates a new RabbitMQ client
func NewRabbitMQClient(config Config) (*RabbitMQClient, error) {
	client := &RabbitMQClient{
		config:    config,
		consumers: make(map[string]*amqp.Channel),
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// connect establishes a connection to RabbitMQ
func (c *RabbitMQClient) connect() error {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		c.config.Username,
		c.config.Password,
		c.config.Host,
		c.config.Port,
		c.config.Database,
	)

	conn, err := amqp.Dial(url)
	if err != nil {
		return &MessageError{
			Operation: "connect",
			Topic:     "",
			Err:       err,
		}
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return &MessageError{
			Operation: "channel",
			Topic:     "",
			Err:       err,
		}
	}

	c.mu.Lock()
	c.connection = conn
	c.channel = ch
	c.mu.Unlock()

	return nil
}

// Publish publishes a message to a RabbitMQ exchange
func (c *RabbitMQClient) Publish(ctx context.Context, topic string, msg *Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.channel == nil {
		return &MessageError{
			Operation: "publish",
			Topic:     topic,
			Err:       fmt.Errorf("channel is nil"),
		}
	}

	// Declare exchange if it doesn't exist
	err := c.channel.ExchangeDeclare(
		topic,     // name
		"fanout",  // type
		true,      // durable
		false,     // auto-deleted
		false,     // internal
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return &MessageError{
			Operation: "exchange_declare",
			Topic:     topic,
			Err:       err,
		}
	}

	// Prepare message headers
	headers := make(amqp.Table)
	for k, v := range msg.Headers {
		headers[k] = v
	}

	// Publish message
	err = c.channel.PublishWithContext(
		ctx,
		topic, // exchange
		"",    // routing key (fanout ignores this)
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Headers:     headers,
			Body:        msg.Payload,
			Timestamp:   msg.Timestamp,
		},
	)
	if err != nil {
		return &MessageError{
			Operation: "publish",
			Topic:     topic,
			Err:       err,
		}
	}

	return nil
}

// PublishAsync publishes a message asynchronously
func (c *RabbitMQClient) PublishAsync(ctx context.Context, topic string, msg *Message) error {
	go func() {
		if err := c.Publish(ctx, topic, msg); err != nil {
			log.Printf("Failed to publish message asynchronously: %v", err)
		}
	}()
	return nil
}

// Subscribe subscribes to a RabbitMQ queue
func (c *RabbitMQClient) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create a dedicated channel for this consumer
	consumerCh, err := c.connection.Channel()
	if err != nil {
		return &MessageError{
			Operation: "consumer_channel",
			Topic:     topic,
			Err:       err,
		}
	}

	// Declare exchange
	err = consumerCh.ExchangeDeclare(
		topic,     // name
		"fanout",  // type
		true,      // durable
		false,     // auto-deleted
		false,     // internal
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		consumerCh.Close()
		return &MessageError{
			Operation: "exchange_declare",
			Topic:     topic,
			Err:       err,
		}
	}

	// Declare queue
	q, err := consumerCh.QueueDeclare(
		"",    // name (random)
		true,  // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		consumerCh.Close()
		return &MessageError{
			Operation: "queue_declare",
			Topic:     topic,
			Err:       err,
		}
	}

	// Bind queue to exchange
	err = consumerCh.QueueBind(
		q.Name, // queue name
		"",     // routing key
		topic,  // exchange
		false,  // no-wait
		nil,    // arguments
	)
	if err != nil {
		consumerCh.Close()
		return &MessageError{
			Operation: "queue_bind",
			Topic:     topic,
			Err:       err,
		}
	}

	// Start consuming messages
	msgs, err := consumerCh.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		consumerCh.Close()
		return &MessageError{
			Operation: "consume",
			Topic:     topic,
			Err:       err,
		}
	}

	c.consumers[topic] = consumerCh

	// Start goroutine to handle messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				consumerCh.Close()
				delete(c.consumers, topic)
				return
			case delivery, ok := <-msgs:
				if !ok {
					consumerCh.Close()
					delete(c.consumers, topic)
					return
				}

				// Create message object
				msg := &Message{
					Topic:     topic,
					Payload:   delivery.Body,
					Timestamp: delivery.Timestamp,
					Headers:   make(map[string]interface{}),
				}

				// Convert headers
				for k, v := range delivery.Headers {
					msg.Headers[k] = v
				}

				// Handle message
				if err := handler(ctx, msg); err != nil {
					log.Printf("Error handling message: %v", err)
					// Negative acknowledge
					delivery.Nack(false, false)
				} else {
					// Acknowledge message
					delivery.Ack(false)
				}
			}
		}
	}()

	return nil
}

// Unsubscribe unsubscribes from a topic
func (c *RabbitMQClient) Unsubscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if consumerCh, exists := c.consumers[topic]; exists {
		consumerCh.Close()
		delete(c.consumers, topic)
	}

	return nil
}

// Close closes the RabbitMQ connection
func (c *RabbitMQClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close all consumer channels
	for _, consumerCh := range c.consumers {
		consumerCh.Close()
	}
	c.consumers = make(map[string]*amqp.Channel)

	// Close main channel and connection
	if c.channel != nil {
		c.channel.Close()
	}
	if c.connection != nil {
		c.connection.Close()
	}

	return nil
}

// Reconnect attempts to reconnect to RabbitMQ
func (c *RabbitMQClient) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connection != nil && !c.connection.IsClosed() {
		c.connection.Close()
	}

	return c.connect()
}
