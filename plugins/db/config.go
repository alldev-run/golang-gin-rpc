// Package db provides configuration loading from YAML/JSON files.
package db

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfigFromYAML loads database configuration from YAML file.
func LoadConfigFromYAML(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Apply defaults based on type
	applyDefaults(&cfg)

	return cfg, nil
}

// LoadConfigFromJSON loads database configuration from JSON file.
func LoadConfigFromJSON(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// Apply defaults based on type
	applyDefaults(&cfg)

	return cfg, nil
}

// LoadConfigsFromYAML loads multiple database configurations from YAML file.
// The file should contain a map of database name to config.
func LoadConfigsFromYAML(path string) (map[string]Config, error) {
	var cfgs map[string]Config

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfgs); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Apply defaults for each config
	for key := range cfgs {
		cfg := cfgs[key]
		applyDefaults(&cfg)
		cfgs[key] = cfg
	}

	return cfgs, nil
}

// LoadConfigsFromJSON loads multiple database configurations from JSON file.
func LoadConfigsFromJSON(path string) (map[string]Config, error) {
	var cfgs map[string]Config

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &cfgs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// Apply defaults for each config
	for key := range cfgs {
		cfg := cfgs[key]
		applyDefaults(&cfg)
		cfgs[key] = cfg
	}

	return cfgs, nil
}

// applyDefaults applies default values based on database type.
func applyDefaults(cfg *Config) {
	switch cfg.Type {
	case TypeMySQL:
		if cfg.MySQL.Host == "" {
			cfg.MySQL.Host = "localhost"
		}
		if cfg.MySQL.Port == 0 {
			cfg.MySQL.Port = 3306
		}
		if cfg.MySQL.Charset == "" {
			cfg.MySQL.Charset = "utf8mb4"
		}
	case TypeRedis:
		if cfg.Redis.Host == "" {
			cfg.Redis.Host = "localhost"
		}
		if cfg.Redis.Port == 0 {
			cfg.Redis.Port = 6379
		}
	case TypePostgres:
		if cfg.PG.Host == "" {
			cfg.PG.Host = "localhost"
		}
		if cfg.PG.Port == 0 {
			cfg.PG.Port = 5432
		}
		if cfg.PG.SSLMode == "" {
			cfg.PG.SSLMode = "disable"
		}
	case TypeClickHouse:
		if len(cfg.CH.Hosts) == 0 {
			cfg.CH.Hosts = []string{"localhost:9000"}
		}
		if cfg.CH.Database == "" {
			cfg.CH.Database = "default"
		}
	case TypeES:
		if len(cfg.ES.Addresses) == 0 {
			cfg.ES.Addresses = []string{"http://localhost:9200"}
		}
	case TypeMemcache:
		if len(cfg.Memcache.Hosts) == 0 {
			cfg.Memcache.Hosts = []string{"localhost:11211"}
		}
	case TypeMongoDB:
		if cfg.MongoDB.URI == "" {
			cfg.MongoDB.URI = "mongodb://localhost:27017"
		}
		if cfg.MongoDB.Database == "" {
			cfg.MongoDB.Database = "default"
		}
	}
}

// SaveConfigToYAML saves configuration to YAML file.
func SaveConfigToYAML(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// SaveConfigToJSON saves configuration to JSON file.
func SaveConfigToJSON(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
