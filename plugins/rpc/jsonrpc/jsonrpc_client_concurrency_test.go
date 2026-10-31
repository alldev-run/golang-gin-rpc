package jsonrpc

import (
	"sync"
	"testing"
)

func TestClientPoolConcurrentGetSameURL(t *testing.T) {
	pool := NewClientPool(DefaultClientConfig())
	url := "http://127.0.0.1:18080/rpc"

	const workers = 64
	clients := make([]*Client, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			clients[i] = pool.Get(url)
		}()
	}
	wg.Wait()

	if pool.Size() != 1 {
		t.Fatalf("expected pool size 1, got %d", pool.Size())
	}
	first := clients[0]
	for i := 1; i < workers; i++ {
		if clients[i] != first {
			t.Fatalf("expected shared client instance at index %d", i)
		}
	}
}

func TestClientPoolConcurrentMixedAccess(t *testing.T) {
	pool := NewClientPool(DefaultClientConfig())
	urls := []string{
		"http://127.0.0.1:18081/rpc",
		"http://127.0.0.1:18082/rpc",
		"http://127.0.0.1:18083/rpc",
	}

	const rounds = 100
	var wg sync.WaitGroup
	for i := 0; i < rounds; i++ {
		for _, u := range urls {
			u := u
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = pool.Get(u)
				_ = pool.URLs()
				_ = pool.Size()
			}()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			pool.Remove(urls[i%len(urls)])
		}
	}()
	wg.Wait()

	if pool.Size() < 0 {
		t.Fatal("pool size should never be negative")
	}
}
