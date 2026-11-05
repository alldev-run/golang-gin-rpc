package pluginconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
)

func TestLoadOverlaysSectionsOnDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "app:\n  name: demo\nredis:\n  host: redis.internal\ndiscovery:\n  enabled: true\n  type: etcd\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := config.NewLoader()
	loader.Set(config.DefaultConfig())
	if err := loader.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	cfg, err := Load(loader)
	if err != nil {
		t.Fatalf("pluginconfig.Load() error = %v", err)
	}
	if cfg.Redis.Host != "redis.internal" || cfg.Redis.Port != Default().Redis.Port {
		t.Fatalf("redis section not overlaid on defaults: %+v", cfg.Redis)
	}
	if !cfg.Discovery.Enabled || cfg.Discovery.Type != "etcd" {
		t.Fatalf("discovery section not loaded: %+v", cfg.Discovery)
	}
	if cfg.RPC.Protocol != Default().RPC.Protocol {
		t.Fatalf("missing rpc section should keep defaults, got %+v", cfg.RPC)
	}
}

func TestLoadNilSourceReturnsDefaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil || cfg.Redis.Port != Default().Redis.Port {
		t.Fatalf("Load(nil) = %+v, %v", cfg, err)
	}
}
