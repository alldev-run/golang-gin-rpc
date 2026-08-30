package messaging

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSClient implements the Client interface for NATS.
type NATSClient struct {
	config Config
	nc     *nats.Conn
	mu     sync.RWMutex
	subs   map[string]*nats.Subscription
}

// NewNATSClient creates a new NATS client.
func NewNATSClient(config Config) (*NATSClient, error) {
	client := &NATSClient{
		config: config,
		subs:   make(map[string]*nats.Subscription),
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *NATSClient) connect() error {
	url := c.config.GetConnectionString()
	if url == "" {
		url = nats.DefaultURL
	}

	opts := []nats.Option{
		nats.Name(c.getOptionString("client_name", "golang-gin-rpc")),
		nats.Timeout(c.config.ConnectionTimeout),
		nats.ReconnectWait(c.config.ReconnectDelay),
		nats.MaxReconnects(c.config.MaxReconnectAttempts),
	}

	if noEcho, _ := c.config.Options["no_echo"].(bool); noEcho {
		opts = append(opts, nats.NoEcho())
	}

	if c.config.Username != "" {
		opts = append(opts, nats.UserInfo(c.config.Username, c.config.Password))
	}

	nc, err := nats.Connect(url, opts...)
	if err != nil {
		return &MessageError{
			Operation: "connect",
			Topic:     "",
			Err:       err,
		}
	}

	c.mu.Lock()
	c.nc = nc
	c.mu.Unlock()

	return nil
}

func (c *NATSClient) getOptionString(key, defaultVal string) string {
	if v, ok := c.config.Options[key].(string); ok {
		return v
	}
	return defaultVal
}

// Publish publishes a message to a NATS subject.
func (c *NATSClient) Publish(ctx context.Context, topic string, msg *Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.RLock()
	nc := c.nc
	c.mu.RUnlock()

	if nc == nil {
		return &MessageError{
			Operation: "publish",
			Topic:     topic,
			Err:       fmt.Errorf("not connected"),
		}
	}

	nmsg := nats.NewMsg(topic)
	nmsg.Data = msg.Payload
	if msg.Headers != nil {
		for k, v := range msg.Headers {
			nmsg.Header.Set(k, fmt.Sprint(v))
		}
	}

	if err := nc.PublishMsg(nmsg); err != nil {
		return &MessageError{
			Operation: "publish",
			Topic:     topic,
			Err:       err,
		}
	}

	if err := nc.FlushWithContext(ctx); err != nil {
		return &MessageError{
			Operation: "flush",
			Topic:     topic,
			Err:       err,
		}
	}

	return nil
}

// PublishAsync publishes a message asynchronously.
func (c *NATSClient) PublishAsync(ctx context.Context, topic string, msg *Message) error {
	go func() {
		if err := c.Publish(context.Background(), topic, msg); err != nil {
			log.Printf("NATS async publish failed: %v", err)
		}
	}()
	return nil
}

// Subscribe subscribes to a NATS subject and dispatches messages to the handler.
func (c *NATSClient) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	c.mu.Lock()
	if _, exists := c.subs[topic]; exists {
		c.mu.Unlock()
		return &MessageError{
			Operation: "subscribe",
			Topic:     topic,
			Err:       fmt.Errorf("already subscribed"),
		}
	}
	nc := c.nc
	c.mu.Unlock()

	if nc == nil {
		return &MessageError{
			Operation: "subscribe",
			Topic:     topic,
			Err:       fmt.Errorf("not connected"),
		}
	}

	sub, err := nc.Subscribe(topic, func(m *nats.Msg) {
		msg := &Message{
			Topic:     m.Subject,
			Payload:   m.Data,
			Timestamp: time.Now(),
			Headers:   make(map[string]interface{}),
		}
		for k, v := range m.Header {
			if len(v) > 0 {
				msg.Headers[k] = v[0]
			}
		}
		if err := handler(ctx, msg); err != nil {
			log.Printf("NATS handler error on %s: %v", topic, err)
		}
	})
	if err != nil {
		return &MessageError{
			Operation: "subscribe",
			Topic:     topic,
			Err:       err,
		}
	}

	c.mu.Lock()
	c.subs[topic] = sub
	c.mu.Unlock()

	go func() {
		<-ctx.Done()
		c.mu.Lock()
		if s, ok := c.subs[topic]; ok {
			s.Unsubscribe()
			delete(c.subs, topic)
		}
		c.mu.Unlock()
	}()

	return nil
}

// Unsubscribe unsubscribes from a NATS subject.
func (c *NATSClient) Unsubscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if sub, ok := c.subs[topic]; ok {
		if err := sub.Unsubscribe(); err != nil {
			return &MessageError{
				Operation: "unsubscribe",
				Topic:     topic,
				Err:       err,
			}
		}
		delete(c.subs, topic)
	}

	return nil
}

// Close closes the NATS connection and all subscriptions.
func (c *NATSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sub := range c.subs {
		sub.Unsubscribe()
	}
	c.subs = make(map[string]*nats.Subscription)

	if c.nc != nil {
		c.nc.Close()
		c.nc = nil
	}

	return nil
}
