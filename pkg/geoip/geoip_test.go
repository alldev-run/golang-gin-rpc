package geoip

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"127.0.0.1", true},
		{"169.254.0.1", true},
		{"::1", true},
		{"fc00::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := IsPrivateIP(tt.ip)
			assert.Equal(t, tt.private, result)
		})
	}
}

func TestNewManager_NoDB(t *testing.T) {
	_, err := NewManager("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "geoip database path is required")
}

func TestNewManager_InvalidPath(t *testing.T) {
	_, err := NewManager("/nonexistent/path/to/db.mmdb")
	assert.Error(t, err)
}

func TestGeoIPManager_GetCountry_NotInitialized(t *testing.T) {
	manager := &GeoIPManager{}
	_, err := manager.GetCountry("8.8.8.8")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestGeoIPManager_GetCountry_InvalidIP(t *testing.T) {
	// This will fail if no valid database is present
	// In CI environment, skip if no DB available
	t.Skip("Skipping test - requires GeoIP2 database")
}

func TestDefaultGetCountry_NotInitialized(t *testing.T) {
	// Reset default manager
	defaultManager = nil
	defaultOnce = sync.Once{}

	_, err := DefaultGetCountry("8.8.8.8")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}
