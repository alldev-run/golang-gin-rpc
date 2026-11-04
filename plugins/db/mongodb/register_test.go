package mongodb

import (
	"testing"
)

// ==================== applyConfigDefaults ====================

func TestApplyConfigDefaults(t *testing.T) {
	t.Run("empty_config", func(t *testing.T) {
		cfg := Config{}
		applyConfigDefaults(&cfg)
		if cfg.URI != "mongodb://localhost:27017" {
			t.Errorf("URI = %q, want mongodb://localhost:27017", cfg.URI)
		}
		if cfg.Database != "default" {
			t.Errorf("Database = %q, want default", cfg.Database)
		}
	})
	t.Run("already_set", func(t *testing.T) {
		cfg := Config{URI: "mongodb://custom:27017", Database: "mydb"}
		applyConfigDefaults(&cfg)
		if cfg.URI != "mongodb://custom:27017" {
			t.Error("URI should not be overridden")
		}
		if cfg.Database != "mydb" {
			t.Error("Database should not be overridden")
		}
	})
}

// ==================== decodeConfig ====================

func TestDecodeConfig(t *testing.T) {
	t.Run("direct_assertion", func(t *testing.T) {
		input := Config{URI: "mongodb://h1", Database: "db1"}
		got, ok := decodeConfig[Config](input)
		if !ok {
			t.Fatal("expected ok=true for direct assertion")
		}
		if got.Database != "db1" {
			t.Errorf("Database = %q, want db1", got.Database)
		}
	})
	t.Run("map_assertion", func(t *testing.T) {
		input := map[string]any{
			"uri":      "mongodb://h1",
			"database": "db1",
		}
		got, ok := decodeConfig[Config](input)
		if !ok {
			t.Fatal("expected ok=true for map assertion")
		}
		if got.Database != "db1" {
			t.Errorf("Database = %q, want db1", got.Database)
		}
	})
	t.Run("nil_map", func(t *testing.T) {
		var input map[string]any
		_, ok := decodeConfig[Config](input)
		if ok {
			t.Error("expected ok=false for nil map")
		}
	})
	t.Run("wrong_type", func(t *testing.T) {
		_, ok := decodeConfig[Config]("not a config")
		if ok {
			t.Error("expected ok=false for wrong type")
		}
	})
}

// ==================== dbClient adapter ====================

func TestDbClient_Close(t *testing.T) {
	// We can't easily test Close without a real mongo.Client
	// but we can verify the structure exists
	dbc := &dbClient{client: nil}
	_ = dbc // Just ensure no panic
}
