package audit

import (
	"context"
	"sync"
	"testing"
	"time"
)

type countingSink struct {
	mu    sync.Mutex
	count int
}

func (c *countingSink) Write(ctx context.Context, event Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
	return nil
}

func (c *countingSink) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func TestAsyncSinkWriteAndClose(t *testing.T) {
	target := &countingSink{}
	sink := NewAsyncSink(target, AsyncSinkConfig{BufferSize: 8, Workers: 2})

	for i := 0; i < 5; i++ {
		if err := sink.Write(context.Background(), Event{Action: ActionCustom}); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	sink.Close()
	if got := target.Count(); got != 5 {
		t.Fatalf("expected 5 events, got %d", got)
	}
}

func TestAsyncSinkDropOnFull(t *testing.T) {
	target := &countingSink{}
	sink := NewAsyncSink(target, AsyncSinkConfig{BufferSize: 1, Workers: 1, DropOnFull: true})
	defer sink.Close()

	_ = sink.Write(context.Background(), Event{Action: ActionCustom})
	_ = sink.Write(context.Background(), Event{Action: ActionCustom})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := sink.Write(ctx, Event{Action: ActionCustom})
	if err != nil && err != ErrAsyncSinkBufferFull {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAsyncSinkClosed(t *testing.T) {
	target := &countingSink{}
	sink := NewAsyncSink(target, DefaultAsyncSinkConfig())
	sink.Close()

	err := sink.Write(context.Background(), Event{Action: ActionCustom})
	if err != ErrAsyncSinkClosed {
		t.Fatalf("expected ErrAsyncSinkClosed, got %v", err)
	}
}
