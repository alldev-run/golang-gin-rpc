package configcenter

import "time"

// ChangeType represents the type of configuration change event.
type ChangeType string

const (
	// ChangeTypeSet indicates a key was created or updated.
	ChangeTypeSet ChangeType = "set"
	// ChangeTypeDelete indicates a key was deleted.
	ChangeTypeDelete ChangeType = "delete"
)

// ConfigChange represents a change event received from the backend provider.
type ConfigChange struct {
	Namespace string
	Key       string
	Value     []byte
	Version   int64
	Change    ChangeType
	Metadata  map[string]string
	Timestamp time.Time
}

// Subscription represents an active change listener subscription.
type Subscription interface {
	// Close stops the subscription and removes its handler from the event bus.
	Close()
}
