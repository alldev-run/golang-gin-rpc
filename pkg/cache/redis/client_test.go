package redis

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("DefaultConfig() Host = %v, want localhost", cfg.Host)
	}
	if cfg.Port != 6379 {
		t.Errorf("DefaultConfig() Port = %v, want 6379", cfg.Port)
	}
	if cfg.Database != 0 {
		t.Errorf("DefaultConfig() Database = %v, want 0", cfg.Database)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("DefaultConfig() PoolSize = %v, want 10", cfg.PoolSize)
	}
	if cfg.MinIdleConns != 2 {
		t.Errorf("DefaultConfig() MinIdleConns = %v, want 2", cfg.MinIdleConns)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("DefaultConfig() MaxRetries = %v, want 3", cfg.MaxRetries)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("DefaultConfig() DialTimeout = %v, want 5s", cfg.DialTimeout)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Host:         "127.0.0.1",
		Port:         6380,
		Password:     "secret",
		Database:     1,
		PoolSize:     20,
		MinIdleConns: 5,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolTimeout:  10 * time.Second,
	}

	if cfg.Host != "127.0.0.1" {
		t.Error("Config struct assignment failed for Host")
	}
	if cfg.Port != 6380 {
		t.Error("Config struct assignment failed for Port")
	}
	if cfg.Password != "secret" {
		t.Error("Config struct assignment failed for Password")
	}
	if cfg.Database != 1 {
		t.Error("Config struct assignment failed for Database")
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Host:     "invalid.host.that.does.not.exist",
		Port:     6379,
		Database: 0,
		PoolSize: 1,
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
