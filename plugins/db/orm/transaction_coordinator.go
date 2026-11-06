package orm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPCoordinator provides HTTP-based distributed transaction coordination.
type HTTPCoordinator struct {
	baseURL string
	client  *http.Client
}

// NewHTTPCoordinator creates a coordinator with a 10-second HTTP timeout.
func NewHTTPCoordinator(baseURL string) *HTTPCoordinator {
	return NewHTTPCoordinatorWithClient(baseURL, nil)
}

// NewHTTPCoordinatorWithClient creates a coordinator using a custom HTTP client.
func NewHTTPCoordinatorWithClient(baseURL string, client *http.Client) *HTTPCoordinator {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPCoordinator{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

type transactionRequest struct {
	TxID string `json:"tx_id"`
}

type transactionResponse struct {
	Status TransactionStatus `json:"status"`
}

// BeginTransaction starts a transaction with the coordinator.
func (hc *HTTPCoordinator) BeginTransaction(ctx context.Context, txID string) error {
	body, err := json.Marshal(transactionRequest{TxID: txID})
	if err != nil {
		return fmt.Errorf("encode transaction begin request: %w", err)
	}
	return hc.do(ctx, http.MethodPost, hc.baseURL+"/transactions", body, nil)
}

// CommitTransaction commits a transaction with the coordinator.
func (hc *HTTPCoordinator) CommitTransaction(ctx context.Context, txID string) error {
	return hc.do(ctx, http.MethodPost, hc.transactionURL(txID, "commit"), nil, nil)
}

// RollbackTransaction rolls back a transaction with the coordinator.
func (hc *HTTPCoordinator) RollbackTransaction(ctx context.Context, txID string) error {
	return hc.do(ctx, http.MethodPost, hc.transactionURL(txID, "rollback"), nil, nil)
}

// GetTransactionStatus returns a transaction's status from the coordinator.
func (hc *HTTPCoordinator) GetTransactionStatus(ctx context.Context, txID string) (TransactionStatus, error) {
	var response transactionResponse
	if err := hc.do(ctx, http.MethodGet, hc.transactionURL(txID, ""), nil, &response); err != nil {
		return "", err
	}
	switch response.Status {
	case TxStatusActive, TxStatusCommitted, TxStatusRolledBack, TxStatusFailed:
		return response.Status, nil
	default:
		return "", fmt.Errorf("unknown transaction status %q", response.Status)
	}
}

func (hc *HTTPCoordinator) transactionURL(txID, action string) string {
	path := "/transactions/" + url.PathEscape(txID)
	if action != "" {
		path += "/" + action
	}
	return hc.baseURL + path
}

func (hc *HTTPCoordinator) do(ctx context.Context, method, endpoint string, body []byte, result interface{}) error {
	var requestBody io.Reader
	if body != nil {
		requestBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return fmt.Errorf("create transaction coordinator request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		return fmt.Errorf("transaction coordinator request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("transaction coordinator returned status %s", resp.Status)
	}
	if result != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result); err != nil {
			return fmt.Errorf("decode transaction coordinator response: %w", err)
		}
	}
	return nil
}
