package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Config{
		Type: TypeMySQL,
		MySQL: mysql.Config{
			Host: "localhost",
			Port: 3306,
		},
	}

	if cfg.Type != TypeMySQL {
		t.Errorf("Config Type = %v, want mysql", cfg.Type)
	}
	if cfg.MySQL.Host != "localhost" {
		t.Errorf("MySQL Host = %v, want localhost", cfg.MySQL.Host)
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("MySQL Port = %v, want 3306", cfg.MySQL.Port)
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	yamlContent := `
type: mysql
mysql:
  host: "127.0.0.1"
  port: 3307
  database: "testdb"
  username: "testuser"
  password: "testpass"
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	cfg, err := LoadConfigFromYAML(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfigFromYAML() error = %v", err)
	}

	if cfg.Type != TypeMySQL {
		t.Errorf("Config Type = %v, want mysql", cfg.Type)
	}
	if cfg.MySQL.Host != "127.0.0.1" {
		t.Errorf("MySQL Host = %v, want 127.0.0.1", cfg.MySQL.Host)
	}
	if cfg.MySQL.Port != 3307 {
		t.Errorf("MySQL Port = %v, want 3307", cfg.MySQL.Port)
	}
}

func TestLoadConfigFromJSON(t *testing.T) {
	jsonContent := `{
  "type": "redis",
  "redis": {
    "host": "127.0.0.1",
    "port": 6380,
    "database": 1
  }
}`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")

	if err := os.WriteFile(tmpFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	cfg, err := LoadConfigFromJSON(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfigFromJSON() error = %v", err)
	}

	if cfg.Type != TypeRedis {
		t.Errorf("Config Type = %v, want redis", cfg.Type)
	}
	if cfg.Redis.Host != "127.0.0.1" {
		t.Errorf("Redis Host = %v, want 127.0.0.1", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Errorf("Redis Port = %v, want 6380", cfg.Redis.Port)
	}
}

func TestLoadConfigsFromYAML(t *testing.T) {
	yamlContent := `
primary:
  type: mysql
  mysql:
    host: "localhost"
    port: 3306
    database: "mydb"

secondary:
  type: redis
  redis:
    host: "localhost"
    port: 6379
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "configs.yaml")

	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	cfgs, err := LoadConfigsFromYAML(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfigsFromYAML() error = %v", err)
	}

	if len(cfgs) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(cfgs))
	}

	if cfg, ok := cfgs["primary"]; !ok || cfg.Type != TypeMySQL {
		t.Error("Primary config should be mysql")
	}

	if cfg, ok := cfgs["secondary"]; !ok || cfg.Type != TypeRedis {
		t.Error("Secondary config should be redis")
	}
}

func TestApplyDefaults(t *testing.T) {
	// Test MySQL defaults
	cfg := Config{Type: TypeMySQL}
	applyDefaults(&cfg)
	if cfg.MySQL.Host != "localhost" {
		t.Errorf("MySQL default Host = %v, want localhost", cfg.MySQL.Host)
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("MySQL default Port = %v, want 3306", cfg.MySQL.Port)
	}

	// Test Redis defaults
	cfg = Config{Type: TypeRedis}
	applyDefaults(&cfg)
	if cfg.Redis.Host != "localhost" {
		t.Errorf("Redis default Host = %v, want localhost", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("Redis default Port = %v, want 6379", cfg.Redis.Port)
	}

	// Test PostgreSQL defaults
	cfg = Config{Type: TypePostgres}
	applyDefaults(&cfg)
	if cfg.PG.Host != "localhost" {
		t.Errorf("PG default Host = %v, want localhost", cfg.PG.Host)
	}
	if cfg.PG.Port != 5432 {
		t.Errorf("PG default Port = %v, want 5432", cfg.PG.Port)
	}

	// Test ClickHouse defaults
	cfg = Config{Type: TypeClickHouse}
	applyDefaults(&cfg)
	if len(cfg.CH.Hosts) != 1 || cfg.CH.Hosts[0] != "localhost:9000" {
		t.Errorf("CH default Hosts = %v, want [localhost:9000]", cfg.CH.Hosts)
	}

	// Test ES defaults
	cfg = Config{Type: TypeES}
	applyDefaults(&cfg)
	if len(cfg.ES.Addresses) != 1 || cfg.ES.Addresses[0] != "http://localhost:9200" {
		t.Errorf("ES default Addresses = %v, want [http://localhost:9200]", cfg.ES.Addresses)
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	if factory == nil {
		t.Error("NewFactory() returned nil")
	}
	if factory.clients == nil {
		t.Error("Factory clients map is nil")
	}
}

func TestFactoryCreateInvalidType(t *testing.T) {
	factory := NewFactory()
	cfg := Config{Type: "invalid"}

	_, err := factory.Create(cfg)
	if err == nil {
		t.Error("Create() with invalid type should return error")
	}
}

func TestFactoryCreateEmptyType(t *testing.T) {
	factory := NewFactory()
	cfg := Config{Type: ""}

	_, err := factory.Create(cfg)
	if err == nil {
		t.Error("Create() with empty type should return error")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Test YAML
	yamlFile := filepath.Join(tmpDir, "save_test.yaml")
	cfg := Config{
		Type: TypeMySQL,
		MySQL: mysql.Config{
			Host: "testhost",
			Port: 3307,
		},
	}

	if err := SaveConfigToYAML(yamlFile, cfg); err != nil {
		t.Errorf("SaveConfigToYAML() error = %v", err)
	}

	loaded, err := LoadConfigFromYAML(yamlFile)
	if err != nil {
		t.Errorf("LoadConfigFromYAML() error = %v", err)
	}
	if loaded.Type != cfg.Type {
		t.Error("Loaded config type mismatch")
	}

	// Test JSON
	jsonFile := filepath.Join(tmpDir, "save_test.json")
	cfg2 := Config{
		Type: TypeRedis,
		Redis: redis.Config{
			Host: "redishost",
			Port: 6380,
		},
	}

	if err := SaveConfigToJSON(jsonFile, cfg2); err != nil {
		t.Errorf("SaveConfigToJSON() error = %v", err)
	}

	loaded2, err := LoadConfigFromJSON(jsonFile)
	if err != nil {
		t.Errorf("LoadConfigFromJSON() error = %v", err)
	}
	if loaded2.Type != cfg2.Type {
		t.Error("Loaded config type mismatch")
	}
}
