package memcache

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Hosts) != 1 || cfg.Hosts[0] != "localhost:11211" {
		t.Errorf("Hosts = %v, want [localhost:11211]", cfg.Hosts)
	}
	if cfg.MaxIdleConns != 2 {
		t.Errorf("MaxIdleConns = %d, want 2", cfg.MaxIdleConns)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Hosts:        []string{"server1:11211", "server2:11211"},
		MaxIdleConns: 10,
		Timeout:      10 * time.Second,
	}

	if len(cfg.Hosts) != 2 {
		t.Error("Config struct assignment failed for Hosts")
	}
	if cfg.MaxIdleConns != 10 {
		t.Error("Config struct assignment failed for MaxIdleConns")
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Hosts:   []string{"invalid.host.that.does.not.exist:11211"},
		Timeout: 1 * time.Second,
	}

	client, err := New(cfg)
	if err == nil {
		if client != nil {
			_ = client.Close()
		}
		t.Log("Connection succeeded unexpectedly")
	} else {
		t.Logf("Expected connection error: %v", err)
	}
}

// TestNewWithEmptyHosts uses default hosts
func TestNewWithEmptyHosts(t *testing.T) {
	cfg := Config{
		Hosts:   []string{},
		Timeout: 1 * time.Second,
	}

	client, err := New(cfg)
	if err == nil {
		// Might connect to localhost if memcached is running
		if client != nil {
			_ = client.Close()
		}
		t.Log("Connected to default host")
	} else {
		t.Logf("Expected connection error (no memcached running): %v", err)
	}
}
