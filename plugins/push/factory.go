package push

import (
	"context"
	"fmt"
	"sync"
)

// Factory creates and manages push notification clients
type Factory struct {
	clients map[string]Client
	mu      sync.RWMutex
}

// NewFactory creates a new push notification factory
func NewFactory() *Factory {
	return &Factory{
		clients: make(map[string]Client),
	}
}

// NewClient creates a new push notification client based on configuration
func NewClient(config Config) (Client, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("push notifications are disabled")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid push notification config: %w", err)
	}

	switch config.Provider {
	case ProviderTypeFCM:
		return NewFCMClient(config)
	case ProviderTypeAPNs:
		return NewAPNsClient(config)
	default:
		return nil, fmt.Errorf("unsupported push provider: %s", config.Provider)
	}
}

// CreateClient creates a client by provider type string
func CreateClient(providerType string, config Config) (Client, error) {
	provider, err := ParseProviderType(providerType)
	if err != nil {
		return nil, err
	}

	config.Provider = provider
	return NewClient(config)
}

// CreateFCMClient creates a FCM client with the provided service account
func CreateFCMClient(projectID, serviceAccountKey string) (*FCMClient, error) {
	config := DefaultConfig()
	config.Provider = ProviderTypeFCM
	config.FCM.ProjectID = projectID
	config.FCM.ServiceAccountKey = serviceAccountKey

	return NewFCMClient(config)
}

// CreateAPNsClient creates an APNs client with token-based authentication
func CreateAPNsClient(teamID, keyID, bundleID, authKey string, useSandbox bool) (*APNsClient, error) {
	config := DefaultConfig()
	config.Provider = ProviderTypeAPNs
	config.APNs.TeamID = teamID
	config.APNs.KeyID = keyID
	config.APNs.BundleID = bundleID
	config.APNs.AuthKey = authKey
	config.APNs.UseTokenAuth = true
	config.APNs.UseSandbox = useSandbox

	return NewAPNsClient(config)
}

// CreateLegacyFCMClient creates a FCM client using the legacy API
func CreateLegacyFCMClient(serverKey string) (*FCMClient, error) {
	config := DefaultConfig()
	config.Provider = ProviderTypeFCM
	config.FCM.UseLegacyAPI = true
	config.FCM.LegacyServerKey = serverKey

	return NewFCMClient(config)
}

// RegisterClient registers a client with a name for reuse
func (f *Factory) RegisterClient(name string, client Client) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients[name] = client
}

// GetClient retrieves a registered client by name
func (f *Factory) GetClient(name string) (Client, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	client, exists := f.clients[name]
	return client, exists
}

// RemoveClient removes a registered client
func (f *Factory) RemoveClient(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if client, exists := f.clients[name]; exists {
		client.Close()
		delete(f.clients, name)
		return true
	}
	return false
}

// Close closes all registered clients
func (f *Factory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var lastErr error
	for name, client := range f.clients {
		if err := client.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close client %s: %w", name, err)
		}
	}
	f.clients = make(map[string]Client)
	return lastErr
}

// HealthCheck checks the health of all registered clients
func (f *Factory) HealthCheck(ctx context.Context) map[string]error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	results := make(map[string]error)
	for name, client := range f.clients {
		results[name] = client.IsHealthy(ctx)
	}
	return results
}

// ClientNames returns all registered client names
func (f *Factory) ClientNames() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.clients))
	for name := range f.clients {
		names = append(names, name)
	}
	return names
}

// GetSupportedProviders returns all supported provider types
func GetSupportedPushProviders() []ProviderType {
	return GetSupportedProviders()
}

// ValidateConfig validates the push notification configuration
func ValidateConfig(config Config) error {
	return config.Validate()
}

// IsProviderSupported checks if a provider type is supported
func IsProviderSupported(provider string) bool {
	providerType, err := ParseProviderType(provider)
	if err != nil {
		return false
	}
	return providerType.IsValid()
}
