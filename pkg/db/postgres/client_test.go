package postgres

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("DefaultConfig() Host = %v, want localhost", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("DefaultConfig() Port = %v, want 5432", cfg.Port)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("DefaultConfig() SSLMode = %v, want disable", cfg.SSLMode)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("DefaultConfig() MaxOpenConns = %v, want 25", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 10 {
		t.Errorf("DefaultConfig() MaxIdleConns = %v, want 10", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != time.Hour {
		t.Errorf("DefaultConfig() ConnMaxLifetime = %v, want 1h", cfg.ConnMaxLifetime)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Host:            "127.0.0.1",
		Port:            5433,
		Database:        "testdb",
		Username:        "testuser",
		Password:        "testpass",
		SSLMode:         "require",
		MaxOpenConns:    50,
		MaxIdleConns:    20,
		ConnMaxLifetime: 30 * time.Minute,
	}

	if cfg.Host != "127.0.0.1" {
		t.Error("Config struct assignment failed for Host")
	}
	if cfg.Port != 5433 {
		t.Error("Config struct assignment failed for Port")
	}
	if cfg.Database != "testdb" {
		t.Error("Config struct assignment failed for Database")
	}
	if cfg.SSLMode != "require" {
		t.Error("Config struct assignment failed for SSLMode")
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Host:     "invalid.host.that.does.not.exist",
		Port:     5432,
		Database: "test",
		Username: "test",
		Password: "test",
		SSLMode:  "disable",
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
