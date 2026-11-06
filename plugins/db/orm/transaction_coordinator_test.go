package orm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPCoordinatorProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/transactions":
			var request transactionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode begin request: %v", err)
			}
			if request.TxID != "tx-1" {
				t.Errorf("tx_id = %q, want tx-1", request.TxID)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/transactions/tx-1/commit":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/transactions/tx-1/rollback":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/transactions/"):
			txID := strings.TrimPrefix(r.URL.Path, "/transactions/")
			_ = json.NewEncoder(w).Encode(transactionResponse{Status: TransactionStatus(txID)})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	coordinator := NewHTTPCoordinator(server.URL)
	if coordinator.client.Timeout == 0 {
		t.Fatal("default client should have a timeout")
	}
	ctx := context.Background()
	if err := coordinator.BeginTransaction(ctx, "tx-1"); err != nil {
		t.Fatalf("BeginTransaction() error = %v", err)
	}
	if err := coordinator.CommitTransaction(ctx, "tx-1"); err != nil {
		t.Fatalf("CommitTransaction() error = %v", err)
	}
	if err := coordinator.RollbackTransaction(ctx, "tx-1"); err != nil {
		t.Fatalf("RollbackTransaction() error = %v", err)
	}

	for _, status := range []TransactionStatus{
		TxStatusActive,
		TxStatusCommitted,
		TxStatusRolledBack,
		TxStatusFailed,
	} {
		got, err := coordinator.GetTransactionStatus(ctx, string(status))
		if err != nil {
			t.Errorf("GetTransactionStatus(%s) error = %v", status, err)
		}
		if got != status {
			t.Errorf("GetTransactionStatus(%s) = %s, want %s", status, got, status)
		}
	}

	if _, err := coordinator.GetTransactionStatus(ctx, "unknown"); err == nil {
		t.Fatal("GetTransactionStatus() with unknown status returned nil error")
	}
}

func TestHTTPCoordinatorNon2xxAndCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	coordinator := NewHTTPCoordinator(server.URL)
	err := coordinator.BeginTransaction(context.Background(), "tx-1")
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("BeginTransaction() error = %v, want HTTP 503 status", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = coordinator.CommitTransaction(ctx, "tx-1")
	if err == nil {
		t.Fatal("CommitTransaction() with canceled context returned nil")
	}
}
