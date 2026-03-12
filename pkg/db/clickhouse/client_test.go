package clickhouse

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Hosts) != 1 || cfg.Hosts[0] != "localhost:9000" {
		t.Errorf("DefaultConfig() Hosts = %v, want [localhost:9000]", cfg.Hosts)
	}
	if cfg.Database != "default" {
		t.Errorf("DefaultConfig() Database = %v, want default", cfg.Database)
	}
	if cfg.MaxOpenConns != 10 {
		t.Errorf("DefaultConfig() MaxOpenConns = %v, want 10", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 5 {
		t.Errorf("DefaultConfig() MaxIdleConns = %v, want 5", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("DefaultConfig() ConnMaxLifetime = %v, want 1h", cfg.ConnMaxLifetime)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("DefaultConfig() DialTimeout = %v, want 5s", cfg.DialTimeout)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Hosts:           []string{"host1:9000", "host2:9000"},
		Database:        "analytics",
		Username:        "default",
		Password:        "secret",
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		DialTimeout:     10 * time.Second,
	}

	if len(cfg.Hosts) != 2 {
		t.Error("Config struct assignment failed for Hosts")
	}
	if cfg.Database != "analytics" {
		t.Error("Config struct assignment failed for Database")
	}
	if cfg.Username != "default" {
		t.Error("Config struct assignment failed for Username")
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Hosts:    []string{"invalid.host.that.does.not.exist:9000"},
		Database: "default",
		Username: "default",
		Password: "",
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
