package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestJSONRPCClientConcurrentCallUniqueRequestID(t *testing.T) {
	var (
		mu  sync.Mutex
		ids = map[uint64]struct{}{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req struct {
			ID uint64 `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		ids[req.ID] = struct{}{}
		mu.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  map[string]interface{}{"ok": true},
		})
	}))
	defer srv.Close()

	host, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host/port failed: %v", err)
	}

	cfg := DefaultClientConfig()
	cfg.Type = ClientTypeJSONRPC
	cfg.Host = host
	cfg.Port = mustAtoi(t, port)
	cfg.RetryCount = 1

	client := NewJSONRPCClient(cfg)

	const workers = 80
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			var out map[string]interface{}
			if err := client.Call(context.Background(), "system.ping", map[string]interface{}{"n": 1}, &out); err != nil {
				t.Errorf("call failed: %v", err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(ids) != workers {
		t.Fatalf("expected %d unique ids, got %d", workers, len(ids))
	}
}

func mustAtoi(t *testing.T, v string) int {
	t.Helper()
	var n int
	_, err := fmt.Sscanf(v, "%d", &n)
	if err != nil {
		t.Fatalf("atoi failed: %v", err)
	}
	return n
}
