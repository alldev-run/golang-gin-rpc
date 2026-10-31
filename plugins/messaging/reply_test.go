package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// memClient 是测试用内存消息总线,实现 Client 接口。
type memClient struct {
	mu      sync.RWMutex
	subs    map[string][]MessageHandler
	latency time.Duration
}

func newMemClient(latency time.Duration) *memClient {
	return &memClient{subs: make(map[string][]MessageHandler), latency: latency}
}

func (m *memClient) Publish(_ context.Context, topic string, msg *Message) error {
	m.mu.RLock()
	handlers := append([]MessageHandler(nil), m.subs[topic]...)
	m.mu.RUnlock()
	go func() {
		if m.latency > 0 {
			time.Sleep(m.latency)
		}
		for _, h := range handlers {
			_ = h(context.Background(), msg)
		}
	}()
	return nil
}

func (m *memClient) PublishAsync(_ context.Context, topic string, msg *Message) error {
	return m.Publish(context.Background(), topic, msg)
}

func (m *memClient) Subscribe(_ context.Context, topic string, handler MessageHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[topic] = append(m.subs[topic], handler)
	return nil
}

func (m *memClient) Unsubscribe(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subs, topic)
	return nil
}

func (m *memClient) Close() error { return nil }

func TestRequestReplyRoundTrip(t *testing.T) {
	bus := newMemClient(5 * time.Millisecond)
	ctx := context.Background()

	// 应答方:处理 "calc.double" 请求。
	if err := bus.Subscribe(ctx, "calc.double", HandleRequests(bus, func(_ context.Context, payload []byte) ([]byte, error) {
		var n int
		if err := decodeInt(payload, &n); err != nil {
			return nil, err
		}
		return encodeInt(n * 2), nil
	})); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	out, err := Request(ctx, bus, "calc.double", encodeInt(21), time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	var got int
	if err := decodeInt(out, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestRequestRemoteError(t *testing.T) {
	bus := newMemClient(0)
	ctx := context.Background()
	_ = bus.Subscribe(ctx, "svc.fail", HandleRequests(bus, func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, errors.New("boom")
	}))

	_, err := Request(ctx, bus, "svc.fail", []byte("{}"), time.Second)
	var rerr *ReplyError
	if !errors.As(err, &rerr) {
		t.Fatalf("expected ReplyError, got %v", err)
	}
	if rerr.Error() != "messaging: remote error: boom" {
		t.Fatalf("unexpected error text: %s", rerr.Error())
	}
}

func TestRequestTimeout(t *testing.T) {
	bus := newMemClient(0) // 没有应答方
	ctx := context.Background()
	start := time.Now()
	_, err := Request(ctx, bus, "nobody.home", []byte("x"), 80*time.Millisecond)
	if !errors.Is(err, ErrRequestTimeout) {
		t.Fatalf("expected ErrRequestTimeout, got %v", err)
	}
	if time.Since(start) < 70*time.Millisecond {
		t.Fatal("returned before timeout")
	}
}

func TestRequestJSON(t *testing.T) {
	bus := newMemClient(0)
	ctx := context.Background()
	_ = bus.Subscribe(ctx, "svc.hello", HandleRequests(bus, func(_ context.Context, payload []byte) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"hello":%s}`, string(payload))), nil
	}))
	out, err := RequestJSON(ctx, bus, "svc.hello", "world", time.Second)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if string(out) != `{"hello":"world"}` {
		t.Fatalf("unexpected reply: %s", out)
	}
}

// encodeInt/decodeInt 是测试用的最小 JSON 编解码。
func encodeInt(n int) []byte { return []byte(fmt.Sprintf("%d", n)) }
func decodeInt(raw []byte, out *int) error {
	_, err := fmt.Sscanf(string(raw), "%d", out)
	return err
}
