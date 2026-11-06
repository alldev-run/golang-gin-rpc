package gateway

import (
	"sync"
	"testing"
)

func TestJSONRPCProxyGetOrCreateConcurrentSingleInstance(t *testing.T) {
	cfg := DefaultConfig()
	gw := NewGateway(cfg)
	proxy := NewJSONRPCProxy(gw)

	target := "http://127.0.0.1:19090/rpc"
	const workers = 64
	clients := make([]interface{}, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			client, err := proxy.getOrCreateJSONRPCClient(target)
			if err != nil {
				t.Errorf("getOrCreateJSONRPCClient failed: %v", err)
				return
			}
			clients[i] = client
		}()
	}
	wg.Wait()

	proxy.mu.RLock()
	defer proxy.mu.RUnlock()
	if len(proxy.clients) != 1 {
		t.Fatalf("expected exactly 1 cached client, got %d", len(proxy.clients))
	}

	first := clients[0]
	for i := 1; i < workers; i++ {
		if clients[i] != first {
			t.Fatalf("expected same client instance at index %d", i)
		}
	}
}
