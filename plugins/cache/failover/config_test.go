package failover

import (
	"strings"
	"testing"
)

func TestValidateRequiresFallbackBackendConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		order       []string
		wantMessage string
	}{
		{name: "redis", order: []string{"redis"}, wantMessage: "redis configuration is required"},
		{name: "memcache", order: []string{"memcache"}, wantMessage: "memcache configuration is required"},
		{name: "unknown", order: []string{"unknown"}, wantMessage: "unsupported fallback storage type"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := Config{FallbackOrder: test.order}
			if err := config.Validate(); err == nil || !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("Validate() error = %v, want it to contain %q", err, test.wantMessage)
			}
		})
	}
}

func TestValidateAcceptsConfiguredRemoteFallbacks(t *testing.T) {
	config := Config{
		FallbackOrder: []string{"redis", "memcache"},
		Redis:         &RedisStorageConfig{Host: "localhost", Port: 6379},
		Memcache:      &MemcacheStorageConfig{Hosts: []string{"localhost:11211"}},
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
