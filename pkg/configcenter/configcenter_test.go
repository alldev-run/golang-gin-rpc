package configcenter

import (
	"context"
	"testing"
	"time"
)

func TestConfigCenterSubscribeAndSet(t *testing.T) {
	provider := NewMemoryProvider()
	cc := New(provider)
	defer cc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	received := make(chan ConfigChange, 1)
	sub, err := cc.Subscribe(ctx, "app", func(change ConfigChange) {
		received <- change
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer sub.Close()

	_, err = cc.Set(ctx, "app", "feature_x", []byte("enabled"), nil)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	select {
	case change := <-received:
		if change.Namespace != "app" || change.Key != "feature_x" {
			t.Fatalf("unexpected change: %+v", change)
		}
		if change.Change != ChangeTypeSet {
			t.Fatalf("unexpected change type: %s", change.Change)
		}
	case <-ctx.Done():
		t.Fatalf("timeout waiting for config change")
	}
}

func TestConfigCenterRejectsOperationsAfterClose(t *testing.T) {
	provider := NewMemoryProvider()
	cc := New(provider)

	if err := cc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	ctx := context.Background()
	if _, _, err := cc.Get(ctx, "app", "k"); err == nil {
		t.Fatalf("expected get error after close")
	}
	if _, err := cc.Set(ctx, "app", "k", []byte("v"), nil); err == nil {
		t.Fatalf("expected set error after close")
	}
	if err := cc.Delete(ctx, "app", "k"); err == nil {
		t.Fatalf("expected delete error after close")
	}
	if _, err := cc.Subscribe(ctx, "app", func(ConfigChange) {}); err == nil {
		t.Fatalf("expected subscribe error after close")
	}
}

func TestMemoryProviderCloseIdempotent(t *testing.T) {
	provider := NewMemoryProvider()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := provider.Watch(ctx, "app")
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}

	if err := provider.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := provider.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}
