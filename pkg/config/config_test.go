package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// loadFromString 模拟 bootstrap 的加载路径：先 Set(DefaultConfig()) 预填默认值，
// 再 Load 覆盖。
func loadFromString(t *testing.T, content string) *GlobalConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := NewLoader()
	loader.Set(DefaultConfig())
	if err := loader.Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return loader.Get()
}

func TestFlatServerSectionMapsToHTTP(t *testing.T) {
	cfg := loadFromString(t, "app:\n  name: demo\nserver:\n  host: \"localhost\"\n  port: \"18099\"\n")
	if cfg.Server.HTTP.Host != "localhost" {
		t.Fatalf("flat server.host not mapped: %q", cfg.Server.HTTP.Host)
	}
	if cfg.Server.HTTP.Port != 18099 {
		t.Fatalf("flat server.port not mapped: %d", cfg.Server.HTTP.Port)
	}
}

func TestFlatServerSectionAcceptsIntPort(t *testing.T) {
	cfg := loadFromString(t, "server:\n  host: \"127.0.0.1\"\n  port: 18098\n")
	if cfg.Server.HTTP.Port != 18098 {
		t.Fatalf("int server.port not mapped: %d", cfg.Server.HTTP.Port)
	}
}

func TestServerHTTPSectionWins(t *testing.T) {
	cfg := loadFromString(t, "server:\n  host: \"ignored\"\n  port: \"1\"\n  http:\n    host: \"0.0.0.0\"\n    port: 18097\n")
	if cfg.Server.HTTP.Port != 18097 || cfg.Server.HTTP.Host != "0.0.0.0" {
		t.Fatalf("server.http should win over flat section: host=%q port=%d", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port)
	}
}

func TestShutdownTimeoutAndMetricsListenAddr(t *testing.T) {
	cfg := loadFromString(t, "server:\n  http:\n    port: 8080\n  shutdown_timeout: \"5s\"\nobservability:\n  metrics:\n    enabled: true\n    listen_addr: \":19090\"\n")
	if cfg.Server.ShutdownTimeout != 5*time.Second {
		t.Fatalf("shutdown_timeout not parsed: %v", cfg.Server.ShutdownTimeout)
	}
	if cfg.Observability.Metrics.ListenAddr != ":19090" {
		t.Fatalf("metrics listen_addr not parsed: %q", cfg.Observability.Metrics.ListenAddr)
	}
}
