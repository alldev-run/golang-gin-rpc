package nats

import (
	"context"
	"sync"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/actor"
	"github.com/alldev-run/golang-gin-rpc/pkg/messaging"
	"github.com/stretchr/testify/assert"
)

type mockClient struct {
	mu          sync.Mutex
	published   []*messaging.Message
	subscribed  map[string]messaging.MessageHandler
	unsubscribed []string
	closed      bool
	publishErr  error
	subscribeErr error
}

func newMockClient() *mockClient {
	return &mockClient{
		subscribed: make(map[string]messaging.MessageHandler),
	}
}

func (m *mockClient) Publish(ctx context.Context, topic string, msg *messaging.Message) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.mu.Lock()
	m.published = append(m.published, msg)
	m.mu.Unlock()
	return nil
}

func (m *mockClient) PublishAsync(ctx context.Context, topic string, msg *messaging.Message) error {
	return m.Publish(ctx, topic, msg)
}

func (m *mockClient) Subscribe(ctx context.Context, topic string, handler messaging.MessageHandler) error {
	if m.subscribeErr != nil {
		return m.subscribeErr
	}
	m.mu.Lock()
	m.subscribed[topic] = handler
	m.mu.Unlock()
	return nil
}

func (m *mockClient) Unsubscribe(topic string) error {
	m.mu.Lock()
	m.unsubscribed = append(m.unsubscribed, topic)
	delete(m.subscribed, topic)
	m.mu.Unlock()
	return nil
}

func (m *mockClient) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	return nil
}

type msgActor struct {
	msgs chan actor.Message
}

func (a *msgActor) Receive(ctx actor.Context, msg actor.Message) {
	a.msgs <- msg
}

func TestBridge_Publish(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockClient()
	bridge := NewBridge(sys, mock)

	err := bridge.Publish(context.Background(), "orders", []byte("hello"), map[string]string{"id": "1"})
	assert.NoError(t, err)

	mock.mu.Lock()
	assert.Len(t, mock.published, 1)
	published := mock.published[0]
	mock.mu.Unlock()

	assert.Equal(t, "orders", published.Topic)
	assert.Equal(t, []byte("hello"), published.Payload)
	assert.Equal(t, "1", published.Headers["id"])
}

func TestBridge_SubscribeAndForward(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	a := &msgActor{msgs: make(chan actor.Message, 1)}
	pid, err := sys.SpawnNamed("receiver", a)
	assert.NoError(t, err)

	mock := newMockClient()
	bridge := NewBridge(sys, mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = bridge.Subscribe(ctx, "orders", pid)
	assert.NoError(t, err)

	mock.mu.Lock()
	handler := mock.subscribed["orders"]
	mock.mu.Unlock()
	assert.NotNil(t, handler)

	err = handler(ctx, &messaging.Message{
		Topic:     "orders",
		Payload:   []byte("world"),
		Headers:   map[string]interface{}{"id": "2"},
	})
	assert.NoError(t, err)

	select {
	case received := <-a.msgs:
		nm, ok := received.(*Message)
		assert.True(t, ok)
		assert.Equal(t, "orders", nm.Subject)
		assert.Equal(t, []byte("world"), nm.Payload)
		assert.Equal(t, "2", nm.Headers["id"])
	default:
		t.Fatal("actor did not receive NATS message")
	}
}

func TestBridge_Unsubscribe(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	a := &msgActor{msgs: make(chan actor.Message)}
	pid, err := sys.SpawnNamed("receiver", a)
	assert.NoError(t, err)

	mock := newMockClient()
	bridge := NewBridge(sys, mock)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = bridge.Subscribe(ctx, "orders", pid)
	assert.NoError(t, err)

	err = bridge.Unsubscribe("orders")
	assert.NoError(t, err)

	mock.mu.Lock()
	assert.Contains(t, mock.unsubscribed, "orders")
	mock.mu.Unlock()
}

func TestBridge_Close(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockClient()
	bridge := NewBridge(sys, mock)

	err := bridge.Close()
	assert.NoError(t, err)

	mock.mu.Lock()
	assert.True(t, mock.closed)
	mock.mu.Unlock()
}
