package messaging

import (
	"context"
	"errors"
	"strings"
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

func (m *MockClient) Close() error {
	if m.MockPublisher != nil {
		m.MockPublisher.closeCalled = true
	}
	if m.MockSubscriber != nil {
		m.MockSubscriber.closeCalled = true
	}
	return nil
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
	if !mock.MockPublisher.closeCalled {
		t.Error("Client.Close() should close publisher")
	}
	if !mock.MockSubscriber.closeCalled {
		t.Error("Client.Close() should close subscriber")
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

// ---------------------------------------------------------------------------
// MessageType 方法测试
// ---------------------------------------------------------------------------

func TestMessageType_IsValid(t *testing.T) {
	valid := []MessageType{
		MessageTypeRabbitMQ,
		MessageTypeKafka,
		MessageTypeNATS,
		MessageTypeRedis,
		MessageTypeMemory,
	}
	for _, mt := range valid {
		if !mt.IsValid() {
			t.Errorf("MessageType(%q).IsValid() = false, want true", mt)
		}
	}

	if MessageType("unknown").IsValid() {
		t.Error("unknown MessageType should not be valid")
	}
}

func TestMessageType_String(t *testing.T) {
	if MessageTypeRabbitMQ.String() != string(MessageTypeRabbitMQ) {
		t.Errorf("MessageTypeRabbitMQ.String() = %q, want %q", MessageTypeRabbitMQ.String(), string(MessageTypeRabbitMQ))
	}
}

func TestMessageType_DisplayName(t *testing.T) {
	cases := map[MessageType]string{
		MessageTypeRabbitMQ: "RabbitMQ",
		MessageTypeKafka:    "Apache Kafka",
		MessageTypeNATS:     "NATS",
		MessageTypeRedis:    "Redis",
		MessageTypeMemory:   "Memory",
		MessageType("xxx"):  "Unknown",
	}
	for mt, want := range cases {
		if got := mt.DisplayName(); got != want {
			t.Errorf("MessageType(%q).DisplayName() = %q, want %q", mt, got, want)
		}
	}
}

func TestMessageType_DefaultPort(t *testing.T) {
	cases := map[MessageType]int{
		MessageTypeRabbitMQ: 5672,
		MessageTypeKafka:    9092,
		MessageTypeNATS:     4222,
		MessageTypeRedis:    6379,
		MessageTypeMemory:   0,
		MessageType("xxx"):  0,
	}
	for mt, want := range cases {
		if got := mt.DefaultPort(); got != want {
			t.Errorf("MessageType(%q).DefaultPort() = %d, want %d", mt, got, want)
		}
	}
}

func TestMessageType_IsCloudBased(t *testing.T) {
	for _, mt := range GetSupportedTypes() {
		if mt.IsCloudBased() {
			t.Errorf("MessageType(%q).IsCloudBased() = true, want false", mt)
		}
	}
}

func TestMessageType_IsOpenSource(t *testing.T) {
	openSource := []MessageType{
		MessageTypeRabbitMQ,
		MessageTypeKafka,
		MessageTypeNATS,
		MessageTypeRedis,
	}
	for _, mt := range openSource {
		if !mt.IsOpenSource() {
			t.Errorf("MessageType(%q).IsOpenSource() = false, want true", mt)
		}
	}
	if MessageTypeMemory.IsOpenSource() {
		t.Error("MessageTypeMemory.IsOpenSource() = true, want false")
	}
}

func TestMessageType_IsImplemented(t *testing.T) {
	implemented := []MessageType{
		MessageTypeRabbitMQ,
		MessageTypeKafka,
		MessageTypeNATS,
	}
	for _, mt := range implemented {
		if !mt.IsImplemented() {
			t.Errorf("MessageType(%q).IsImplemented() = false, want true", mt)
		}
	}
	notImplemented := []MessageType{
		MessageTypeRedis,
		MessageTypeMemory,
	}
	for _, mt := range notImplemented {
		if mt.IsImplemented() {
			t.Errorf("MessageType(%q).IsImplemented() = true, want false", mt)
		}
	}
}

func TestMessageType_RequiresAuthentication(t *testing.T) {
	requireAuth := []MessageType{MessageTypeRabbitMQ, MessageTypeKafka}
	noAuth := []MessageType{MessageTypeNATS, MessageTypeRedis, MessageTypeMemory}
	for _, mt := range requireAuth {
		if !mt.RequiresAuthentication() {
			t.Errorf("MessageType(%q).RequiresAuthentication() = false, want true", mt)
		}
	}
	for _, mt := range noAuth {
		if mt.RequiresAuthentication() {
			t.Errorf("MessageType(%q).RequiresAuthentication() = true, want false", mt)
		}
	}
}

// ---------------------------------------------------------------------------
// 类型集合函数测试
// ---------------------------------------------------------------------------

func TestGetSupportedTypes(t *testing.T) {
	types := GetSupportedTypes()
	if len(types) != 5 {
		t.Errorf("GetSupportedTypes() returned %d types, want 5", len(types))
	}
	seen := make(map[MessageType]bool, len(types))
	for _, mt := range types {
		seen[mt] = true
	}
	for _, expected := range []MessageType{MessageTypeRabbitMQ, MessageTypeKafka, MessageTypeNATS, MessageTypeRedis, MessageTypeMemory} {
		if !seen[expected] {
			t.Errorf("GetSupportedTypes() missing %q", expected)
		}
	}
}

func TestGetImplementedTypes(t *testing.T) {
	types := GetImplementedTypes()
	if len(types) != 3 {
		t.Errorf("GetImplementedTypes() returned %d types, want 3", len(types))
	}
	for _, mt := range types {
		if !mt.IsImplemented() {
			t.Errorf("GetImplementedTypes() returned unimplemented type %q", mt)
		}
	}
}

func TestGetFutureTypes(t *testing.T) {
	types := GetFutureTypes()
	// 当前仅 Redis 列为未来类型。
	if len(types) != 1 {
		t.Errorf("GetFutureTypes() returned %d types, want 1", len(types))
	}
	if len(types) > 0 && types[0] != MessageTypeRedis {
		t.Errorf("GetFutureTypes()[0] = %q, want %q", types[0], MessageTypeRedis)
	}
}

// ---------------------------------------------------------------------------
// ParseMessageType 测试
// ---------------------------------------------------------------------------

func TestParseMessageType(t *testing.T) {
	cases := map[string]MessageType{
		"rabbitmq": MessageTypeRabbitMQ,
		"amqp":     MessageTypeRabbitMQ,
		"kafka":    MessageTypeKafka,
		"nats":     MessageTypeNATS,
		"redis":    MessageTypeRedis,
		"memory":   MessageTypeMemory,
		// 大小写与空格应被规范化
		"  RabbitMQ ": MessageTypeRabbitMQ,
		"KAFKA":       MessageTypeKafka,
	}
	for input, want := range cases {
		got, err := ParseMessageType(input)
		if err != nil {
			t.Errorf("ParseMessageType(%q) error = %v, want nil", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseMessageType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseMessageType_Invalid(t *testing.T) {
	_, err := ParseMessageType("unknown-broker")
	if err == nil {
		t.Fatal("ParseMessageType() error = nil, want error for unknown type")
	}
	if !strings.Contains(err.Error(), "unsupported messaging type") {
		t.Errorf("ParseMessageType() error = %q, want contains %q", err.Error(), "unsupported messaging type")
	}
}

// ---------------------------------------------------------------------------
// Capabilities 测试
// ---------------------------------------------------------------------------

func TestGetCapabilities(t *testing.T) {
	// Kafka 应具备全部能力。
	kafka := GetCapabilities(MessageTypeKafka)
	if !kafka.SupportsPublish || !kafka.SupportsSubscribe || !kafka.SupportsPartitioning ||
		!kafka.SupportsOrdering || !kafka.SupportsPersistence || !kafka.SupportsTransactions ||
		!kafka.SupportsDLQ || !kafka.SupportsReplay || !kafka.SupportsCompression || !kafka.SupportsHeaders {
		t.Errorf("Kafka capabilities incomplete: %+v", kafka)
	}

	// Memory 应不支持持久化/事务/DLQ/重放/压缩/headers。
	mem := GetCapabilities(MessageTypeMemory)
	if mem.SupportsPersistence || mem.SupportsTransactions || mem.SupportsDLQ ||
		mem.SupportsReplay || mem.SupportsCompression || mem.SupportsHeaders {
		t.Errorf("Memory capabilities should be minimal: %+v", mem)
	}

	// 未知类型应返回零值。
	unknown := GetCapabilities(MessageType("xxx"))
	if unknown != (MessagingCapabilities{}) {
		t.Errorf("Unknown capabilities = %+v, want zero value", unknown)
	}
}

func TestGetClientCapabilities(t *testing.T) {
	// GetClientCapabilities 应与 GetCapabilities 等价。
	for _, mt := range GetSupportedTypes() {
		if GetClientCapabilities(mt) != GetCapabilities(mt) {
			t.Errorf("GetClientCapabilities(%q) != GetCapabilities(%q)", mt, mt)
		}
	}
}

// ---------------------------------------------------------------------------
// Config 构造函数测试
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Type != MessageTypeRabbitMQ {
		t.Errorf("DefaultConfig().Type = %q, want %q", cfg.Type, MessageTypeRabbitMQ)
	}
	if cfg.Host != "localhost" {
		t.Errorf("DefaultConfig().Host = %q, want localhost", cfg.Host)
	}
	if cfg.Port != 5672 {
		t.Errorf("DefaultConfig().Port = %d, want 5672", cfg.Port)
	}
	if !cfg.Enabled {
		t.Error("DefaultConfig().Enabled = false, want true")
	}
	if cfg.Options == nil {
		t.Error("DefaultConfig().Options = nil, want non-nil")
	}
}

func TestRabbitMQConfig(t *testing.T) {
	cfg := RabbitMQConfig("rabbit.example.com", 5672)
	if cfg.Type != MessageTypeRabbitMQ {
		t.Errorf("RabbitMQConfig().Type = %q, want %q", cfg.Type, MessageTypeRabbitMQ)
	}
	if cfg.Host != "rabbit.example.com" {
		t.Errorf("RabbitMQConfig().Host = %q, want rabbit.example.com", cfg.Host)
	}
	if cfg.Port != 5672 {
		t.Errorf("RabbitMQConfig().Port = %d, want 5672", cfg.Port)
	}
	if cfg.DefaultExchange != "amq.direct" {
		t.Errorf("RabbitMQConfig().DefaultExchange = %q, want amq.direct", cfg.DefaultExchange)
	}
}

func TestKafkaConfig(t *testing.T) {
	cfg := KafkaConfig([]string{"kafka1:9092", "kafka2:9092"})
	if cfg.Type != MessageTypeKafka {
		t.Errorf("KafkaConfig().Type = %q, want %q", cfg.Type, MessageTypeKafka)
	}
	if cfg.Host != "kafka1:9092" {
		t.Errorf("KafkaConfig().Host = %q, want kafka1:9092", cfg.Host)
	}
	brokers, ok := cfg.Options["brokers"]
	if !ok {
		t.Fatal("KafkaConfig().Options[brokers] missing")
	}
	if _, ok := brokers.([]string); !ok {
		t.Errorf("KafkaConfig().Options[brokers] type = %T, want []string", brokers)
	}
}

func TestNATSConfig(t *testing.T) {
	cfg := NATSConfig("nats.example.com", 4222)
	if cfg.Type != MessageTypeNATS {
		t.Errorf("NATSConfig().Type = %q, want %q", cfg.Type, MessageTypeNATS)
	}
	if cfg.Host != "nats.example.com" {
		t.Errorf("NATSConfig().Host = %q, want nats.example.com", cfg.Host)
	}
	if cfg.DeliveryMode != DeliveryModeTransient {
		t.Errorf("NATSConfig().DeliveryMode = %q, want %q", cfg.DeliveryMode, DeliveryModeTransient)
	}
}

func TestRedisConfig(t *testing.T) {
	cfg := RedisConfig("redis.example.com", 6379)
	if cfg.Type != MessageTypeRedis {
		t.Errorf("RedisConfig().Type = %q, want %q", cfg.Type, MessageTypeRedis)
	}
	if cfg.Database != "0" {
		t.Errorf("RedisConfig().Database = %q, want 0", cfg.Database)
	}
}

func TestMemoryConfig(t *testing.T) {
	cfg := MemoryConfig()
	if cfg.Type != MessageTypeMemory {
		t.Errorf("MemoryConfig().Type = %q, want %q", cfg.Type, MessageTypeMemory)
	}
	if cfg.Host != "" {
		t.Errorf("MemoryConfig().Host = %q, want empty", cfg.Host)
	}
	if cfg.Port != 0 {
		t.Errorf("MemoryConfig().Port = %d, want 0", cfg.Port)
	}
}

func TestDevelopmentConfig(t *testing.T) {
	cfg := DevelopmentConfig()
	if cfg.Type != MessageTypeMemory {
		t.Errorf("DevelopmentConfig().Type = %q, want %q", cfg.Type, MessageTypeMemory)
	}
	if cfg.DefaultQueue != "dev-queue" {
		t.Errorf("DevelopmentConfig().DefaultQueue = %q, want dev-queue", cfg.DefaultQueue)
	}
}

func TestProductionConfig(t *testing.T) {
	cases := []MessageType{MessageTypeRabbitMQ, MessageTypeKafka, MessageTypeNATS, MessageTypeRedis}
	for _, mt := range cases {
		cfg := ProductionConfig(mt, "prod.example.com", 1234)
		if cfg.Type != mt {
			t.Errorf("ProductionConfig(%q).Type = %q, want %q", mt, cfg.Type, mt)
		}
		if cfg.Host != "prod.example.com" {
			t.Errorf("ProductionConfig(%q).Host = %q, want prod.example.com", mt, cfg.Host)
		}
	}
	// 未知类型应回退到 RabbitMQ。
	cfg := ProductionConfig(MessageType("xxx"), "prod.example.com", 1234)
	if cfg.Type != MessageTypeRabbitMQ {
		t.Errorf("ProductionConfig(unknown).Type = %q, want %q", cfg.Type, MessageTypeRabbitMQ)
	}
}

// ---------------------------------------------------------------------------
// Config 方法测试
// ---------------------------------------------------------------------------

func TestConfig_Validate(t *testing.T) {
	// Validate 不修改原配置（值接收者），仅返回 nil；验证不会报错。
	cfg := Config{Type: MessageTypeRabbitMQ, Host: "localhost", Port: 5672}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Config.Validate() error = %v, want nil", err)
	}

	// 空 Type 应被默认为 RabbitMQ 且不报错。
	emptyType := Config{Host: "localhost"}
	if err := emptyType.Validate(); err != nil {
		t.Errorf("Config.Validate() with empty Type error = %v, want nil", err)
	}

	// Memory 类型允许空 Host。
	mem := Config{Type: MessageTypeMemory}
	if err := mem.Validate(); err != nil {
		t.Errorf("Memory Config.Validate() error = %v, want nil", err)
	}
}

func TestConfig_GetConnectionString(t *testing.T) {
	cases := []struct {
		cfg  Config
		want string
	}{
		{Config{Type: MessageTypeRabbitMQ, Host: "h", Port: 5672, Username: "u", Password: "p", Database: "/"}, "amqp://u:p@h:5672/"},
		{Config{Type: MessageTypeKafka, Host: "h", Port: 9092}, "h:9092"},
		{Config{Type: MessageTypeNATS, Host: "h", Port: 4222}, "nats://h:4222"},
		{Config{Type: MessageTypeRedis, Host: "h", Port: 6379, Database: "0"}, "redis://h:6379/0"},
		{Config{Type: MessageTypeMemory}, ""},
	}
	for _, c := range cases {
		if got := c.cfg.GetConnectionString(); got != c.want {
			t.Errorf("Config(%q).GetConnectionString() = %q, want %q", c.cfg.Type, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Factory / ValidateConfig 测试（错误路径，无需真实 broker）
// ---------------------------------------------------------------------------

func TestNewClient_InvalidType(t *testing.T) {
	_, err := NewClient(Config{Type: MessageType("unknown")})
	if err == nil {
		t.Fatal("NewClient() with invalid type error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid messaging type") {
		t.Errorf("NewClient() error = %q, want contains %q", err.Error(), "invalid messaging type")
	}
}

func TestNewClient_UnimplementedType(t *testing.T) {
	for _, mt := range []MessageType{MessageTypeRedis, MessageTypeMemory} {
		_, err := NewClient(Config{Type: mt, Host: "localhost", Port: mt.DefaultPort()})
		if err == nil {
			t.Errorf("NewClient(%q) error = nil, want error (not implemented)", mt)
			continue
		}
		if !strings.Contains(err.Error(), "not yet implemented") {
			t.Errorf("NewClient(%q) error = %q, want contains %q", mt, err.Error(), "not yet implemented")
		}
	}
}

func TestNewPublisher(t *testing.T) {
	// 无效类型应直接报错。
	_, err := NewPublisher(Config{Type: MessageType("unknown")})
	if err == nil {
		t.Fatal("NewPublisher() with invalid type error = nil, want error")
	}
}

func TestNewSubscriber(t *testing.T) {
	// 无效类型应直接报错。
	_, err := NewSubscriber(Config{Type: MessageType("unknown")})
	if err == nil {
		t.Fatal("NewSubscriber() with invalid type error = nil, want error")
	}
}

func TestCreateClientByType_Invalid(t *testing.T) {
	_, err := CreateClientByType(MessageType("unknown"), Config{})
	if err == nil {
		t.Fatal("CreateClientByType() with invalid type error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid message type") {
		t.Errorf("CreateClientByType() error = %q, want contains %q", err.Error(), "invalid message type")
	}
}

func TestCreateClientByType_Unimplemented(t *testing.T) {
	_, err := CreateClientByType(MessageTypeRedis, Config{Host: "localhost", Port: 6379})
	if err == nil {
		t.Fatal("CreateClientByType(Redis) error = nil, want error (not implemented)")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("CreateClientByType(Redis) error = %q, want contains %q", err.Error(), "not yet implemented")
	}
}

func TestValidateConfig_InvalidType(t *testing.T) {
	err := ValidateConfig(Config{Type: MessageType("unknown")})
	if err == nil {
		t.Fatal("ValidateConfig() with invalid type error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid messaging type") {
		t.Errorf("ValidateConfig() error = %q, want contains %q", err.Error(), "invalid messaging type")
	}
}

func TestValidateConfig_Unimplemented(t *testing.T) {
	err := ValidateConfig(Config{Type: MessageTypeRedis, Host: "localhost", Port: 6379})
	if err == nil {
		t.Fatal("ValidateConfig(Redis) error = nil, want error (not implemented)")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("ValidateConfig(Redis) error = %q, want contains %q", err.Error(), "not implemented")
	}
}

func TestValidateConfig_RabbitMQMissingCredentials(t *testing.T) {
	// RabbitMQ 缺少 username 应报错。
	err := ValidateConfig(Config{Type: MessageTypeRabbitMQ, Host: "localhost", Port: 5672, Password: "guest"})
	if err == nil {
		t.Fatal("ValidateConfig(RabbitMQ without username) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "username") {
		t.Errorf("ValidateConfig() error = %q, want contains %q", err.Error(), "username")
	}

	// 缺少 password 应报错。
	err = ValidateConfig(Config{Type: MessageTypeRabbitMQ, Host: "localhost", Port: 5672, Username: "guest"})
	if err == nil {
		t.Fatal("ValidateConfig(RabbitMQ without password) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("ValidateConfig() error = %q, want contains %q", err.Error(), "password")
	}
}

func TestValidateConfig_MissingHost(t *testing.T) {
	for _, mt := range []MessageType{MessageTypeRabbitMQ, MessageTypeKafka, MessageTypeNATS} {
		err := ValidateConfig(Config{Type: mt, Port: mt.DefaultPort(), Username: "u", Password: "p"})
		if err == nil {
			t.Errorf("ValidateConfig(%q without host) error = nil, want error", mt)
			continue
		}
		if !strings.Contains(err.Error(), "requires host") {
			t.Errorf("ValidateConfig(%q) error = %q, want contains %q", mt, err.Error(), "requires host")
		}
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	// 完整的 RabbitMQ 配置应通过。
	cfg := Config{
		Type:     MessageTypeRabbitMQ,
		Host:     "localhost",
		Port:     5672,
		Username: "guest",
		Password: "guest",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("ValidateConfig(valid RabbitMQ) error = %v, want nil", err)
	}

	// Kafka 仅需 host。
	if err := ValidateConfig(Config{Type: MessageTypeKafka, Host: "localhost", Port: 9092}); err != nil {
		t.Errorf("ValidateConfig(valid Kafka) error = %v, want nil", err)
	}

	// NATS 仅需 host。
	if err := ValidateConfig(Config{Type: MessageTypeNATS, Host: "localhost", Port: 4222}); err != nil {
		t.Errorf("ValidateConfig(valid NATS) error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// DeliveryMode 常量测试
// ---------------------------------------------------------------------------

func TestDeliveryMode(t *testing.T) {
	if DeliveryModeTransient != "transient" {
		t.Errorf("DeliveryModeTransient = %q, want transient", DeliveryModeTransient)
	}
	if DeliveryModePersistent != "persistent" {
		t.Errorf("DeliveryModePersistent = %q, want persistent", DeliveryModePersistent)
	}
}
