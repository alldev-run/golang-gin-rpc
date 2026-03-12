package mongodb

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.URI != "mongodb://localhost:27017" {
		t.Errorf("URI = %v, want mongodb://localhost:27017", cfg.URI)
	}
	if cfg.Database != "default" {
		t.Errorf("Database = %v, want default", cfg.Database)
	}
	if cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("ConnectTimeout = %v, want 10s", cfg.ConnectTimeout)
	}
	if cfg.MaxPoolSize != 100 {
		t.Errorf("MaxPoolSize = %d, want 100", cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize != 10 {
		t.Errorf("MinPoolSize = %d, want 10", cfg.MinPoolSize)
	}
	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want 30m", cfg.MaxConnIdleTime)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		URI:             "mongodb://localhost:27018",
		Database:        "testdb",
		ConnectTimeout:  15 * time.Second,
		MaxPoolSize:     50,
		MinPoolSize:     5,
		MaxConnIdleTime: 60 * time.Minute,
	}

	if cfg.URI != "mongodb://localhost:27018" {
		t.Error("Config struct assignment failed for URI")
	}
	if cfg.Database != "testdb" {
		t.Error("Config struct assignment failed for Database")
	}
	if cfg.MaxPoolSize != 50 {
		t.Error("Config struct assignment failed for MaxPoolSize")
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		URI:            "mongodb://invalid.host.that.does.not.exist:27017",
		ConnectTimeout: 2 * time.Second,
	}

	client, err := New(cfg)
	if err == nil {
		if client != nil {
			_ = client.Close(nil)
		}
		t.Log("Connection succeeded unexpectedly")
	} else {
		t.Logf("Expected connection error: %v", err)
	}
}

// TestNewWithEmptyURI uses default URI
func TestNewWithEmptyURI(t *testing.T) {
	cfg := Config{
		URI:            "",
		ConnectTimeout: 2 * time.Second,
	}

	client, err := New(cfg)
	if err == nil {
		// Might connect to localhost if MongoDB is running
		if client != nil {
			_ = client.Close(nil)
		}
		t.Log("Connected to default URI")
	} else {
		t.Logf("Expected connection error (no MongoDB running): %v", err)
	}
}
