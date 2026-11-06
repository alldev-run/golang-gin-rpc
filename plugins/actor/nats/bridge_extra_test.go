package nats

import (
	"context"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/plugins/actor"
	"github.com/alldev-run/golang-gin-rpc/plugins/messaging"
	"github.com/stretchr/testify/assert"
)

func TestBridge_PublishAsync(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockClient()
	bridge := NewBridge(sys, mock)

	err := bridge.PublishAsync(context.Background(), "events", []byte("async"), map[string]string{"k": "v"})
	assert.NoError(t, err)

	mock.mu.Lock()
	published := mock.published
	mock.mu.Unlock()

	if assert.Len(t, published, 1) {
		assert.Equal(t, "events", published[0].Topic)
		assert.Equal(t, []byte("async"), published[0].Payload)
		assert.Equal(t, "v", published[0].Headers["k"])
	}
}

func TestBridge_PublishAsync_PropagatesError(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockClient()
	mock.publishErr = assert.AnError
	bridge := NewBridge(sys, mock)

	assert.ErrorIs(t, bridge.PublishAsync(context.Background(), "t", nil, nil), assert.AnError)
}

func TestBridge_Accessors(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockClient()
	bridge := NewBridge(sys, mock)

	assert.Same(t, sys, bridge.System())
	assert.Same(t, mock, bridge.Client())
}

func TestNewBridgeFromConfig_InvalidConfig(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	_, err := NewBridgeFromConfig(sys, messaging.Config{Type: messaging.MessageType("bogus")})
	assert.Error(t, err, "invalid messaging type should surface construction errors")
}
