package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// KafkaClient implements the Client interface for Apache Kafka
type KafkaClient struct {
	config      Config
	producer    sarama.SyncProducer
	asyncProducer sarama.AsyncProducer
	consumer    sarama.ConsumerGroup
	mu          sync.RWMutex
	consumers   map[string]sarama.ConsumerGroupHandler
}

// NewKafkaClient creates a new Kafka client
func NewKafkaClient(config Config) (*KafkaClient, error) {
	client := &KafkaClient{
		config:    config,
		consumers: make(map[string]sarama.ConsumerGroupHandler),
	}

	// Create Kafka configuration
	kafkaConfig := sarama.NewConfig()

	// Producer configuration
	kafkaConfig.Producer.Return.Successes = true
	kafkaConfig.Producer.Return.Errors = true
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Producer.Retry.Max = 5
	kafkaConfig.Producer.Retry.Backoff = 100 * time.Millisecond

	// Consumer configuration
	kafkaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	kafkaConfig.Consumer.Group.Session.Timeout = 10 * time.Second
	kafkaConfig.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	// Set brokers
	brokers := []string{fmt.Sprintf("%s:%d", config.Host, config.Port)}

	// Create sync producer
	producer, err := sarama.NewSyncProducer(brokers, kafkaConfig)
	if err != nil {
		return nil, &MessageError{
			Operation: "sync_producer",
			Topic:     "",
			Err:       err,
		}
	}
	client.producer = producer

	// Create async producer
	asyncProducer, err := sarama.NewAsyncProducer(brokers, kafkaConfig)
	if err != nil {
		producer.Close()
		return nil, &MessageError{
			Operation: "async_producer",
			Topic:     "",
			Err:       err,
		}
	}
	client.asyncProducer = asyncProducer

	// Start error handler for async producer
	go func() {
		for err := range asyncProducer.Errors() {
			log.Printf("Kafka async producer error: %v", err)
		}
	}()

	return client, nil
}

// Publish publishes a message to a Kafka topic
func (c *KafkaClient) Publish(ctx context.Context, topic string, msg *Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.producer == nil {
		return &MessageError{
			Operation: "publish",
			Topic:     topic,
			Err:       fmt.Errorf("producer is nil"),
		}
	}

	// Create Kafka message
	kafkaMsg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Payload),
		Headers: []sarama.RecordHeader{},
		Timestamp: msg.Timestamp,
	}

	// Add headers
	for k, v := range msg.Headers {
		headerValue, err := json.Marshal(v)
		if err != nil {
			log.Printf("Failed to marshal header %s: %v", k, err)
			continue
		}
		kafkaMsg.Headers = append(kafkaMsg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: headerValue,
		})
	}

	// Send message
	partition, offset, err := c.producer.SendMessage(kafkaMsg)
	if err != nil {
		return &MessageError{
			Operation: "publish",
			Topic:     topic,
			Err:       err,
		}
	}

	log.Printf("Message published to topic %s, partition %d, offset %d", topic, partition, offset)
	return nil
}

// PublishAsync publishes a message asynchronously
func (c *KafkaClient) PublishAsync(ctx context.Context, topic string, msg *Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.asyncProducer == nil {
		return &MessageError{
			Operation: "publish_async",
			Topic:     topic,
			Err:       fmt.Errorf("async producer is nil"),
		}
	}

	// Create Kafka message
	kafkaMsg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Payload),
		Headers: []sarama.RecordHeader{},
		Timestamp: msg.Timestamp,
	}

	// Add headers
	for k, v := range msg.Headers {
		headerValue, err := json.Marshal(v)
		if err != nil {
			log.Printf("Failed to marshal header %s: %v", k, err)
			continue
		}
		kafkaMsg.Headers = append(kafkaMsg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: headerValue,
		})
	}

	// Send message asynchronously
	c.asyncProducer.Input() <- kafkaMsg
	return nil
}

// Subscribe subscribes to a Kafka topic
func (c *KafkaClient) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create consumer group configuration
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	kafkaConfig.Consumer.Group.Session.Timeout = 10 * time.Second
	kafkaConfig.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	// Set brokers
	brokers := []string{fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)}

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(brokers, topic, kafkaConfig)
	if err != nil {
		return &MessageError{
			Operation: "consumer_group",
			Topic:     topic,
			Err:       err,
		}
	}

	// Create consumer handler
	consumerHandler := &kafkaConsumerHandler{
		topic:   topic,
		handler: handler,
		client:  c,
	}

	c.consumers[topic] = consumerHandler
	c.consumer = consumerGroup

	// Start consuming in a goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				consumerGroup.Close()
				delete(c.consumers, topic)
				return
			default:
				err := consumerGroup.Consume(ctx, []string{topic}, consumerHandler)
				if err != nil {
					log.Printf("Error consuming from topic %s: %v", topic, err)
					time.Sleep(time.Second)
				}
			}
		}
	}()

	return nil
}

// Unsubscribe unsubscribes from a topic
func (c *KafkaClient) Unsubscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.consumers[topic]; exists {
		if c.consumer != nil {
			c.consumer.Close()
		}
		delete(c.consumers, topic)
	}

	return nil
}

// Close closes the Kafka client
func (c *KafkaClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close consumer
	if c.consumer != nil {
		c.consumer.Close()
	}

	// Close producers
	if c.producer != nil {
		c.producer.Close()
	}
	if c.asyncProducer != nil {
		c.asyncProducer.Close()
	}

	return nil
}

// kafkaConsumerHandler implements sarama.ConsumerGroupHandler
type kafkaConsumerHandler struct {
	topic   string
	handler MessageHandler
	client  *KafkaClient
}

// Setup is called at the beginning of a new session
func (h *kafkaConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is called at the end of a session
func (h *kafkaConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim processes messages from a partition
func (h *kafkaConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			// Parse headers
			headers := make(map[string]interface{})
			for _, header := range message.Headers {
				var value interface{}
				if err := json.Unmarshal(header.Value, &value); err != nil {
					log.Printf("Failed to unmarshal header %s: %v", string(header.Key), err)
					headers[string(header.Key)] = string(header.Value)
				} else {
					headers[string(header.Key)] = value
				}
			}

			// Create message object
			msg := &Message{
				Topic:     message.Topic,
				Key:       string(message.Key),
				Payload:   message.Value,
				Headers:   headers,
				Timestamp: message.Timestamp,
			}

			// Handle message
			if err := h.handler(session.Context(), msg); err != nil {
				log.Printf("Error handling message: %v", err)
				// Don't mark message as processed
			} else {
				// Mark message as processed
				session.MarkMessage(message, "")
			}

		case <-session.Context().Done():
			return nil
		}
	}
}
