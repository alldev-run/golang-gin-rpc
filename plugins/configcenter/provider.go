package configcenter

import (
	"context"
	"errors"
)

// Provider defines the backend implementation used by ConfigCenter.
type Provider interface {
	// Get fetches the value for the given namespace and key.
	Get(ctx context.Context, namespace, key string) ([]byte, int64, error)
	// Set stores the value and returns the new version.
	Set(ctx context.Context, namespace, key string, value []byte, metadata map[string]string) (int64, error)
	// Delete removes the key from the namespace.
	Delete(ctx context.Context, namespace, key string) error
	// Watch streams change events for the namespace until the context is canceled.
	Watch(ctx context.Context, namespace string) (<-chan ConfigChange, error)
	// Close releases provider resources.
	Close() error
}

var (
	// ErrNotFound indicates the requested key does not exist.
	ErrNotFound = errors.New("configcenter: key not found")
)
