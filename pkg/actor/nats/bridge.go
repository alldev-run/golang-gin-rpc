// Package nats provides an Actor-to-NATS bridge.
package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/actor"
	"github.com/alldev-run/golang-gin-rpc/pkg/messaging"
)

// Message represents a NATS message delivered to an Actor.
type Message struct {
	Subject string
	Reply   string
	Payload []byte
	Headers map[string]string
}

// Bridge connects an Actor system to a messaging client (NATS or any
// messaging.Client implementation) so that Actors can publish and subscribe.
type Bridge struct {
	system *actor.System
	client messaging.Client
	mu     sync.RWMutex
	subs   map[string]subscription
}

type subscription struct {
	subject string
	pid     actor.PID
}

// NewBridge creates an Actor/NATS bridge using the provided messaging client.
func NewBridge(system *actor.System, client messaging.Client) *Bridge {
	return &Bridge{
		system: system,
		client: client,
		subs:   make(map[string]subscription),
	}
}

// NewBridgeFromConfig creates a bridge by building a messaging client from config.
func NewBridgeFromConfig(system *actor.System, config messaging.Config) (*Bridge, error) {
	client, err := messaging.NewClient(config)
	if err != nil {
		return nil, err
	}
	return NewBridge(system, client), nil
}

// Publish sends a payload to a NATS subject.
func (b *Bridge) Publish(ctx context.Context, subject string, payload []byte, headers map[string]string) error {
	msg := &messaging.Message{
		Topic:     subject,
		Payload:   payload,
		Headers:   toInterfaceMap(headers),
		Timestamp: time.Now(),
	}
	return b.client.Publish(ctx, subject, msg)
}

// PublishAsync sends a payload to a NATS subject asynchronously.
func (b *Bridge) PublishAsync(ctx context.Context, subject string, payload []byte, headers map[string]string) error {
	msg := &messaging.Message{
		Topic:     subject,
		Payload:   payload,
		Headers:   toInterfaceMap(headers),
		Timestamp: time.Now(),
	}
	return b.client.PublishAsync(ctx, subject, msg)
}

// Subscribe registers an Actor to receive NATS messages on a subject.
// The Actor will receive *nats.Message values.
func (b *Bridge) Subscribe(ctx context.Context, subject string, pid actor.PID) error {
	b.mu.Lock()
	if _, exists := b.subs[subject]; exists {
		b.mu.Unlock()
		return fmt.Errorf("actor/nats: already subscribed to %s", subject)
	}
	// 占位，避免并发订阅同一 subject 时双重注册。
	b.subs[subject] = subscription{subject: subject, pid: pid}
	b.mu.Unlock()

	handler := func(ctx context.Context, msg *messaging.Message) error {
		return b.system.Send(pid, &Message{
			Subject: msg.Topic,
			Reply:   replyFromHeaders(msg.Headers),
			Payload: msg.Payload,
			Headers: toStringMap(msg.Headers),
		})
	}

	if err := b.client.Subscribe(ctx, subject, handler); err != nil {
		// 订阅失败，回滚占位。
		b.mu.Lock()
		delete(b.subs, subject)
		b.mu.Unlock()
		return err
	}

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subs, subject)
		b.mu.Unlock()
	}()

	return nil
}

// replyFromHeaders 从消息头中提取 NATS reply subject（如有）。
// 约定 header key 为 "nats_reply"。
func replyFromHeaders(headers map[string]interface{}) string {
	if headers == nil {
		return ""
	}
	if v, ok := headers["nats_reply"]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

// Unsubscribe removes a subscription.
func (b *Bridge) Unsubscribe(subject string) error {
	b.mu.Lock()
	delete(b.subs, subject)
	b.mu.Unlock()

	return b.client.Unsubscribe(subject)
}

// Close closes the bridge and its underlying client.
func (b *Bridge) Close() error {
	return b.client.Close()
}

// System returns the underlying Actor system.
func (b *Bridge) System() *actor.System {
	return b.system
}

// Client returns the underlying messaging client.
func (b *Bridge) Client() messaging.Client {
	return b.client
}

func toInterfaceMap(m map[string]string) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func toStringMap(m map[string]interface{}) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = fmt.Sprint(v)
	}
	return out
}
