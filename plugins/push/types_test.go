package push

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		want     bool
	}{
		{"FCM is valid", ProviderTypeFCM, true},
		{"APNs is valid", ProviderTypeAPNs, true},
		{"Empty is invalid", "", false},
		{"Unknown is invalid", "unknown", false},
		{"Random is invalid", "random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provider.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProviderType_String(t *testing.T) {
	assert.Equal(t, "fcm", ProviderTypeFCM.String())
	assert.Equal(t, "apns", ProviderTypeAPNs.String())
}

func TestProviderType_DisplayName(t *testing.T) {
	tests := []struct {
		provider ProviderType
		want     string
	}{
		{ProviderTypeFCM, "Firebase Cloud Messaging (FCM)"},
		{ProviderTypeAPNs, "Apple Push Notification Service (APNs)"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			got := tt.provider.DisplayName()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProviderType_DefaultEndpoint(t *testing.T) {
	tests := []struct {
		provider ProviderType
		want     string
	}{
		{ProviderTypeFCM, "https://fcm.googleapis.com/v1"},
		{ProviderTypeAPNs, "https://api.push.apple.com"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			got := tt.provider.DefaultEndpoint()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProviderType_SandboxEndpoint(t *testing.T) {
	tests := []struct {
		provider ProviderType
		want     string
	}{
		{ProviderTypeFCM, "https://fcm.googleapis.com/v1"},
		{ProviderTypeAPNs, "https://api.sandbox.push.apple.com"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			got := tt.provider.SandboxEndpoint()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProviderType_RequiresAuthentication(t *testing.T) {
	assert.True(t, ProviderTypeFCM.RequiresAuthentication())
	assert.True(t, ProviderTypeAPNs.RequiresAuthentication())
}

func TestProviderType_SupportsTopic(t *testing.T) {
	assert.True(t, ProviderTypeFCM.SupportsTopic())
	assert.True(t, ProviderTypeAPNs.SupportsTopic())
}

func TestProviderType_SupportsMulticast(t *testing.T) {
	assert.True(t, ProviderTypeFCM.SupportsMulticast())
	assert.True(t, ProviderTypeAPNs.SupportsMulticast())
}

func TestProviderType_MaxBatchSize(t *testing.T) {
	assert.Equal(t, 500, ProviderTypeFCM.MaxBatchSize())
	assert.Equal(t, 100, ProviderTypeAPNs.MaxBatchSize())
	assert.Equal(t, 0, ProviderType("").MaxBatchSize())
}

func TestGetSupportedProviders(t *testing.T) {
	providers := GetSupportedProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, ProviderTypeFCM)
	assert.Contains(t, providers, ProviderTypeAPNs)
}

func TestParseProviderType(t *testing.T) {
	tests := []struct {
		input   string
		want    ProviderType
		wantErr bool
	}{
		{"fcm", ProviderTypeFCM, false},
		{"firebase", ProviderTypeFCM, false},
		{"gcm", ProviderTypeFCM, false},
		{"apns", ProviderTypeAPNs, false},
		{"apple", ProviderTypeAPNs, false},
		{"ios", ProviderTypeAPNs, false},
		{"unknown", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseProviderType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPlatform_Constants(t *testing.T) {
	assert.Equal(t, Platform("android"), PlatformAndroid)
	assert.Equal(t, Platform("ios"), PlatformIOS)
}

func TestPriority_Constants(t *testing.T) {
	assert.Equal(t, Priority("high"), PriorityHigh)
	assert.Equal(t, Priority("normal"), PriorityNormal)
}
