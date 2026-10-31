package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseComponents(t *testing.T) {
	got, err := parseComponents("api-gateway, db , cache,db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"api-gateway", "db", "cache"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}

	if _, err := parseComponents("api-gateway,nope"); err == nil {
		t.Fatal("expected error for unknown component")
	}
	if _, err := parseComponents("  "); err == nil {
		t.Fatal("expected error for empty selection")
	}
}

func TestParseSelectionInput(t *testing.T) {
	got, err := parseSelectionInput("1, 5,db\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"api-gateway", "cache", "db"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}

	defaults, err := parseSelectionInput("\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(defaults, ",") != strings.Join(initDefaults, ",") {
		t.Fatalf("empty input should yield defaults, got %v", defaults)
	}

	if _, err := parseSelectionInput("99"); err == nil {
		t.Fatal("expected error for out-of-range number")
	}
}

func TestBuildServicesList(t *testing.T) {
	if got := buildServicesList([]string{"api-gateway", "db"}); got != `"api-gateway"` {
		t.Fatalf("got %q", got)
	}
	if got := buildServicesList([]string{"websocket", "rpc", "db"}); got != `"rpc", "websocket"` {
		t.Fatalf("got %q (catalog order expected)", got)
	}
	if got := buildServicesList([]string{"db"}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestBuildExtraImportsAndWiring(t *testing.T) {
	if got := buildExtraImports([]string{"db"}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
	if got := buildExtraImports([]string{"websocket"}); !strings.Contains(got, "plugins/websocket") {
		t.Fatalf("got %q", got)
	}

	wiring := buildServiceWiring([]string{"websocket"}, 8080)
	if !strings.Contains(wiring, `wsCfg.Addr = ":8081"`) {
		t.Fatalf("wiring should set Addr to port+1, got %q", wiring)
	}
	if !strings.Contains(wiring, "bootstrap.WebSocketServiceOptions") {
		t.Fatalf("wiring missing options type, got %q", wiring)
	}
	if got := buildServiceWiring([]string{"db"}, 8080); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestBuildConfigYAML(t *testing.T) {
	sel := []string{"api-gateway", "db", "metrics", "tracing"}
	data := buildConfigYAML(sel, "demo", 8090)

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("generated config is not valid yaml: %v\n%s", err, data)
	}

	// plugins toggles reflect selection
	plugins := cfg["plugins"].(map[string]interface{})
	if plugins["db"].(map[string]interface{})["enabled"] != true {
		t.Fatalf("db should be enabled: %v", plugins["db"])
	}
	if plugins["cache"].(map[string]interface{})["enabled"] != false {
		t.Fatalf("cache should be disabled: %v", plugins["cache"])
	}

	// framework services
	fw := cfg["framework"].(map[string]interface{})
	svcs, _ := fw["services"].([]interface{})
	if len(svcs) != 1 || svcs[0] != "api-gateway" {
		t.Fatalf("services = %v", svcs)
	}

	// db section present, cache section absent
	if _, ok := cfg["database"]; !ok {
		t.Fatal("database section missing")
	}
	if _, ok := cfg["redis"]; ok {
		t.Fatal("redis section should be absent when cache not selected")
	}

	// observability has both metrics and tracing
	obs := cfg["observability"].(map[string]interface{})
	if _, ok := obs["metrics"]; !ok {
		t.Fatal("metrics section missing")
	}
	if _, ok := obs["tracing"]; !ok {
		t.Fatal("tracing section missing")
	}

	// server port
	port := cfg["server"].(map[string]interface{})["http"].(map[string]interface{})["port"]
	if port != 8090 {
		t.Fatalf("port = %v, want 8090", port)
	}

	// minimal selection: no db, no observability sections
	minimal := buildConfigYAML([]string{"api-gateway", "health", "errors"}, "mini", 8080)
	var m map[string]interface{}
	if err := yaml.Unmarshal(minimal, &m); err != nil {
		t.Fatalf("minimal config invalid yaml: %v", err)
	}
	if _, ok := m["database"]; ok {
		t.Fatal("database section should be absent")
	}
	if _, ok := m["observability"]; ok {
		t.Fatal("observability section should be absent")
	}
}
