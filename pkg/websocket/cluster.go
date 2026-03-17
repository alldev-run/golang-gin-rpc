package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"alldev-gin-rpc/pkg/messaging"
	"github.com/google/uuid"
)

type ClusterTransport interface {
	Publish(ctx context.Context, topic string, payload []byte) error
	Subscribe(ctx context.Context, topic string, handler func(context.Context, []byte) error) error
	Close() error
}

type ClusterConfig struct {
	NodeID   string
	Topic    string
	Transport ClusterTransport
}

func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		NodeID: uuid.NewString(),
		Topic:  "websocket.cluster.events",
	}
}

type clusterEnvelope struct {
	ID        string    `json:"id"`
	SourceNode string   `json:"source_node"`
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	Payload   []byte    `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type MessagingClusterTransport struct {
	client messaging.Client
}

func NewMessagingClusterTransport(client messaging.Client) *MessagingClusterTransport {
	return &MessagingClusterTransport{client: client}
}

func (t *MessagingClusterTransport) Publish(ctx context.Context, topic string, payload []byte) error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.Publish(ctx, topic, &messaging.Message{
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
	})
}

func (t *MessagingClusterTransport) Subscribe(ctx context.Context, topic string, handler func(context.Context, []byte) error) error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.Subscribe(ctx, topic, func(msgCtx context.Context, msg *messaging.Message) error {
		if msg == nil {
			return nil
		}
		return handler(msgCtx, msg.Payload)
	})
}

func (t *MessagingClusterTransport) Close() error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.Close()
}

type InMemoryClusterTransport struct {
	bus *inMemoryClusterBus
}

type inMemoryClusterBus struct {
	mu          sync.RWMutex
	subscribers map[string][]func(context.Context, []byte) error
}

func NewInMemoryClusterBus() *InMemoryClusterTransport {
	return &InMemoryClusterTransport{bus: &inMemoryClusterBus{subscribers: make(map[string][]func(context.Context, []byte) error)}}
}

func (t *InMemoryClusterTransport) Clone() *InMemoryClusterTransport {
	if t == nil {
		return NewInMemoryClusterBus()
	}
	return &InMemoryClusterTransport{bus: t.bus}
}

func (t *InMemoryClusterTransport) Publish(ctx context.Context, topic string, payload []byte) error {
	if t == nil || t.bus == nil {
		return nil
	}
	t.bus.mu.RLock()
	handlers := append([]func(context.Context, []byte) error(nil), t.bus.subscribers[topic]...)
	t.bus.mu.RUnlock()
	for _, handler := range handlers {
		if err := handler(ctx, append([]byte(nil), payload...)); err != nil {
			return err
		}
	}
	return nil
}

func (t *InMemoryClusterTransport) Subscribe(ctx context.Context, topic string, handler func(context.Context, []byte) error) error {
	if t == nil || t.bus == nil {
		return nil
	}
	t.bus.mu.Lock()
	t.bus.subscribers[topic] = append(t.bus.subscribers[topic], handler)
	t.bus.mu.Unlock()
	go func() {
		<-ctx.Done()
		t.bus.mu.Lock()
		handlers := t.bus.subscribers[topic]
		for i, candidate := range handlers {
			if fmt.Sprintf("%p", candidate) == fmt.Sprintf("%p", handler) {
				t.bus.subscribers[topic] = append(handlers[:i], handlers[i+1:]...)
				break
			}
		}
		t.bus.mu.Unlock()
	}()
	return nil
}

func (t *InMemoryClusterTransport) Close() error { return nil }

func marshalClusterEnvelope(envelope clusterEnvelope) ([]byte, error) {
	return json.Marshal(envelope)
}

func unmarshalClusterEnvelope(payload []byte) (clusterEnvelope, error) {
	var envelope clusterEnvelope
	err := json.Unmarshal(payload, &envelope)
	return envelope, err
}
