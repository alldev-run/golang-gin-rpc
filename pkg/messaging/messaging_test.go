package messaging

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMessage(t *testing.T) {
	msg := &Message{
		Topic:     "test-topic",
		Key:       "test-key",
		Payload:   []byte("test payload"),
		Headers:   map[string]interface{}{"content-type": "application/json"},
		Timestamp: time.Now(),
	}

	if msg.Topic != "test-topic" {
		t.Errorf("Message.Topic = %v, want %v", msg.Topic, "test-topic")
	}
	if msg.Key != "test-key" {
		t.Errorf("Message.Key = %v, want %v", msg.Key, "test-key")
	}
	if string(msg.Payload) != "test payload" {
		t.Errorf("Message.Payload = %v, want %v", string(msg.Payload), "test payload")
	}
	if msg.Headers["content-type"] != "application/json" {
		t.Errorf("Message.Headers[content-type] = %v, want %v", msg.Headers["content-type"], "application/json")
	}
}

func TestMessageError(t *testing.T) {
	originalErr := errors.New("original error")
	msgErr := &MessageError{
		Operation: "publish",
		Topic:     "test-topic",
		Err:       originalErr,
	}

	expectedError := "publish failed for topic test-topic: original error"
	if msgErr.Error() != expectedError {
		t.Errorf("MessageError.Error() = %v, want %v", msgErr.Error(), expectedError)
	}

	if msgErr.Unwrap() != originalErr {
		t.Errorf("MessageError.Unwrap() = %v, want %v", msgErr.Unwrap(), originalErr)
	}

	// Test error wrapping
	if !errors.Is(msgErr, originalErr) {
		t.Error("MessageError should wrap the original error")
	}
}

func TestMessageError_NilError(t *testing.T) {
	msgErr := &MessageError{
		Operation: "publish",
		Topic:     "test-topic",
		Err:       nil,
	}

	expectedError := "publish failed for topic test-topic: "
	if msgErr.Error() != expectedError {
		t.Errorf("MessageError.Error() with nil Err = %v, want %v", msgErr.Error(), expectedError)
	}

	if msgErr.Unwrap() != nil {
		t.Errorf("MessageError.Unwrap() with nil Err = %v, want nil", msgErr.Unwrap())
	}
}

// MockPublisher implements Publisher interface for testing
type MockPublisher struct {
	publishCalled   bool
	publishAsyncCalled bool
	closeCalled     bool
	lastTopic       string
	lastMessage     *Message
	publishError    error
}

func (m *MockPublisher) Publish(ctx context.Context, topic string, msg *Message) error {
	m.publishCalled = true
	m.lastTopic = topic
	m.lastMessage = msg
	return m.publishError
}

func (m *MockPublisher) PublishAsync(ctx context.Context, topic string, msg *Message) error {
	m.publishAsyncCalled = true
	m.lastTopic = topic
	m.lastMessage = msg
	return m.publishError
}

func (m *MockPublisher) Close() error {
	m.closeCalled = true
	return nil
}

func TestPublisherInterface(t *testing.T) {
	mock := &MockPublisher{}
	
	// Test Publish
	ctx := context.Background()
	msg := &Message{
		Topic:   "test-topic",
		Payload: []byte("test"),
	}
	
	err := mock.Publish(ctx, "test-topic", msg)
	if err != nil {
		t.Errorf("Publish() error = %v, want nil", err)
	}
	
	if !mock.publishCalled {
		t.Error("Publish() was not called")
	}
	if mock.lastTopic != "test-topic" {
		t.Errorf("Publish() lastTopic = %v, want %v", mock.lastTopic, "test-topic")
	}
	if mock.lastMessage != msg {
		t.Error("Publish() lastMessage was not set correctly")
	}
	
	// Test PublishAsync
	mock.publishCalled = false
	err = mock.PublishAsync(ctx, "async-topic", msg)
	if err != nil {
		t.Errorf("PublishAsync() error = %v, want nil", err)
	}
	
	if !mock.publishAsyncCalled {
		t.Error("PublishAsync() was not called")
	}
	if mock.lastTopic != "async-topic" {
		t.Errorf("PublishAsync() lastTopic = %v, want %v", mock.lastTopic, "async-topic")
	}
	
	// Test Close
	err = mock.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	
	if !mock.closeCalled {
		t.Error("Close() was not called")
	}
}

// MockSubscriber implements Subscriber interface for testing
type MockSubscriber struct {
	subscribeCalled   bool
	unsubscribeCalled bool
	closeCalled       bool
	lastTopic         string
	lastHandler       MessageHandler
	subscribeError    error
}

func (m *MockSubscriber) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	m.subscribeCalled = true
	m.lastTopic = topic
	m.lastHandler = handler
	return m.subscribeError
}

func (m *MockSubscriber) Unsubscribe(topic string) error {
	m.unsubscribeCalled = true
	m.lastTopic = topic
	return nil
}

func (m *MockSubscriber) Close() error {
	m.closeCalled = true
	return nil
}

func TestSubscriberInterface(t *testing.T) {
	mock := &MockSubscriber{}
	
	// Test Subscribe
	ctx := context.Background()
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	
	err := mock.Subscribe(ctx, "test-topic", handler)
	if err != nil {
		t.Errorf("Subscribe() error = %v, want nil", err)
	}
	
	if !mock.subscribeCalled {
		t.Error("Subscribe() was not called")
	}
	if mock.lastTopic != "test-topic" {
		t.Errorf("Subscribe() lastTopic = %v, want %v", mock.lastTopic, "test-topic")
	}
	if mock.lastHandler == nil {
		t.Error("Subscribe() lastHandler was not set")
	}
	
	// Test Unsubscribe
	mock.subscribeCalled = false
	err = mock.Unsubscribe("test-topic")
	if err != nil {
		t.Errorf("Unsubscribe() error = %v, want nil", err)
	}
	
	if !mock.unsubscribeCalled {
		t.Error("Unsubscribe() was not called")
	}
	if mock.lastTopic != "test-topic" {
		t.Errorf("Unsubscribe() lastTopic = %v, want %v", mock.lastTopic, "test-topic")
	}
	
	// Test Close
	err = mock.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
	
	if !mock.closeCalled {
		t.Error("Close() was not called")
	}
}

// MockClient implements Client interface for testing
type MockClient struct {
	*MockPublisher
	*MockSubscriber
}

func TestClientInterface(t *testing.T) {
	mock := &MockClient{
		MockPublisher: &MockPublisher{},
		MockSubscriber: &MockSubscriber{},
	}
	
	// Test that Client implements both Publisher and Subscriber
	var _ Client = mock
	
	// Test Publisher methods
	ctx := context.Background()
	msg := &Message{Topic: "test", Payload: []byte("test")}
	
	err := mock.Publish(ctx, "test", msg)
	if err != nil {
		t.Errorf("Client.Publish() error = %v, want nil", err)
	}
	
	err = mock.PublishAsync(ctx, "test", msg)
	if err != nil {
		t.Errorf("Client.PublishAsync() error = %v, want nil", err)
	}
	
	// Test Subscriber methods
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	
	err = mock.Subscribe(ctx, "test", handler)
	if err != nil {
		t.Errorf("Client.Subscribe() error = %v, want nil", err)
	}
	
	err = mock.Unsubscribe("test")
	if err != nil {
		t.Errorf("Client.Unsubscribe() error = %v, want nil", err)
	}
	
	// Test Close
	err = mock.Close()
	if err != nil {
		t.Errorf("Client.Close() error = %v, want nil", err)
	}
}

func TestConfig(t *testing.T) {
	config := Config{
		Type:     "kafka",
		Host:     "localhost",
		Port:     9092,
		Username: "user",
		Password: "pass",
		Database: "vhost",
		Options: map[string]interface{}{
			"batch.size": 1000,
			"timeout":    "30s",
		},
	}
	
	if config.Type != "kafka" {
		t.Errorf("Config.Type = %v, want %v", config.Type, "kafka")
	}
	if config.Host != "localhost" {
		t.Errorf("Config.Host = %v, want %v", config.Host, "localhost")
	}
	if config.Port != 9092 {
		t.Errorf("Config.Port = %v, want %v", config.Port, 9092)
	}
	if config.Username != "user" {
		t.Errorf("Config.Username = %v, want %v", config.Username, "user")
	}
	if config.Password != "pass" {
		t.Errorf("Config.Password = %v, want %v", config.Password, "pass")
	}
	if config.Database != "vhost" {
		t.Errorf("Config.Database = %v, want %v", config.Database, "vhost")
	}
	if config.Options["batch.size"] != 1000 {
		t.Errorf("Config.Options[batch.size] = %v, want %v", config.Options["batch.size"], 1000)
	}
	if config.Options["timeout"] != "30s" {
		t.Errorf("Config.Options[timeout] = %v, want %v", config.Options["timeout"], "30s")
	}
}

func TestMessageHandler(t *testing.T) {
	// Test that MessageHandler is a function type
	var handler MessageHandler = func(ctx context.Context, msg *Message) error {
		return nil
	}
	
	ctx := context.Background()
	msg := &Message{
		Topic:   "test",
		Payload: []byte("test"),
	}
	
	err := handler(ctx, msg)
	if err != nil {
		t.Errorf("MessageHandler() error = %v, want nil", err)
	}
	
	// Test handler with error
	errorHandler := func(ctx context.Context, msg *Message) error {
		return errors.New("handler error")
	}
	
	err = errorHandler(ctx, msg)
	if err == nil {
		t.Error("MessageHandler() error = nil, want error")
	}
	if err.Error() != "handler error" {
		t.Errorf("MessageHandler() error = %v, want %v", err.Error(), "handler error")
	}
}

func TestPublisherError(t *testing.T) {
	mock := &MockPublisher{
		publishError: errors.New("publish failed"),
	}
	
	ctx := context.Background()
	msg := &Message{Topic: "test", Payload: []byte("test")}
	
	err := mock.Publish(ctx, "test", msg)
	if err == nil {
		t.Error("Publish() error = nil, want error")
	}
	if err.Error() != "publish failed" {
		t.Errorf("Publish() error = %v, want %v", err.Error(), "publish failed")
	}
}

func TestSubscriberError(t *testing.T) {
	mock := &MockSubscriber{
		subscribeError: errors.New("subscribe failed"),
	}
	
	ctx := context.Background()
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}
	
	err := mock.Subscribe(ctx, "test", handler)
	if err == nil {
		t.Error("Subscribe() error = nil, want error")
	}
	if err.Error() != "subscribe failed" {
		t.Errorf("Subscribe() error = %v, want %v", err.Error(), "subscribe failed")
	}
}
