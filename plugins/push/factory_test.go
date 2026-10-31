package push

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.clients)
	assert.Empty(t, factory.ClientNames())
}

func TestFactory_RegisterAndGetClient(t *testing.T) {
	factory := NewFactory()

	// Create a mock client using a disabled config (no real credentials needed)
	mockClient := &mockPushClient{
		provider: ProviderTypeFCM,
	}

	factory.RegisterClient("test-client", mockClient)

	retrievedClient, exists := factory.GetClient("test-client")
	assert.True(t, exists)
	assert.NotNil(t, retrievedClient)
	assert.Equal(t, ProviderTypeFCM, retrievedClient.Provider())

	// Check names
	names := factory.ClientNames()
	assert.Len(t, names, 1)
	assert.Contains(t, names, "test-client")
}

func TestFactory_GetClient_NotFound(t *testing.T) {
	factory := NewFactory()

	client, exists := factory.GetClient("non-existent")
	assert.False(t, exists)
	assert.Nil(t, client)
}

func TestFactory_RemoveClient(t *testing.T) {
	factory := NewFactory()

	mockClient := &mockPushClient{provider: ProviderTypeFCM}
	factory.RegisterClient("client-1", mockClient)
	factory.RegisterClient("client-2", &mockPushClient{provider: ProviderTypeAPNs})

	assert.Len(t, factory.ClientNames(), 2)

	removed := factory.RemoveClient("client-1")
	assert.True(t, removed)
	assert.Len(t, factory.ClientNames(), 1)

	// Try to remove again
	removedAgain := factory.RemoveClient("client-1")
	assert.False(t, removedAgain)
}

func TestFactory_Close(t *testing.T) {
	factory := NewFactory()

	mockClient1 := &mockPushClient{provider: ProviderTypeFCM}
	mockClient2 := &mockPushClient{provider: ProviderTypeAPNs}

	factory.RegisterClient("client-1", mockClient1)
	factory.RegisterClient("client-2", mockClient2)

	err := factory.Close()
	assert.NoError(t, err)
	assert.Empty(t, factory.ClientNames())
}

func TestFactory_HealthCheck(t *testing.T) {
	factory := NewFactory()

	healthyClient := &mockPushClient{
		provider: ProviderTypeFCM,
		healthy:  true,
	}
	unhealthyClient := &mockPushClient{
		provider:  ProviderTypeAPNs,
		healthy:   false,
		healthErr: assert.AnError,
	}

	factory.RegisterClient("healthy", healthyClient)
	factory.RegisterClient("unhealthy", unhealthyClient)

	results := factory.HealthCheck(context.Background())

	assert.Len(t, results, 2)
	assert.NoError(t, results["healthy"])
	assert.Error(t, results["unhealthy"])
}

func TestNewClient_Disabled(t *testing.T) {
	config := Config{
		Enabled:  false,
		Provider: ProviderTypeFCM,
	}

	client, err := NewClient(config)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "disabled")
}

func TestNewClient_InvalidConfig(t *testing.T) {
	config := Config{
		Enabled:  true,
		Provider: ProviderTypeFCM,
		// Missing required FCM config
	}

	client, err := NewClient(config)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "invalid push notification config")
}

func TestNewClient_UnsupportedProvider(t *testing.T) {
	config := Config{
		Enabled:  true,
		Provider: ProviderType("unsupported"),
	}

	client, err := NewClient(config)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "invalid provider type")
}

func TestCreateClient(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		wantErr   bool
		errContains string
	}{
		{
			name:      "valid FCM",
			provider:  "fcm",
			wantErr:   true, // Will error due to missing credentials, but provider parsing works
			errContains: "invalid push notification config",
		},
		{
			name:      "valid APNs",
			provider:  "apns",
			wantErr:   true,
			errContains: "invalid push notification config",
		},
		{
			name:      "invalid provider",
			provider:  "invalid",
			wantErr:   true,
			errContains: "unsupported push provider type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			client, err := CreateClient(tt.provider, config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestCreateFCMClient(t *testing.T) {
	// This will fail because service account key is not valid JSON
	client, err := CreateFCMClient("test-project", "invalid-json")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestCreateAPNsClient_InvalidKey(t *testing.T) {
	// This will fail because auth key is not valid
	client, err := CreateAPNsClient("TEAM123", "KEY123", "com.test.app", "invalid-key", false)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestCreateLegacyFCMClient(t *testing.T) {
	client, err := CreateLegacyFCMClient("server-key-123")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, ProviderTypeFCM, client.Provider())
}

func TestGetSupportedPushProviders(t *testing.T) {
	providers := GetSupportedPushProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, ProviderTypeFCM)
	assert.Contains(t, providers, ProviderTypeAPNs)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid disabled config",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			config: Config{
				Enabled:  true,
				Provider: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsProviderSupported(t *testing.T) {
	tests := []struct {
		provider string
		want     bool
	}{
		{"fcm", true},
		{"firebase", true},
		{"apns", true},
		{"apple", true},
		{"ios", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := IsProviderSupported(tt.provider)
			assert.Equal(t, tt.want, got)
		})
	}
}

// mockPushClient is a mock implementation of the Client interface for testing
type mockPushClient struct {
	provider  ProviderType
	healthy   bool
	healthErr error
	closed    bool
}

func (m *mockPushClient) Send(ctx context.Context, notification *Notification) (*Response, error) {
	return &Response{
		Success:  true,
		Provider: m.provider,
	}, nil
}

func (m *mockPushClient) SendMulticast(ctx context.Context, notification *Notification, tokens []string) (*BatchResponse, error) {
	return &BatchResponse{
		SuccessCount: len(tokens),
		FailureCount: 0,
		Provider:     m.provider,
	}, nil
}

func (m *mockPushClient) SendToTopic(ctx context.Context, notification *Notification, topic string) (*Response, error) {
	return &Response{
		Success:  true,
		Provider: m.provider,
	}, nil
}

func (m *mockPushClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	return nil
}

func (m *mockPushClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	return nil
}

func (m *mockPushClient) Close() error {
	m.closed = true
	return nil
}

func (m *mockPushClient) Provider() ProviderType {
	return m.provider
}

func (m *mockPushClient) IsHealthy(ctx context.Context) error {
	if m.healthy {
		return nil
	}
	return m.healthErr
}
