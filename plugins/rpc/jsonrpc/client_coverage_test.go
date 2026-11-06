package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJSONRPCClientOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request method/content type = %s/%s", r.Method, r.Header.Get("Content-Type"))
		}
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		switch request.Method {
		case "fail":
			_ = json.NewEncoder(w).Encode(Response{
				JSONRPC: "2.0", ID: request.ID, Error: &RPCError{Code: -32601, Message: "not found"},
			})
		default:
			_ = json.NewEncoder(w).Encode(Response{JSONRPC: "2.0", ID: request.ID, Result: map[string]bool{"ok": true}})
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		URL: server.URL, Timeout: time.Second, Headers: map[string]string{"X-Test": "configured"},
		UserAgent: "coverage-test", MaxRetries: 1, RetryDelay: time.Millisecond,
	})
	var result map[string]bool
	if err := client.Call(context.Background(), "success", map[string]string{"key": "value"}, &result); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if !result["ok"] {
		t.Fatalf("Call() result = %v", result)
	}
	if err := client.CallWithID(context.Background(), "success", nil, &result, "request-1"); err != nil {
		t.Fatalf("CallWithID() error = %v", err)
	}
	if err := client.Call(context.Background(), "fail", nil, nil); err == nil {
		t.Fatal("Call() succeeded for a JSON-RPC error response")
	}
	if err := client.CallWithRetry(context.Background(), "fail", nil, nil); err == nil {
		t.Fatal("CallWithRetry() succeeded for a non-retryable method error")
	}
	if err := client.Notify(context.Background(), "notify", nil); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
}

func TestJSONRPCBatchAndHTTPFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/failure" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`[{"jsonrpc":"2.0","result":1,"id":1},{"jsonrpc":"2.0","result":2,"id":2}]`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{URL: server.URL, Timeout: time.Second})
	responses, err := client.CallBatch(context.Background(), BatchRequest{
		{JSONRPC: "2.0", Method: "one", ID: 1},
		{JSONRPC: "2.0", Method: "two", ID: 2},
	})
	if err != nil || len(responses) != 2 {
		t.Fatalf("CallBatch() = %v, %v", responses, err)
	}
	client.config.URL = server.URL + "/failure"
	if _, err := client.CallBatch(context.Background(), nil); err == nil {
		t.Fatal("CallBatch() succeeded for an HTTP error")
	}
	if err := client.Call(context.Background(), "call", nil, nil); err == nil {
		t.Fatal("Call() succeeded for an HTTP error")
	}
}
